package aplicacion_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"reservas-go/internal/aplicacion"
	"reservas-go/internal/dominio"
	"reservas-go/internal/dominio/entidades"
)

func nuevoServicioAutenticacionDePrueba() (*aplicacion.ServicioAutenticacion, *repositorioUsuariosMemoria) {
	usuarios := nuevoRepositorioUsuariosMemoria()
	reloj := &relojFijo{momento: time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)}
	servicio := aplicacion.NuevoServicioAutenticacion(usuarios, hasheadorFalso{}, generadorTokensFalso{}, reloj)
	return servicio, usuarios
}

func TestServicioAutenticacion_Registrar(t *testing.T) {
	t.Run("con datos válidos, crea un cliente", func(t *testing.T) {
		servicio, _ := nuevoServicioAutenticacionDePrueba()

		usuario, err := servicio.Registrar(context.Background(), "Ana", "ana@ejemplo.com", "password123")
		if err != nil {
			t.Fatalf("no se esperaba error: %v", err)
		}
		if usuario.Rol != entidades.RolCliente {
			t.Errorf("Rol = %q, se esperaba %q (el registro público nunca crea administradores)", usuario.Rol, entidades.RolCliente)
		}
		if usuario.PasswordHash == "password123" {
			t.Error("la contraseña no debería guardarse en texto plano")
		}
	})

	t.Run("con email duplicado, devuelve ErrEmailYaRegistrado", func(t *testing.T) {
		servicio, _ := nuevoServicioAutenticacionDePrueba()
		ctx := context.Background()

		if _, err := servicio.Registrar(ctx, "Ana", "ana@ejemplo.com", "password123"); err != nil {
			t.Fatalf("no se esperaba error en el primer registro: %v", err)
		}

		_, err := servicio.Registrar(ctx, "Otra Ana", "ana@ejemplo.com", "otraPassword")
		if !errors.Is(err, dominio.ErrEmailYaRegistrado) {
			t.Errorf("error = %v, se esperaba dominio.ErrEmailYaRegistrado", err)
		}
	})

	t.Run("con datos inválidos, propaga el error de validación del dominio", func(t *testing.T) {
		servicio, _ := nuevoServicioAutenticacionDePrueba()

		_, err := servicio.Registrar(context.Background(), "", "no-es-un-email", "password123")
		if err == nil {
			t.Fatal("se esperaba un error por nombre vacío")
		}
	})
}

func TestServicioAutenticacion_Login(t *testing.T) {
	t.Run("con credenciales correctas, devuelve un token", func(t *testing.T) {
		servicio, _ := nuevoServicioAutenticacionDePrueba()
		ctx := context.Background()
		if _, err := servicio.Registrar(ctx, "Ana", "ana@ejemplo.com", "password123"); err != nil {
			t.Fatalf("no se esperaba error registrando: %v", err)
		}

		token, usuario, err := servicio.Login(ctx, "ana@ejemplo.com", "password123")
		if err != nil {
			t.Fatalf("no se esperaba error: %v", err)
		}
		if token == "" {
			t.Error("se esperaba un token no vacío")
		}
		if usuario.Email != "ana@ejemplo.com" {
			t.Errorf("Email = %q, se esperaba %q", usuario.Email, "ana@ejemplo.com")
		}
	})

	t.Run("con contraseña incorrecta, devuelve ErrCredencialesInvalidas", func(t *testing.T) {
		servicio, _ := nuevoServicioAutenticacionDePrueba()
		ctx := context.Background()
		if _, err := servicio.Registrar(ctx, "Ana", "ana@ejemplo.com", "password123"); err != nil {
			t.Fatalf("no se esperaba error registrando: %v", err)
		}

		_, _, err := servicio.Login(ctx, "ana@ejemplo.com", "otra-contraseña")
		if !errors.Is(err, dominio.ErrCredencialesInvalidas) {
			t.Errorf("error = %v, se esperaba dominio.ErrCredencialesInvalidas", err)
		}
	})

	t.Run("con email inexistente, devuelve EXACTAMENTE el mismo error que una contraseña incorrecta", func(t *testing.T) {
		// Este test verifica algo que importa por seguridad, no solo por
		// corrección: si el error fuera distinto según "no existe el
		// email" vs. "existe pero la contraseña está mal", cualquiera
		// podría usar el login para averiguar qué emails están
		// registrados, probando de a uno (ver docs/CONCURRENCIA.md... en
		// realidad, ver el comentario de ErrCredencialesInvalidas en
		// internal/dominio/errores.go).
		servicio, _ := nuevoServicioAutenticacionDePrueba()

		_, _, err := servicio.Login(context.Background(), "no-existe@ejemplo.com", "cualquier-cosa")
		if !errors.Is(err, dominio.ErrCredencialesInvalidas) {
			t.Errorf("error = %v, se esperaba dominio.ErrCredencialesInvalidas", err)
		}
	})
}
