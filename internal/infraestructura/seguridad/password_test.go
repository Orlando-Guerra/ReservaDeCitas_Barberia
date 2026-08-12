package seguridad_test

import (
	"testing"

	"reservas-go/internal/infraestructura/seguridad"
)

func TestHashearYVerificarPassword(t *testing.T) {
	hash, err := seguridad.HashearPassword("miContraseñaSegura123")
	if err != nil {
		t.Fatalf("no se esperaba error: %v", err)
	}

	if hash == "miContraseñaSegura123" {
		t.Fatal("el hash no debería ser igual a la contraseña en texto plano")
	}
	if !seguridad.VerificarPassword(hash, "miContraseñaSegura123") {
		t.Error("la contraseña correcta debería verificar como válida")
	}
	if seguridad.VerificarPassword(hash, "otraContraseñaDistinta") {
		t.Error("una contraseña incorrecta no debería verificar como válida")
	}
}

func TestHashearPassword_MismaContraseñaDaHashesDistintos(t *testing.T) {
	hash1, err := seguridad.HashearPassword("igual123")
	if err != nil {
		t.Fatalf("no se esperaba error: %v", err)
	}
	hash2, err := seguridad.HashearPassword("igual123")
	if err != nil {
		t.Fatalf("no se esperaba error: %v", err)
	}

	if hash1 == hash2 {
		t.Error("dos hashes de la misma contraseña no deberían ser iguales (falta el salt aleatorio de bcrypt)")
	}
}
