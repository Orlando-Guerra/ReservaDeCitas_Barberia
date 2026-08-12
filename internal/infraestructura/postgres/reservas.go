package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"reservas-go/internal/dominio"
	"reservas-go/internal/dominio/entidades"
	"reservas-go/internal/dominio/puertos"
)

// codigoViolacionExclusion es el SQLSTATE que Postgres devuelve cuando un
// INSERT viola un constraint EXCLUDE — en nuestro caso, el constraint
// reservas_no_solapadas de la migración 000006_crear_reservas. Es el
// mismo mecanismo que un código HTTP: un identificador estable que no
// depende del idioma ni del texto exacto del mensaje de error.
const codigoViolacionExclusion = "23P01"

// RepositorioReservas implementa puertos.RepositorioReservas contra
// PostgreSQL.
type RepositorioReservas struct {
	pool *pgxpool.Pool
}

// NuevoRepositorioReservas crea un RepositorioReservas.
func NuevoRepositorioReservas(pool *pgxpool.Pool) *RepositorioReservas {
	return &RepositorioReservas{pool: pool}
}

// Guardar inserta una Reserva nueva.
//
// A propósito, NO hacemos acá ningún "SELECT para ver si el slot está
// libre" antes del INSERT. Ese chequeo previo es exactamente el patrón
// que docs/CONCURRENCIA.md explica que NO alcanza bajo concurrencia (dos
// goroutines pueden pasar el SELECT al mismo tiempo, antes de que
// cualquiera haga el INSERT). En cambio, mandamos el INSERT directo y
// dejamos que el constraint de exclusión de la base de datos decida: si
// dos reservas se solapan, Postgres rechaza la segunda con el código
// 23P01, sin importar en qué orden lleguen ni cuántas lleguen a la vez.
// Acá simplemente traducimos ese código a un error de dominio.
func (r *RepositorioReservas) Guardar(ctx context.Context, reserva entidades.Reserva) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO reservas (id, recurso_id, cliente_id, servicio_id, inicio, fin, estado, creada_en)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, reserva.ID, RecursoUnicoID, reserva.ClienteID, reserva.ServicioID, reserva.Inicio, reserva.Fin, reserva.Estado, reserva.CreadaEn)
	if err != nil {
		var errPg *pgconn.PgError
		// errors.As busca, a lo largo de toda la cadena de errores
		// envueltos, uno que se pueda convertir al tipo *pgconn.PgError.
		// pgx envuelve el error de bajo nivel del protocolo de Postgres
		// dentro de otros errores propios, así que un simple type
		// assertion (err.(*pgconn.PgError)) podría no encontrarlo.
		if errors.As(err, &errPg) && errPg.Code == codigoViolacionExclusion {
			return fmt.Errorf("guardando reserva: %w", dominio.ErrSlotNoDisponible)
		}
		return fmt.Errorf("guardando reserva: %w", err)
	}
	return nil
}

// Cancelar marca una reserva como cancelada.
func (r *RepositorioReservas) Cancelar(ctx context.Context, id entidades.ID, canceladaEn time.Time) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE reservas
		SET estado = $2, cancelada_en = $3
		WHERE id = $1
	`, id, entidades.ReservaCancelada, canceladaEn)
	if err != nil {
		return fmt.Errorf("cancelando reserva: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("reserva %s: %w", id, dominio.ErrNoEncontrado)
	}
	return nil
}

// BuscarPorID busca una Reserva por su ID.
func (r *RepositorioReservas) BuscarPorID(ctx context.Context, id entidades.ID) (entidades.Reserva, error) {
	fila := r.pool.QueryRow(ctx, `
		SELECT id, cliente_id, servicio_id, inicio, fin, estado, creada_en, cancelada_en
		FROM reservas
		WHERE id = $1
	`, id)
	return escanearReserva(fila)
}

// ListarPorFecha devuelve las reservas cuyo inicio cae dentro del día de
// "fecha" (en UTC). Es el insumo que necesita
// dominio.CalcularSlotsDisponibles para saber qué slots están ocupados.
func (r *RepositorioReservas) ListarPorFecha(ctx context.Context, fecha time.Time) ([]entidades.Reserva, error) {
	inicioDia := time.Date(fecha.Year(), fecha.Month(), fecha.Day(), 0, 0, 0, 0, time.UTC)
	finDia := inicioDia.AddDate(0, 0, 1)

	filas, err := r.pool.Query(ctx, `
		SELECT id, cliente_id, servicio_id, inicio, fin, estado, creada_en, cancelada_en
		FROM reservas
		WHERE inicio >= $1 AND inicio < $2
		ORDER BY inicio
	`, inicioDia, finDia)
	if err != nil {
		return nil, fmt.Errorf("listando reservas por fecha: %w", err)
	}
	defer filas.Close()
	return escanearReservas(filas)
}

// ListarConFiltros devuelve reservas para la vista de administración,
// aplicando solo los filtros que vengan definidos (no nil).
//
// Armamos el SQL agregando pedazos de texto según qué filtros haya, pero
// los VALORES siempre viajan como parámetros ($1, $2, ...), nunca
// concatenados directamente en el string. Eso es lo que evita una
// inyección SQL: lo dinámico acá es la forma de la consulta, no los
// datos que entran en ella.
func (r *RepositorioReservas) ListarConFiltros(ctx context.Context, filtros puertos.FiltrosReservas) ([]entidades.Reserva, error) {
	consulta := `
		SELECT id, cliente_id, servicio_id, inicio, fin, estado, creada_en, cancelada_en
		FROM reservas
		WHERE 1 = 1
	`
	var args []any

	if filtros.Desde != nil {
		args = append(args, *filtros.Desde)
		consulta += fmt.Sprintf(" AND inicio >= $%d", len(args))
	}
	if filtros.Hasta != nil {
		args = append(args, *filtros.Hasta)
		consulta += fmt.Sprintf(" AND inicio < $%d", len(args))
	}
	if filtros.Estado != nil {
		args = append(args, string(*filtros.Estado))
		consulta += fmt.Sprintf(" AND estado = $%d", len(args))
	}
	if filtros.ClienteID != nil {
		args = append(args, string(*filtros.ClienteID))
		consulta += fmt.Sprintf(" AND cliente_id = $%d", len(args))
	}
	consulta += " ORDER BY inicio"

	filas, err := r.pool.Query(ctx, consulta, args...)
	if err != nil {
		return nil, fmt.Errorf("listando reservas con filtros: %w", err)
	}
	defer filas.Close()
	return escanearReservas(filas)
}

func escanearReserva(fila pgx.Row) (entidades.Reserva, error) {
	var res entidades.Reserva
	var id, clienteID, servicioID, estado string
	var canceladaEn *time.Time

	err := fila.Scan(&id, &clienteID, &servicioID, &res.Inicio, &res.Fin, &estado, &res.CreadaEn, &canceladaEn)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entidades.Reserva{}, fmt.Errorf("reserva no encontrada: %w", dominio.ErrNoEncontrado)
		}
		return entidades.Reserva{}, fmt.Errorf("leyendo reserva: %w", err)
	}

	res.ID = entidades.ID(id)
	res.ClienteID = entidades.ID(clienteID)
	res.ServicioID = entidades.ID(servicioID)
	res.Estado = entidades.EstadoReserva(estado)
	// pgx decodifica timestamptz con el time.Time resultante en la
	// ubicación (Location) horaria LOCAL del proceso Go, no en UTC — el
	// instante que representa es el correcto, pero al formatearlo (JSON,
	// logs) mostraría el offset local de esta máquina en vez de "Z". Como
	// la regla del proyecto es "todo se guarda y se muestra en UTC salvo
	// que se pida explícitamente lo contrario" (ver docs/CONCURRENCIA.md),
	// normalizamos accá, en el borde donde los datos entran desde la base.
	res.Inicio = res.Inicio.UTC()
	res.Fin = res.Fin.UTC()
	res.CreadaEn = res.CreadaEn.UTC()
	if canceladaEn != nil {
		t := canceladaEn.UTC()
		res.CanceladaEn = &t
	}
	return res, nil
}

func escanearReservas(filas pgx.Rows) ([]entidades.Reserva, error) {
	var reservas []entidades.Reserva
	for filas.Next() {
		reserva, err := escanearReserva(filas)
		if err != nil {
			return nil, err
		}
		reservas = append(reservas, reserva)
	}
	if err := filas.Err(); err != nil {
		return nil, fmt.Errorf("leyendo reservas: %w", err)
	}
	return reservas, nil
}
