// Package aplicacion contiene los casos de uso: la orquestación de la
// lógica de negocio. Cada caso de uso recibe sus dependencias como
// puertos (interfaces del dominio), nunca como tipos concretos de
// infraestructura — así se pueden testear con implementaciones falsas,
// sin base de datos ni bcrypt ni JWT reales.
package aplicacion

import (
	"context"
	"errors"
	"fmt"

	"reservas-go/internal/dominio"
	"reservas-go/internal/dominio/entidades"
	"reservas-go/internal/dominio/puertos"
)

// ServicioAutenticacion agrupa los casos de uso de registro y login.
type ServicioAutenticacion struct {
	usuarios  puertos.RepositorioUsuarios
	hasheador puertos.HasheadorPasswords
	tokens    puertos.GeneradorTokens
	reloj     puertos.Reloj
}

// NuevoServicioAutenticacion crea un ServicioAutenticacion.
func NuevoServicioAutenticacion(usuarios puertos.RepositorioUsuarios, hasheador puertos.HasheadorPasswords, tokens puertos.GeneradorTokens, reloj puertos.Reloj) *ServicioAutenticacion {
	return &ServicioAutenticacion{usuarios: usuarios, hasheador: hasheador, tokens: tokens, reloj: reloj}
}

// Registrar crea una cuenta de Cliente nueva.
//
// El rol queda fijo en RolCliente a propósito: el registro público NUNCA
// puede crear administradores (si aceptáramos un "rol" desde afuera,
// cualquiera podría auto-asignarse permisos de admin mandando el campo
// correcto en el pedido HTTP). La única cuenta de administrador de este
// proyecto se crea con un comando aparte (cmd/seed-admin).
func (s *ServicioAutenticacion) Registrar(ctx context.Context, nombre, email, passwordPlano string) (entidades.Usuario, error) {
	hash, err := s.hasheador.Hashear(passwordPlano)
	if err != nil {
		return entidades.Usuario{}, fmt.Errorf("hasheando contraseña: %w", err)
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

// Login valida credenciales y, si son correctas, devuelve un token de
// sesión junto con el Usuario autenticado.
func (s *ServicioAutenticacion) Login(ctx context.Context, email, passwordPlano string) (token string, usuario entidades.Usuario, err error) {
	usuario, err = s.usuarios.BuscarPorEmail(ctx, email)
	if err != nil {
		if errors.Is(err, dominio.ErrNoEncontrado) {
			return "", entidades.Usuario{}, dominio.ErrCredencialesInvalidas
		}
		return "", entidades.Usuario{}, err
	}

	if !s.hasheador.Verificar(usuario.PasswordHash, passwordPlano) {
		return "", entidades.Usuario{}, dominio.ErrCredencialesInvalidas
	}

	token, err = s.tokens.Generar(usuario.ID, usuario.Rol)
	if err != nil {
		return "", entidades.Usuario{}, fmt.Errorf("generando token: %w", err)
	}
	return token, usuario, nil
}
