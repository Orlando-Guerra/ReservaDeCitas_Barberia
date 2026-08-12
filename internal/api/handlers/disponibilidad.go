package handlers

import (
	"net/http"
	"time"

	"reservas-go/internal/api/dto"
	"reservas-go/internal/api/middleware"
	"reservas-go/internal/aplicacion"
	"reservas-go/internal/dominio/entidades"
)

// HandlerDisponibilidad expone la consulta de slots.
type HandlerDisponibilidad struct {
	servicio *aplicacion.ServicioDisponibilidad
}

// NuevoHandlerDisponibilidad crea un HandlerDisponibilidad.
func NuevoHandlerDisponibilidad(servicio *aplicacion.ServicioDisponibilidad) *HandlerDisponibilidad {
	return &HandlerDisponibilidad{servicio: servicio}
}

// ConsultarSlots atiende GET /slots?fecha=YYYY-MM-DD&servicio_id=....
func (h *HandlerDisponibilidad) ConsultarSlots(w http.ResponseWriter, r *http.Request) {
	usuario, ok := middleware.UsuarioDesdeContexto(r.Context())
	if !ok {
		responderError(w, http.StatusUnauthorized, "no autenticado")
		return
	}

	fechaStr := r.URL.Query().Get("fecha")
	servicioID := r.URL.Query().Get("servicio_id")
	if fechaStr == "" || servicioID == "" {
		responderError(w, http.StatusBadRequest, "los parámetros 'fecha' (YYYY-MM-DD) y 'servicio_id' son requeridos")
		return
	}

	fecha, err := time.Parse("2006-01-02", fechaStr)
	if err != nil {
		responderError(w, http.StatusBadRequest, "fecha inválida, formato esperado YYYY-MM-DD")
		return
	}

	slots, err := h.servicio.ConsultarSlots(r.Context(), fecha, entidades.ID(servicioID), usuario.Rol)
	if err != nil {
		manejarError(w, err)
		return
	}

	respuestas := make([]dto.SlotResponse, len(slots))
	for i, s := range slots {
		respuestas[i] = dto.NuevoSlotResponse(s)
	}
	responderJSON(w, http.StatusOK, respuestas)
}
