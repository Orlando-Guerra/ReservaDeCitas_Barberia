package aplicacion_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"reservas-go/internal/aplicacion"
	"reservas-go/internal/dominio"
	"reservas-go/internal/dominio/entidades"
)

// entorno agrupa todas las dependencias falsas de un test de
// ServicioReservas, para no repetir el armado en cada test.
type entorno struct {
	usuarios         *repositorioUsuariosMemoria
	servicios        *repositorioServiciosMemoria
	horarios         *repositorioHorariosMemoria
	diasBloqueados   *repositorioDiasBloqueadosMemoria
	reservas         *repositorioReservasMemoria
	notificador      *notificadorFalso
	reloj            *relojFijo
	servicioReservas *aplicacion.ServicioReservas
}

// lunes10a12 arma un entorno con: un cliente, un servicio, horario de
// lunes 10:00-12:00 (2 slots), y "ahora" fijado el domingo anterior (así
// los dos slots del lunes quedan en el futuro). Cada test parte de acá y
// ajusta lo que necesite.
func nuevoEntornoDePrueba(t *testing.T) (*entorno, entidades.Usuario, entidades.Servicio) {
	t.Helper()

	e := &entorno{
		usuarios:       nuevoRepositorioUsuariosMemoria(),
		servicios:      nuevoRepositorioServiciosMemoria(),
		horarios:       nuevoRepositorioHorariosMemoria(),
		diasBloqueados: &repositorioDiasBloqueadosMemoria{},
		reservas:       nuevoRepositorioReservasMemoria(),
		notificador:    nuevoNotificadorFalso(),
		reloj:          &relojFijo{momento: time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)}, // domingo
	}
	e.servicioReservas = aplicacion.NuevoServicioReservas(
		e.usuarios, e.servicios, e.horarios, e.diasBloqueados, e.reservas, e.notificador, e.reloj,
	)

	ctx := context.Background()

	cliente, err := entidades.NuevoUsuario("Cliente de prueba", "cliente@ejemplo.com", "hash", entidades.RolCliente, e.reloj.momento)
	if err != nil {
		t.Fatalf("no se esperaba error creando el cliente: %v", err)
	}
	if err := e.usuarios.Guardar(ctx, cliente); err != nil {
		t.Fatalf("no se esperaba error guardando el cliente: %v", err)
	}

	servicio, err := entidades.NuevoServicio("Corte", 30, 100000)
	if err != nil {
		t.Fatalf("no se esperaba error creando el servicio: %v", err)
	}
	if err := e.servicios.Guardar(ctx, servicio); err != nil {
		t.Fatalf("no se esperaba error guardando el servicio: %v", err)
	}

	horaInicio, _ := entidades.NuevaHoraDelDia(10, 0)
	horaFin, _ := entidades.NuevaHoraDelDia(12, 0)
	horario, err := entidades.NuevoHorarioAtencion(time.Monday, horaInicio, horaFin)
	if err != nil {
		t.Fatalf("no se esperaba error creando el horario: %v", err)
	}
	if err := e.horarios.Guardar(ctx, horario); err != nil {
		t.Fatalf("no se esperaba error guardando el horario: %v", err)
	}

	return e, cliente, servicio
}

// lunes10 es el primer slot disponible en el entorno de prueba.
var lunes10 = time.Date(2026, 8, 17, 10, 0, 0, 0, time.UTC)

func TestServicioReservas_CrearReserva(t *testing.T) {
	t.Run("con datos válidos, crea la reserva y dispara la notificación async", func(t *testing.T) {
		e, cliente, servicio := nuevoEntornoDePrueba(t)

		reserva, err := e.servicioReservas.CrearReserva(context.Background(), aplicacion.ParametrosCrearReserva{
			ClienteID:      cliente.ID,
			ServicioID:     servicio.ID,
			Inicio:         lunes10,
			RolSolicitante: entidades.RolCliente,
		})
		if err != nil {
			t.Fatalf("no se esperaba error: %v", err)
		}
		if reserva.Estado != entidades.ReservaConfirmada {
			t.Errorf("Estado = %q, se esperaba %q", reserva.Estado, entidades.ReservaConfirmada)
		}

		// La notificación se manda en una goroutine aparte (Fase 6): la
		// leemos del canal falso con un timeout, en vez de revisar un
		// campo directamente — si no llega en 1 segundo, algo está mal.
		select {
		case confirmada := <-e.notificador.confirmaciones:
			if confirmada.ID != reserva.ID {
				t.Errorf("se notificó la reserva %s, se esperaba %s", confirmada.ID, reserva.ID)
			}
		case <-time.After(time.Second):
			t.Fatal("no se recibió la notificación de confirmación a tiempo")
		}
	})

	t.Run("sin horario ese día, devuelve ErrSlotNoDisponible", func(t *testing.T) {
		e, cliente, servicio := nuevoEntornoDePrueba(t)
		martes := lunes10.AddDate(0, 0, 1) // el entorno de prueba solo configura horario los lunes

		// Usamos RolAdministrador acá a propósito: queremos aislar la
		// regla "no hay horario ese día" de la regla de anticipación del
		// cliente (que ya se prueba en el caso de abajo) — el martes cae
		// fuera de la ventana "hoy o mañana" de un cliente, así que con
		// RolCliente el test estaría probando otra cosa sin darse cuenta.
		_, err := e.servicioReservas.CrearReserva(context.Background(), aplicacion.ParametrosCrearReserva{
			ClienteID:      cliente.ID,
			ServicioID:     servicio.ID,
			Inicio:         martes,
			RolSolicitante: entidades.RolAdministrador,
		})
		if !errors.Is(err, dominio.ErrSlotNoDisponible) {
			t.Errorf("error = %v, se esperaba dominio.ErrSlotNoDisponible", err)
		}
	})

	t.Run("un cliente no puede reservar más allá de mañana", func(t *testing.T) {
		e, cliente, servicio := nuevoEntornoDePrueba(t)
		// "ahora" es domingo, así que la ventana del cliente es
		// domingo/lunes. El lunes de la semana SIGUIENTE (+7 días) ya
		// excede ese límite.
		lunesQueViene := lunes10.AddDate(0, 0, 7)

		_, err := e.servicioReservas.CrearReserva(context.Background(), aplicacion.ParametrosCrearReserva{
			ClienteID:      cliente.ID,
			ServicioID:     servicio.ID,
			Inicio:         lunesQueViene,
			RolSolicitante: entidades.RolCliente,
		})
		if !errors.Is(err, dominio.ErrFechaFueraDeAnticipacion) {
			t.Errorf("error = %v, se esperaba dominio.ErrFechaFueraDeAnticipacion", err)
		}
	})

	t.Run("el administrador puede reservar más allá de mañana (walk-in)", func(t *testing.T) {
		e, cliente, servicio := nuevoEntornoDePrueba(t)
		lunesQueViene := lunes10.AddDate(0, 0, 7)

		_, err := e.servicioReservas.CrearReserva(context.Background(), aplicacion.ParametrosCrearReserva{
			ClienteID:      cliente.ID,
			ServicioID:     servicio.ID,
			Inicio:         lunesQueViene,
			RolSolicitante: entidades.RolAdministrador,
		})
		if err != nil {
			t.Errorf("no se esperaba error para una reserva de administrador: %v", err)
		}
	})

	t.Run("un slot ya ocupado devuelve ErrSlotNoDisponible", func(t *testing.T) {
		e, cliente, servicio := nuevoEntornoDePrueba(t)
		ctx := context.Background()
		params := aplicacion.ParametrosCrearReserva{
			ClienteID:      cliente.ID,
			ServicioID:     servicio.ID,
			Inicio:         lunes10,
			RolSolicitante: entidades.RolCliente,
		}

		if _, err := e.servicioReservas.CrearReserva(ctx, params); err != nil {
			t.Fatalf("no se esperaba error en la primera reserva: %v", err)
		}
		<-e.notificador.confirmaciones // drenamos la notificación para no dejar la goroutine escribiendo a un canal que nadie lee

		_, err := e.servicioReservas.CrearReserva(ctx, params)
		if !errors.Is(err, dominio.ErrSlotNoDisponible) {
			t.Errorf("error = %v, se esperaba dominio.ErrSlotNoDisponible", err)
		}
	})
}

func TestServicioReservas_CancelarReserva(t *testing.T) {
	crearReservaDePrueba := func(t *testing.T, e *entorno, cliente entidades.Usuario, servicio entidades.Servicio, inicio time.Time) entidades.Reserva {
		t.Helper()
		reserva, err := e.servicioReservas.CrearReserva(context.Background(), aplicacion.ParametrosCrearReserva{
			ClienteID:      cliente.ID,
			ServicioID:     servicio.ID,
			Inicio:         inicio,
			RolSolicitante: entidades.RolCliente,
		})
		if err != nil {
			t.Fatalf("no se esperaba error creando la reserva de prueba: %v", err)
		}
		<-e.notificador.confirmaciones
		return reserva
	}

	t.Run("el dueño puede cancelar con más de 2 horas de anticipación", func(t *testing.T) {
		e, cliente, servicio := nuevoEntornoDePrueba(t)
		reserva := crearReservaDePrueba(t, e, cliente, servicio, lunes10)

		err := e.servicioReservas.CancelarReserva(context.Background(), aplicacion.ParametrosCancelarReserva{
			ReservaID:      reserva.ID,
			SolicitanteID:  cliente.ID,
			RolSolicitante: entidades.RolCliente,
		})
		if err != nil {
			t.Fatalf("no se esperaba error: %v", err)
		}

		select {
		case cancelada := <-e.notificador.cancelaciones:
			if cancelada.ID != reserva.ID {
				t.Errorf("se notificó la reserva %s, se esperaba %s", cancelada.ID, reserva.ID)
			}
		case <-time.After(time.Second):
			t.Fatal("no se recibió la notificación de cancelación a tiempo")
		}
	})

	t.Run("con menos de 2 horas de anticipación, un cliente no puede cancelar", func(t *testing.T) {
		e, cliente, servicio := nuevoEntornoDePrueba(t)
		reserva := crearReservaDePrueba(t, e, cliente, servicio, lunes10)
		e.reloj.momento = lunes10.Add(-1 * time.Hour) // a 1 hora del turno

		err := e.servicioReservas.CancelarReserva(context.Background(), aplicacion.ParametrosCancelarReserva{
			ReservaID:      reserva.ID,
			SolicitanteID:  cliente.ID,
			RolSolicitante: entidades.RolCliente,
		})
		if !errors.Is(err, dominio.ErrCancelacionFueraDePlazo) {
			t.Errorf("error = %v, se esperaba dominio.ErrCancelacionFueraDePlazo", err)
		}
	})

	t.Run("un cliente no puede cancelar la reserva de otro", func(t *testing.T) {
		e, cliente, servicio := nuevoEntornoDePrueba(t)
		reserva := crearReservaDePrueba(t, e, cliente, servicio, lunes10)

		err := e.servicioReservas.CancelarReserva(context.Background(), aplicacion.ParametrosCancelarReserva{
			ReservaID:      reserva.ID,
			SolicitanteID:  "otro-cliente",
			RolSolicitante: entidades.RolCliente,
		})
		if !errors.Is(err, dominio.ErrNoAutorizado) {
			t.Errorf("error = %v, se esperaba dominio.ErrNoAutorizado", err)
		}
	})

	t.Run("el administrador puede cancelar sin restricción de plazo ni de dueño", func(t *testing.T) {
		e, cliente, servicio := nuevoEntornoDePrueba(t)
		reserva := crearReservaDePrueba(t, e, cliente, servicio, lunes10)
		e.reloj.momento = lunes10.Add(-1 * time.Minute) // muy poco margen, no le importa al admin

		err := e.servicioReservas.CancelarReserva(context.Background(), aplicacion.ParametrosCancelarReserva{
			ReservaID:      reserva.ID,
			SolicitanteID:  "admin-1",
			RolSolicitante: entidades.RolAdministrador,
		})
		if err != nil {
			t.Errorf("no se esperaba error para una cancelación de administrador: %v", err)
		}
	})

	t.Run("cancelar una reserva ya cancelada devuelve ErrReservaYaCancelada", func(t *testing.T) {
		e, cliente, servicio := nuevoEntornoDePrueba(t)
		reserva := crearReservaDePrueba(t, e, cliente, servicio, lunes10)

		params := aplicacion.ParametrosCancelarReserva{
			ReservaID:      reserva.ID,
			SolicitanteID:  cliente.ID,
			RolSolicitante: entidades.RolCliente,
		}
		if err := e.servicioReservas.CancelarReserva(context.Background(), params); err != nil {
			t.Fatalf("no se esperaba error en la primera cancelación: %v", err)
		}
		<-e.notificador.cancelaciones

		err := e.servicioReservas.CancelarReserva(context.Background(), params)
		if !errors.Is(err, dominio.ErrReservaYaCancelada) {
			t.Errorf("error = %v, se esperaba dominio.ErrReservaYaCancelada", err)
		}
	})
}
