package dto

import (
	"fmt"
	"strings"
	"time"

	"reservas-go/internal/dominio/entidades"
)

// CrearBloqueoRequest es el cuerpo de POST /admin/dias-bloqueados.
type CrearBloqueoRequest struct {
	Fecha     string  `json:"fecha"`      // "YYYY-MM-DD"
	HoraDesde *string `json:"hora_desde"` // "HH:MM", opcional: ausente = bloquea el día completo
	Motivo    string  `json:"motivo"`
}

// Validar chequea la forma de los datos.
func (r CrearBloqueoRequest) Validar() error {
	if _, err := time.Parse("2006-01-02", r.Fecha); err != nil {
		return fmt.Errorf("fecha inválida, formato esperado YYYY-MM-DD")
	}
	if r.HoraDesde != nil {
		if _, err := ParsearHoraDelDia(*r.HoraDesde); err != nil {
			return fmt.Errorf("hora_desde inválida: %w", err)
		}
	}
	if strings.TrimSpace(r.Motivo) == "" {
		return fmt.Errorf("el motivo es requerido")
	}
	return nil
}

// BloqueoResponse es cómo se ve un DiaBloqueado hacia afuera.
type BloqueoResponse struct {
	ID        string  `json:"id"`
	Fecha     string  `json:"fecha"`
	HoraDesde *string `json:"hora_desde,omitempty"`
	Motivo    string  `json:"motivo"`
}

// NuevoBloqueoResponse convierte un entidades.DiaBloqueado en su
// representación pública.
func NuevoBloqueoResponse(b entidades.DiaBloqueado) BloqueoResponse {
	resp := BloqueoResponse{
		ID:     string(b.ID),
		Fecha:  b.Fecha.Format("2006-01-02"),
		Motivo: b.Motivo,
	}
	if b.HoraDesde != nil {
		s := b.HoraDesde.String()
		resp.HoraDesde = &s
	}
	return resp
}
