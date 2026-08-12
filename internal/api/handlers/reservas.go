package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"reservas-go/internal/api/dto"
	"reservas-go/internal/api/middleware"
	"reservas-go/internal/aplicacion"
	"reservas-go/internal/dominio/entidades"
	"reservas-go/internal/dominio/puertos"
)

// HandlerReservas expone crear, cancelar y listar reservas.
type HandlerReservas struct {
	servicio *aplicacion.ServicioReservas
}

// NuevoHandlerReservas crea un HandlerReservas.
func NuevoHandlerReservas(servicio *aplicacion.ServicioReservas) *HandlerReservas {
	return &HandlerReservas{servicio: servicio}
}

// Crear atiende POST /reservas: el cliente autenticado reserva un slot
// para sí mismo.
func (h *HandlerReservas) Crear(w http.ResponseWriter, r *http.Request) {
	usuario, ok := middleware.UsuarioDesdeContexto(r.Context())
	if !ok {
		responderError(w, http.StatusUnauthorized, "no autenticado")
		return
	}

	var req dto.CrearReservaRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		responderError(w, http.StatusBadRequest, "cuerpo del pedido inválido")
		return
	}
	if err := req.Validar(); err != nil {
		responderError(w, http.StatusBadRequest, err.Error())
		return
	}
	inicio, _ := time.Parse(time.RFC3339, req.Inicio) // ya validado en req.Validar()

	reserva, err := h.servicio.CrearReserva(r.Context(), aplicacion.ParametrosCrearReserva{
		ClienteID:      usuario.ID,
		ServicioID:     entidades.ID(req.ServicioID),
		Inicio:         inicio.UTC(),
		RolSolicitante: usuario.Rol,
	})
	if err != nil {
		manejarError(w, err)
		return
	}
	responderJSON(w, http.StatusCreated, dto.NuevaReservaResponse(reserva))
}

// CrearAdmin atiende POST /admin/reservas: el administrador crea una
// reserva manual para un cliente presencial (walk-in).
func (h *HandlerReservas) CrearAdmin(w http.ResponseWriter, r *http.Request) {
	var req dto.CrearReservaAdminRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		responderError(w, http.StatusBadRequest, "cuerpo del pedido inválido")
		return
	}
	if err := req.Validar(); err != nil {
		responderError(w, http.StatusBadRequest, err.Error())
		return
	}
	inicio, _ := time.Parse(time.RFC3339, req.Inicio)

	reserva, err := h.servicio.CrearReserva(r.Context(), aplicacion.ParametrosCrearReserva{
		ClienteID:      entidades.ID(req.ClienteID),
		ServicioID:     entidades.ID(req.ServicioID),
		Inicio:         inicio.UTC(),
		RolSolicitante: entidades.RolAdministrador,
	})
	if err != nil {
		manejarError(w, err)
		return
	}
	responderJSON(w, http.StatusCreated, dto.NuevaReservaResponse(reserva))
}

// Cancelar atiende POST /reservas/{id}/cancelar.
func (h *HandlerReservas) Cancelar(w http.ResponseWriter, r *http.Request) {
	usuario, ok := middleware.UsuarioDesdeContexto(r.Context())
	if !ok {
		responderError(w, http.StatusUnauthorized, "no autenticado")
		return
	}

	id := r.PathValue("id")
	if id == "" {
		responderError(w, http.StatusBadRequest, "falta el id de la reserva")
		return
	}

	err := h.servicio.CancelarReserva(r.Context(), aplicacion.ParametrosCancelarReserva{
		ReservaID:      entidades.ID(id),
		SolicitanteID:  usuario.ID,
		RolSolicitante: usuario.Rol,
	})
	if err != nil {
		manejarError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// MisReservas atiende GET /reservas/mias: las reservas del cliente
// autenticado.
func (h *HandlerReservas) MisReservas(w http.ResponseWriter, r *http.Request) {
	usuario, ok := middleware.UsuarioDesdeContexto(r.Context())
	if !ok {
		responderError(w, http.StatusUnauthorized, "no autenticado")
		return
	}

	clienteID := usuario.ID
	reservas, err := h.servicio.ListarReservas(r.Context(), puertos.FiltrosReservas{ClienteID: &clienteID})
	if err != nil {
		manejarError(w, err)
		return
	}
	responderJSON(w, http.StatusOK, respuestasDeReservas(reservas))
}

// ListarAdmin atiende GET /admin/reservas?desde=&hasta=&estado=.
func (h *HandlerReservas) ListarAdmin(w http.ResponseWriter, r *http.Request) {
	filtros, err := dto.FiltrosDesdeQuery(r.URL.Query())
	if err != nil {
		responderError(w, http.StatusBadRequest, err.Error())
		return
	}

	reservas, err := h.servicio.ListarReservas(r.Context(), filtros)
	if err != nil {
		manejarError(w, err)
		return
	}
	responderJSON(w, http.StatusOK, respuestasDeReservas(reservas))
}

func respuestasDeReservas(reservas []entidades.Reserva) []dto.ReservaResponse {
	respuestas := make([]dto.ReservaResponse, len(reservas))
	for i, res := range reservas {
		respuestas[i] = dto.NuevaReservaResponse(res)
	}
	return respuestas
}
