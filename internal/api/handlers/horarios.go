package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"reservas-go/internal/api/dto"
	"reservas-go/internal/aplicacion"
)

// HandlerHorarios expone el CRUD de horarios de atención (admin-only).
type HandlerHorarios struct {
	servicio *aplicacion.ServicioAdministracion
}

// NuevoHandlerHorarios crea un HandlerHorarios.
func NuevoHandlerHorarios(servicio *aplicacion.ServicioAdministracion) *HandlerHorarios {
	return &HandlerHorarios{servicio: servicio}
}

// Definir atiende POST /admin/horarios (crea o actualiza el horario de
// un día de la semana).
func (h *HandlerHorarios) Definir(w http.ResponseWriter, r *http.Request) {
	var req dto.DefinirHorarioRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		responderError(w, http.StatusBadRequest, "cuerpo del pedido inválido")
		return
	}
	if err := req.Validar(); err != nil {
		responderError(w, http.StatusBadRequest, err.Error())
		return
	}

	horaInicio, _ := dto.ParsearHoraDelDia(req.HoraInicio) // ya validado en req.Validar()
	horaFin, _ := dto.ParsearHoraDelDia(req.HoraFin)

	horario, err := h.servicio.DefinirHorario(r.Context(), time.Weekday(req.DiaSemana), horaInicio, horaFin)
	if err != nil {
		manejarError(w, err)
		return
	}
	responderJSON(w, http.StatusOK, dto.NuevoHorarioResponse(horario))
}

// Listar atiende GET /admin/horarios.
func (h *HandlerHorarios) Listar(w http.ResponseWriter, r *http.Request) {
	horarios, err := h.servicio.ListarHorarios(r.Context())
	if err != nil {
		manejarError(w, err)
		return
	}

	respuestas := make([]dto.HorarioResponse, len(horarios))
	for i, hor := range horarios {
		respuestas[i] = dto.NuevoHorarioResponse(hor)
	}
	responderJSON(w, http.StatusOK, respuestas)
}
