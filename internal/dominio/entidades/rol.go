package entidades

// Rol determina qué puede hacer un Usuario dentro del sistema.
type Rol string

const (
	// RolCliente puede consultar slots disponibles, reservar y cancelar
	// sus propios turnos.
	RolCliente Rol = "cliente"

	// RolAdministrador es el barbero: define servicios, horarios, días
	// bloqueados, y puede crear reservas manualmente para clientes
	// presenciales.
	RolAdministrador Rol = "administrador"
)

// EsValido indica si r es uno de los roles reconocidos por el sistema.
func (r Rol) EsValido() bool {
	return r == RolCliente || r == RolAdministrador
}
