package handlers

import (
	"encoding/json"
	"net/http"

	"reservas-go/internal/api/dto"
	"reservas-go/internal/aplicacion"
)

// HandlerClientes expone el endpoint de administrador para dar de alta
// clientes walk-in.
type HandlerClientes struct {
	servicio *aplicacion.ServicioClientes
}

// NuevoHandlerClientes crea un HandlerClientes.
func NuevoHandlerClientes(servicio *aplicacion.ServicioClientes) *HandlerClientes {
	return &HandlerClientes{servicio: servicio}
}

// Crear atiende POST /admin/clientes.
func (h *HandlerClientes) Crear(w http.ResponseWriter, r *http.Request) {
	var req dto.CrearClienteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		responderError(w, http.StatusBadRequest, "cuerpo del pedido inválido")
		return
	}
	if err := req.Validar(); err != nil {
		responderError(w, http.StatusBadRequest, err.Error())
		return
	}

	cliente, err := h.servicio.CrearCliente(r.Context(), req.Nombre, req.Email)
	if err != nil {
		manejarError(w, err)
		return
	}
	responderJSON(w, http.StatusCreated, dto.NuevoUsuarioResponse(cliente))
}
