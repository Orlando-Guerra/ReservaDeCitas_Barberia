package dto

import (
	"fmt"

	"reservas-go/internal/dominio/entidades"
)

// DefinirHorarioRequest es el cuerpo de POST /admin/horarios.
type DefinirHorarioRequest struct {
	DiaSemana  int    `json:"dia_semana"`  // 0 = domingo ... 6 = sábado (igual que time.Weekday)
	HoraInicio string `json:"hora_inicio"` // "HH:MM"
	HoraFin    string `json:"hora_fin"`    // "HH:MM"
}

// Validar chequea la forma de los datos.
func (r DefinirHorarioRequest) Validar() error {
	if r.DiaSemana < 0 || r.DiaSemana > 6 {
		return fmt.Errorf("dia_semana debe estar entre 0 (domingo) y 6 (sábado)")
	}
	if _, err := ParsearHoraDelDia(r.HoraInicio); err != nil {
		return fmt.Errorf("hora_inicio inválida: %w", err)
	}
	if _, err := ParsearHoraDelDia(r.HoraFin); err != nil {
		return fmt.Errorf("hora_fin inválida: %w", err)
	}
	return nil
}

// HorarioResponse es cómo se ve un HorarioAtencion hacia afuera.
type HorarioResponse struct {
	ID         string `json:"id"`
	DiaSemana  int    `json:"dia_semana"`
	HoraInicio string `json:"hora_inicio"`
	HoraFin    string `json:"hora_fin"`
}

// NuevoHorarioResponse convierte un entidades.HorarioAtencion en su
// representación pública.
func NuevoHorarioResponse(h entidades.HorarioAtencion) HorarioResponse {
	return HorarioResponse{
		ID:         string(h.ID),
		DiaSemana:  int(h.DiaSemana),
		HoraInicio: h.HoraInicio.String(),
		HoraFin:    h.HoraFin.String(),
	}
}
