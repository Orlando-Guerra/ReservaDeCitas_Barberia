package puertos

import (
	"context"
	"time"

	"reservas-go/internal/dominio/entidades"
)

// RepositorioUsuarios abstrae la persistencia de Usuarios.
type RepositorioUsuarios interface {
	Guardar(ctx context.Context, usuario entidades.Usuario) error
	BuscarPorEmail(ctx context.Context, email string) (entidades.Usuario, error)
	BuscarPorID(ctx context.Context, id entidades.ID) (entidades.Usuario, error)
	// ActualizarPassword sobreescribe el hash de contraseña de un
	// usuario. Hoy solo la usa cmd/seed-admin, para poder resetear la
	// contraseña del administrador sin tener que borrar y recrear la
	// base.
	ActualizarPassword(ctx context.Context, id entidades.ID, passwordHash string) error
}

// RepositorioServicios abstrae la persistencia de Servicios.
type RepositorioServicios interface {
	Guardar(ctx context.Context, servicio entidades.Servicio) error
	Actualizar(ctx context.Context, servicio entidades.Servicio) error
	BuscarPorID(ctx context.Context, id entidades.ID) (entidades.Servicio, error)
	Listar(ctx context.Context) ([]entidades.Servicio, error)
}

// RepositorioHorarios abstrae la persistencia de HorarioAtencion.
type RepositorioHorarios interface {
	Guardar(ctx context.Context, horario entidades.HorarioAtencion) error
	Actualizar(ctx context.Context, horario entidades.HorarioAtencion) error
	// BuscarPorDia devuelve el horario configurado para ese día de la
	// semana, o nil si ese día no tiene horario (día de descanso).
	BuscarPorDia(ctx context.Context, dia time.Weekday) (*entidades.HorarioAtencion, error)
	Listar(ctx context.Context) ([]entidades.HorarioAtencion, error)
}

// RepositorioDiasBloqueados abstrae la persistencia de DiaBloqueado.
type RepositorioDiasBloqueados interface {
	Guardar(ctx context.Context, bloqueo entidades.DiaBloqueado) error
	Eliminar(ctx context.Context, id entidades.ID) error
	// ListarEnRango devuelve los bloqueos cuya fecha cae entre desde y
	// hasta (inclusive), insumo para calcular slots o validar un bloqueo
	// nuevo.
	ListarEnRango(ctx context.Context, desde, hasta time.Time) ([]entidades.DiaBloqueado, error)
}

// FiltrosReservas agrupa los filtros opcionales para listar reservas
// desde la vista de administración.
//
// Los campos son punteros para poder distinguir "no se pidió este
// filtro" (nil) de "se pidió, con este valor". Con un time.Time o un
// EstadoReserva normal (sin puntero) no habría forma de representar
// "no filtrar por esto": todo valor, incluido el "vacío", sería un valor
// real y ambiguo.
type FiltrosReservas struct {
	Desde     *time.Time
	Hasta     *time.Time
	Estado    *entidades.EstadoReserva
	ClienteID *entidades.ID
}

// RepositorioReservas abstrae la persistencia de Reservas.
type RepositorioReservas interface {
	Guardar(ctx context.Context, reserva entidades.Reserva) error
	// Cancelar marca una reserva como cancelada. Recibe el ID en vez de
	// la entidad completa porque, a partir de la Fase 3, esta operación
	// tiene que convivir con posibles reservas concurrentes sobre el
	// mismo recurso — la implementación concreta decide cómo protegerla.
	Cancelar(ctx context.Context, id entidades.ID, canceladaEn time.Time) error
	BuscarPorID(ctx context.Context, id entidades.ID) (entidades.Reserva, error)
	// ListarPorFecha devuelve las reservas de un día puntual: es el
	// insumo que necesita dominio.CalcularSlotsDisponibles.
	ListarPorFecha(ctx context.Context, fecha time.Time) ([]entidades.Reserva, error)
	// ListarConFiltros devuelve reservas para la vista de administración.
	ListarConFiltros(ctx context.Context, filtros FiltrosReservas) ([]entidades.Reserva, error)
}
