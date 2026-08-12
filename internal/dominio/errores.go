// Package dominio contiene las reglas de negocio que combinan varias
// entidades: el cálculo de disponibilidad y los errores de negocio que
// las capas superiores necesitan reconocer. Solo importa la librería
// estándar de Go y sus propios subpaquetes (entidades, puertos) — nunca
// una librería de terceros ni nada de infraestructura.
package dominio

import "errors"

// Errores de negocio que las capas superiores (casos de uso en Fase 5,
// handlers HTTP) necesitan poder reconocer con errors.Is, para reaccionar
// distinto según cuál haya ocurrido — por ejemplo, un handler HTTP
// devolvería 409 Conflict para ErrSlotNoDisponible pero 422 Unprocessable
// Entity para ErrCancelacionFueraDePlazo.
//
// Son distintos de los errores que devuelven los constructores en
// entidades/ (esos son errores de validación de datos de entrada, con un
// mensaje descriptivo — a quien los recibe normalmente le alcanza con
// saber "hubo un error", no necesita distinguir cuál). Estos, en cambio,
// representan reglas de negocio puntuales que si o si hay que poder
// identificar programáticamente.
var (
	// ErrSlotNoDisponible: se intentó reservar un slot que ya está
	// ocupado por otra reserva confirmada, o que ya empezó / ya pasó.
	ErrSlotNoDisponible = errors.New("el slot solicitado no está disponible")

	// ErrCancelacionFueraDePlazo: se intentó cancelar una reserva con
	// menos de 2 horas de anticipación (ver docs/CONTEXTO.md).
	ErrCancelacionFueraDePlazo = errors.New("no se puede cancelar con menos de 2 horas de anticipación")

	// ErrReservaYaCancelada: se intentó cancelar una reserva que ya
	// estaba cancelada.
	ErrReservaYaCancelada = errors.New("la reserva ya está cancelada")

	// ErrDiaBloqueadoConReservas: se intentó bloquear un día (o un rango
	// de horas) que ya tiene reservas confirmadas en ese rango.
	ErrDiaBloqueadoConReservas = errors.New("no se puede bloquear: ya hay reservas confirmadas en ese rango")

	// ErrNoEncontrado: se buscó una entidad (usuario, servicio, reserva...)
	// por ID y no existe. Genérico a propósito — el mensaje de más
	// contexto (qué tipo de entidad, con qué ID) lo agrega quien envuelve
	// este error con fmt.Errorf("...: %w", dominio.ErrNoEncontrado).
	ErrNoEncontrado = errors.New("no encontrado")

	// ErrCredencialesInvalidas: el login falló, sea porque el email no
	// existe o porque la contraseña no coincide. Se usa el mismo error
	// para los dos casos a propósito: si devolviéramos un error distinto
	// para "el email no existe" que para "la contraseña es incorrecta",
	// cualquiera podría usar el login para averiguar qué emails están
	// registrados en el sistema, probando de a uno.
	ErrCredencialesInvalidas = errors.New("email o contraseña incorrectos")

	// ErrEmailYaRegistrado: se intentó registrar un usuario con un email
	// que ya existe.
	ErrEmailYaRegistrado = errors.New("ese email ya está registrado")

	// ErrNoAutorizado: el usuario autenticado no tiene permiso para
	// realizar esta acción puntual sobre este recurso puntual (distinto
	// de un rol incorrecto, que ya lo filtra el middleware de
	// autorización de la Fase 4 — esto es, por ejemplo, "sos un cliente
	// válido, pero esta reserva no es tuya").
	ErrNoAutorizado = errors.New("no autorizado para realizar esta acción")

	// ErrFechaFueraDeAnticipacion: un cliente pidió slots o una reserva
	// para una fecha fuera de la ventana permitida (hoy o mañana, ver
	// docs/CONTEXTO.md). No aplica a reservas creadas por el
	// administrador.
	ErrFechaFueraDeAnticipacion = errors.New("la fecha solicitada está fuera del rango permitido de anticipación")
)
