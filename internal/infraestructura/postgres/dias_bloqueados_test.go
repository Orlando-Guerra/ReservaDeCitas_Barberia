package postgres_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"reservas-go/internal/dominio"
	"reservas-go/internal/dominio/entidades"
	"reservas-go/internal/infraestructura/postgres"
)

// TestRepositorioDiasBloqueados_RechazaBloqueoConReservas prueba, contra
// la base real, la regla de negocio que vive en la transacción con
// SELECT ... FOR UPDATE de RepositorioDiasBloqueados.Guardar (ver
// docs/CONCURRENCIA.md, sección 4b): no se puede bloquear un rango que ya
// tiene una reserva confirmada.
func TestRepositorioDiasBloqueados_RechazaBloqueoConReservas(t *testing.T) {
	pool := conectarDBDePrueba(t)
	ctx := context.Background()
	limpiarTablas(t, ctx, pool)

	repoUsuarios := postgres.NuevoRepositorioUsuarios(pool)
	repoServicios := postgres.NuevoRepositorioServicios(pool)
	repoReservas := postgres.NuevoRepositorioReservas(pool)
	repoBloqueos := postgres.NuevoRepositorioDiasBloqueados(pool)

	ahora := time.Now().UTC()

	cliente, err := entidades.NuevoUsuario("Cliente de prueba", "bloqueos@prueba.com", "hash", entidades.RolCliente, ahora)
	if err != nil {
		t.Fatalf("no se esperaba error: %v", err)
	}
	if err := repoUsuarios.Guardar(ctx, cliente); err != nil {
		t.Fatalf("no se esperaba error: %v", err)
	}

	servicio, err := entidades.NuevoServicio("Corte", 30, 100000)
	if err != nil {
		t.Fatalf("no se esperaba error: %v", err)
	}
	if err := repoServicios.Guardar(ctx, servicio); err != nil {
		t.Fatalf("no se esperaba error: %v", err)
	}

	fecha := time.Date(ahora.Year(), ahora.Month(), ahora.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, 3)
	inicioReserva := fecha.Add(10 * time.Hour) // 10:00 del día bloqueado

	reserva, err := entidades.NuevaReserva(cliente.ID, servicio.ID, inicioReserva, ahora)
	if err != nil {
		t.Fatalf("no se esperaba error: %v", err)
	}
	if err := repoReservas.Guardar(ctx, reserva); err != nil {
		t.Fatalf("no se esperaba error guardando la reserva: %v", err)
	}

	t.Run("bloquear el día completo se rechaza porque hay una reserva", func(t *testing.T) {
		bloqueo, err := entidades.NuevoDiaBloqueado(fecha, nil, "vacaciones")
		if err != nil {
			t.Fatalf("no se esperaba error: %v", err)
		}

		err = repoBloqueos.Guardar(ctx, bloqueo)
		if !errors.Is(err, dominio.ErrDiaBloqueadoConReservas) {
			t.Errorf("error = %v, se esperaba dominio.ErrDiaBloqueadoConReservas", err)
		}
	})

	t.Run("bloquear desde una hora ANTES de la reserva también se rechaza", func(t *testing.T) {
		horaDesde, _ := entidades.NuevaHoraDelDia(9, 0)
		bloqueo, err := entidades.NuevoDiaBloqueado(fecha, &horaDesde, "emergencia")
		if err != nil {
			t.Fatalf("no se esperaba error: %v", err)
		}

		err = repoBloqueos.Guardar(ctx, bloqueo)
		if !errors.Is(err, dominio.ErrDiaBloqueadoConReservas) {
			t.Errorf("error = %v, se esperaba dominio.ErrDiaBloqueadoConReservas", err)
		}
	})

	t.Run("bloquear desde una hora DESPUÉS de que termina la reserva sí se permite", func(t *testing.T) {
		horaDesde, _ := entidades.NuevaHoraDelDia(11, 0) // la reserva termina a las 11:00
		bloqueo, err := entidades.NuevoDiaBloqueado(fecha, &horaDesde, "resto del día libre")
		if err != nil {
			t.Fatalf("no se esperaba error: %v", err)
		}

		if err := repoBloqueos.Guardar(ctx, bloqueo); err != nil {
			t.Errorf("no se esperaba error: %v", err)
		}
	})

	t.Run("un día sin ninguna reserva se puede bloquear sin problema", func(t *testing.T) {
		otroDia := fecha.AddDate(0, 0, 10)
		bloqueo, err := entidades.NuevoDiaBloqueado(otroDia, nil, "vacaciones")
		if err != nil {
			t.Fatalf("no se esperaba error: %v", err)
		}

		if err := repoBloqueos.Guardar(ctx, bloqueo); err != nil {
			t.Errorf("no se esperaba error: %v", err)
		}
	})
}
