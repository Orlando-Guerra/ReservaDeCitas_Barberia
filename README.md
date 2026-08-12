# Reservas Go

Plataforma de reservas para una barbería (un solo recurso/calendario),
construida en Go con arquitectura hexagonal, como proyecto de aprendizaje
de backend. Caso de uso: un barbero, un calendario, clientes que reservan
online y un administrador que gestiona servicios, horarios y turnos
presenciales.

El objetivo del proyecto fue aprender backend a fondo — no llegar rápido.
El corazón técnico es la Fase 3: garantizar, a nivel de base de datos, que
nunca haya dos reservas solapadas, ni bajo peticiones concurrentes. Ver
[`docs/CONCURRENCIA.md`](docs/CONCURRENCIA.md) para la explicación
completa.

## Stack

- Go 1.23+, `net/http` de la librería estándar (sin frameworks de router)
- PostgreSQL 16 (Docker) + `pgx/v5`
- `golang-migrate` para migraciones
- Mailpit (Docker) como servidor SMTP falso en desarrollo
- `golang-jwt/jwt/v5` + `bcrypt`

## Requisitos

- Go 1.23+
- Docker
- [`golang-migrate`](https://github.com/golang-migrate/migrate) CLI
- `make` (en Windows: instalado vía `winget install GnuWin32.Make`)

## Cómo levantar el entorno de desarrollo

```bash
cp .env.example .env      # ajustar valores si hace falta
make docker-up            # levanta Postgres 16 + Mailpit
make migrate-up           # aplica las migraciones
```

Antes de arrancar la API, necesitás una cuenta de administrador (el
registro público nunca puede crear una — ver
[`docs/CONTEXTO.md`](docs/CONTEXTO.md), Fase 5):

```bash
ADMIN_EMAIL=vos@ejemplo.com ADMIN_PASSWORD=lo-que-quieras go run ./cmd/seed-admin
```

o simplemente definí `ADMIN_EMAIL`/`ADMIN_PASSWORD` en tu `.env` y corré
`go run ./cmd/seed-admin` sin nada más. Después:

```bash
make run                  # levanta la API en http://localhost:8080
```

- API: http://localhost:8080/health
- Mailpit (interfaz web de correos): http://localhost:8025

## Endpoints

Todos (salvo `/health`, `/auth/registro` y `/auth/login`) requieren
`Authorization: Bearer <token>`.

| Método | Ruta | Quién | Qué hace |
|---|---|---|---|
| GET | `/health` | público | estado del servicio + conexión a la base |
| POST | `/auth/registro` | público | crea una cuenta de **cliente** (nunca admin) |
| POST | `/auth/login` | público | devuelve un JWT |
| GET | `/servicios` | autenticado | lista los servicios ofrecidos |
| GET | `/slots?fecha=YYYY-MM-DD&servicio_id=` | autenticado | slots disponibles/ocupados de un día |
| POST | `/reservas` | autenticado | reserva un slot para uno mismo |
| POST | `/reservas/{id}/cancelar` | autenticado | cancela una reserva propia (hasta 2hs antes) |
| GET | `/reservas/mias` | autenticado | las reservas del usuario logueado |
| POST | `/admin/servicios` | admin | crea un servicio |
| PUT | `/admin/servicios/{id}` | admin | actualiza un servicio |
| POST | `/admin/horarios` | admin | define el horario de un día de la semana |
| GET | `/admin/horarios` | admin | lista los horarios configurados |
| POST | `/admin/dias-bloqueados` | admin | bloquea un día (completo o desde una hora) |
| DELETE | `/admin/dias-bloqueados/{id}` | admin | elimina un bloqueo |
| GET | `/admin/dias-bloqueados?desde=&hasta=` | admin | lista bloqueos en un rango |
| POST | `/admin/clientes` | admin | da de alta un cliente walk-in |
| POST | `/admin/reservas` | admin | reserva manual para un cliente existente |
| GET | `/admin/reservas?desde=&hasta=&estado=` | admin | todas las reservas, con filtros |

## Tests

```bash
make test          # toda la suite, incluidos los tests de integración contra Postgres real
make test-rapido    # solo los tests que no tocan la base de datos (-short)
```

Los tests de integración (`internal/infraestructura/postgres/`,
`cmd/api/main_test.go`) necesitan `docker compose up -d` corriendo — si no
la encuentran, se saltan solos en vez de fallar.

`make test` corre los paquetes en secuencia (`-p 1`) a propósito: dos
paquetes de test tocan la misma base de datos real, y correrlos en
paralelo (el comportamiento por defecto de Go) podría hacer que uno
trunque una tabla mientras el otro la está usando.

## Estructura del proyecto

```
cmd/
├── api/            # entry point del servidor HTTP
└── seed-admin/      # crea/actualiza la cuenta de administrador
internal/
├── dominio/          # reglas de negocio puras (solo librería estándar)
│   ├── entidades/     # Usuario, Servicio, Reserva, etc.
│   └── puertos/         # interfaces: repositorios, Notificador, Reloj...
├── aplicacion/         # casos de uso
├── infraestructura/
│   ├── postgres/         # repositorios (pgx)
│   ├── seguridad/          # bcrypt, JWT
│   └── notificaciones/      # adaptador SMTP + plantillas HTML
└── api/
    ├── handlers/           # adaptadores HTTP
    ├── middleware/           # autenticación y autorización
    └── dto/                    # request/response + validación manual
```

## Documentación

- [`docs/ARQUITECTURA.md`](docs/ARQUITECTURA.md) — arquitectura hexagonal del proyecto
- [`docs/CONTEXTO.md`](docs/CONTEXTO.md) — estado y decisiones de negocio, fase por fase
- [`docs/APRENDIZAJE.md`](docs/APRENDIZAJE.md) — conceptos de Go explicados en simple
- [`docs/CONCURRENCIA.md`](docs/CONCURRENCIA.md) — transacciones, race conditions, anti-doble-reserva, y el resumen final del proyecto
