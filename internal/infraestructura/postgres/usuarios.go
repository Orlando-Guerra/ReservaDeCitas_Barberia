package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"reservas-go/internal/dominio"
	"reservas-go/internal/dominio/entidades"
)

// codigoViolacionUnicidad es el SQLSTATE que Postgres devuelve cuando un
// INSERT o UPDATE viola un constraint UNIQUE — en usuarios, la columna
// email. Mismo mecanismo que usamos en reservas.go para la violación de
// exclusión, aplicado acá a un constraint más simple.
const codigoViolacionUnicidad = "23505"

// RepositorioUsuarios implementa puertos.RepositorioUsuarios contra
// PostgreSQL.
//
// No declaramos en ningún lado "RepositorioUsuarios implementa
// puertos.RepositorioUsuarios" — Go no tiene esa sintaxis. Alcanza con
// que este struct tenga los tres métodos que pide la interfaz, con las
// firmas exactas. Es interfaces implícitas: la conexión entre este tipo
// y el puerto del dominio la arma el compilador solo, en el punto donde
// alguien intente usar un *RepositorioUsuarios como puertos.RepositorioUsuarios
// (eso va a pasar en cmd/api/main.go a partir de la Fase 5).
type RepositorioUsuarios struct {
	pool *pgxpool.Pool
}

// NuevoRepositorioUsuarios crea un RepositorioUsuarios que usa el pool
// de conexiones dado.
func NuevoRepositorioUsuarios(pool *pgxpool.Pool) *RepositorioUsuarios {
	return &RepositorioUsuarios{pool: pool}
}

// Guardar inserta un Usuario nuevo.
func (r *RepositorioUsuarios) Guardar(ctx context.Context, usuario entidades.Usuario) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO usuarios (id, nombre, email, password_hash, rol, creado_en)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, usuario.ID, usuario.Nombre, usuario.Email, usuario.PasswordHash, usuario.Rol, usuario.CreadoEn)
	if err != nil {
		var errPg *pgconn.PgError
		if errors.As(err, &errPg) && errPg.Code == codigoViolacionUnicidad {
			return fmt.Errorf("guardando usuario: %w", dominio.ErrEmailYaRegistrado)
		}
		return fmt.Errorf("guardando usuario: %w", err)
	}
	return nil
}

// BuscarPorEmail busca un Usuario por su email (usado en el login, Fase 4).
func (r *RepositorioUsuarios) BuscarPorEmail(ctx context.Context, email string) (entidades.Usuario, error) {
	fila := r.pool.QueryRow(ctx, `
		SELECT id, nombre, email, password_hash, rol, creado_en
		FROM usuarios
		WHERE email = $1
	`, email)
	return escanearUsuario(fila)
}

// ActualizarPassword sobreescribe el hash de contraseña de un usuario.
func (r *RepositorioUsuarios) ActualizarPassword(ctx context.Context, id entidades.ID, passwordHash string) error {
	tag, err := r.pool.Exec(ctx, `UPDATE usuarios SET password_hash = $2 WHERE id = $1`, id, passwordHash)
	if err != nil {
		return fmt.Errorf("actualizando contraseña: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("usuario %s: %w", id, dominio.ErrNoEncontrado)
	}
	return nil
}

// BuscarPorID busca un Usuario por su ID.
func (r *RepositorioUsuarios) BuscarPorID(ctx context.Context, id entidades.ID) (entidades.Usuario, error) {
	fila := r.pool.QueryRow(ctx, `
		SELECT id, nombre, email, password_hash, rol, creado_en
		FROM usuarios
		WHERE id = $1
	`, id)
	return escanearUsuario(fila)
}

// escanearUsuario vuelca una fila de la tabla usuarios en un
// entidades.Usuario. pgx.Row es la interfaz que satisfacen tanto
// QueryRow (una fila) como cada fila al iterar con Query — por eso esta
// función sirve para ambos casos sin duplicar código.
//
// Escaneamos id y rol como string plano (no directamente como
// entidades.ID / entidades.Rol) y los convertimos a mano después: es más
// explícito que confiar en que pgx sepa convertir automáticamente a un
// tipo con nombre propio.
func escanearUsuario(fila pgx.Row) (entidades.Usuario, error) {
	var u entidades.Usuario
	var id, rol string

	err := fila.Scan(&id, &u.Nombre, &u.Email, &u.PasswordHash, &rol, &u.CreadoEn)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entidades.Usuario{}, fmt.Errorf("usuario no encontrado: %w", dominio.ErrNoEncontrado)
		}
		return entidades.Usuario{}, fmt.Errorf("leyendo usuario: %w", err)
	}

	u.ID = entidades.ID(id)
	u.Rol = entidades.Rol(rol)
	// Ver el comentario equivalente en reservas.go: pgx decodifica
	// timestamptz con la ubicación horaria local del proceso, no UTC.
	u.CreadoEn = u.CreadoEn.UTC()
	return u, nil
}
