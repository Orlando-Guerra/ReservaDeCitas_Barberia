# Carga las variables de .env (si existe) y las exporta a todos los
# comandos de abajo, para no repetir host/puerto/usuario en cada uno.
-include .env
export

DATABASE_URL=postgres://$(DB_USER):$(DB_PASSWORD)@$(DB_HOST):$(DB_PORT)/$(DB_NAME)?sslmode=$(DB_SSLMODE)

.PHONY: run build test docker-up docker-down migrate-up migrate-down migrate-create

run:
	go run ./cmd/api

build:
	go build -o bin/api.exe ./cmd/api

# -p 1: corre los paquetes de test de a uno, no en paralelo. Dos paquetes
# distintos (internal/infraestructura/postgres y cmd/api) hacen tests de
# integración contra la MISMA base de datos real, tocando las mismas
# tablas — si Go los corriera en paralelo (su comportamiento por
# defecto), uno podría truncar una tabla mientras el otro todavía la está
# usando. Los tests que no tocan la base (dominio, seguridad, middleware)
# no necesitan esto, pero igual corren rápido incluso en secuencial.
test:
	go test -p 1 ./...

# Como test, pero sin los tests de integración (se saltan solos si no hay
# Postgres disponible, pero -short los salta explícitamente sin siquiera
# intentar conectarse).
test-rapido:
	go test -short ./...

docker-up:
	docker compose up -d

docker-down:
	docker compose down

migrate-up:
	migrate -path migrations -database "$(DATABASE_URL)" up

migrate-down:
	migrate -path migrations -database "$(DATABASE_URL)" down 1

# Uso: make migrate-create name=crear_tabla_usuarios
migrate-create:
	migrate create -ext sql -dir migrations -seq $(name)
