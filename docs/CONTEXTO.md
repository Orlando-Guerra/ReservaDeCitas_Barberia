# CONTEXTO.md — Estado del proyecto

> Este documento se actualiza al cerrar cada fase. Es la fuente de verdad de
> las decisiones de negocio y técnicas tomadas durante el desarrollo.

## Proyecto

**Plataforma de Reservas / Citas** — segundo proyecto de aprendizaje de
Orlando, en Go, con arquitectura hexagonal. El objetivo es aprender backend
a fondo, no terminar rápido.

Caso de uso elegido: **Barbería**, con **un solo recurso** (un único
calendario — un barbero/la barbería como entidad indivisible). No hay
múltiples barberos en la v1; la arquitectura debe permitir agregarlo después
sin romper el dominio.

## Estado actual

**Fase 0 y Fase 1 completadas.**

- Go `1.26.5` instalado (cumple el mínimo de 1.23+ pedido).
- Docker corriendo. Ya existen otros contenedores del usuario en la máquina
  (Postgres de otro proyecto en el puerto 5433, pgAdmin, n8n, etc.) — se
  eligieron puertos que no chocan: Postgres de este proyecto en `5434`,
  Mailpit en `1025` (SMTP) y `8025` (web UI).
- `golang-migrate` CLI instalado (`go install`, con soporte para
  PostgreSQL), disponible como `migrate` en el PATH.
- `make` no venía instalado en Windows; se instaló con
  `winget install GnuWin32.Make` y se agregó al PATH de usuario (hace
  falta reabrir la terminal para que una terminal nueva lo vea).
- Git inicializado, rama `feature/setup-inicial` creada.
- `go.mod` creado: `module reservas-go`, `go 1.26.5`.

### Fase 1 — Esqueleto, Docker y configuración

- Estructura de carpetas completa según arquitectura hexagonal (ver
  `docs/ARQUITECTURA.md`), con `.gitkeep` en las carpetas que todavía no
  tienen código (se van llenando en las próximas fases).
- `docker-compose.yml`: servicios `db` (Postgres 16, volumen persistente
  `reservas_db_data`, puerto `5434`) y `mailpit` (SMTP falso, puertos
  `1025`/`8025`). Verificado: ambos contenedores levantan `healthy`.
- `.env.example` (plantilla versionada) y `.env` (real, gitignored) con la
  configuración de puerto de la app, conexión a Postgres y SMTP.
- `Makefile` con targets `run`, `build`, `test`, `docker-up`,
  `docker-down`, `migrate-up`, `migrate-down`, `migrate-create`.
- `cmd/api/main.go`: lee la configuración desde variables de entorno y
  levanta un servidor HTTP mínimo con `net/http` (patrón `"GET /health"` de
  Go 1.22+). Probado: compila (`go vet`, `go build`), levanta, y
  `GET /health` responde `200 {"estado":"ok"}`.
- `.gitignore` (ignora `.env`, binarios, artefactos de IDE).
- Documentación creada: `docs/ARQUITECTURA.md` (capas hexagonales y qué va
  en cada carpeta), `docs/APRENDIZAJE.md` (conceptos Go explicados:
  `go.mod`, `internal/`, `net/http` con patrones, `if err != nil`),
  `docs/CONCURRENCIA.md` (esqueleto, se completa a fondo en la Fase 3),
  `README.md` con instrucciones de arranque.
- Todavía **no hay código de dominio ni conexión real a la base de datos**
  desde Go — `main.go` solo lee la configuración y expone `/health`. Eso
  arranca en la Fase 2 (dominio) y Fase 3 (persistencia).

## Reglas de negocio (definidas en Fase 0)

### Slots y disponibilidad

- **Granularidad fija de 1 hora**: cada reserva ocupa siempre un bloque
  completo de 1 hora, sin importar la duración configurada del servicio.
  La duración del servicio queda como dato informativo (y de precio), no
  afecta el cálculo de slots en la v1.
- Los slots se muestran en tiempo real como ocupados/libres.

### Anticipación (cliente normal, rol Cliente)

- **Máxima**: el cliente solo puede ver y reservar slots de **hoy o de
  mañana** (día calendario desde el momento de la consulta), nunca más
  adelante.
- **Mínima**: no se puede reservar un slot que ya comenzó o quedó en el
  pasado; si el cliente no reserva dentro de la hora en curso, debe esperar
  al siguiente slot disponible.

### Reserva manual del administrador (rol Administrador)

- El barbero puede crear una reserva manualmente para un cliente presencial
  (walk-in) a la hora que decida.
- Esta reserva **salta el límite de anticipación máxima de 1 día** (puede
  agendar más allá de mañana), pero **respeta el horario de atención
  configurado** (no puede agendar fuera de las horas en que la barbería
  atiende).
- **Ninguna reserva, ni siquiera la del administrador, puede saltarse el
  constraint anti-doble-reserva.** Esa garantía vive siempre en la base de
  datos (ver Fase 3 / `docs/CONCURRENCIA.md`).

### Cancelación

- El cliente puede cancelar libremente hasta **2 horas antes** del turno.
  Pasado ese margen, no se permite cancelar desde la app.

### Horario semanal y días de descanso

- El horario de atención se define por día de la semana
  (`HorarioAtencion`). Un día de descanso fijo (ej. domingo) simplemente
  **no tiene horario configurado** ese día de la semana — no se modela como
  `DiaBloqueado`.
- `DiaBloqueado` queda para excepciones puntuales: feriados, vacaciones, y
  el caso de **bloqueo de emergencia** (ver abajo).

### Bloqueo de emergencia (extensión de `DiaBloqueado`)

Caso de uso: el barbero tiene una emergencia y no puede trabajar.

- Puede bloquear **desde una hora elegida hasta el fin del día de hoy**, o
  **el día completo de mañana**.
- Esto **no genera slots nuevos** en el rango bloqueado.
- Regla de validación importante (más estricta que un `DiaBloqueado`
  simple): **el bloqueo solo se permite si no hay reservas ya creadas** en
  el rango que se quiere bloquear. Si ya hay una reserva confirmada ahí, el
  sistema debe rechazar el bloqueo (el admin tendría que cancelar esa
  reserva primero).
- Modelado propuesto (a confirmar en Fase 2): `DiaBloqueado` con un campo
  `hora_desde` opcional — si es `NULL`, bloquea el día completo; si tiene
  valor, bloquea desde esa hora hasta el fin del día.

### Fase 2 — Dominio

- Entidades creadas en `internal/dominio/entidades/`: `Usuario`,
  `Servicio`, `HorarioAtencion`, `HoraDelDia` (value type propio, Go no
  tiene un tipo "solo hora"), `Reserva`, `DiaBloqueado`.
- `ID` (`internal/dominio/entidades/id.go`): UUID v4 generado a mano con
  `crypto/rand`, sin depender de una librería externa — decisión tomada
  para respetar la regla de que `dominio` solo importa la librería
  estándar, ya elegiste UUID como estrategia de identificadores.
- `dominio.CalcularSlotsDisponibles` (`internal/dominio/disponibilidad.go`):
  función pura que calcula los slots de un día (disponibles u ocupados) a
  partir de horario, bloqueos, reservas existentes y la hora actual — sin
  tocar la base de datos. Tiene tests desde ya
  (`disponibilidad_test.go`), la batería completa queda para la Fase 7.
- Puertos definidos en `internal/dominio/puertos/`: `Reloj`,
  `Notificador`, y los cuatro repositorios (`RepositorioUsuarios`,
  `RepositorioServicios`, `RepositorioHorarios`,
  `RepositorioDiasBloqueados`, `RepositorioReservas`).
- Errores de dominio (`errors.Is`) en `internal/dominio/errores.go`:
  `ErrSlotNoDisponible`, `ErrCancelacionFueraDePlazo`,
  `ErrReservaYaCancelada`, `ErrDiaBloqueadoConReservas` — se van a usar
  recién en la Fase 5 (casos de uso).

### Fase 3 — Persistencia y el problema de la doble reserva

- 6 migraciones aplicadas (`migrations/000001` a `000006`): extensión
  `btree_gist`, y las tablas `usuarios`, `servicios`,
  `horarios_atencion`, `dias_bloqueados`, `reservas`.
- **Decisión clave**: la tabla `reservas` tiene una columna `recurso_id`
  (hoy siempre un UUID constante, `postgres.RecursoUnicoID`), aunque el
  dominio modela un solo recurso. Se agregó para poder usar el patrón
  completo de constraint de exclusión con `btree_gist`
  (`recurso_id WITH =, rango WITH &&`), dejando la tabla lista para
  múltiples recursos el día de mañana sin necesitar una migración que
  rompa nada. El dominio (`entidades.Reserva`) NO conoce este concepto —
  vive solo en la capa de persistencia.
- El constraint `reservas_no_solapadas` (`EXCLUDE USING gist`) es lo que
  garantiza, a nivel de base de datos, que nunca se guarden dos reservas
  confirmadas solapadas — con cualquier cantidad de escrituras
  concurrentes. Comparación completa de estrategias en
  `docs/CONCURRENCIA.md`.
- Repositorios implementados en `internal/infraestructura/postgres/`
  (`usuarios.go`, `servicios.go`, `horarios.go`, `dias_bloqueados.go`,
  `reservas.go`) usando pgx/v5 con pool de conexiones
  (`pgxpool.Pool`, ver `pool.go`).
- `RepositorioReservas.Guardar` traduce el código de error de Postgres
  `23P01` (`exclusion_violation`) a `dominio.ErrSlotNoDisponible`.
- `RepositorioDiasBloqueados.Guardar` usa una transacción explícita con
  `SELECT ... FOR UPDATE` (bloqueo pesimista) para la regla "no bloquear
  un rango que ya tiene reservas" — la única regla del proyecto que
  cruza dos tablas y por eso no tiene un constraint declarativo posible.
- **Test central del proyecto**:
  `reservas_concurrencia_test.go` — 20 goroutines reservando el mismo
  slot al mismo tiempo contra la base real; verificado que gana
  exactamente 1 (confirmado también con una consulta directa a la tabla).
  Es un test de integración: se salta solo (`t.Skip`) si Postgres no está
  disponible.
- `cmd/api/main.go` ahora conecta el pool de pgx al arrancar, y
  `GET /health` hace un `Ping` real a la base.
- `docs/CONCURRENCIA.md` completo: ACID, niveles de aislamiento, race
  conditions, comparación de las 3 estrategias anti-doble-reserva,
  `timestamp` vs `timestamptz` y por qué todo se guarda en UTC.

### Fase 4 — Autenticación y autorización por rol

- `internal/infraestructura/seguridad/`: `password.go` (bcrypt —
  `HashearPassword`, `VerificarPassword`) y `jwt.go` (`golang-jwt/jwt/v5`
  — `GenerarToken`, `ValidarToken`, con chequeo explícito del algoritmo de
  firma para evitar ataques de "confusión de algoritmo").
- El JWT viaja como `Authorization: Bearer <token>` (decisión confirmada)
  y lleva el `Rol` del usuario adentro de sus claims.
- `internal/api/middleware/`: `Autenticacion` (valida el JWT, deja al
  usuario en el `context.Context` del pedido) y `RequiereRol`
  (autorización por rol) — dos middlewares separados a propósito, cada
  uno respondiendo una pregunta distinta (ver `docs/APRENDIZAJE.md`).
- `JWT_SECRET` agregado a la configuración (`.env`, `.env.example`,
  `Config` en `cmd/api/main.go`) — todavía no se usa en el arranque,
  porque las rutas HTTP reales llegan en la Fase 5.
- Tests con `httptest` para ambos middlewares (401 sin token, 401 con
  token inválido, 200 con token válido y rol correcto, 403 con rol
  incorrecto) y para `seguridad` (round-trip de hash/verificación, tokens
  expirados, secreto incorrecto).
- Todo el proyecto sigue compilando y testeando limpio (`gofmt`, `go vet`,
  `go build`, `go test ./...`).

### Fase 5 — Casos de uso y endpoints

- Nuevos errores de dominio: `ErrNoEncontrado`, `ErrCredencialesInvalidas`,
  `ErrEmailYaRegistrado`, `ErrNoAutorizado`, `ErrFechaFueraDeAnticipacion`.
- Nuevos puertos: `HasheadorPasswords` y `GeneradorTokens` (para que los
  casos de uso no dependan de bcrypt/JWT directamente — se pueden testear
  con implementaciones falsas y rápidas).
- `internal/aplicacion/`: `ServicioAutenticacion` (registro siempre
  `RolCliente`, nunca acepta el rol del pedido), `ServicioClientes`
  (alta de clientes walk-in con contraseña temporal aleatoria),
  `ServicioDisponibilidad`, `ServicioReservas` (crear/cancelar/listar),
  `ServicioAdministracion` (servicios, horarios, bloqueos).
- `internal/api/dto/`: DTOs con validación manual (`Validar() error`, sin
  librerías externas — ver `docs/APRENDIZAJE.md`).
- `internal/api/handlers/`: un handler por área, más
  `manejarError` centralizado que traduce errores de dominio a códigos
  HTTP (409 solapamiento/email duplicado, 422 fuera de plazo/anticipación,
  401 credenciales, 403 no autorizado, 404 no encontrado, 500 genérico sin
  filtrar detalles internos al cliente).
- **Decisión confirmada**: la reserva walk-in del admin requiere que el
  cliente ya exista como `Usuario` (`POST /admin/clientes` lo crea con una
  contraseña temporal aleatoria si hace falta) — no se agregaron campos
  nulos a la tabla `reservas`.
- **Decisión confirmada**: el registro público (`POST /auth/registro`)
  jamás puede crear administradores. La cuenta de admin se crea con
  `cmd/seed-admin` (`go run ./cmd/seed-admin`, lee `ADMIN_EMAIL` /
  `ADMIN_PASSWORD`), un segundo punto de entrada que reutiliza todo el
  código de `internal/`.
- `cmd/api/main.go` cablea por primera vez todo el sistema: pool → 5
  repositorios → `RelojSistema` + `HasheadorBcrypt` + `GeneradorTokensJWT`
  → 5 servicios de aplicación → 7 handlers → rutas con
  `middleware.Autenticacion` / `RequiereRol`.
- **Bug real encontrado y corregido probando con curl**: pgx decodifica
  `timestamptz` con la zona horaria local del proceso, no UTC — se
  normaliza con `.UTC()` al leer cada fila en `infraestructura/postgres/`
  (detalle completo en `docs/APRENDIZAJE.md`).
- Probado end-to-end con curl contra la base real: registro, login (con
  intento de auto-asignarse rol admin, rechazado), consulta de slots
  (con el límite de anticipación del cliente), creación y rechazo de
  doble reserva, cancelación con y sin las 2 horas de plazo, cancelación
  cruzada entre clientes (403), flujo completo de walk-in, bloqueo de día
  rechazado por reservas existentes, 401/403 en rutas protegidas.
- Suite completa (`gofmt`, `go vet`, `go build`, `go test ./...`) sigue
  limpia.

### Fase 6 — Notificaciones por correo

- `internal/infraestructura/notificaciones/`: `NotificadorSMTP` implementa
  `puertos.Notificador` con `net/smtp` (sin autenticación, así funciona
  Mailpit) y plantillas HTML embebidas con `go:embed`
  (`plantillas/confirmacion.html`, `plantillas/cancelacion.html`).
- Conversión a hora local: el dominio y los casos de uso siguen en UTC
  siempre; `NotificadorSMTP` es el único lugar que convierte a la zona
  horaria del negocio (`ZONA_HORARIA_NEGOCIO`, default
  `America/Argentina/Buenos_Aires`) — justo en el borde donde el dato
  pasa a mostrársele a una persona.
- `ServicioReservas.CrearReserva` y `CancelarReserva` disparan el envío
  del correo correspondiente en una goroutine aparte
  (`notificarConfirmacionAsync` / `notificarCancelacionAsync`), con su
  propio `context.Context` independiente del pedido HTTP — no bloquean
  la respuesta, y un fallo de envío solo se registra en el log (la
  reserva ya está guardada, eso no se revierte).
- **Dos bugs reales encontrados probando contra Mailpit de verdad** (no
  solo compilando): (1) los `time.Time` que vuelven de Postgres traían la
  zona horaria local del proceso en vez de UTC — corregido en la Fase 5;
  (2) el asunto y el cuerpo de los correos con tildes se corrompían por
  no declarar la codificación de los headers (RFC 2047, vía
  `mime.QEncoding`) ni el `Content-Transfer-Encoding` del cuerpo —
  corregido acá. Ambos quedaron documentados en `docs/APRENDIZAJE.md`.
- Probado end-to-end: reserva → correo de confirmación en Mailpit con
  tildes correctas y hora ya convertida a Buenos Aires; cancelación →
  correo de cancelación en Mailpit.
- Suite completa (`gofmt`, `go vet`, `go build`, `go test ./...`) sigue
  limpia.

### Fase 7 — Tests y cierre

- `internal/dominio/disponibilidad_test.go`: reescrito como test de tabla
  (`[]struct{...}` + `t.Run`), 12 casos — incluye límites exactos (slot
  que empieza justo "ahora", solapamiento no alineado a la grilla,
  múltiples bloqueos el mismo día) que antes no estaban cubiertos.
- `internal/aplicacion/mocks_test.go`: implementaciones en memoria de
  los 9 puertos del dominio (repositorios, `Notificador`, `Reloj`,
  `HasheadorPasswords`, `GeneradorTokens`), usadas por
  `autenticacion_test.go` y `reservas_test.go` para testear las reglas de
  negocio de los casos de uso sin base de datos real. El notificador
  falso usa canales para poder verificar, de forma determinística, que
  la notificación asíncrona (Fase 6) se disparó.
- `internal/infraestructura/postgres/dias_bloqueados_test.go`: test de
  integración nuevo para la regla "no bloquear un rango con reservas"
  (antes solo se había probado a mano con curl en la Fase 5).
- `cmd/api/main.go` refactorizado: se extrajo `construirAplicacion(ctx,
  cfg)` de `main()`, para que el test de integración pueda levantar la
  aplicación completa sin duplicar el cableado.
- `cmd/api/main_test.go`: test de integración de punta a punta con
  `httptest` contra Postgres real — registro, login, alta de servicio y
  horario, consulta de slots, doble reserva rechazada, reglas de
  cancelación.
- `Makefile`: `test` corre los paquetes en secuencia (`-p 1`), porque dos
  paquetes de test tocan la misma base real; se agregó `test-rapido`
  (`-short`) para correr solo lo que no toca la base.
- `docs/CONCURRENCIA.md` cerrado con un resumen final (sección 8).
- `README.md` reescrito: instrucciones completas de arranque (incluido
  `cmd/seed-admin`), tabla de todos los endpoints, cómo correr los tests,
  estructura del proyecto.
- Suite completa verificada junta: `gofmt`, `go vet`, `go build`,
  `go test -p 1 ./...` (con Postgres real) y `go test -short ./...`
  (sin tocar la base) — ambas limpias.

## Estado final

Las 8 fases del plan original están completas. El proyecto tiene:
autenticación con JWT y roles, CRUD completo de administración,
consulta de disponibilidad, reservas con garantía anti-doble-reserva a
nivel de base de datos (verificada con un test de concurrencia real),
notificaciones por correo asíncronas, y una suite de tests en tres
niveles (dominio puro, casos de uso con mocks, integración end-to-end).
