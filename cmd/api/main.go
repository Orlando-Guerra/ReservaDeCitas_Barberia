// Comando api es el único punto de entrada de la aplicación. Acá se lee
// la configuración, se conecta a la base de datos, se instancian los
// repositorios y los casos de uso, y se cablean las rutas HTTP con sus
// middlewares — es el único archivo del proyecto con permiso para
// conocer, a la vez, el dominio, los casos de uso y todos los adaptadores
// de infraestructura concretos.
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"reservas-go/internal/api/handlers"
	"reservas-go/internal/api/middleware"
	"reservas-go/internal/aplicacion"
	"reservas-go/internal/dominio/entidades"
	"reservas-go/internal/infraestructura/notificaciones"
	"reservas-go/internal/infraestructura/postgres"
	"reservas-go/internal/infraestructura/seguridad"
)

// Config agrupa todos los valores que la aplicación necesita leer del
// entorno para arrancar: puerto HTTP, datos de conexión a PostgreSQL,
// datos del servidor SMTP (Mailpit en desarrollo) y el secreto de JWT.
type Config struct {
	AppPort     string
	DBHost      string
	DBPort      string
	DBUser      string
	DBPassword  string
	DBName      string
	DBSSLMode   string
	SMTPHost    string
	SMTPPort    string
	SMTPFrom    string
	JWTSecret   string
	ZonaHoraria string
}

// DSN arma la cadena de conexión a PostgreSQL que espera pgx, a partir de
// los campos sueltos de Config.
func (c Config) DSN() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s", c.DBUser, c.DBPassword, c.DBHost, c.DBPort, c.DBName, c.DBSSLMode)
}

// cargarConfig lee la configuración desde variables de entorno. Si una
// variable no está definida, usa el mismo valor por defecto que
// .env.example, para que el proyecto funcione "out of the box" en
// desarrollo local.
func cargarConfig() Config {
	return Config{
		AppPort:     envODefault("APP_PORT", "8080"),
		DBHost:      envODefault("DB_HOST", "localhost"),
		DBPort:      envODefault("DB_PORT", "5434"),
		DBUser:      envODefault("DB_USER", "reservas_user"),
		DBPassword:  envODefault("DB_PASSWORD", "reservas_pass"),
		DBName:      envODefault("DB_NAME", "reservas_db"),
		DBSSLMode:   envODefault("DB_SSLMODE", "disable"),
		SMTPHost:    envODefault("SMTP_HOST", "localhost"),
		SMTPPort:    envODefault("SMTP_PORT", "1025"),
		SMTPFrom:    envODefault("SMTP_FROM", "no-reply@barberia.local"),
		JWTSecret:   envODefault("JWT_SECRET", "clave-secreta-de-desarrollo-cambiar-en-produccion"),
		ZonaHoraria: envODefault("ZONA_HORARIA_NEGOCIO", "America/Argentina/Buenos_Aires"),
	}
}

// envODefault devuelve el valor de la variable de entorno "clave", o
// "porDefecto" si esa variable no está definida (o está vacía).
func envODefault(clave, porDefecto string) string {
	if valor := os.Getenv(clave); valor != "" {
		return valor
	}
	return porDefecto
}

// RelojSistema implementa puertos.Reloj devolviendo la hora real del
// sistema, siempre en UTC. Es la única implementación de Reloj que se usa
// fuera de los tests (que usan un reloj fijo — ver docs/APRENDIZAJE.md,
// Fase 2).
type RelojSistema struct{}

// Ahora devuelve time.Now() convertido a UTC.
func (RelojSistema) Ahora() time.Time { return time.Now().UTC() }

// construirAplicacion conecta a la base y cablea todo el sistema: pool →
// repositorios → adaptadores → casos de uso → handlers → rutas. Devuelve
// el router HTTP ya armado y el pool (para poder cerrarlo prolijamente y
// para que /health pueda usarlo).
//
// Está separado de main() a propósito: así cmd/api/main_test.go puede
// levantar la aplicación completa contra una base de test real y
// ejercitarla con httptest, sin duplicar toda esta lista de cableado en
// el archivo de test.
func construirAplicacion(ctx context.Context, cfg Config) (http.Handler, *pgxpool.Pool, error) {
	pool, err := postgres.NuevoPool(ctx, cfg.DSN())
	if err != nil {
		return nil, nil, fmt.Errorf("conectando a la base de datos: %w", err)
	}

	// --- Repositorios (adaptadores de persistencia) ---
	repoUsuarios := postgres.NuevoRepositorioUsuarios(pool)
	repoServicios := postgres.NuevoRepositorioServicios(pool)
	repoHorarios := postgres.NuevoRepositorioHorarios(pool)
	repoDiasBloqueados := postgres.NuevoRepositorioDiasBloqueados(pool)
	repoReservas := postgres.NuevoRepositorioReservas(pool)

	// --- Otros adaptadores ---
	reloj := RelojSistema{}
	hasheador := seguridad.HasheadorBcrypt{}
	generadorTokens := seguridad.NuevoGeneradorTokensJWT([]byte(cfg.JWTSecret), 24*time.Hour)
	notificador, err := notificaciones.NuevoNotificadorSMTP(cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPFrom, cfg.ZonaHoraria)
	if err != nil {
		pool.Close()
		return nil, nil, fmt.Errorf("configurando el notificador de correo: %w", err)
	}

	// --- Casos de uso (aplicación), cableados con los puertos de arriba ---
	servicioAuth := aplicacion.NuevoServicioAutenticacion(repoUsuarios, hasheador, generadorTokens, reloj)
	servicioClientes := aplicacion.NuevoServicioClientes(repoUsuarios, hasheador, reloj)
	servicioDisponibilidad := aplicacion.NuevoServicioDisponibilidad(repoServicios, repoHorarios, repoDiasBloqueados, repoReservas, reloj)
	servicioReservas := aplicacion.NuevoServicioReservas(repoUsuarios, repoServicios, repoHorarios, repoDiasBloqueados, repoReservas, notificador, reloj)
	servicioAdmin := aplicacion.NuevoServicioAdministracion(repoServicios, repoHorarios, repoDiasBloqueados)

	// --- Handlers HTTP ---
	handlerAuth := handlers.NuevoHandlerAuth(servicioAuth)
	handlerClientes := handlers.NuevoHandlerClientes(servicioClientes)
	handlerDisponibilidad := handlers.NuevoHandlerDisponibilidad(servicioDisponibilidad)
	handlerReservas := handlers.NuevoHandlerReservas(servicioReservas)
	handlerServicios := handlers.NuevoHandlerServicios(servicioAdmin)
	handlerHorarios := handlers.NuevoHandlerHorarios(servicioAdmin)
	handlerBloqueos := handlers.NuevoHandlerBloqueos(servicioAdmin)

	// --- Middlewares ---
	// autenticado exige un JWT válido, sin importar el rol.
	// soloAdmin además exige que el rol sea Administrador. Se combinan
	// envolviendo un middleware con el otro (ver docs/APRENDIZAJE.md,
	// "middleware como función de orden superior").
	autenticado := middleware.Autenticacion([]byte(cfg.JWTSecret))
	soloAdmin := func(h http.HandlerFunc) http.Handler {
		return autenticado(middleware.RequiereRol(entidades.RolAdministrador)(h))
	}
	// soloCliente es para el único endpoint que no tiene sentido que use
	// un administrador: "reservame un turno a mí mismo". El admin no es
	// un cliente del negocio — si necesita crear una reserva para
	// alguien (incluso presencialmente para sí mismo como cliente de
	// paso), existe POST /admin/reservas, que pide explícitamente un
	// cliente_id en vez de inferirlo del token.
	soloCliente := func(h http.HandlerFunc) http.Handler {
		return autenticado(middleware.RequiereRol(entidades.RolCliente)(h))
	}
	cualquieraAutenticado := func(h http.HandlerFunc) http.Handler {
		return autenticado(h)
	}

	mux := http.NewServeMux()

	// Frontend estático (HTML/CSS/JS puro, sin build step): vive en
	// web/ y se sirve desde el mismo servidor que la API, en el mismo
	// origen — así el JS del frontend puede hacer fetch() a /auth/login,
	// /reservas, etc. sin tener que lidiar con CORS. El patrón "GET /"
	// es un catch-all: solo atiende los pedidos que ninguna ruta más
	// específica de arriba (como "GET /servicios") ya haya capturado.
	mux.Handle("GET /", http.FileServer(http.Dir("web")))

	// Salud del servicio (sin autenticación).
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		w.Header().Set("Content-Type", "application/json")
		if err := pool.Ping(ctx); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{"estado":"error","detalle":"no se pudo conectar a la base de datos"}`))
			return
		}
		w.Write([]byte(`{"estado":"ok"}`))
	})

	// Autenticación (públicas).
	mux.HandleFunc("POST /auth/registro", handlerAuth.Registrar)
	mux.HandleFunc("POST /auth/login", handlerAuth.Login)

	// Cliente (o administrador): cualquier usuario autenticado.
	mux.Handle("GET /servicios", cualquieraAutenticado(handlerServicios.Listar))
	mux.Handle("GET /slots", cualquieraAutenticado(handlerDisponibilidad.ConsultarSlots))
	mux.Handle("POST /reservas", soloCliente(handlerReservas.Crear))
	mux.Handle("POST /reservas/{id}/cancelar", cualquieraAutenticado(handlerReservas.Cancelar))
	mux.Handle("GET /reservas/mias", cualquieraAutenticado(handlerReservas.MisReservas))
	// El calendario del cliente necesita saber qué días de la semana
	// atiende el barbero y qué días están bloqueados, para pintar el mes
	// — son los mismos handlers que ya usa el panel de administración
	// (GET /admin/horarios, GET /admin/dias-bloqueados), reexpuestos acá
	// sin exigir rol admin: no es información sensible, es el horario
	// público del negocio.
	mux.Handle("GET /horarios", cualquieraAutenticado(handlerHorarios.Listar))
	mux.Handle("GET /dias-bloqueados", cualquieraAutenticado(handlerBloqueos.Listar))

	// Administrador.
	mux.Handle("POST /admin/servicios", soloAdmin(handlerServicios.Crear))
	mux.Handle("PUT /admin/servicios/{id}", soloAdmin(handlerServicios.Actualizar))
	mux.Handle("POST /admin/horarios", soloAdmin(handlerHorarios.Definir))
	mux.Handle("GET /admin/horarios", soloAdmin(handlerHorarios.Listar))
	mux.Handle("POST /admin/dias-bloqueados", soloAdmin(handlerBloqueos.Crear))
	mux.Handle("DELETE /admin/dias-bloqueados/{id}", soloAdmin(handlerBloqueos.Eliminar))
	mux.Handle("GET /admin/dias-bloqueados", soloAdmin(handlerBloqueos.Listar))
	mux.Handle("POST /admin/clientes", soloAdmin(handlerClientes.Crear))
	mux.Handle("POST /admin/reservas", soloAdmin(handlerReservas.CrearAdmin))
	mux.Handle("GET /admin/reservas", soloAdmin(handlerReservas.ListarAdmin))

	return mux, pool, nil
}

func main() {
	cfg := cargarConfig()

	ctxArranque, cancelarArranque := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelarArranque()

	mux, pool, err := construirAplicacion(ctxArranque, cfg)
	if err != nil {
		log.Fatalf("error armando la aplicación: %v", err)
	}
	defer pool.Close()

	log.Printf("conectado a la base de datos en %s:%s/%s", cfg.DBHost, cfg.DBPort, cfg.DBName)
	log.Printf("servidor escuchando en http://localhost:%s", cfg.AppPort)

	if err := http.ListenAndServe(":"+cfg.AppPort, mux); err != nil {
		log.Fatalf("error al iniciar el servidor: %v", err)
	}
}
