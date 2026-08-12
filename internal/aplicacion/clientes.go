package aplicacion

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"reservas-go/internal/dominio/entidades"
	"reservas-go/internal/dominio/puertos"
)

// ServicioClientes agrupa el caso de uso que usa el administrador para
// dar de alta la cuenta de un cliente presencial (walk-in), como paso
// previo a crearle una reserva manual (ver ServicioReservas.CrearReserva).
type ServicioClientes struct {
	usuarios  puertos.RepositorioUsuarios
	hasheador puertos.HasheadorPasswords
	reloj     puertos.Reloj
}

// NuevoServicioClientes crea un ServicioClientes.
func NuevoServicioClientes(usuarios puertos.RepositorioUsuarios, hasheador puertos.HasheadorPasswords, reloj puertos.Reloj) *ServicioClientes {
	return &ServicioClientes{usuarios: usuarios, hasheador: hasheador, reloj: reloj}
}

// CrearCliente da de alta un Usuario con rol Cliente, con una contraseña
// temporal aleatoria que el cliente no necesita conocer todavía (es un
// walk-in: hoy no se loguea, el barbero le está creando la reserva). Si
// el día de mañana el cliente quiere empezar a reservar online, puede
// recuperar su contraseña por el flujo normal (fuera del alcance de este
// proyecto).
func (s *ServicioClientes) CrearCliente(ctx context.Context, nombre, email string) (entidades.Usuario, error) {
	passwordTemporal, err := generarPasswordTemporal()
	if err != nil {
		return entidades.Usuario{}, fmt.Errorf("generando contraseña temporal: %w", err)
	}

	hash, err := s.hasheador.Hashear(passwordTemporal)
	if err != nil {
		return entidades.Usuario{}, fmt.Errorf("hasheando contraseña temporal: %w", err)
	}

	usuario, err := entidades.NuevoUsuario(nombre, email, hash, entidades.RolCliente, s.reloj.Ahora())
	if err != nil {
		return entidades.Usuario{}, err
	}

	if err := s.usuarios.Guardar(ctx, usuario); err != nil {
		return entidades.Usuario{}, err
	}
	return usuario, nil
}

// generarPasswordTemporal arma una contraseña aleatoria a partir de 16
// bytes de crypto/rand, codificados como texto hexadecimal (32
// caracteres). Nadie va a memorizar ni usar esta contraseña — solo tiene
// que ser lo bastante aleatoria como para que nadie la adivine.
func generarPasswordTemporal() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
