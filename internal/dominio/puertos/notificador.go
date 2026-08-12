package puertos

import (
	"context"

	"reservas-go/internal/dominio/entidades"
)

// Notificador abstrae el envío de notificaciones al cliente (hoy, por
// correo electrónico). La infraestructura implementa este puerto contra
// Mailpit en desarrollo (Fase 6); el día de mañana podría implementarse
// contra Resend o SendGrid sin que el dominio ni los casos de uso se
// enteren del cambio.
type Notificador interface {
	// EnviarConfirmacionReserva notifica al cliente que su reserva quedó
	// confirmada.
	EnviarConfirmacionReserva(ctx context.Context, reserva entidades.Reserva, cliente entidades.Usuario, servicio entidades.Servicio) error

	// EnviarCancelacionReserva notifica al cliente que su reserva fue
	// cancelada.
	EnviarCancelacionReserva(ctx context.Context, reserva entidades.Reserva, cliente entidades.Usuario, servicio entidades.Servicio) error
}
