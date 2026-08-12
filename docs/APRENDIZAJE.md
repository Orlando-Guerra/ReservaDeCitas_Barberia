# APRENDIZAJE.md — Conceptos de Go explicados en simple

> Bitácora de conceptos nuevos de Go (para alguien que viene de Python /
> frontend), explicados a medida que van apareciendo en el código.

## `go.mod` y el module path (Fase 0)

En Python usás `pyproject.toml`/`requirements.txt` para dependencias, e
importás módulos por ruta relativa de carpetas. En Go, `go.mod` define un
**module path** (acá `reservas-go`) que es además el prefijo obligatorio
para importar tus propios paquetes internos, por ejemplo:

```go
import "reservas-go/internal/dominio/entidades"
```

Si el proyecto se sube a GitHub, normalmente el module path sería
`github.com/usuario/reservas-go`, para que otros puedan hacer `go get`.

## `internal/` (Fase 1)

`internal/` es una carpeta especial que el propio compilador de Go
reconoce por su nombre. **Cualquier paquete dentro de `internal/` solo
puede ser importado por código que esté en el mismo módulo, dentro del
directorio que contiene a `internal/` o sus subdirectorios.** No es una
convención — es una regla que el compilador impone.

¿Para qué sirve? Para marcar explícitamente "esto es un detalle interno de
implementación, no una API pública". Como todo nuestro código vive bajo
`internal/`, estamos diciendo: nada de este proyecto está pensado para que
otro proyecto lo importe como librería. Si algún día quisiéramos publicar,
por ejemplo, los DTOs como un paquete cliente reusable, ese paquete tendría
que vivir *fuera* de `internal/`.

Es parecido en espíritu a un módulo Python con un `_privado.py` (el guion
bajo como convención de "no tocar desde afuera"), pero en Go es una regla
real del compilador, no una convención que se pueda ignorar.

## `net/http` con patrones de método + ruta (Fase 1)

Desde Go 1.22, `http.ServeMux` (el router de la librería estándar) entiende
patrones como `"GET /health"`: método HTTP + ruta en el mismo string. Antes
de 1.22 había que chequear `r.Method` a mano dentro del handler. Por eso el
proyecto puede cumplir la regla de "no usar Gin/Echo/Fiber": la librería
estándar ya resuelve lo que antes era la razón principal para usar un
framework externo.

## `if err != nil` (Fase 1, va a aparecer todo el tiempo)

Go no tiene excepciones para errores esperables (sí tiene `panic`, pero se
reserva para errores de programación, no para "el archivo no existe" o
"la fecha es inválida"). En cambio, las funciones que pueden fallar
devuelven **dos valores**: el resultado y un `error`. La convención es
chequear ese error inmediatamente:

```go
if err := http.ListenAndServe(":"+cfg.AppPort, mux); err != nil {
    log.Fatalf("error al iniciar el servidor: %v", err)
}
```

Esto es muy distinto a Python, donde un error no manejado se propaga solo
como excepción. En Go, si no chequeás el error, el error queda ahí,
ignorado en silencio — por eso el patrón `if err != nil` aparece
constantemente y hay que tomarlo en serio: es la forma en que Go obliga a
pensar explícitamente qué pasa cuando algo falla.

## Goroutines y `sync.WaitGroup` (Fase 3)

Una **goroutine** es una función que corre de forma concurrente con el
resto del programa — se lanza con la palabra `go` delante de una llamada:

```go
go func(i int) {
    // esto corre "en paralelo" con el resto de main()
}(i)
```

Es parecido a lanzar un hilo (`thread`) o una tarea `async` de Python,
pero mucho más liviano: Go puede tener cientos de miles de goroutines
corriendo sin problema, porque el runtime de Go las multiplexa sobre unos
pocos hilos de sistema operativo reales (esto se llama *M:N scheduling*,
pero no hace falta el nombre técnico para usarlo).

El problema: `go func(){...}()` no espera a que la goroutine termine —
`main()` (o el test) seguiría de largo inmediatamente. Por eso, en el
test de concurrencia (`reservas_concurrencia_test.go`) usamos un
`sync.WaitGroup`, que es básicamente un contador seguro para usar desde
varias goroutines a la vez:

```go
var wg sync.WaitGroup
for i := 0; i < n; i++ {
    wg.Add(1)          // "una tarea más pendiente"
    go func(i int) {
        defer wg.Done() // "esta tarea ya terminó"
        // ... trabajo ...
    }(i)
}
wg.Wait() // bloquea hasta que el contador llegue a 0
```

Un detalle sutil: la goroutine recibe `i` **como parámetro** (`func(i
int)`) en vez de usar directamente la variable `i` del `for` de afuera.
Antes de Go 1.22 esto era obligatorio para evitar un bug clásico (todas
las goroutines terminaban compartiendo la misma variable `i`, viendo su
valor final en vez del que tenía cuando se lanzaron). Go 1.22 arregló
ese comportamiento por defecto, pero lo dejamos explícito igual: es más
claro de leer, y es el patrón que vas a encontrar en casi todo código Go
existente.

## `errors.As` vs. `errors.Is` (Fase 3)

Ya habíamos visto `errors.Is`, que pregunta "¿este error ES (o envuelve a)
este error específico que ya conozco?" — sirve para sentinels como
`dominio.ErrSlotNoDisponible`.

`errors.As` pregunta algo distinto: "¿en algún punto de esta cadena de
errores hay uno de ESTE TIPO, sin importar cuál sea su valor exacto?" — y
si lo encuentra, te lo entrega ya convertido a ese tipo, con todos sus
campos:

```go
var errPg *pgconn.PgError
if errors.As(err, &errPg) && errPg.Code == "23P01" {
    // errPg.Code, errPg.Message, errPg.Detail... ya están disponibles
}
```

Usamos esto en `RepositorioReservas.Guardar` porque necesitamos leer el
campo `Code` que Postgres manda en su error — no alcanza con saber "hubo
un error", necesitamos saber específicamente cuál, para decidir si es una
violación del constraint de exclusión (y traducirlo a
`dominio.ErrSlotNoDisponible`) o un problema distinto.

## El patrón `defer tx.Rollback(ctx)` (Fase 3)

En `RepositorioDiasBloqueados.Guardar` aparece este patrón, común en
código Go que maneja transacciones:

```go
tx, err := r.pool.Begin(ctx)
if err != nil { return err }
defer tx.Rollback(ctx)

// ... trabajo con tx ...

return tx.Commit(ctx)
```

`defer` programa una llamada para que se ejecute cuando la función
termine, sin importar por qué camino termine (un `return` normal, un
`return` con error, o incluso un `panic`). La idea acá es "por defecto,
deshacé todo" — y si en algún momento llegamos a `tx.Commit(ctx)` con
éxito, el `Rollback` que se ejecuta después (por el `defer`) ya no tiene
nada que deshacer: pgx lo detecta y no hace nada. Es una forma compacta
de asegurarse de que **ningún camino de salida** de la función deje una
transacción a medio abrir, sin tener que repetir `tx.Rollback(ctx)` en
cada `return` de error.

## `context.Context` (Fase 3, ya venía apareciendo desde antes)

Ya lo usábamos en las firmas de los puertos (`ctx context.Context` en
cada método de repositorio), pero en la Fase 3 se vuelve concreto: es el
mecanismo de Go para propagar **cancelación** y **plazos límite** (o
"deadlines") a través de llamadas anidadas.

En `cmd/api/main.go` aparecen dos usos distintos:

- Un contexto con timeout para el arranque (`context.WithTimeout(...,
  10*time.Second)`): si conectar a la base tarda más de 10 segundos,
  abortamos en vez de colgarnos para siempre.
- `r.Context()` dentro del handler de `/health`: cada pedido HTTP que
  llega ya trae su propio contexto, que `net/http` cancela solo si el
  cliente cierra la conexión antes de que terminemos de responder. Le
  agregamos encima nuestro propio timeout de 2 segundos para el `Ping` a
  la base.

La idea general: cualquier función que pueda tardar (llamadas de red,
consultas a la base) debería recibir un `context.Context` como primer
parámetro, para que quien la llama pueda decirle "no esperes más de tanto
tiempo" o "cancelá si ya no hace falta el resultado".

## Middleware como función de orden superior (Fase 4)

Sin un framework, "middleware" en Go no es más que una función con esta
forma:

```go
func Autenticacion(secreto []byte) func(http.Handler) http.Handler
```

Recibe un `http.Handler` (el siguiente paso de la cadena) y devuelve otro
`http.Handler` que hace algo extra alrededor. Se combinan envolviendo:

```go
protegido := middleware.Autenticacion(secreto)(middleware.RequiereRol(entidades.RolAdministrador)(handlerFinal))
```

Cada capa decide si sigue la cadena (`siguiente.ServeHTTP(w, r)`) o corta
acá (escribe una respuesta de error y hace `return` sin llamar a
`siguiente`). No hay magia: es una función que devuelve una función que
devuelve una función — lo que en otros lenguajes se llama "función de
orden superior" o "decorador".

## Autenticación vs. autorización (Fase 4)

Son dos preguntas distintas, y por eso son dos middlewares distintos:

- **Autenticación** = "¿quién sos, y puedo confiar en que sos de
  verdad quien decís ser?". La responde `Autenticacion`, verificando la
  firma del JWT.
- **Autorización** = "vos, que ya sé quién sos, ¿tenés permiso para
  hacer ESTO puntual?". La responde `RequiereRol`, mirando el rol.

## ¿Por qué hay que verificar el rol del lado del servidor si ya viene firmado en el JWT?

Precisamente porque *está* firmado, el cliente no puede **falsificar**
un rol distinto sin conocer el secreto (si lo intentara, `ValidarToken`
fallaría). Pero eso no es lo mismo que decir "entonces ya está
protegido". La firma garantiza que el dato no fue **alterado** — no
garantiza que solo se **use** donde corresponde.

Nada impide que un usuario con rol `cliente`, sabiendo (o adivinando) la
URL de un endpoint de administrador, mande la petición HTTP directamente
con curl o Postman, saltándose por completo cualquier botón que el
frontend hubiera ocultado para ese rol. Si el servidor confiara en que
"total, el frontend no muestra esa opción a los clientes" y no revisara
el rol en cada pedido, esa petición se ejecutaría igual — el backend
respondería exactamente lo mismo sin importar qué interfaz la generó.
**El backend es la única barrera real**, porque es el único lugar que
ningún cliente puede saltarse. Por eso `RequiereRol` corre en el
servidor, en cada pedido a una ruta protegida, sin excepciones — nunca
alcanza con "ocultarlo en el frontend".

## Claves de contexto con un tipo propio, no `string` (Fase 4)

En `Autenticacion` guardamos el usuario en el contexto con:

```go
type claveContexto int
const claveUsuarioAutenticado claveContexto = iota

ctx := context.WithValue(r.Context(), claveUsuarioAutenticado, usuario)
```

Si hubiéramos usado un `string` como clave (`"usuario"`), cualquier otro
paquete de la aplicación que también decidiera guardar algo en el
contexto con la clave `"usuario"` pisaría o leería el valor por
accidente — los `string` iguales son iguales, sin importar de qué
paquete vengan. Con un tipo propio, no exportado, definido adentro de
`middleware`, es imposible que otro paquete cree una clave que compare
igual a la nuestra, aunque adivine el valor numérico exacto: los tipos
no coinciden, así que `context.Value` nunca la va a confundir con otra
cosa. Es un patrón estándar de Go para este problema específico.

## DTOs y validación manual: ¿por qué Go no tiene un Pydantic? (Fase 5)

En Python, Pydantic (o FastAPI, que lo usa por debajo) valida automáticamente
con solo declarar tipos: `edad: int` ya rechaza un string, `email: EmailStr`
ya valida el formato. Eso funciona porque Python tiene introspección de tipos
en tiempo de ejecución muy rica, y Pydantic la explota con metaclases y
decoradores para generar validadores solos, a partir de las anotaciones de
tipo.

Go elige otro balance. `encoding/json` sí usa reflection para volcar un
JSON en un struct (por eso `json.NewDecoder(r.Body).Decode(&req)`
funciona sin que escribamos ningún parser), pero **ahí termina la magia**:
si el JSON dice `"duracion_minutos": "treinta"` y el campo es `int`,
`Decode` directamente falla (con un error genérico de tipo). Pero nada
adicional valida que `duracion_minutos` sea mayor a cero, o que `email`
tenga un `@` — Go no tiene un sistema de anotaciones lo bastante rico
como para expresar "y además validá esto" sin una librería externa, y
este proyecto eligió no sumar una (como `go-playground/validator`) para
quedarse con lo mínimo indispensable de la librería estándar.

Por eso cada DTO de `internal/api/dto/` tiene su propio método
`Validar() error`, escrito a mano:

```go
func (r CrearServicioRequest) Validar() error {
    if strings.TrimSpace(r.Nombre) == "" {
        return fmt.Errorf("el nombre es requerido")
    }
    if r.DuracionMinutos <= 0 {
        return fmt.Errorf("duracion_minutos debe ser mayor a 0")
    }
    ...
}
```

Es más código que una anotación declarativa, pero también es más
explícito: leyendo `Validar()` se sabe exactamente qué se exige, sin tener
que recordar qué significa cada anotación de una librería. Es una
compensación deliberada de Go en general: menos "magia" automática, más
código visible — la misma filosofía que ya vimos con `if err != nil` en
vez de excepciones.

## El bug real de zona horaria que encontramos probando la Fase 5

Al probar los endpoints con curl, el listado de administración devolvía
fechas como `"2026-08-14T06:00:00-04:00"` en vez de terminar en `"Z"`
(UTC) — inconsistente con las respuestas de crear una reserva, que sí
mostraban `"Z"`.

La causa: `pgx`, al decodificar una columna `timestamptz`, arma el
`time.Time` resultante usando la zona horaria **local del proceso Go**
(`time.Local`, que en esta máquina resultó ser UTC-4), no UTC. El
*instante* que representa siempre fue correcto — decodificar y volver a
codificar no pierde ni corrompe el momento exacto — pero el campo interno
`Location` del `time.Time` quedaba en la zona local, y `encoding/json`
usa esa `Location` para decidir cómo imprimir el string (`Z` para UTC,
`-04:00` para otra zona).

Esto es exactamente el tipo de problema que `docs/CONCURRENCIA.md` ya
advertía en abstracto ("guardamos todo en UTC, convertimos solo al
mostrar") — y acá apareció en la práctica, en el punto exacto donde los
datos entran desde la base. La solución: normalizar con `.UTC()`
explícitamente en el momento de leer cada fila (`escanearReserva`,
`escanearUsuario`, `escanearDiaBloqueado` en `internal/infraestructura/postgres/`),
así el resto del sistema — dominio, casos de uso, DTOs de salida — nunca
tiene que volver a pensar en esto.

## Envío asíncrono: goroutines sin canales, y qué pasa si falla (Fase 6)

`ServicioReservas.CrearReserva` termina así:

```go
if err := s.reservas.Guardar(ctx, reserva); err != nil {
    return entidades.Reserva{}, err
}
s.notificarConfirmacionAsync(reserva, servicio)
return reserva, nil
```

`notificarConfirmacionAsync` lanza una goroutine (`go func() { ... }()`) y
vuelve inmediatamente — no espera a que el correo termine de mandarse.
Quien llamó a `CrearReserva` (el handler HTTP) recibe la reserva ya
creada y le responde al cliente al toque, sin que la latencia de mandar
un correo (conectarse a un servidor SMTP, esperar su respuesta) se sume a
la latencia de la petición HTTP.

**¿Por qué esta goroutine no usa ningún canal (`chan`)?** Un canal sirve
para que dos goroutines se comuniquen: mandar un valor de una a otra, o
avisar "ya terminé". Acá no hace falta nada de eso — nadie necesita el
resultado del envío del correo. Es un patrón "fire-and-forget" (lanzar y
olvidarse). Si en cambio quisiéramos, por ejemplo, esperar a que
terminen varios envíos antes de seguir, ahí sí usaríamos algo como un
`sync.WaitGroup` (como en el test de concurrencia de la Fase 3) o un
canal para juntar resultados — pero eso resolvería un problema distinto
al que tenemos acá.

**¿Qué pasa si el envío falla?** Nada le pasa a la reserva — ya quedó
guardada en la base antes de lanzar la goroutine, con su propia
transacción implícita de un solo `INSERT`. Si Mailpit estuviera caído, o
la red fallara, lo único que ocurre es:

```go
if err := s.notificador.EnviarConfirmacionReserva(ctx, reserva, cliente, servicio); err != nil {
    log.Printf("no se pudo enviar el correo de confirmación de la reserva %s: %v", reserva.ID, err)
}
```

Se registra el error en el log del servidor, y ahí termina: no hay
ningún lugar al que devolver ese error (el handler HTTP ya respondió
hace rato), y no reintentamos automáticamente. Es una decisión
consciente: la reserva es el dato que importa y ya está a salvo; el
correo es una cortesía que puede fallar sin comprometer la operación.

**¿Por qué esta goroutine arma su propio `context.Context` en vez de
recibir el que le pasaron?** El contexto de un pedido HTTP se cancela
automáticamente en cuanto termina de escribirse la respuesta (`net/http`
lo hace solo). Como el handler responde bien antes de que la goroutine
llegue a conectarse al servidor de correo, usar ese mismo contexto
cortaría el envío casi siempre. Por eso se crea uno nuevo e
independiente: `context.WithTimeout(context.Background(), 10*time.Second)`
— vive por su cuenta, no depende de que el pedido HTTP original siga
"vivo".

## Tests de tabla (*table-driven tests*) (Fase 7)

Es el patrón más común de Go para testear una función con muchos casos
parecidos: en vez de escribir una función `Test...` por caso, se arma un
slice de structs (la "tabla"), cada uno con las entradas y lo que se
espera, y un solo bucle `for` los corre a todos con `t.Run`:

```go
casos := []struct {
    nombre string
    // ...entradas...
    // ...esperado...
}{
    {nombre: "caso 1", ...},
    {nombre: "caso 2", ...},
}

for _, caso := range casos {
    t.Run(caso.nombre, func(t *testing.T) {
        // ...ejecutar y comparar...
    })
}
```

`internal/dominio/disponibilidad_test.go` es el ejemplo del proyecto: 12
casos (sin horario, con reservas, con bloqueos parciales y completos,
límites exactos de tiempo...) comparten exactamente la misma lógica de
ejecución y verificación — lo único que cambia entre casos son los datos.
Agregar un caso nuevo es agregar una entrada al slice, no escribir una
función nueva. `t.Run(caso.nombre, ...)` además hace que cada caso
aparezca como un subtest con nombre propio en la salida de `go test -v`,
y que si uno falla, se sepa exactamente cuál sin tener que leer el
código.

## Mocks/fakes vía interfaces implícitas (Fase 7)

Todos los tests de `internal/aplicacion/` usan implementaciones en
memoria de los puertos (`internal/aplicacion/mocks_test.go`) en vez de
una base de datos real. Esto es una consecuencia directa de la Fase 2:
como los casos de uso dependen de interfaces (`puertos.RepositorioUsuarios`,
`puertos.Reloj`, etc.), y Go conecta cualquier tipo que tenga los métodos
correctos con esas interfaces sin declaración explícita, un
`repositorioUsuariosMemoria` (un mapa protegido con un mutex) sirve
exactamente igual que el `RepositorioUsuarios` real de
`infraestructura/postgres` — el caso de uso no tiene forma de notar la
diferencia. Esto es lo que hace que la arquitectura hexagonal "valga la
pena" en la práctica: no es solo prolijidad, es lo que permite testear
`ServicioReservas.CrearReserva` (con todas sus reglas de negocio) sin
levantar Postgres, en milisegundos, tantas veces como haga falta.

## Tests de integración con `httptest` (Fase 7)

`cmd/api/main_test.go` prueba la aplicación completa por HTTP de verdad,
sin mockear nada: usa `httptest.NewServer(handler)` para levantar un
servidor HTTP real (en un puerto libre elegido al azar por el sistema
operativo) que envuelve el mismo router que arma `cmd/api/main.go`, y le
manda pedidos con el cliente HTTP estándar de Go
(`http.DefaultClient.Do`). Es la diferencia entre "probé que la lógica es
correcta" (los tests de `aplicacion/` con mocks) y "probé que si alguien
le pega a `POST /reservas` con un JSON real, por una conexión TCP real,
contra una base Postgres real, la cosa entera funciona" — ambas capas de
test importan, y prueban cosas distintas.

Para que `cmd/api/main_test.go` pudiera reusar exactamente el mismo
cableado que usa el servidor real (y no una copia desactualizable a
mano), esta fase separó `construirAplicacion` de `main()`: `main()` ahora
es apenas cuatro líneas (cargar config, construir la app, loggear,
escuchar), y toda la lógica de armado vive en una función que tanto el
binario real como el test pueden llamar.
