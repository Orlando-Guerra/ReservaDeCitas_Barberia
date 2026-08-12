package aplicacion

import (
	"context"
	"time"

	"reservas-go/internal/dominio/entidades"
	"reservas-go/internal/dominio/puertos"
)

// ServicioAdministracion agrupa los casos de uso que solo el
// administrador usa: definir servicios, horarios de atención y días
// bloqueados.
type ServicioAdministracion struct {
	servicios      puertos.RepositorioServicios
	horarios       puertos.RepositorioHorarios
	diasBloqueados puertos.RepositorioDiasBloqueados
}

// NuevoServicioAdministracion crea un ServicioAdministracion.
func NuevoServicioAdministracion(
	servicios puertos.RepositorioServicios,
	horarios puertos.RepositorioHorarios,
	diasBloqueados puertos.RepositorioDiasBloqueados,
) *ServicioAdministracion {
	return &ServicioAdministracion{servicios: servicios, horarios: horarios, diasBloqueados: diasBloqueados}
}

// --- Servicios ---

// CrearServicio da de alta un Servicio nuevo.
func (s *ServicioAdministracion) CrearServicio(ctx context.Context, nombre string, duracionMinutos int, precioCentavos int64) (entidades.Servicio, error) {
	servicio, err := entidades.NuevoServicio(nombre, duracionMinutos, precioCentavos)
	if err != nil {
		return entidades.Servicio{}, err
	}
	if err := s.servicios.Guardar(ctx, servicio); err != nil {
		return entidades.Servicio{}, err
	}
	return servicio, nil
}

// ActualizarServicio sobreescribe los datos de un Servicio existente.
func (s *ServicioAdministracion) ActualizarServicio(ctx context.Context, id entidades.ID, nombre string, duracionMinutos int, precioCentavos int64, activo bool) (entidades.Servicio, error) {
	servicio, err := s.servicios.BuscarPorID(ctx, id)
	if err != nil {
		return entidades.Servicio{}, err
	}

	actualizado, err := entidades.NuevoServicio(nombre, duracionMinutos, precioCentavos)
	if err != nil {
		return entidades.Servicio{}, err
	}
	actualizado.ID = servicio.ID // conservamos el ID original; NuevoServicio generó uno nuevo que no usamos
	actualizado.Activo = activo

	if err := s.servicios.Actualizar(ctx, actualizado); err != nil {
		return entidades.Servicio{}, err
	}
	return actualizado, nil
}

// ListarServicios devuelve todos los servicios.
func (s *ServicioAdministracion) ListarServicios(ctx context.Context) ([]entidades.Servicio, error) {
	return s.servicios.Listar(ctx)
}

// --- Horarios de atención ---

// DefinirHorario crea o actualiza el horario de atención de un día de la
// semana (a lo sumo un horario por día, ver Fase 2 y el UNIQUE de la
// migración 000004): si ya existía uno para ese día, lo sobreescribe; si
// no, lo crea.
func (s *ServicioAdministracion) DefinirHorario(ctx context.Context, diaSemana time.Weekday, horaInicio, horaFin entidades.HoraDelDia) (entidades.HorarioAtencion, error) {
	nuevo, err := entidades.NuevoHorarioAtencion(diaSemana, horaInicio, horaFin)
	if err != nil {
		return entidades.HorarioAtencion{}, err
	}

	existente, err := s.horarios.BuscarPorDia(ctx, diaSemana)
	if err != nil {
		return entidades.HorarioAtencion{}, err
	}

	if existente == nil {
		if err := s.horarios.Guardar(ctx, nuevo); err != nil {
			return entidades.HorarioAtencion{}, err
		}
		return nuevo, nil
	}

	nuevo.ID = existente.ID
	if err := s.horarios.Actualizar(ctx, nuevo); err != nil {
		return entidades.HorarioAtencion{}, err
	}
	return nuevo, nil
}

// ListarHorarios devuelve todos los horarios configurados.
func (s *ServicioAdministracion) ListarHorarios(ctx context.Context) ([]entidades.HorarioAtencion, error) {
	return s.horarios.Listar(ctx)
}

// --- Días bloqueados ---

// CrearBloqueo bloquea un día completo (horaDesde nil) o parte de un día
// (horaDesde con valor). RepositorioDiasBloqueados.Guardar es quien
// valida, dentro de una transacción, que no haya reservas confirmadas en
// el rango (ver docs/CONCURRENCIA.md).
func (s *ServicioAdministracion) CrearBloqueo(ctx context.Context, fecha time.Time, horaDesde *entidades.HoraDelDia, motivo string) (entidades.DiaBloqueado, error) {
	bloqueo, err := entidades.NuevoDiaBloqueado(fecha, horaDesde, motivo)
	if err != nil {
		return entidades.DiaBloqueado{}, err
	}
	if err := s.diasBloqueados.Guardar(ctx, bloqueo); err != nil {
		return entidades.DiaBloqueado{}, err
	}
	return bloqueo, nil
}

// EliminarBloqueo borra un bloqueo (por ejemplo, si el barbero cambió de
// opinión).
func (s *ServicioAdministracion) EliminarBloqueo(ctx context.Context, id entidades.ID) error {
	return s.diasBloqueados.Eliminar(ctx, id)
}

// ListarBloqueos devuelve los bloqueos en un rango de fechas.
func (s *ServicioAdministracion) ListarBloqueos(ctx context.Context, desde, hasta time.Time) ([]entidades.DiaBloqueado, error) {
	return s.diasBloqueados.ListarEnRango(ctx, desde, hasta)
}
