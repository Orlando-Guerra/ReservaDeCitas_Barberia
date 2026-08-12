package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"reservas-go/internal/dominio"
	"reservas-go/internal/dominio/entidades"
)

// RepositorioServicios implementa puertos.RepositorioServicios contra
// PostgreSQL.
type RepositorioServicios struct {
	pool *pgxpool.Pool
}

// NuevoRepositorioServicios crea un RepositorioServicios.
func NuevoRepositorioServicios(pool *pgxpool.Pool) *RepositorioServicios {
	return &RepositorioServicios{pool: pool}
}

// Guardar inserta un Servicio nuevo.
func (r *RepositorioServicios) Guardar(ctx context.Context, servicio entidades.Servicio) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO servicios (id, nombre, duracion_minutos, precio_centavos, activo)
		VALUES ($1, $2, $3, $4, $5)
	`, servicio.ID, servicio.Nombre, servicio.DuracionMinutos, servicio.PrecioCentavos, servicio.Activo)
	if err != nil {
		return fmt.Errorf("guardando servicio: %w", err)
	}
	return nil
}

// Actualizar sobreescribe los datos de un Servicio existente.
func (r *RepositorioServicios) Actualizar(ctx context.Context, servicio entidades.Servicio) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE servicios
		SET nombre = $2, duracion_minutos = $3, precio_centavos = $4, activo = $5
		WHERE id = $1
	`, servicio.ID, servicio.Nombre, servicio.DuracionMinutos, servicio.PrecioCentavos, servicio.Activo)
	if err != nil {
		return fmt.Errorf("actualizando servicio: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("servicio %s: %w", servicio.ID, dominio.ErrNoEncontrado)
	}
	return nil
}

// BuscarPorID busca un Servicio por su ID.
func (r *RepositorioServicios) BuscarPorID(ctx context.Context, id entidades.ID) (entidades.Servicio, error) {
	fila := r.pool.QueryRow(ctx, `
		SELECT id, nombre, duracion_minutos, precio_centavos, activo
		FROM servicios
		WHERE id = $1
	`, id)
	return escanearServicio(fila)
}

// Listar devuelve todos los servicios.
func (r *RepositorioServicios) Listar(ctx context.Context) ([]entidades.Servicio, error) {
	filas, err := r.pool.Query(ctx, `
		SELECT id, nombre, duracion_minutos, precio_centavos, activo
		FROM servicios
		ORDER BY nombre
	`)
	if err != nil {
		return nil, fmt.Errorf("listando servicios: %w", err)
	}
	defer filas.Close()

	var servicios []entidades.Servicio
	for filas.Next() {
		servicio, err := escanearServicio(filas)
		if err != nil {
			return nil, err
		}
		servicios = append(servicios, servicio)
	}
	if err := filas.Err(); err != nil {
		return nil, fmt.Errorf("leyendo servicios: %w", err)
	}
	return servicios, nil
}

func escanearServicio(fila pgx.Row) (entidades.Servicio, error) {
	var s entidades.Servicio
	var id string

	err := fila.Scan(&id, &s.Nombre, &s.DuracionMinutos, &s.PrecioCentavos, &s.Activo)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entidades.Servicio{}, fmt.Errorf("servicio no encontrado: %w", dominio.ErrNoEncontrado)
		}
		return entidades.Servicio{}, fmt.Errorf("leyendo servicio: %w", err)
	}

	s.ID = entidades.ID(id)
	return s, nil
}
