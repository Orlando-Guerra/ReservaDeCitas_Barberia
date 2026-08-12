package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"reservas-go/internal/api/middleware"
	"reservas-go/internal/dominio/entidades"
	"reservas-go/internal/infraestructura/seguridad"
)

func TestAutenticacion(t *testing.T) {
	secreto := []byte("secreto-de-prueba")

	handlerFinal := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		usuario, ok := middleware.UsuarioDesdeContexto(r.Context())
		if !ok {
			t.Error("se esperaba encontrar un UsuarioAutenticado en el contexto")
		}
		if usuario.Rol != entidades.RolAdministrador {
			t.Errorf("rol en el contexto = %q, se esperaba %q", usuario.Rol, entidades.RolAdministrador)
		}
		w.WriteHeader(http.StatusOK)
	})

	protegido := middleware.Autenticacion(secreto)(handlerFinal)

	t.Run("sin header Authorization, devuelve 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()

		protegido.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("código = %d, se esperaba %d", rec.Code, http.StatusUnauthorized)
		}
	})

	t.Run("con token inválido, devuelve 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer token-basura")
		rec := httptest.NewRecorder()

		protegido.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("código = %d, se esperaba %d", rec.Code, http.StatusUnauthorized)
		}
	})

	t.Run("con token válido, deja pasar y carga el usuario en el contexto", func(t *testing.T) {
		token, err := seguridad.GenerarToken("admin-1", entidades.RolAdministrador, secreto, time.Now().UTC(), time.Hour)
		if err != nil {
			t.Fatalf("no se esperaba error generando el token: %v", err)
		}

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()

		protegido.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("código = %d, se esperaba %d", rec.Code, http.StatusOK)
		}
	})
}
