// Comando api es el punto de entrada para correr el servidor como
// proceso persistente (desarrollo local, Railway, o cualquier host
// tradicional). El cableado de la aplicación en sí vive en
// internal/api/app, compartido con la función serverless de Vercel
// (api/index.go).
package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"reservas-go/internal/api/app"
)

func main() {
	cfg := app.CargarConfig()
	if cfg.JWTSecret == "" {
		log.Fatal("falta la variable de entorno JWT_SECRET (ver .env.example)")
	}

	ctxArranque, cancelarArranque := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelarArranque()

	mux, pool, err := app.Construir(ctxArranque, cfg)
	if err != nil {
		log.Fatalf("error armando la aplicación: %v", err)
	}
	defer pool.Close()

	log.Printf("servidor escuchando en http://localhost:%s", cfg.AppPort)

	if err := http.ListenAndServe(":"+cfg.AppPort, mux); err != nil {
		log.Fatalf("error al iniciar el servidor: %v", err)
	}
}
