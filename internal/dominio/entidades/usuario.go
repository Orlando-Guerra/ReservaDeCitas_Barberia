package entidades

import (
	"fmt"
	"strings"
	"time"
)

// Usuario representa a cualquier persona que puede autenticarse en el
// sistema, sea Cliente o Administrador.
type Usuario struct {
	ID           ID
	Nombre       string
	Email        string
	PasswordHash string
	Rol          Rol
	CreadoEn     time.Time
}

// NuevoUsuario crea un Usuario nuevo validando sus datos básicos.
//
// No calcula el hash de la contraseña: eso es responsabilidad de la capa
// de seguridad (infraestructura, Fase 4), que vive fuera del dominio. Acá
// recibimos directamente el hash ya calculado.
//
// Recibe "ahora" como parámetro en vez de llamar a time.Now() interna-
// mente: el dominio nunca pregunta la hora por su cuenta, siempre la
// recibe de quien la llama (que sí tiene acceso a un puertos.Reloj). Así
// esta función queda determinística y fácil de testear.
func NuevoUsuario(nombre, email, passwordHash string, rol Rol, ahora time.Time) (Usuario, error) {
	nombre = strings.TrimSpace(nombre)
	email = strings.TrimSpace(email)

	if nombre == "" {
		return Usuario{}, fmt.Errorf("el nombre no puede estar vacío")
	}
	if email == "" || !strings.Contains(email, "@") {
		return Usuario{}, fmt.Errorf("el email %q no es válido", email)
	}
	if passwordHash == "" {
		return Usuario{}, fmt.Errorf("el usuario necesita un hash de contraseña")
	}
	if !rol.EsValido() {
		return Usuario{}, fmt.Errorf("el rol %q no es válido", rol)
	}

	return Usuario{
		ID:           NuevoID(),
		Nombre:       nombre,
		Email:        email,
		PasswordHash: passwordHash,
		Rol:          rol,
		CreadoEn:     ahora,
	}, nil
}
