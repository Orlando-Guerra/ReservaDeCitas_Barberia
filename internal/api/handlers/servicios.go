package handlers

import (
	"encoding/json"
	"net/http"

	"reservas-go/internal/api/dto"
	"reservas-go/internal/aplicacion"
	"reservas-go/internal/dominio/entidades"
)

// HandlerServicios expone el CRUD de servicios. Crear/Actualizar son
// admin-only; Listar lo usa cualquier usuario autenticado (un cliente
// necesita ver los servicios disponibles para elegir uno al reservar).
type HandlerServicios struct {
	servicio *aplicacion.ServicioAdministracion
}

// NuevoHandlerServicios crea un HandlerServicios.
func NuevoHandlerServicios(servicio *aplicacion.ServicioAdministracion) *HandlerServicios {
	return &HandlerServicios{servicio: servicio}
}

// Crear atiende POST /admin/servicios.
func (h *HandlerServicios) Crear(w http.ResponseWriter, r *http.Request) {
	var req dto.CrearServicioRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		responderError(w, http.StatusBadRequest, "cuerpo del pedido inválido")
		return
	}
	if err := req.Validar(); err != nil {
		responderError(w, http.StatusBadRequest, err.Error())
		return
	}

	servicio, err := h.servicio.CrearServicio(r.Context(), req.Nombre, req.DuracionMinutos, req.PrecioCentavos)
	if err != nil {
		manejarError(w, err)
		return
	}
	responderJSON(w, http.StatusCreated, dto.NuevoServicioResponse(servicio))
}

// Actualizar atiende PUT /admin/servicios/{id}.
func (h *HandlerServicios) Actualizar(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req dto.ActualizarServicioRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		responderError(w, http.StatusBadRequest, "cuerpo del pedido inválido")
		return
	}
	if err := req.Validar(); err != nil {
		responderError(w, http.StatusBadRequest, err.Error())
		return
	}

	servicio, err := h.servicio.ActualizarServicio(r.Context(), entidades.ID(id), req.Nombre, req.DuracionMinutos, req.PrecioCentavos, req.Activo)
	if err != nil {
		manejarError(w, err)
		return
	}
	responderJSON(w, http.StatusOK, dto.NuevoServicioResponse(servicio))
}

// Listar atiende GET /servicios.
func (h *HandlerServicios) Listar(w http.ResponseWriter, r *http.Request) {
	servicios, err := h.servicio.ListarServicios(r.Context())
	if err != nil {
		manejarError(w, err)
		return
	}

	respuestas := make([]dto.ServicioResponse, len(servicios))
	for i, s := range servicios {
		respuestas[i] = dto.NuevoServicioResponse(s)
	}
	responderJSON(w, http.StatusOK, respuestas)
}
