package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"reservas-go/internal/dominio"
	"reservas-go/internal/dominio/entidades"
)

// RepositorioDiasBloqueados implementa puertos.RepositorioDiasBloqueados
// contra PostgreSQL.
type RepositorioDiasBloqueados struct {
	pool *pgxpool.Pool
}

// NuevoRepositorioDiasBloqueados crea un RepositorioDiasBloqueados.
func NuevoRepositorioDiasBloqueados(pool *pgxpool.Pool) *RepositorioDiasBloqueados {
	return &RepositorioDiasBloqueados{pool: pool}
}

// Guardar inserta un DiaBloqueado nuevo, pero antes verifica que no haya
// reservas confirmadas en el rango que se quiere bloquear (regla de
// negocio de Fase 0: no se puede bloquear un rango que ya tiene reservas).
//
// Este es el único lugar del proyecto donde usamos una transacción
// explícita con SELECT ... FOR UPDATE (bloqueo pesimista). A diferencia
// de la doble reserva (Fase 3, tabla reservas), acá NO existe un
// constraint declarativo posible para "no bloquear si hay reservas": es
// una regla que cruza dos tablas distintas (dias_bloqueados y reservas),
// así que la garantizamos a mano, con una transacción que:
//
//  1. Bloquea (FOR UPDATE) las filas de reservas que caen en el rango.
//  2. Si encuentra alguna, aborta sin insertar nada.
//  3. Si no encuentra ninguna, inserta el bloqueo.
//
// Mientras esta transacción está abierta, cualquier otra transacción que
// intente tocar esas mismas filas de reservas (por ejemplo, otra petición
// intentando reservar justo en ese rango) tiene que esperar a que esta
// termine. Ver docs/CONCURRENCIA.md para la comparación completa entre
// esta estrategia y la de la Fase 3.
func (r *RepositorioDiasBloqueados) Guardar(ctx context.Context, bloqueo entidades.DiaBloqueado) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("iniciando transacción: %w", err)
	}
	// Si la función termina por cualquier camino sin haber hecho Commit,
	// Rollback deshace lo que se haya hecho. Si ya hubo Commit, este
	// Rollback no tiene nada que deshacer y pgx lo ignora sin error.
	defer tx.Rollback(ctx)

	inicioRango, finRango := rangoAfectadoPorBloqueo(bloqueo)

	filas, err := tx.Query(ctx, `
		SELECT 1 FROM reservas
		WHERE estado = 'confirmada' AND inicio < $2 AND fin > $1
		FOR UPDATE
	`, inicioRango, finRango)
	if err != nil {
		return fmt.Errorf("verificando reservas existentes: %w", err)
	}
	hayReservas := filas.Next()
	filas.Close()
	if err := filas.Err(); err != nil {
		return fmt.Errorf("verificando reservas existentes: %w", err)
	}
	if hayReservas {
		return dominio.ErrDiaBloqueadoConReservas
	}

	var horaDesdeMinutos *int
	if bloqueo.HoraDesde != nil {
		m := minutosDesdeHoraDelDia(*bloqueo.HoraDesde)
		horaDesdeMinutos = &m
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO dias_bloqueados (id, fecha, hora_desde_minutos, motivo)
		VALUES ($1, $2, $3, $4)
	`, bloqueo.ID, bloqueo.Fecha, horaDesdeMinutos, bloqueo.Motivo)
	if err != nil {
		return fmt.Errorf("guardando bloqueo: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("confirmando transacción: %w", err)
	}
	return nil
}

// rangoAfectadoPorBloqueo devuelve [inicio, fin) del rango que un
// DiaBloqueado deja sin turnos: desde medianoche (o desde HoraDesde, si
// es un bloqueo parcial) hasta la medianoche siguiente. Usamos la
// medianoche siguiente como límite superior en vez del horario de
// atención configurado porque una reserva nunca puede existir fuera del
// horario de atención (eso lo garantiza el caso de uso que la crea, en
// la Fase 5) — así que medianoche es un límite seguro y más simple, sin
// tener que consultar horarios_atencion acá también.
func rangoAfectadoPorBloqueo(b entidades.DiaBloqueado) (time.Time, time.Time) {
	finDia := time.Date(b.Fecha.Year(), b.Fecha.Month(), b.Fecha.Day()+1, 0, 0, 0, 0, time.UTC)
	if b.HoraDesde == nil {
		inicioDia := time.Date(b.Fecha.Year(), b.Fecha.Month(), b.Fecha.Day(), 0, 0, 0, 0, time.UTC)
		return inicioDia, finDia
	}
	return b.HoraDesde.EnFecha(b.Fecha), finDia
}

// Eliminar borra un DiaBloqueado.
func (r *RepositorioDiasBloqueados) Eliminar(ctx context.Context, id entidades.ID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM dias_bloqueados WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("eliminando bloqueo: %w", err)
	}
	return nil
}

// ListarEnRango devuelve los bloqueos cuya fecha cae entre desde y hasta
// (inclusive).
func (r *RepositorioDiasBloqueados) ListarEnRango(ctx context.Context, desde, hasta time.Time) ([]entidades.DiaBloqueado, error) {
	filas, err := r.pool.Query(ctx, `
		SELECT id, fecha, hora_desde_minutos, motivo
		FROM dias_bloqueados
		WHERE fecha BETWEEN $1 AND $2
		ORDER BY fecha
	`, desde, hasta)
	if err != nil {
		return nil, fmt.Errorf("listando bloqueos: %w", err)
	}
	defer filas.Close()

	var bloqueos []entidades.DiaBloqueado
	for filas.Next() {
		bloqueo, err := escanearDiaBloqueado(filas)
		if err != nil {
			return nil, err
		}
		bloqueos = append(bloqueos, bloqueo)
	}
	if err := filas.Err(); err != nil {
		return nil, fmt.Errorf("leyendo bloqueos: %w", err)
	}
	return bloqueos, nil
}

func escanearDiaBloqueado(fila pgx.Row) (entidades.DiaBloqueado, error) {
	var b entidades.DiaBloqueado
	var id string
	var horaDesdeMinutos *int

	err := fila.Scan(&id, &b.Fecha, &horaDesdeMinutos, &b.Motivo)
	if err != nil {
		return entidades.DiaBloqueado{}, fmt.Errorf("leyendo bloqueo: %w", err)
	}

	b.ID = entidades.ID(id)
	b.Fecha = b.Fecha.UTC() // ver el comentario en reservas.go sobre pgx y timestamptz/date
	if horaDesdeMinutos != nil {
		h, err := horaDelDiaDesdeMinutos(*horaDesdeMinutos)
		if err != nil {
			return entidades.DiaBloqueado{}, fmt.Errorf("hora_desde_minutos inválido en la base: %w", err)
		}
		b.HoraDesde = &h
	}
	return b, nil
}
