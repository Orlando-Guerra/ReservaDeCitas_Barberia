# ARQUITECTURA.md — Arquitectura hexagonal en este proyecto

## La idea central

En arquitectura hexagonal (también llamada "puertos y adaptadores"), el
**dominio** — las reglas de negocio puras — no sabe nada del mundo exterior:
no sabe si los datos vienen de PostgreSQL o de un archivo, no sabe si se
expone por HTTP o por línea de comandos, no sabe cómo se manda un correo.

El dominio define **interfaces** (a las que llamamos "puertos"): contratos
que dicen "necesito algo que sepa guardar una reserva" o "necesito algo que
sepa mandar una notificación", sin decir cómo. La infraestructura (Postgres,
SMTP, etc.) implementa esos contratos (los "adaptadores"). El dominio nunca
importa la infraestructura; es la infraestructura la que se adapta al
dominio.

Regla dura de este proyecto: **el paquete `dominio` no importa nada fuera
de la librería estándar de Go.** Ni `pgx`, ni `jwt`, ni nada de terceros.

## Las carpetas y qué va en cada una

```
internal/
├── dominio/          # El núcleo. Reglas de negocio puras.
│   ├── entidades/     # Structs: Usuario, Servicio, Reserva, etc.
│   ├── puertos/        # Interfaces: RepositorioReservas, Notificador, Reloj...
│   └── errores.go       # Errores de negocio (ej. "slot ya reservado")
│
├── aplicacion/        # Casos de uso: orquestan el dominio.
│                       # Ej.: "crear una reserva" = validar reglas del
│                       # dominio + llamar al puerto del repositorio.
│                       # Tampoco sabe de HTTP ni de SQL directamente.
│
├── infraestructura/    # Adaptadores: implementan los puertos del dominio.
│   ├── postgres/        # Implementa los repositorios usando pgx.
│   ├── seguridad/        # bcrypt, JWT.
│   └── notificaciones/    # Adaptador SMTP (Mailpit en dev).
│
└── api/                # Adaptador de entrada: expone los casos de uso
    ├── handlers/          # por HTTP.
    ├── middleware/         # Autenticación y autorización.
    └── dto/                # Structs de entrada/salida HTTP (JSON).
```

## Por qué importa en la práctica

Con esto separado, podemos:

- **Testear el dominio sin base de datos.** El cálculo de slots disponibles
  (el corazón de la Fase 2) se prueba con Go puro, sin levantar Postgres.
- **Cambiar de proveedor de correo sin tocar el dominio ni los casos de
  uso.** Hoy es Mailpit por SMTP; el día de mañana puede ser Resend o
  SendGrid — solo cambia el adaptador en `infraestructura/notificaciones`,
  porque todo el resto del código depende del puerto `Notificador`
  (interfaz), no del adaptador concreto.
- **Controlar el tiempo en los tests** con un puerto `Reloj` (más detalle
  en la Fase 2), en vez de que el dominio llame directamente a
  `time.Now()`.

## `cmd/api/main.go`: el único lugar que conoce todo

Es el único archivo del proyecto que tiene permitido importar tanto el
dominio como la infraestructura concreta (Postgres, SMTP) — porque su
trabajo es exactamente ese: "cablear" (conectar) las piezas. Crea las
instancias concretas de los adaptadores y se las pasa a los casos de uso a
través de las interfaces del dominio. Todavía no hace este cableado (eso
empieza en la Fase 3, cuando exista algo que conectar); por ahora solo
levanta el servidor HTTP con un endpoint de salud.
