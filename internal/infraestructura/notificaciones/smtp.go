// Package notificaciones implementa el puerto puertos.Notificador contra
// un servidor SMTP — en desarrollo, Mailpit (ver docker-compose.yml). No
// manda correos reales fuera de este entorno: Mailpit los intercepta y
// los muestra en su interfaz web (http://localhost:8025) en vez de
// entregarlos de verdad. Cambiar a un proveedor real (Resend, SendGrid)
// el día de mañana implica escribir OTRO adaptador que implemente el
// mismo puerto — el resto del sistema no se entera del cambio.
package notificaciones

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	"html/template"
	"mime"
	"net"
	"net/smtp"
	"time"

	"reservas-go/internal/dominio/entidades"
)

//go:embed plantillas/*.html
var plantillasFS embed.FS

// NotificadorSMTP implementa puertos.Notificador enviando correos HTML
// por SMTP sin autenticación (así funciona Mailpit).
type NotificadorSMTP struct {
	host            string
	port            string
	from            string
	zonaLocal       *time.Location
	tplConfirmacion *template.Template
	tplCancelacion  *template.Template
}

// NuevoNotificadorSMTP crea un NotificadorSMTP. "zonaHoraria" es la zona
// horaria del negocio (ej. "America/Argentina/Buenos_Aires"): todo el
// resto del sistema trabaja en UTC (ver docs/CONCURRENCIA.md); acá, y
// solo acá, convertimos a la hora local del negocio, porque es
// exactamente el punto donde el dato deja de ser interno y pasa a
// mostrársele a una persona.
func NuevoNotificadorSMTP(host, port, from, zonaHoraria string) (*NotificadorSMTP, error) {
	zona, err := time.LoadLocation(zonaHoraria)
	if err != nil {
		return nil, fmt.Errorf("cargando zona horaria %q: %w", zonaHoraria, err)
	}

	tplConfirmacion, err := template.ParseFS(plantillasFS, "plantillas/confirmacion.html")
	if err != nil {
		return nil, fmt.Errorf("parseando plantilla de confirmación: %w", err)
	}
	tplCancelacion, err := template.ParseFS(plantillasFS, "plantillas/cancelacion.html")
	if err != nil {
		return nil, fmt.Errorf("parseando plantilla de cancelación: %w", err)
	}

	return &NotificadorSMTP{
		host:            host,
		port:            port,
		from:            from,
		zonaLocal:       zona,
		tplConfirmacion: tplConfirmacion,
		tplCancelacion:  tplCancelacion,
	}, nil
}

// datosCorreo son los valores que las plantillas HTML pueden usar con
// {{.Campo}}.
type datosCorreo struct {
	NombreCliente    string
	NombreServicio   string
	Fecha            string
	Hora             string
	PrecioFormateado string
}

func (n *NotificadorSMTP) datosDesde(reserva entidades.Reserva, cliente entidades.Usuario, servicio entidades.Servicio) datosCorreo {
	inicioLocal := reserva.Inicio.In(n.zonaLocal)
	return datosCorreo{
		NombreCliente:    cliente.Nombre,
		NombreServicio:   servicio.Nombre,
		Fecha:            inicioLocal.Format("02/01/2006"),
		Hora:             inicioLocal.Format("15:04"),
		PrecioFormateado: formatearCentavos(servicio.PrecioCentavos),
	}
}

func formatearCentavos(centavos int64) string {
	return fmt.Sprintf("$%d.%02d", centavos/100, centavos%100)
}

// EnviarConfirmacionReserva implementa puertos.Notificador.
func (n *NotificadorSMTP) EnviarConfirmacionReserva(ctx context.Context, reserva entidades.Reserva, cliente entidades.Usuario, servicio entidades.Servicio) error {
	var cuerpo bytes.Buffer
	if err := n.tplConfirmacion.Execute(&cuerpo, n.datosDesde(reserva, cliente, servicio)); err != nil {
		return fmt.Errorf("renderizando plantilla de confirmación: %w", err)
	}
	return n.enviar(cliente.Email, "Tu turno quedó confirmado", cuerpo.Bytes())
}

// EnviarCancelacionReserva implementa puertos.Notificador.
func (n *NotificadorSMTP) EnviarCancelacionReserva(ctx context.Context, reserva entidades.Reserva, cliente entidades.Usuario, servicio entidades.Servicio) error {
	var cuerpo bytes.Buffer
	if err := n.tplCancelacion.Execute(&cuerpo, n.datosDesde(reserva, cliente, servicio)); err != nil {
		return fmt.Errorf("renderizando plantilla de cancelación: %w", err)
	}
	return n.enviar(cliente.Email, "Tu turno fue cancelado", cuerpo.Bytes())
}

// enviar arma un mensaje de correo mínimo (headers + cuerpo HTML) y lo
// manda con la función de la librería estándar net/smtp.SendMail.
//
// Limitación conocida: smtp.SendMail no acepta un context.Context (es
// una función más vieja que ese patrón en la librería estándar), así que
// "ctx" en los métodos de arriba no cancela este envío en particular.
// Contra un servidor local como Mailpit esto no es un problema práctico
// — si mañana esto hablara con un proveedor real por internet, valdría
// la pena reemplazar esto por una conexión SMTP armada a mano con
// net.DialContext para poder respetar cancelación y timeouts.
func (n *NotificadorSMTP) enviar(destinatario, asunto string, cuerpoHTML []byte) error {
	// Los headers de un correo (RFC 5322) solo pueden tener texto ASCII.
	// Como nuestros asuntos tienen tildes ("Tu turno quedó confirmado"),
	// hay que codificarlos con "encoded-words" (RFC 2047) — mime.QEncoding
	// hace exactamente eso, produciendo algo como
	// "=?utf-8?q?Tu_turno_quedó...?=" en ASCII puro. Sin este paso, la
	// primera prueba real contra Mailpit mostró el asunto corrompido.
	asuntoCodificado := mime.QEncoding.Encode("UTF-8", asunto)

	var mensaje bytes.Buffer
	mensaje.WriteString(fmt.Sprintf("From: %s\r\n", n.from))
	mensaje.WriteString(fmt.Sprintf("To: %s\r\n", destinatario))
	mensaje.WriteString(fmt.Sprintf("Subject: %s\r\n", asuntoCodificado))
	mensaje.WriteString("MIME-Version: 1.0\r\n")
	mensaje.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
	// El cuerpo sí es UTF-8 crudo (8 bits por caracter con tilde), así que
	// hay que declararlo: sin este header, el valor por defecto es
	// "7bit", que promete bytes de 7 bits — una promesa que nuestro
	// cuerpo no cumple, y por eso también salía corrompido.
	mensaje.WriteString("Content-Transfer-Encoding: 8bit\r\n")
	mensaje.WriteString("\r\n")
	mensaje.Write(cuerpoHTML)

	addr := net.JoinHostPort(n.host, n.port)
	// El segundo parámetro (smtp.Auth) va nil: Mailpit no pide
	// autenticación. Contra un proveedor real, acá iría smtp.PlainAuth
	// con usuario/contraseña.
	if err := smtp.SendMail(addr, nil, n.from, []string{destinatario}, mensaje.Bytes()); err != nil {
		return fmt.Errorf("enviando correo a %s: %w", destinatario, err)
	}
	return nil
}
