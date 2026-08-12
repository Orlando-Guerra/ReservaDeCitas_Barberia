// Comando seed-admin crea (o actualiza la contraseña de) la única cuenta
// de administrador del sistema, a partir de ADMIN_EMAIL y ADMIN_PASSWORD.
//
// Existe porque el registro público (POST /auth/registro) nunca puede
// crear administradores — si lo hiciera, cualquiera podría auto-
// asignarse ese rol. Este es un segundo punto de entrada (cmd/seed-admin,
// además de cmd/api) que reutiliza el mismo código interno del proyecto
// (internal/...): en Go es común tener varios "cmd/" chicos que comparten
// toda su lógica a través de los paquetes de internal/, en vez de
// duplicar código entre ellos.
//
// Uso: go run ./cmd/seed-admin
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"reservas-go/internal/dominio"
	"reservas-go/internal/dominio/entidades"
	"reservas-go/internal/infraestructura/postgres"
	"reservas-go/internal/infraestructura/seguridad"
)

func main() {
	email := os.Getenv("ADMIN_EMAIL")
	password := os.Getenv("ADMIN_PASSWORD")
	if email == "" || password == "" {
		log.Fatal("definí ADMIN_EMAIL y ADMIN_PASSWORD en el entorno (o en .env) antes de correr este comando")
	}

	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		envODefault("DB_USER", "reservas_user"),
		envODefault("DB_PASSWORD", "reservas_pass"),
		envODefault("DB_HOST", "localhost"),
		envODefault("DB_PORT", "5434"),
		envODefault("DB_NAME", "reservas_db"),
		envODefault("DB_SSLMODE", "disable"),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := postgres.NuevoPool(ctx, dsn)
	if err != nil {
		log.Fatalf("error conectando a la base de datos: %v", err)
	}
	defer pool.Close()

	repoUsuarios := postgres.NuevoRepositorioUsuarios(pool)
	hasheador := seguridad.HasheadorBcrypt{}

	hash, err := hasheador.Hashear(password)
	if err != nil {
		log.Fatalf("error hasheando la contraseña: %v", err)
	}

	existente, err := repoUsuarios.BuscarPorEmail(ctx, email)
	switch {
	case errors.Is(err, dominio.ErrNoEncontrado):
		admin, err := entidades.NuevoUsuario("Administrador", email, hash, entidades.RolAdministrador, time.Now().UTC())
		if err != nil {
			log.Fatalf("error creando el usuario administrador: %v", err)
		}
		if err := repoUsuarios.Guardar(ctx, admin); err != nil {
			log.Fatalf("error guardando el usuario administrador: %v", err)
		}
		log.Printf("administrador creado: %s", email)

	case err != nil:
		log.Fatalf("error buscando el usuario administrador: %v", err)

	default:
		if err := repoUsuarios.ActualizarPassword(ctx, existente.ID, hash); err != nil {
			log.Fatalf("error actualizando la contraseña del administrador: %v", err)
		}
		log.Printf("administrador ya existía (%s): contraseña actualizada", email)
	}
}

func envODefault(clave, porDefecto string) string {
	if valor := os.Getenv(clave); valor != "" {
		return valor
	}
	return porDefecto
}
