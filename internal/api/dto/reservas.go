package dto

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"reservas-go/internal/aplicacion"
	"reservas-go/internal/dominio"
	"reservas-go/internal/dominio/entidades"
	"reservas-go/internal/dominio/puertos"
)

// CrearReservaRequest es el cuerpo de POST /reservas — el cliente
// autenticado reserva un slot para sí mismo. No lleva "cliente_id": ese
// valor sale siempre del usuario autenticado (ver HandlerReservas.Crear),
// nunca de algo que mande el cliente en el pedido — si lo aceptáramos del
// cuerpo, cualquiera podría reservar "en nombre de" otro usuario con solo
// cambiar ese campo.
type CrearReservaRequest struct {
	ServicioID string `json:"servicio_id"`
	Inicio     string `json:"inicio"` // RFC3339, ej. "2026-08-17T10:00:00Z"
}

// Validar chequea la forma de los datos.
func (r CrearReservaRequest) Validar() error {
	if strings.TrimSpace(r.ServicioID) == "" {
		return fmt.Errorf("servicio_id es requerido")
	}
	if _, err := time.Parse(time.RFC3339, r.Inicio); err != nil {
		return fmt.Errorf("inicio inválido, formato esperado RFC3339 (ej. 2026-08-17T10:00:00Z)")
	}
	return nil
}

// CrearReservaAdminRequest es el cuerpo de POST /admin/reservas — el
// administrador crea una reserva manual para un cliente presencial. A
// diferencia de CrearReservaRequest, acá "cliente_id" sí es un campo del
// pedido, porque quien reserva (el admin) no es el cliente de la
// reserva.
type CrearReservaAdminRequest struct {
	ClienteID  string `json:"cliente_id"`
	ServicioID string `json:"servicio_id"`
	Inicio     string `json:"inicio"`
}

// Validar chequea la forma de los datos.
func (r CrearReservaAdminRequest) Validar() error {
	if strings.TrimSpace(r.ClienteID) == "" {
		return fmt.Errorf("cliente_id es requerido")
	}
	if strings.TrimSpace(r.ServicioID) == "" {
		return fmt.Errorf("servicio_id es requerido")
	}
	if _, err := time.Parse(time.RFC3339, r.Inicio); err != nil {
		return fmt.Errorf("inicio inválido, formato esperado RFC3339 (ej. 2026-08-17T10:00:00Z)")
	}
	return nil
}

// ReservaResponse es cómo se ve una Reserva hacia afuera.
type ReservaResponse struct {
	ID          string     `json:"id"`
	ClienteID   string     `json:"cliente_id"`
	ServicioID  string     `json:"servicio_id"`
	Inicio      time.Time  `json:"inicio"`
	Fin         time.Time  `json:"fin"`
	Estado      string     `json:"estado"`
	CanceladaEn *time.Time `json:"cancelada_en,omitempty"`
}

// NuevaReservaResponse convierte un entidades.Reserva en su
// representación pública.
func NuevaReservaResponse(r entidades.Reserva) ReservaResponse {
	return ReservaResponse{
		ID:          string(r.ID),
		ClienteID:   string(r.ClienteID),
		ServicioID:  string(r.ServicioID),
		Inicio:      r.Inicio,
		Fin:         r.Fin,
		Estado:      string(r.Estado),
		CanceladaEn: r.CanceladaEn,
	}
}

// ReservaAdminResponse es cómo ve una Reserva la vista de
// administración: además de los datos de la reserva, incluye el nombre y
// el email del cliente — un cliente_id (un uuid) no le sirve de nada al
// barbero para saber a quién tiene que atender.
type ReservaAdminResponse struct {
	ID            string     `json:"id"`
	ClienteID     string     `json:"cliente_id"`
	ClienteNombre string     `json:"cliente_nombre"`
	ClienteEmail  string     `json:"cliente_email"`
	ServicioID    string     `json:"servicio_id"`
	Inicio        time.Time  `json:"inicio"`
	Fin           time.Time  `json:"fin"`
	Estado        string     `json:"estado"`
	CanceladaEn   *time.Time `json:"cancelada_en,omitempty"`
}

// NuevaReservaAdminResponse convierte un aplicacion.ReservaConCliente en
// su representación pública para la vista de administración.
func NuevaReservaAdminResponse(rc aplicacion.ReservaConCliente) ReservaAdminResponse {
	r := rc.Reserva
	return ReservaAdminResponse{
		ID:            string(r.ID),
		ClienteID:     string(r.ClienteID),
		ClienteNombre: rc.ClienteNombre,
		ClienteEmail:  rc.ClienteEmail,
		ServicioID:    string(r.ServicioID),
		Inicio:        r.Inicio,
		Fin:           r.Fin,
		Estado:        string(r.Estado),
		CanceladaEn:   r.CanceladaEn,
	}
}

// SlotResponse es cómo se ve un dominio.Slot hacia afuera.
type SlotResponse struct {
	Inicio     time.Time `json:"inicio"`
	Fin        time.Time `json:"fin"`
	Disponible bool      `json:"disponible"`
}

// NuevoSlotResponse convierte un dominio.Slot en su representación
// pública.
func NuevoSlotResponse(s dominio.Slot) SlotResponse {
	return SlotResponse{Inicio: s.Inicio, Fin: s.Fin, Disponible: s.Disponible}
}

// FiltrosDesdeQuery interpreta los query params "desde", "hasta" y
// "estado" (todos opcionales) de GET /admin/reservas como
// puertos.FiltrosReservas.
func FiltrosDesdeQuery(q url.Values) (puertos.FiltrosReservas, error) {
	var filtros puertos.FiltrosReservas

	if desdeStr := q.Get("desde"); desdeStr != "" {
		desde, err := time.Parse("2006-01-02", desdeStr)
		if err != nil {
			return filtros, fmt.Errorf("desde inválido, formato esperado YYYY-MM-DD")
		}
		filtros.Desde = &desde
	}

	if hastaStr := q.Get("hasta"); hastaStr != "" {
		hasta, err := time.Parse("2006-01-02", hastaStr)
		if err != nil {
			return filtros, fmt.Errorf("hasta inválido, formato esperado YYYY-MM-DD")
		}
		hasta = hasta.AddDate(0, 0, 1) // "hasta" incluye el día completo
		filtros.Hasta = &hasta
	}

	if estadoStr := q.Get("estado"); estadoStr != "" {
		estado := entidades.EstadoReserva(estadoStr)
		if estado != entidades.ReservaConfirmada && estado != entidades.ReservaCancelada {
			return filtros, fmt.Errorf("estado inválido, debe ser 'confirmada' o 'cancelada'")
		}
		filtros.Estado = &estado
	}

	return filtros, nil
}
