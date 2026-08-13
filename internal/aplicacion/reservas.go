package aplicacion

import (
	"context"
	"fmt"
	"log"
	"time"

	"reservas-go/internal/dominio"
	"reservas-go/internal/dominio/entidades"
	"reservas-go/internal/dominio/puertos"
)

// ServicioReservas agrupa los casos de uso de crear, cancelar y listar
// reservas.
type ServicioReservas struct {
	usuarios       puertos.RepositorioUsuarios
	servicios      puertos.RepositorioServicios
	horarios       puertos.RepositorioHorarios
	diasBloqueados puertos.RepositorioDiasBloqueados
	reservas       puertos.RepositorioReservas
	notificador    puertos.Notificador
	reloj          puertos.Reloj
}

// NuevoServicioReservas crea un ServicioReservas.
func NuevoServicioReservas(
	usuarios puertos.RepositorioUsuarios,
	servicios puertos.RepositorioServicios,
	horarios puertos.RepositorioHorarios,
	diasBloqueados puertos.RepositorioDiasBloqueados,
	reservas puertos.RepositorioReservas,
	notificador puertos.Notificador,
	reloj puertos.Reloj,
) *ServicioReservas {
	return &ServicioReservas{
		usuarios:       usuarios,
		servicios:      servicios,
		horarios:       horarios,
		diasBloqueados: diasBloqueados,
		reservas:       reservas,
		notificador:    notificador,
		reloj:          reloj,
	}
}

// ParametrosCrearReserva agrupa los datos necesarios para crear una
// reserva. Es un struct (en vez de varios parámetros sueltos) porque ya
// son 4 valores relacionados, y porque así queda claro en el código que
// llama qué significa cada uno (en vez de una lista de posiciones donde
// es fácil confundir el orden).
type ParametrosCrearReserva struct {
	ClienteID      entidades.ID
	ServicioID     entidades.ID
	Inicio         time.Time
	RolSolicitante entidades.Rol // quién está pidiendo la reserva: el propio cliente, o el administrador (walk-in)
}

// CrearReserva valida las reglas de negocio y crea una reserva.
//
// La validación de "¿este slot está realmente disponible?" reutiliza
// dominio.CalcularSlotsDisponibles (la misma función que usa
// ServicioDisponibilidad.ConsultarSlots) en vez de reimplementar la
// lógica acá: así el slot que el cliente vio como disponible en la
// consulta previa es exactamente el mismo criterio que se usa para
// aceptar o rechazar la reserva.
//
// Aun así, esta validación en Go NO es la garantía final contra la doble
// reserva — es una buena experiencia para el usuario (un error claro
// antes de intentar nada), pero dos pedidos concurrentes podrían pasar
// esta misma validación a la vez. La garantía real está en el constraint
// de la base de datos (Fase 3): por eso, más abajo, tratamos el error que
// puede devolver reservas.Guardar como algo esperable, no como una falla
// inesperada.
func (s *ServicioReservas) CrearReserva(ctx context.Context, params ParametrosCrearReserva) (entidades.Reserva, error) {
	servicio, err := s.servicios.BuscarPorID(ctx, params.ServicioID)
	if err != nil {
		return entidades.Reserva{}, err
	}

	ahora := s.reloj.Ahora()

	if params.RolSolicitante == entidades.RolCliente {
		if err := validarAnticipacionCliente(params.Inicio, ahora); err != nil {
			return entidades.Reserva{}, err
		}
	}

	fecha := soloFecha(params.Inicio)
	horario, err := s.horarios.BuscarPorDia(ctx, fecha.Weekday())
	if err != nil {
		return entidades.Reserva{}, err
	}
	bloqueos, err := s.diasBloqueados.ListarEnRango(ctx, fecha, fecha)
	if err != nil {
		return entidades.Reserva{}, err
	}
	reservasDelDia, err := s.reservas.ListarPorFecha(ctx, fecha)
	if err != nil {
		return entidades.Reserva{}, err
	}

	if horario != nil {
		slots := dominio.CalcularSlotsDisponibles(fecha, horario, bloqueos, reservasDelDia, ahora)
		if !hayCoincidenciaDisponible(slots, params.Inicio) {
			return entidades.Reserva{}, dominio.ErrSlotNoDisponible
		}
	} else {
		// No hay horario configurado para este día de la semana. Un
		// cliente no puede reservar acá — no hay agenda que ofrecerle. El
		// administrador sí puede, como excepción puntual (un cliente
		// frecuente que solo puede ese día, una urgencia): igual respeta
		// un día bloqueado por completo, que un turno no puede empezar en
		// el pasado, y que no se solape con otra reserva confirmada. El
		// constraint de la base de datos (Fase 3) sigue siendo la
		// garantía final contra el solapamiento en cualquiera de los dos
		// casos.
		if params.RolSolicitante != entidades.RolAdministrador {
			return entidades.Reserva{}, dominio.ErrSlotNoDisponible
		}
		if diaCompletoBloqueado(bloqueos, fecha) {
			return entidades.Reserva{}, dominio.ErrSlotNoDisponible
		}
		if !params.Inicio.After(ahora) {
			return entidades.Reserva{}, dominio.ErrSlotNoDisponible
		}
		if dominio.SolapaConReservaConfirmada(params.Inicio, params.Inicio.Add(entidades.DuracionSlot), reservasDelDia) {
			return entidades.Reserva{}, dominio.ErrSlotNoDisponible
		}
	}

	reserva, err := entidades.NuevaReserva(params.ClienteID, params.ServicioID, params.Inicio, ahora)
	if err != nil {
		return entidades.Reserva{}, err
	}

	// Acá es donde la base de datos tiene la última palabra: si otra
	// petición ganó la carrera por este mismo slot entre que calculamos
	// los slots (arriba) y este Guardar, el constraint de exclusión de
	// Postgres rechaza el INSERT y RepositorioReservas.Guardar devuelve
	// dominio.ErrSlotNoDisponible — el mismo error que devolveríamos si
	// lo hubiéramos detectado acá arriba.
	if err := s.reservas.Guardar(ctx, reserva); err != nil {
		return entidades.Reserva{}, err
	}

	// La reserva ya está guardada — lo que le importa al negocio ya
	// pasó. Mandar el correo de confirmación es una cortesía para el
	// cliente, no algo de lo que dependa el éxito de la operación: por
	// eso lo lanzamos en una goroutine aparte con "go" y devolvemos la
	// reserva de inmediato, sin esperar a que el correo termine de
	// mandarse. Ver notificarConfirmacionAsync para el detalle de por
	// qué esto necesita su propio contexto y qué pasa si falla.
	s.notificarConfirmacionAsync(reserva, servicio)

	return reserva, nil
}

// notificarConfirmacionAsync manda el correo de confirmación en segundo
// plano, sin bloquear al que llamó a CrearReserva.
//
// Un detalle importante: NO le pasamos a esta goroutine el "ctx" que
// recibió CrearReserva. Ese contexto pertenece al pedido HTTP original, y
// net/http lo cancela automáticamente en cuanto termina de escribirse la
// respuesta — que puede pasar bien antes de que esta goroutine llegue
// siquiera a conectarse al servidor de correo. Si usáramos ese mismo
// contexto, el envío se cortaría a mitad de camino la mayoría de las
// veces. En cambio, armamos un contexto nuevo e independiente
// (context.Background(), con su propio timeout), que sigue vivo aunque
// el pedido HTTP ya haya terminado.
func (s *ServicioReservas) notificarConfirmacionAsync(reserva entidades.Reserva, servicio entidades.Servicio) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		cliente, err := s.usuarios.BuscarPorID(ctx, reserva.ClienteID)
		if err != nil {
			log.Printf("no se pudo notificar la reserva %s: buscando cliente: %v", reserva.ID, err)
			return
		}

		// Si el envío falla (Mailpit caído, red, lo que sea), la única
		// consecuencia es que este correo puntual no llega — lo dejamos
		// registrado en el log del servidor para poder investigarlo, pero
		// la reserva sigue existiendo y confirmada igual. No hay ningún
		// canal ni ningún lugar al que "devolverle" este error: quien
		// llamó a CrearReserva ya recibió su respuesta hace rato, y no
		// tiene sentido reintentar automáticamente acá (para eso, un
		// sistema real usaría una cola de trabajos con reintentos, fuera
		// del alcance de este proyecto).
		if err := s.notificador.EnviarConfirmacionReserva(ctx, reserva, cliente, servicio); err != nil {
			log.Printf("no se pudo enviar el correo de confirmación de la reserva %s: %v", reserva.ID, err)
		}
	}()
}

// diaCompletoBloqueado indica si alguno de los bloqueos dados cubre el
// día completo de "fecha" (HoraDesde nil). Se usa solo en el camino sin
// horario configurado (ver CrearReserva): cuando sí hay horario, esta
// misma verificación ya está adentro de dominio.CalcularSlotsDisponibles.
func diaCompletoBloqueado(bloqueos []entidades.DiaBloqueado, fecha time.Time) bool {
	fy, fm, fd := fecha.Date()
	for _, b := range bloqueos {
		by, bm, bd := b.Fecha.Date()
		if by == fy && bm == fm && bd == fd && b.HoraDesde == nil {
			return true
		}
	}
	return false
}

func hayCoincidenciaDisponible(slots []dominio.Slot, inicio time.Time) bool {
	for _, slot := range slots {
		if slot.Inicio.Equal(inicio) {
			return slot.Disponible
		}
	}
	return false
}

// ParametrosCancelarReserva agrupa los datos necesarios para cancelar una
// reserva.
type ParametrosCancelarReserva struct {
	ReservaID      entidades.ID
	SolicitanteID  entidades.ID
	RolSolicitante entidades.Rol
}

// CancelarReserva cancela una reserva, aplicando las reglas de negocio:
// un cliente solo puede cancelar SU PROPIA reserva, y solo hasta 2 horas
// antes del turno; un administrador puede cancelar cualquier reserva, en
// cualquier momento (es su negocio, y puede necesitar reorganizar la
// agenda sin las restricciones que le aplican a un cliente).
func (s *ServicioReservas) CancelarReserva(ctx context.Context, params ParametrosCancelarReserva) error {
	reserva, err := s.reservas.BuscarPorID(ctx, params.ReservaID)
	if err != nil {
		return err
	}

	if reserva.Estado == entidades.ReservaCancelada {
		return dominio.ErrReservaYaCancelada
	}

	ahora := s.reloj.Ahora()

	if params.RolSolicitante == entidades.RolCliente {
		if reserva.ClienteID != params.SolicitanteID {
			return dominio.ErrNoAutorizado
		}
		if reserva.Inicio.Sub(ahora) < 2*time.Hour {
			return dominio.ErrCancelacionFueraDePlazo
		}
	}

	if err := s.reservas.Cancelar(ctx, params.ReservaID, ahora); err != nil {
		return err
	}

	servicio, err := s.servicios.BuscarPorID(ctx, reserva.ServicioID)
	if err != nil {
		log.Printf("no se pudo notificar la cancelación de la reserva %s: buscando servicio: %v", reserva.ID, err)
		return nil
	}
	s.notificarCancelacionAsync(reserva, servicio)

	return nil
}

// notificarCancelacionAsync manda el correo de cancelación en segundo
// plano. Mismo patrón que notificarConfirmacionAsync: contexto propio,
// nunca falla la operación principal por esto.
func (s *ServicioReservas) notificarCancelacionAsync(reserva entidades.Reserva, servicio entidades.Servicio) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		cliente, err := s.usuarios.BuscarPorID(ctx, reserva.ClienteID)
		if err != nil {
			log.Printf("no se pudo notificar la cancelación de la reserva %s: buscando cliente: %v", reserva.ID, err)
			return
		}
		if err := s.notificador.EnviarCancelacionReserva(ctx, reserva, cliente, servicio); err != nil {
			log.Printf("no se pudo enviar el correo de cancelación de la reserva %s: %v", reserva.ID, err)
		}
	}()
}

// ListarReservas devuelve reservas para la vista de administración,
// aplicando los filtros dados.
func (s *ServicioReservas) ListarReservas(ctx context.Context, filtros puertos.FiltrosReservas) ([]entidades.Reserva, error) {
	return s.reservas.ListarConFiltros(ctx, filtros)
}

// ReservaConCliente es una Reserva acompañada de los datos del cliente
// que la hizo — pensada para la vista de administración, donde ver un
// cliente_id (un uuid) no le sirve de nada al barbero: necesita el
// nombre y el email de la persona que tiene que atender.
type ReservaConCliente struct {
	Reserva       entidades.Reserva
	ClienteNombre string
	ClienteEmail  string
}

// ListarReservasConCliente es como ListarReservas, pero resuelve además
// los datos de cada cliente. Usa un cache local (un map) para no volver
// a buscar el mismo cliente dos veces si tiene varias reservas en el
// resultado — con muchas reservas del mismo cliente esto evita pedidos
// repetidos a la base por algo que ya sabíamos.
func (s *ServicioReservas) ListarReservasConCliente(ctx context.Context, filtros puertos.FiltrosReservas) ([]ReservaConCliente, error) {
	reservas, err := s.reservas.ListarConFiltros(ctx, filtros)
	if err != nil {
		return nil, err
	}

	clientesVistos := make(map[entidades.ID]entidades.Usuario, len(reservas))
	resultado := make([]ReservaConCliente, 0, len(reservas))

	for _, r := range reservas {
		cliente, yaVisto := clientesVistos[r.ClienteID]
		if !yaVisto {
			cliente, err = s.usuarios.BuscarPorID(ctx, r.ClienteID)
			if err != nil {
				return nil, fmt.Errorf("buscando cliente de la reserva %s: %w", r.ID, err)
			}
			clientesVistos[r.ClienteID] = cliente
		}
		resultado = append(resultado, ReservaConCliente{
			Reserva:       r,
			ClienteNombre: cliente.Nombre,
			ClienteEmail:  cliente.Email,
		})
	}

	return resultado, nil
}
