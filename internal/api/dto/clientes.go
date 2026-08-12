package dto

import (
	"fmt"
	"strings"
)

// CrearClienteRequest es el cuerpo de POST /admin/clientes — el
// administrador da de alta la cuenta de un cliente presencial (walk-in),
// como paso previo a crearle una reserva manual.
type CrearClienteRequest struct {
	Nombre string `json:"nombre"`
	Email  string `json:"email"`
}

// Validar chequea la forma de los datos.
func (r CrearClienteRequest) Validar() error {
	if strings.TrimSpace(r.Nombre) == "" {
		return fmt.Errorf("el nombre es requerido")
	}
	if strings.TrimSpace(r.Email) == "" || !strings.Contains(r.Email, "@") {
		return fmt.Errorf("el email es requerido y debe ser válido")
	}
	return nil
}
