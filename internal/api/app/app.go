// Paquete app arma la aplicación completa: config → pool de Postgres →
// repositorios → adaptadores → casos de uso → handlers → rutas. Vive acá
// (en vez de directamente en cmd/api/main.go) para que tanto el binario
// de siempre (cmd/api, usado en Railway/local) como la función serverless
// de Vercel (api/index.go) puedan construir la misma aplicación sin
// duplicar el cableado.
package app

import (
	"context"
	"fmt"
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
// entorno para arrancar.
type Config struct {
	AppPort     string
	DatabaseURL string
	SMTPHost    string
	SMTPPort    string
	SMTPFrom    string
	JWTSecret   string
	ZonaHoraria string
}

// CargarConfig lee la configuración desde variables de entorno. Si una
// variable no está definida, usa el mismo valor por defecto que
// .env.example, para que el proyecto funcione "out of the box" en
// desarrollo local.
func CargarConfig() Config {
	return Config{
		// Railway, Vercel (y otras plataformas) inyectan PORT en runtime y
		// esperan que el servidor escuche ahí; en local seguimos usando
		// APP_PORT. En Vercel esta variable no se usa (el runtime serverless
		// no escucha puertos), pero no molesta que quede seteada igual.
		AppPort: envODefault("PORT", envODefault("APP_PORT", "8080")),
		// DATABASE_URL es una connection string completa de Postgres
		// (postgres://user:pass@host:port/db?sslmode=...) — el formato que
		// entregan Neon, Supabase, Railway, etc. En local, .env.example ya
		// arma una a partir de los valores de docker-compose.yml.
		DatabaseURL: os.Getenv("DATABASE_URL"),
		SMTPHost:    envODefault("SMTP_HOST", "localhost"),
		SMTPPort:    envODefault("SMTP_PORT", "1025"),
		SMTPFrom:    envODefault("SMTP_FROM", "no-reply@barberia.local"),
		// Sin default: un fallback hardcodeado acá viviría en el repo
		// público de GitHub, y cualquiera podría usarlo para forjar JWTs de
		// administrador si nos olvidamos de definir esta variable en
		// producción.
		JWTSecret:   os.Getenv("JWT_SECRET"),
		ZonaHoraria: envODefault("ZONA_HORARIA_NEGOCIO", "America/Argentina/Buenos_Aires"),
	}
}

func envODefault(clave, porDefecto string) string {
	if valor := os.Getenv(clave); valor != "" {
		return valor
	}
	return porDefecto
}

// RelojSistema implementa puertos.Reloj devolviendo la hora real del
// sistema, siempre en UTC.
type RelojSistema struct{}

func (RelojSistema) Ahora() time.Time { return time.Now().UTC() }

// Construir conecta a la base y cablea todo el sistema: pool →
// repositorios → adaptadores → casos de uso → handlers → rutas. Devuelve
// el router HTTP ya armado y el pool (para poder cerrarlo prolijamente y
// para que /health pueda usarlo).
func Construir(ctx context.Context, cfg Config) (http.Handler, *pgxpool.Pool, error) {
	pool, err := postgres.NuevoPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, nil, fmt.Errorf("conectando a la base de datos: %w", err)
	}

	repoUsuarios := postgres.NuevoRepositorioUsuarios(pool)
	repoServicios := postgres.NuevoRepositorioServicios(pool)
	repoHorarios := postgres.NuevoRepositorioHorarios(pool)
	repoDiasBloqueados := postgres.NuevoRepositorioDiasBloqueados(pool)
	repoReservas := postgres.NuevoRepositorioReservas(pool)

	reloj := RelojSistema{}
	hasheador := seguridad.HasheadorBcrypt{}
	generadorTokens := seguridad.NuevoGeneradorTokensJWT([]byte(cfg.JWTSecret), 24*time.Hour)
	notificador, err := notificaciones.NuevoNotificadorSMTP(cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPFrom, cfg.ZonaHoraria)
	if err != nil {
		pool.Close()
		return nil, nil, fmt.Errorf("configurando el notificador de correo: %w", err)
	}

	servicioAuth := aplicacion.NuevoServicioAutenticacion(repoUsuarios, hasheador, generadorTokens, reloj)
	servicioClientes := aplicacion.NuevoServicioClientes(repoUsuarios, hasheador, reloj)
	servicioDisponibilidad := aplicacion.NuevoServicioDisponibilidad(repoServicios, repoHorarios, repoDiasBloqueados, repoReservas, reloj)
	servicioReservas := aplicacion.NuevoServicioReservas(repoUsuarios, repoServicios, repoHorarios, repoDiasBloqueados, repoReservas, notificador, reloj)
	servicioAdmin := aplicacion.NuevoServicioAdministracion(repoServicios, repoHorarios, repoDiasBloqueados)

	handlerAuth := handlers.NuevoHandlerAuth(servicioAuth)
	handlerClientes := handlers.NuevoHandlerClientes(servicioClientes)
	handlerDisponibilidad := handlers.NuevoHandlerDisponibilidad(servicioDisponibilidad)
	handlerReservas := handlers.NuevoHandlerReservas(servicioReservas)
	handlerServicios := handlers.NuevoHandlerServicios(servicioAdmin)
	handlerHorarios := handlers.NuevoHandlerHorarios(servicioAdmin)
	handlerBloqueos := handlers.NuevoHandlerBloqueos(servicioAdmin)

	autenticado := middleware.Autenticacion([]byte(cfg.JWTSecret))
	soloAdmin := func(h http.HandlerFunc) http.Handler {
		return autenticado(middleware.RequiereRol(entidades.RolAdministrador)(h))
	}
	soloCliente := func(h http.HandlerFunc) http.Handler {
		return autenticado(middleware.RequiereRol(entidades.RolCliente)(h))
	}
	cualquieraAutenticado := func(h http.HandlerFunc) http.Handler {
		return autenticado(h)
	}

	mux := http.NewServeMux()

	// Frontend estático: en cmd/api (Railway/local) se sirve desde acá. En
	// Vercel, web/ se sirve como estático nativo (ver vercel.json) y esta
	// ruta nunca se llega a invocar, pero no hace daño dejarla.
	mux.Handle("GET /", http.FileServer(http.Dir("web")))

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

	mux.HandleFunc("POST /auth/registro", handlerAuth.Registrar)
	mux.HandleFunc("POST /auth/login", handlerAuth.Login)

	mux.Handle("GET /servicios", cualquieraAutenticado(handlerServicios.Listar))
	mux.Handle("GET /slots", cualquieraAutenticado(handlerDisponibilidad.ConsultarSlots))
	mux.Handle("POST /reservas", soloCliente(handlerReservas.Crear))
	mux.Handle("POST /reservas/{id}/cancelar", cualquieraAutenticado(handlerReservas.Cancelar))
	mux.Handle("GET /reservas/mias", cualquieraAutenticado(handlerReservas.MisReservas))
	mux.Handle("GET /horarios", cualquieraAutenticado(handlerHorarios.Listar))
	mux.Handle("GET /dias-bloqueados", cualquieraAutenticado(handlerBloqueos.Listar))

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
