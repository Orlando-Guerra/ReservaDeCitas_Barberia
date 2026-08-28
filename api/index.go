// Package handler expone la aplicación como una función serverless de
// Vercel. El handler y el pool de Postgres se arman una sola vez a nivel
// de paquete (no en cada invocación) para que las invocaciones "warm" del
// mismo contenedor reusen la misma conexión, en vez de abrir una nueva
// contra Neon en cada request.
package handler

import (
	"context"
	"log"
	"net/http"
	"sync"
	"time"

	"reservas-go/internal/api/app"
)

var (
	once        sync.Once
	mux         http.Handler
	errArranque error
)

func inicializar() {
	cfg := app.CargarConfig()
	if cfg.JWTSecret == "" {
		errArranque = errJWTSecretFaltante
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	h, _, err := app.Construir(ctx, cfg)
	if err != nil {
		log.Printf("error armando la aplicación: %v", err)
		errArranque = err
		return
	}
	mux = h
}

var errJWTSecretFaltante = &configError{"falta la variable de entorno JWT_SECRET"}

type configError struct{ msg string }

func (e *configError) Error() string { return e.msg }

// Handler es el punto de entrada que Vercel invoca por cada request.
func Handler(w http.ResponseWriter, r *http.Request) {
	once.Do(inicializar)

	if errArranque != nil {
		http.Error(w, "error de configuración del servidor", http.StatusInternalServerError)
		return
	}

	mux.ServeHTTP(w, r)
}
