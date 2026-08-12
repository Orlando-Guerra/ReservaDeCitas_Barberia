# CONCURRENCIA.md — Concurrencia, transacciones y el problema de la doble reserva

> Este es el documento más importante del proyecto. Explica, en simple,
> por qué la Fase 3 se resolvió como se resolvió.

## 1. ¿Qué es una transacción? ¿Qué es ACID en la práctica?

Una **transacción** es un grupo de operaciones sobre la base de datos que
Postgres trata como una sola unidad indivisible: o se aplican **todas**, o
no se aplica **ninguna**. `BEGIN` abre una, `COMMIT` la confirma
(aplicando todo), `ROLLBACK` la descarta (como si nunca hubiera pasado).

Cada sentencia SQL suelta (un `INSERT` solo, por ejemplo) ya corre dentro
de su propia transacción implícita, aunque no escribas `BEGIN`/`COMMIT` a
mano — por eso el `Guardar` de `RepositorioReservas` funciona
correctamente sin transacción explícita: un solo `INSERT` ya es atómico
por sí mismo.

**ACID** son cuatro garantías que Postgres promete para toda transacción:

- **Atomicidad**: todo o nada (lo que acabamos de explicar).
- **Consistencia**: una transacción nunca deja la base violando sus
  reglas (constraints, checks). Si violarla, Postgres la rechaza entera.
  Esto es clave para nosotros: el constraint de exclusión de `reservas`
  es, literalmente, una regla de consistencia que la base hace cumplir.
- **Aislamiento** (la "I" de ACID): mientras una transacción está en
  curso, otras transacciones no ven sus cambios a medias. Qué tan estricto
  es este aislamiento se puede ajustar (sección 2).
- **Durabilidad**: una vez que `COMMIT` devuelve éxito, el dato sobrevive
  aunque el servidor se caiga un segundo después (Postgres ya lo escribió
  a disco de forma segura).

En este proyecto usamos transacciones explícitas (`pool.Begin(ctx)`) en
un solo lugar: `RepositorioDiasBloqueados.Guardar` (ver sección 4). Todo
lo demás son sentencias sueltas, que ya son transacciones de una sola
operación.

## 2. Niveles de aislamiento en Postgres — ¿cuál usamos?

Postgres soporta cuatro niveles de aislamiento (de más permisivo a más
estricto): `READ UNCOMMITTED` (en Postgres se comporta igual que READ
COMMITTED, no existe realmente distinto), `READ COMMITTED`, `REPEATABLE
READ`, `SERIALIZABLE`. Más estricto = más garantías, pero más
transacciones abortadas por conflictos y más costo de rendimiento.

**Este proyecto usa el nivel por defecto de Postgres: `READ COMMITTED`.**
No lo cambiamos, y es una decisión deliberada, no un descuido: bajo `READ
COMMITTED`, cada sentencia dentro de una transacción ve los datos tal
como están confirmados en ese instante (no ve cambios de otras
transacciones aún sin confirmar, pero sí ve cambios ya confirmados por
otras transacciones que terminaron mientras la nuestra seguía abierta).

¿Por qué nos alcanza con el nivel más bajo, si el objetivo es evitar una
condición de carrera tan delicada como la doble reserva? Porque **no
dependemos del nivel de aislamiento para esa garantía** — dependemos de
un constraint a nivel de índice (sección 4), que Postgres hace cumplir
sin importar el nivel de aislamiento con el que se ejecute la
transacción. Es la diferencia central de este documento: subir el nivel
de aislamiento (hasta `SERIALIZABLE`) es **una** estrategia posible contra
las condiciones de carrera, pero no es la que elegimos, y explicamos por
qué en la sección 4.

## 3. ¿Qué es una *race condition*? ¿Por qué `if yaExiste {...}` no alcanza?

Una **race condition** (condición de carrera) ocurre cuando el resultado
de un programa depende del orden exacto — a nivel de microsegundos — en
que se intercalan operaciones de distintos hilos/goroutines/procesos, y
ese orden no está garantizado.

Imaginemos que, en vez del constraint de exclusión, hubiéramos escrito la
protección así, en Go, dentro de `Guardar`:

```go
// ESTO NO ESTÁ EN EL PROYECTO. Es un ejemplo de lo que NO hay que hacer.
func (r *RepositorioReservas) Guardar(ctx context.Context, reserva entidades.Reserva) error {
    yaExiste, _ := r.hayReservaEnEseHorario(ctx, reserva.Inicio, reserva.Fin)
    if yaExiste {
        return dominio.ErrSlotNoDisponible
    }
    return r.insertar(ctx, reserva) // INSERT real
}
```

Con dos goroutines (o dos peticiones HTTP de dos clientes distintos)
intentando reservar el mismo slot al mismo tiempo, esto puede pasar:

```
tiempo →
Goroutine A: hayReservaEnEseHorario() → false (todavía no hay nada)
Goroutine B: hayReservaEnEseHorario() → false (todavía no hay nada, A no insertó aún)
Goroutine A: insertar() → OK
Goroutine B: insertar() → OK   ← ¡DOBLE RESERVA!
```

Las dos goroutines hicieron el chequeo **antes** de que cualquiera
terminara de insertar. El chequeo `if yaExiste` era correcto en el
instante exacto en que se ejecutó — el problema es que entre "chequear" e
"insertar" hay una ventana de tiempo (por más chica que sea) en la que
otra goroutine puede colarse. Esto se llama **TOCTOU** (*time-of-check to
time-of-use*): el bug no está en la lógica del `if`, está en que hay dos
pasos separados donde debería haber uno solo.

El test [`reservas_concurrencia_test.go`](../internal/infraestructura/postgres/reservas_concurrencia_test.go)
existe exactamente para demostrar esto en la práctica: lanza 20 goroutines
que llaman a `Guardar` al mismo tiempo, sin ningún chequeo previo. Si la
protección dependiera de un `if` en Go como el de arriba, el test
fallaría de forma intermitente (a veces pasarían 1, a veces 2, a veces
más, dependiendo de qué tan rápido corra la máquina ese día). Con el
constraint de exclusión, pasa siempre 1 — lo corrimos y se confirmó:
exactamente 1 fila `confirmada` en la tabla, sin importar que 20
goroutines lo intentaran literalmente al mismo tiempo.

## 4. Estrategias contra la doble reserva, comparadas

### a) Constraint de unicidad simple (`UNIQUE`)

Un `UNIQUE (recurso_id, inicio)` evitaría que dos reservas empiecen
**exactamente** en el mismo instante. Pero no evita solapamientos
parciales: una reserva de 9:00–10:00 y otra de 9:30–10:30 no violan esa
unicidad (sus valores de `inicio` son distintos), y sin embargo se
solapan 30 minutos. En nuestro caso particular, como todos los slots son
bloques fijos de 1 hora alineados a la misma grilla, un `UNIQUE` a secas
casi alcanzaría — pero es frágil: alcanza solo mientras se cumpla esa
suposición de grilla fija, y deja de servir el día que alguien cambie las
reglas de negocio (por ejemplo, servicios de duración variable).

### b) Bloqueo pesimista (`SELECT ... FOR UPDATE`)

Consiste en, dentro de una transacción explícita, hacer un `SELECT` de
las filas candidatas con `FOR UPDATE`: eso le pide a Postgres que las
"bloquee" para que ninguna otra transacción pueda tocarlas hasta que la
nuestra termine (`COMMIT` o `ROLLBACK`). Cualquier otra transacción que
intente lo mismo sobre esas filas tiene que esperar en fila.

**Esta es la estrategia que sí usamos, pero para un problema distinto**:
en [`RepositorioDiasBloqueados.Guardar`](../internal/infraestructura/postgres/dias_bloqueados.go),
para la regla "no bloquear un rango que ya tiene reservas". No existe un
constraint declarativo que exprese esa regla (cruza dos tablas:
`dias_bloqueados` y `reservas`), así que la garantizamos a mano: dentro de
una transacción, bloqueamos con `FOR UPDATE` las filas de `reservas` que
caen en el rango a bloquear, chequeamos si hay alguna, y solo si no hay
ninguna insertamos el bloqueo — todo antes de liberar el bloqueo con
`COMMIT`. Mientras tanto, cualquier intento de reservar en ese rango
(que también toca esas filas) tiene que esperar a que termine.

Es una herramienta poderosa, pero tiene un costo: hay que acordarse de
usarla en **cada** lugar del código que toque esos datos de forma
insegura, y si alguien se olvida en un solo lugar, la protección
desaparece silenciosamente. Por eso no es la primera opción para la
doble reserva si existe una alternativa declarativa (la de abajo).

### c) Constraint de exclusión con `tstzrange` + `btree_gist` (la elegida para `reservas`)

```sql
EXCLUDE USING gist (recurso_id WITH =, rango WITH &&)
WHERE (estado = 'confirmada')
```

Esto le dice al motor: "para el mismo `recurso_id`, nunca aceptes dos
filas cuyos rangos de tiempo se solapen (`&&`)". A diferencia de las
otras dos estrategias:

- No depende de que el código Go recuerde hacer un chequeo antes de
  escribir — la regla vive en el **esquema**, no en ningún caso de uso.
  Aunque alguien escriba un `INSERT` a mano desde `psql`, sin pasar por
  Go para nada, el constraint igual se aplica.
- No requiere una transacción explícita ni bloquear filas a mano: el
  chequeo de solapamiento pasa a nivel del índice GiST, como parte
  intrínseca de cómo Postgres decide si puede insertar la fila o no. Por
  eso `RepositorioReservas.Guardar` es una sola sentencia `INSERT`, sin
  `BEGIN`/`COMMIT` visibles.
- Funciona igual de bien con cualquier nivel de aislamiento (por eso la
  sección 2 podía quedarse tranquila con `READ COMMITTED`).

`btree_gist` hace falta acá porque un índice GiST, por sí solo, solo
entiende operadores de tipos como rangos o geometría — no sabe comparar
igualdad de un `uuid` (`recurso_id`). La extensión le agrega esa
capacidad, permitiendo combinar una columna de igualdad con una de rango
en el mismo índice.

**Por qué esta es la elegida**: es la única de las tres que da la
garantía completa (solapamientos parciales incluidos) sin exigir
disciplina humana en cada punto de escritura. Es exactamente el patrón
recomendado por la documentación de Postgres para este tipo de problema
(reservas, turnos, alquileres — cualquier "nadie puede tener dos cosas en
el mismo rango de tiempo").

## 5. Por qué la garantía final tiene que vivir en la base de datos

Podríamos haber escrito la validación "¿está libre el slot?" en el
dominio de Go — de hecho, `dominio.CalcularSlotsDisponibles` (Fase 2) hace
algo parecido, marcando qué slots están ocupados. Pero esa función es
para **mostrarle opciones al usuario**, no para garantizar que no haya
doble reserva. Son dos problemas distintos:

- "Mostrar qué slots parecen libres ahora mismo" → puede vivir en Go,
  perfectamente, y de hecho vive ahí (es pura, testeable, sin tocar la
  base).
- "Garantizar que nunca se guarden dos reservas solapadas, pase lo que
  pase" → **no puede** vivir solo en Go, porque Go no tiene forma de ver
  ni bloquear lo que están haciendo, en simultáneo, otras instancias de la
  aplicación (recordemos: en producción probablemente haya más de un
  proceso de la API corriendo a la vez, todos hablando con la misma base).
  La única entidad que ve **todas** las escrituras, sin importar desde
  qué proceso vienen, es la propia base de datos. Por eso la garantía
  final — el último "no" antes de que el dato quede guardado — tiene que
  vivir ahí.

La regla general: **usá el dominio para lógica de negocio y para dar una
buena experiencia (mostrar qué está disponible); usá la base de datos
para garantías que no pueden fallar bajo ninguna concurrencia posible.**

## 6. `timestamp` vs. `timestamptz`, UTC, y horario de verano

Postgres tiene dos tipos de fecha/hora:

- `timestamp` (sin zona horaria): guarda literalmente los números que le
  diste ("2026-08-17 10:00:00"), sin ninguna noción de a qué zona horaria
  corresponden. Es ambiguo: ¿son las 10:00 en Argentina? ¿en UTC? ¿en
  Tokio? El tipo no lo sabe ni lo pregunta.
- `timestamptz` (con zona horaria — el que usamos en todo este proyecto,
  columnas `inicio`, `fin`, `creado_en`, etc.): internamente, Postgres
  siempre lo guarda como un instante absoluto en UTC. Cuando lo mostrás,
  Postgres lo convierte a la zona horaria de la sesión — pero el dato
  guardado es siempre el mismo instante real, sin importar desde qué zona
  horaria se escribió o se lee.

**En este proyecto, todo el dominio en Go trabaja en UTC** (mirá, por
ejemplo, `HoraDelDia.EnFecha` en `entidades/hora_del_dia.go`, que
construye explícitamente con `time.UTC`). La conversión a la hora local
del cliente (por ejemplo, hora de Argentina) es responsabilidad exclusiva
de la capa de presentación — cuando exista un frontend o se formatee un
correo, ahí y solo ahí se convierte para mostrar.

¿Por qué esta regla importa tanto? Por el **horario de verano (DST)**:
en zonas que lo aplican, un mismo día del año tiene 23 horas y otro tiene
25. Si guardáramos las reservas en hora local y calculáramos "una hora
después" sumando literalmente 60 minutos, ese cálculo se rompe dos veces
al año, justo en la transición de DST — una reserva de "10 a 11" podría
terminar siendo, en la práctica, de 10 a 12, o desaparecer una hora
entera. Trabajando siempre en UTC, sumar `1 * time.Hour` es sumar
sesenta minutos reales, siempre, sin ambigüedad — el problema del horario
de verano se resuelve solo, porque nunca se plantea en UTC (UTC no tiene
horario de verano). Argentina en particular no aplica DST desde 2009,
así que hoy no lo notaríamos en la práctica — pero la regla se aplica
igual, porque el código no debería depender de que una decisión de
política de un país no cambie nunca.

## 7. Resumen de lo implementado en la Fase 3

- Migración `000001`: extensión `btree_gist`.
- Migración `000006`: tabla `reservas` con columna generada `rango
  tstzrange` y el constraint `EXCLUDE USING gist (recurso_id WITH =,
  rango WITH &&) WHERE (estado = 'confirmada')`.
- `RepositorioReservas.Guardar`: un `INSERT` sin chequeo previo,
  confiando en que Postgres rechace el solapamiento; traduce el código de
  error `23P01` (`exclusion_violation`) a `dominio.ErrSlotNoDisponible`.
- `RepositorioDiasBloqueados.Guardar`: transacción explícita con `SELECT
  ... FOR UPDATE`, para una regla de negocio que sí necesita bloqueo
  pesimista porque cruza dos tablas.
- `reservas_concurrencia_test.go`: 20 goroutines reservando el mismo slot
  a la vez contra la base real; exactamente 1 gana, siempre — verificado
  también con una consulta directa a la tabla.

## 8. Resumen final — lo que este proyecto enseñó sobre concurrencia y fechas

Cerrando el proyecto (Fase 7), esto es lo que quedó demostrado en código,
no solo explicado en teoría:

- **Una transacción no es "protección automática".** Un `INSERT` suelto
  ya es una transacción de una sola operación — pero eso no evita una
  condición de carrera si la lógica que decide "¿puedo insertar?" vive
  afuera, en Go, separada del `INSERT` mismo (sección 3). La protección
  real vino de mover esa decisión *adentro* del motor de base de datos,
  como parte del propio `INSERT`.
- **Hay más de una herramienta contra las condiciones de carrera, y
  elegir la correcta depende del problema.** Este proyecto terminó
  usando las tres estrategias de la sección 4, cada una donde
  corresponde: un constraint de exclusión declarativo para "nunca dos
  reservas solapadas" (la regla vive en el esquema, sin que el código
  tenga que acordarse de nada); una transacción con `SELECT ... FOR
  UPDATE` para "no bloquear un rango con reservas" (una regla que cruza
  tablas, sin constraint declarativo posible); y ninguna de las dos para
  la mayoría de las operaciones, porque no todo necesita esta clase de
  protección — CRUD simple de servicios y horarios no tiene ningún
  problema de concurrencia real detrás.
- **La garantía final siempre quedó en Postgres, nunca en Go.** El
  cálculo de slots en el dominio (`CalcularSlotsDisponibles`) y la
  validación de `ServicioReservas.CrearReserva` existen para dar una
  buena experiencia — un error claro antes de intentar nada — pero
  ninguna de las dos es lo que impide la doble reserva de verdad. Eso lo
  demuestra `reservas_concurrencia_test.go` al saltarse esa validación
  por completo y llamar al repositorio directo: la protección sigue
  funcionando igual, porque no depende de que el código de arriba se
  porte bien.
- **UTC en todos lados, conversión solo al mostrar, no fue un detalle
  menor.** Terminó apareciendo dos veces en la práctica, no solo en la
  teoría de la sección 6: una vez como bug real (pgx devolviendo
  `time.Time` en la zona horaria local del proceso, Fase 5) y otra vez
  como diseño correcto (el correo de confirmación convierte a la hora de
  Buenos Aires justo en el borde donde el dato pasa a mostrársele a una
  persona, Fase 6, y en ningún otro lugar del sistema).
- **Concurrencia y condiciones de carrera no son exclusivas de la base de
  datos.** El mismo tipo de cuidado (con qué protegerse, y por qué) volvió
  a aparecer entre goroutines de Go puro: el envío asíncrono de correos
  (Fase 6) y los mocks con canales de los tests (Fase 7) tuvieron que
  pensar en qué puede leerse/escribirse desde dos goroutines a la vez, con
  las mismas preguntas de fondo que las transacciones de Postgres —
  aunque las herramientas concretas (`chan`, `sync.Mutex`,
  `sync.WaitGroup`) sean distintas a `FOR UPDATE` o `EXCLUDE`.
