package seguridad_test

import (
	"testing"
	"time"

	"reservas-go/internal/dominio/entidades"
	"reservas-go/internal/infraestructura/seguridad"
)

func TestGenerarYValidarToken(t *testing.T) {
	secreto := []byte("secreto-de-prueba")
	ahora := time.Now().UTC()

	token, err := seguridad.GenerarToken("usuario-123", entidades.RolAdministrador, secreto, ahora, time.Hour)
	if err != nil {
		t.Fatalf("no se esperaba error generando el token: %v", err)
	}

	claims, err := seguridad.ValidarToken(token, secreto)
	if err != nil {
		t.Fatalf("no se esperaba error validando el token: %v", err)
	}

	if claims.UsuarioID != "usuario-123" {
		t.Errorf("UsuarioID = %q, se esperaba %q", claims.UsuarioID, "usuario-123")
	}
	if claims.Rol != entidades.RolAdministrador {
		t.Errorf("Rol = %q, se esperaba %q", claims.Rol, entidades.RolAdministrador)
	}
}

func TestValidarToken_TokenExpirado(t *testing.T) {
	secreto := []byte("secreto-de-prueba")
	haceDosHoras := time.Now().UTC().Add(-2 * time.Hour)

	// Un token emitido hace 2 horas con 1 hora de duración ya expiró hace
	// 1 hora.
	token, err := seguridad.GenerarToken("usuario-123", entidades.RolCliente, secreto, haceDosHoras, time.Hour)
	if err != nil {
		t.Fatalf("no se esperaba error generando el token: %v", err)
	}

	if _, err := seguridad.ValidarToken(token, secreto); err == nil {
		t.Error("se esperaba un error por token expirado, no hubo ninguno")
	}
}

func TestValidarToken_SecretoIncorrecto(t *testing.T) {
	token, err := seguridad.GenerarToken("usuario-123", entidades.RolCliente, []byte("secreto-correcto"), time.Now().UTC(), time.Hour)
	if err != nil {
		t.Fatalf("no se esperaba error generando el token: %v", err)
	}

	if _, err := seguridad.ValidarToken(token, []byte("secreto-incorrecto")); err == nil {
		t.Error("se esperaba un error al validar con un secreto distinto al que firmó, no hubo ninguno")
	}
}
