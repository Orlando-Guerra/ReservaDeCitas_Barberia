package middleware

// Este archivo usa "package middleware" (no "middleware_test") a
// propósito: necesita construir un contexto con claveUsuarioAutenticado
// directamente, sin pasar por un JWT real, para poder testear RequiereRol
// de forma aislada de Autenticacion. Esa clave es un detalle interno del
// paquete (no exportado) — un test de caja blanca como este es el único
// lugar, fuera del propio middleware, con permiso para tocarla.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"reservas-go/internal/dominio/entidades"
)

func TestRequiereRol(t *testing.T) {
	handlerFinal := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	protegido := RequiereRol(entidades.RolAdministrador)(handlerFinal)

	t.Run("sin usuario autenticado en el contexto, devuelve 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()

		protegido.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("código = %d, se esperaba %d", rec.Code, http.StatusUnauthorized)
		}
	})

	t.Run("con el rol permitido, deja pasar", func(t *testing.T) {
		req := conUsuarioDePrueba(entidades.RolAdministrador)
		rec := httptest.NewRecorder()

		protegido.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("código = %d, se esperaba %d", rec.Code, http.StatusOK)
		}
	})

	t.Run("con un rol distinto al permitido, devuelve 403", func(t *testing.T) {
		req := conUsuarioDePrueba(entidades.RolCliente)
		rec := httptest.NewRecorder()

		protegido.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Errorf("código = %d, se esperaba %d", rec.Code, http.StatusForbidden)
		}
	})
}

func conUsuarioDePrueba(rol entidades.Rol) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx := context.WithValue(req.Context(), claveUsuarioAutenticado, UsuarioAutenticado{ID: "usuario-de-prueba", Rol: rol})
	return req.WithContext(ctx)
}
