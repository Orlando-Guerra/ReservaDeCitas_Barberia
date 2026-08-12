package aplicacion

import (
	"context"
	"time"

	"reservas-go/internal/dominio"
	"reservas-go/internal/dominio/entidades"
	"reservas-go/internal/dominio/puertos"
)

// ServicioDisponibilidad agrupa el caso de uso de consultar los slots de
// un día.
type ServicioDisponibilidad struct {
	servicios      puertos.RepositorioServicios
	horarios       puertos.RepositorioHorarios
	diasBloqueados puertos.RepositorioDiasBloqueados
	reservas       puertos.RepositorioReservas
	reloj          puertos.Reloj
}

// NuevoServicioDisponibilidad crea un ServicioDisponibilidad.
func NuevoServicioDisponibilidad(
	servicios puertos.RepositorioServicios,
	horarios puertos.RepositorioHorarios,
	diasBloqueados puertos.RepositorioDiasBloqueados,
	reservas puertos.RepositorioReservas,
	reloj puertos.Reloj,
) *ServicioDisponibilidad {
	return &ServicioDisponibilidad{
		servicios:      servicios,
		horarios:       horarios,
		diasBloqueados: diasBloqueados,
		reservas:       reservas,
		reloj:          reloj,
	}
}

// ConsultarSlots devuelve los slots (disponibles u ocupados) de un día
// para un servicio, respetando el límite de anticipación de los clientes
// (solo hoy o mañana) — el administrador no tiene ese límite, porque
// puede necesitar mirar la agenda de cualquier día para organizarse.
func (s *ServicioDisponibilidad) ConsultarSlots(ctx context.Context, fecha time.Time, servicioID entidades.ID, rolSolicitante entidades.Rol) ([]dominio.Slot, error) {
	if _, err := s.servicios.BuscarPorID(ctx, servicioID); err != nil {
		return nil, err
	}

	ahora := s.reloj.Ahora()
	if rolSolicitante == entidades.RolCliente {
		if err := validarAnticipacionCliente(fecha, ahora); err != nil {
			return nil, err
		}
	}

	horario, err := s.horarios.BuscarPorDia(ctx, fecha.Weekday())
	if err != nil {
		return nil, err
	}

	bloqueos, err := s.diasBloqueados.ListarEnRango(ctx, fecha, fecha)
	if err != nil {
		return nil, err
	}

	reservasDelDia, err := s.reservas.ListarPorFecha(ctx, fecha)
	if err != nil {
		return nil, err
	}

	return dominio.CalcularSlotsDisponibles(fecha, horario, bloqueos, reservasDelDia, ahora), nil
}

// validarAnticipacionCliente aplica la regla de Fase 0: un cliente solo
// puede ver/reservar slots de hoy o de mañana (día calendario, contado
// desde "ahora").
func validarAnticipacionCliente(fecha, ahora time.Time) error {
	hoy := soloFecha(ahora)
	manana := hoy.AddDate(0, 0, 1)
	fechaSolicitada := soloFecha(fecha)

	if fechaSolicitada.Before(hoy) || fechaSolicitada.After(manana) {
		return dominio.ErrFechaFueraDeAnticipacion
	}
	return nil
}

func soloFecha(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}
