package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"reservas-go/internal/api/dto"
	"reservas-go/internal/aplicacion"
	"reservas-go/internal/dominio/entidades"
)

// HandlerBloqueos expone el CRUD de días bloqueados (admin-only).
type HandlerBloqueos struct {
	servicio *aplicacion.ServicioAdministracion
}

// NuevoHandlerBloqueos crea un HandlerBloqueos.
func NuevoHandlerBloqueos(servicio *aplicacion.ServicioAdministracion) *HandlerBloqueos {
	return &HandlerBloqueos{servicio: servicio}
}

// Crear atiende POST /admin/dias-bloqueados.
func (h *HandlerBloqueos) Crear(w http.ResponseWriter, r *http.Request) {
	var req dto.CrearBloqueoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		responderError(w, http.StatusBadRequest, "cuerpo del pedido inválido")
		return
	}
	if err := req.Validar(); err != nil {
		responderError(w, http.StatusBadRequest, err.Error())
		return
	}

	fecha, _ := time.Parse("2006-01-02", req.Fecha) // ya validado en req.Validar()
	var horaDesde *entidades.HoraDelDia
	if req.HoraDesde != nil {
		h, _ := dto.ParsearHoraDelDia(*req.HoraDesde)
		horaDesde = &h
	}

	bloqueo, err := h.servicio.CrearBloqueo(r.Context(), fecha, horaDesde, req.Motivo)
	if err != nil {
		manejarError(w, err)
		return
	}
	responderJSON(w, http.StatusCreated, dto.NuevoBloqueoResponse(bloqueo))
}

// Eliminar atiende DELETE /admin/dias-bloqueados/{id}.
func (h *HandlerBloqueos) Eliminar(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.servicio.EliminarBloqueo(r.Context(), entidades.ID(id)); err != nil {
		manejarError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Listar atiende GET /admin/dias-bloqueados?desde=&hasta=.
func (h *HandlerBloqueos) Listar(w http.ResponseWriter, r *http.Request) {
	desdeStr := r.URL.Query().Get("desde")
	hastaStr := r.URL.Query().Get("hasta")
	if desdeStr == "" || hastaStr == "" {
		responderError(w, http.StatusBadRequest, "los parámetros 'desde' y 'hasta' (YYYY-MM-DD) son requeridos")
		return
	}

	desde, err := time.Parse("2006-01-02", desdeStr)
	if err != nil {
		responderError(w, http.StatusBadRequest, "desde inválido, formato esperado YYYY-MM-DD")
		return
	}
	hasta, err := time.Parse("2006-01-02", hastaStr)
	if err != nil {
		responderError(w, http.StatusBadRequest, "hasta inválido, formato esperado YYYY-MM-DD")
		return
	}

	bloqueos, err := h.servicio.ListarBloqueos(r.Context(), desde, hasta)
	if err != nil {
		manejarError(w, err)
		return
	}

	respuestas := make([]dto.BloqueoResponse, len(bloqueos))
	for i, b := range bloqueos {
		respuestas[i] = dto.NuevoBloqueoResponse(b)
	}
	responderJSON(w, http.StatusOK, respuestas)
}
