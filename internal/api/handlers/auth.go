package handlers

import (
	"encoding/json"
	"net/http"

	"reservas-go/internal/api/dto"
	"reservas-go/internal/aplicacion"
)

// HandlerAuth expone los endpoints públicos de registro y login.
type HandlerAuth struct {
	servicio *aplicacion.ServicioAutenticacion
}

// NuevoHandlerAuth crea un HandlerAuth.
func NuevoHandlerAuth(servicio *aplicacion.ServicioAutenticacion) *HandlerAuth {
	return &HandlerAuth{servicio: servicio}
}

// Registrar atiende POST /auth/registro.
func (h *HandlerAuth) Registrar(w http.ResponseWriter, r *http.Request) {
	var req dto.RegistroRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		responderError(w, http.StatusBadRequest, "cuerpo del pedido inválido")
		return
	}
	if err := req.Validar(); err != nil {
		responderError(w, http.StatusBadRequest, err.Error())
		return
	}

	usuario, err := h.servicio.Registrar(r.Context(), req.Nombre, req.Email, req.Password)
	if err != nil {
		manejarError(w, err)
		return
	}
	responderJSON(w, http.StatusCreated, dto.NuevoUsuarioResponse(usuario))
}

// Login atiende POST /auth/login.
func (h *HandlerAuth) Login(w http.ResponseWriter, r *http.Request) {
	var req dto.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		responderError(w, http.StatusBadRequest, "cuerpo del pedido inválido")
		return
	}
	if err := req.Validar(); err != nil {
		responderError(w, http.StatusBadRequest, err.Error())
		return
	}

	token, usuario, err := h.servicio.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		manejarError(w, err)
		return
	}
	responderJSON(w, http.StatusOK, dto.LoginResponse{Token: token, Usuario: dto.NuevoUsuarioResponse(usuario)})
}
