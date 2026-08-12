package dto

import (
	"fmt"
	"strings"

	"reservas-go/internal/dominio/entidades"
)

// CrearServicioRequest es el cuerpo de POST /admin/servicios.
type CrearServicioRequest struct {
	Nombre          string `json:"nombre"`
	DuracionMinutos int    `json:"duracion_minutos"`
	PrecioCentavos  int64  `json:"precio_centavos"`
}

// Validar chequea la forma de los datos.
func (r CrearServicioRequest) Validar() error {
	if strings.TrimSpace(r.Nombre) == "" {
		return fmt.Errorf("el nombre es requerido")
	}
	if r.DuracionMinutos <= 0 {
		return fmt.Errorf("duracion_minutos debe ser mayor a 0")
	}
	if r.PrecioCentavos < 0 {
		return fmt.Errorf("precio_centavos no puede ser negativo")
	}
	return nil
}

// ActualizarServicioRequest es el cuerpo de PUT /admin/servicios/{id}.
type ActualizarServicioRequest struct {
	Nombre          string `json:"nombre"`
	DuracionMinutos int    `json:"duracion_minutos"`
	PrecioCentavos  int64  `json:"precio_centavos"`
	Activo          bool   `json:"activo"`
}

// Validar chequea la forma de los datos.
func (r ActualizarServicioRequest) Validar() error {
	if strings.TrimSpace(r.Nombre) == "" {
		return fmt.Errorf("el nombre es requerido")
	}
	if r.DuracionMinutos <= 0 {
		return fmt.Errorf("duracion_minutos debe ser mayor a 0")
	}
	if r.PrecioCentavos < 0 {
		return fmt.Errorf("precio_centavos no puede ser negativo")
	}
	return nil
}

// ServicioResponse es cómo se ve un Servicio hacia afuera.
type ServicioResponse struct {
	ID              string `json:"id"`
	Nombre          string `json:"nombre"`
	DuracionMinutos int    `json:"duracion_minutos"`
	PrecioCentavos  int64  `json:"precio_centavos"`
	Activo          bool   `json:"activo"`
}

// NuevoServicioResponse convierte un entidades.Servicio en su
// representación pública.
func NuevoServicioResponse(s entidades.Servicio) ServicioResponse {
	return ServicioResponse{
		ID:              string(s.ID),
		Nombre:          s.Nombre,
		DuracionMinutos: s.DuracionMinutos,
		PrecioCentavos:  s.PrecioCentavos,
		Activo:          s.Activo,
	}
}
