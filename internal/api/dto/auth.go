package dto

import (
	"fmt"
	"strings"

	"reservas-go/internal/dominio/entidades"
)

// RegistroRequest es el cuerpo de POST /auth/registro.
type RegistroRequest struct {
	Nombre   string `json:"nombre"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

// Validar chequea la forma de los datos (no reglas de negocio, esas las
// aplica entidades.NuevoUsuario más adelante en la cadena).
func (r RegistroRequest) Validar() error {
	if strings.TrimSpace(r.Nombre) == "" {
		return fmt.Errorf("el nombre es requerido")
	}
	if strings.TrimSpace(r.Email) == "" || !strings.Contains(r.Email, "@") {
		return fmt.Errorf("el email es requerido y debe ser válido")
	}
	if len(r.Password) < 8 {
		return fmt.Errorf("la contraseña debe tener al menos 8 caracteres")
	}
	return nil
}

// LoginRequest es el cuerpo de POST /auth/login.
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// Validar chequea que vengan los dos campos — a propósito NO valida el
// formato del email ni la longitud de la contraseña acá: en el login no
// queremos darle a un atacante ninguna pista sobre por qué falló.
func (r LoginRequest) Validar() error {
	if strings.TrimSpace(r.Email) == "" {
		return fmt.Errorf("el email es requerido")
	}
	if r.Password == "" {
		return fmt.Errorf("la contraseña es requerida")
	}
	return nil
}

// LoginResponse es la respuesta de un login exitoso.
type LoginResponse struct {
	Token   string          `json:"token"`
	Usuario UsuarioResponse `json:"usuario"`
}

// UsuarioResponse es cómo se ve un Usuario hacia afuera. Nunca incluye
// PasswordHash — ni por accidente: ese campo directamente no existe en
// este struct, así que no hay forma de que json.Marshal lo devuelva.
type UsuarioResponse struct {
	ID     string `json:"id"`
	Nombre string `json:"nombre"`
	Email  string `json:"email"`
	Rol    string `json:"rol"`
}

// NuevoUsuarioResponse convierte un entidades.Usuario en su
// representación pública.
func NuevoUsuarioResponse(u entidades.Usuario) UsuarioResponse {
	return UsuarioResponse{
		ID:     string(u.ID),
		Nombre: u.Nombre,
		Email:  u.Email,
		Rol:    string(u.Rol),
	}
}
