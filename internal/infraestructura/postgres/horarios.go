package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"reservas-go/internal/dominio"
	"reservas-go/internal/dominio/entidades"
)

// RepositorioHorarios implementa puertos.RepositorioHorarios contra
// PostgreSQL.
type RepositorioHorarios struct {
	pool *pgxpool.Pool
}

// NuevoRepositorioHorarios crea un RepositorioHorarios.
func NuevoRepositorioHorarios(pool *pgxpool.Pool) *RepositorioHorarios {
	return &RepositorioHorarios{pool: pool}
}

// Guardar inserta un HorarioAtencion nuevo.
func (r *RepositorioHorarios) Guardar(ctx context.Context, horario entidades.HorarioAtencion) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO horarios_atencion (id, dia_semana, hora_inicio_minutos, hora_fin_minutos)
		VALUES ($1, $2, $3, $4)
	`, horario.ID, int(horario.DiaSemana), minutosDesdeHoraDelDia(horario.HoraInicio), minutosDesdeHoraDelDia(horario.HoraFin))
	if err != nil {
		return fmt.Errorf("guardando horario: %w", err)
	}
	return nil
}

// Actualizar sobreescribe un HorarioAtencion existente.
func (r *RepositorioHorarios) Actualizar(ctx context.Context, horario entidades.HorarioAtencion) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE horarios_atencion
		SET dia_semana = $2, hora_inicio_minutos = $3, hora_fin_minutos = $4
		WHERE id = $1
	`, horario.ID, int(horario.DiaSemana), minutosDesdeHoraDelDia(horario.HoraInicio), minutosDesdeHoraDelDia(horario.HoraFin))
	if err != nil {
		return fmt.Errorf("actualizando horario: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("horario %s: %w", horario.ID, dominio.ErrNoEncontrado)
	}
	return nil
}

// BuscarPorDia devuelve el horario configurado para ese día de la semana,
// o nil si ese día no tiene horario (día de descanso) — no es un error,
// es un resultado válido y esperable.
func (r *RepositorioHorarios) BuscarPorDia(ctx context.Context, dia time.Weekday) (*entidades.HorarioAtencion, error) {
	fila := r.pool.QueryRow(ctx, `
		SELECT id, dia_semana, hora_inicio_minutos, hora_fin_minutos
		FROM horarios_atencion
		WHERE dia_semana = $1
	`, int(dia))

	horario, err := escanearHorario(fila)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &horario, nil
}

// Listar devuelve todos los horarios configurados.
func (r *RepositorioHorarios) Listar(ctx context.Context) ([]entidades.HorarioAtencion, error) {
	filas, err := r.pool.Query(ctx, `
		SELECT id, dia_semana, hora_inicio_minutos, hora_fin_minutos
		FROM horarios_atencion
		ORDER BY dia_semana
	`)
	if err != nil {
		return nil, fmt.Errorf("listando horarios: %w", err)
	}
	defer filas.Close()

	var horarios []entidades.HorarioAtencion
	for filas.Next() {
		horario, err := escanearHorario(filas)
		if err != nil {
			return nil, err
		}
		horarios = append(horarios, horario)
	}
	if err := filas.Err(); err != nil {
		return nil, fmt.Errorf("leyendo horarios: %w", err)
	}
	return horarios, nil
}

func escanearHorario(fila pgx.Row) (entidades.HorarioAtencion, error) {
	var h entidades.HorarioAtencion
	var id string
	var diaSemana, horaInicioMin, horaFinMin int

	err := fila.Scan(&id, &diaSemana, &horaInicioMin, &horaFinMin)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entidades.HorarioAtencion{}, err
		}
		return entidades.HorarioAtencion{}, fmt.Errorf("leyendo horario: %w", err)
	}

	horaInicio, err := horaDelDiaDesdeMinutos(horaInicioMin)
	if err != nil {
		return entidades.HorarioAtencion{}, fmt.Errorf("hora_inicio_minutos inválido en la base: %w", err)
	}
	horaFin, err := horaDelDiaDesdeMinutos(horaFinMin)
	if err != nil {
		return entidades.HorarioAtencion{}, fmt.Errorf("hora_fin_minutos inválido en la base: %w", err)
	}

	h.ID = entidades.ID(id)
	h.DiaSemana = time.Weekday(diaSemana)
	h.HoraInicio = horaInicio
	h.HoraFin = horaFin
	return h, nil
}

// minutosDesdeHoraDelDia y horaDelDiaDesdeMinutos convierten entre
// entidades.HoraDelDia y el entero "minutos desde medianoche" que
// guardamos en la base (ver el comentario en la migración
// 000004_crear_horarios_atencion sobre por qué elegimos esta
// representación en vez del tipo `time` nativo de Postgres).
func minutosDesdeHoraDelDia(h entidades.HoraDelDia) int {
	return h.Horas*60 + h.Minutos
}

func horaDelDiaDesdeMinutos(minutos int) (entidades.HoraDelDia, error) {
	return entidades.NuevaHoraDelDia(minutos/60, minutos%60)
}
