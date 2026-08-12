package postgres_test

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"reservas-go/internal/dominio"
	"reservas-go/internal/dominio/entidades"
	"reservas-go/internal/infraestructura/postgres"
)

// dsnPruebaPorDefecto apunta al Postgres de docker-compose.yml (el mismo
// que usa la app en desarrollo). Se puede pisar con la variable de
// entorno DATABASE_URL si hiciera falta apuntar a otro lado.
const dsnPruebaPorDefecto = "postgres://reservas_user:reservas_pass@localhost:5434/reservas_db?sslmode=disable"

// conectarDBDePrueba abre un pool contra la base de datos real. Este test
// es un test de INTEGRACIÓN: a propósito no usa ningún mock, porque lo
// que queremos probar es precisamente que la garantía vive en Postgres,
// no en Go. Si no hay una base disponible (por ejemplo, corriendo
// "go test ./..." sin haber levantado "docker compose up"), el test se
// omite en vez de fallar — así no rompe una corrida rápida de tests que
// no tenía intención de tocar la base de datos.
func conectarDBDePrueba(t *testing.T) *pgxpool.Pool {
	t.Helper()

	if testing.Short() {
		t.Skip("test de integración: se omite con -short")
	}

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = dsnPruebaPorDefecto
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := postgres.NuevoPool(ctx, dsn)
	if err != nil {
		t.Skipf("no se pudo conectar a Postgres para el test de integración (¿está corriendo 'docker compose up -d'?): %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// limpiarTablas deja las tablas relevantes vacías antes de correr el
// test, para que sea repetible: si lo corrés dos veces seguidas, no debe
// importar qué datos quedaron de la corrida anterior.
func limpiarTablas(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	_, err := pool.Exec(ctx, `TRUNCATE reservas, usuarios, servicios, horarios_atencion, dias_bloqueados CASCADE`)
	if err != nil {
		t.Fatalf("limpiando tablas antes del test: %v", err)
	}
}

// TestRepositorioReservas_Concurrencia_SoloUnaGana es el test central de
// todo el proyecto: lanza "n" goroutines que intentan reservar
// exactamente el mismo slot al mismo tiempo, y comprueba que la base de
// datos deja pasar una sola.
//
// A propósito NO hay ningún chequeo previo tipo "¿está libre el slot?"
// antes de llamar a Guardar en cada goroutine: estamos probando el peor
// caso posible, donde n peticiones llegan literalmente al mismo tiempo y
// ninguna sabe de la existencia de las otras. Si la protección viviera
// solo en el código Go (un "if ya existe, rechazar"), este test fallaría
// — más de una goroutine pasaría el chequeo antes de que cualquiera
// terminara de escribir. La única razón por la que este test puede pasar
// es el constraint EXCLUDE de la migración 000006_crear_reservas.
func TestRepositorioReservas_Concurrencia_SoloUnaGana(t *testing.T) {
	pool := conectarDBDePrueba(t)
	ctx := context.Background()

	limpiarTablas(t, ctx, pool)

	repoUsuarios := postgres.NuevoRepositorioUsuarios(pool)
	repoServicios := postgres.NuevoRepositorioServicios(pool)
	repoReservas := postgres.NuevoRepositorioReservas(pool)

	ahora := time.Now().UTC()

	// Un cliente y un servicio "de utilería": todas las goroutines van a
	// usar los mismos, porque lo que queremos poner a prueba es el
	// solapamiento de horario, no la variedad de clientes.
	cliente, err := entidades.NuevoUsuario("Cliente de prueba", "concurrencia@prueba.com", "hash-de-prueba", entidades.RolCliente, ahora)
	if err != nil {
		t.Fatalf("no se esperaba error creando el cliente: %v", err)
	}
	if err := repoUsuarios.Guardar(ctx, cliente); err != nil {
		t.Fatalf("no se esperaba error guardando el cliente: %v", err)
	}

	servicio, err := entidades.NuevoServicio("Corte de prueba", 60, 150000)
	if err != nil {
		t.Fatalf("no se esperaba error creando el servicio: %v", err)
	}
	if err := repoServicios.Guardar(ctx, servicio); err != nil {
		t.Fatalf("no se esperaba error guardando el servicio: %v", err)
	}

	// El slot que van a pelear todas las goroutines: dentro de 2 días a
	// las 10:00 UTC. Cualquier instante futuro fijo sirve; lo importante
	// es que las n reservas usen exactamente el mismo "inicio".
	inicioSlot := time.Date(ahora.Year(), ahora.Month(), ahora.Day(), 10, 0, 0, 0, time.UTC).AddDate(0, 0, 2)

	const n = 20

	var wg sync.WaitGroup
	// Cada goroutine escribe en su propia posición de "errs" (la [i] que
	// recibe como parámetro), así que no hace falta un mutex para
	// protegerla: nunca dos goroutines escriben en la misma posición del
	// slice al mismo tiempo. Si en cambio hubiéramos usado "append" desde
	// varias goroutines a la vez, ahí sí haría falta sincronización,
	// porque append no es seguro para uso concurrente.
	errs := make([]error, n)

	for i := 0; i < n; i++ {
		wg.Add(1) // sumamos 1 al contador del WaitGroup antes de lanzar la goroutine
		go func(i int) {
			defer wg.Done() // al terminar esta goroutine, resta 1 al contador

			reserva, err := entidades.NuevaReserva(cliente.ID, servicio.ID, inicioSlot, time.Now().UTC())
			if err != nil {
				errs[i] = err
				return
			}
			errs[i] = repoReservas.Guardar(ctx, reserva)
		}(i)
	}
	wg.Wait() // bloquea hasta que las n goroutines hayan llamado a Done()

	var exitosas, rechazadasPorSlotOcupado int
	for i, err := range errs {
		switch {
		case err == nil:
			exitosas++
		case errors.Is(err, dominio.ErrSlotNoDisponible):
			rechazadasPorSlotOcupado++
		default:
			t.Errorf("goroutine %d: error inesperado (no es ErrSlotNoDisponible): %v", i, err)
		}
	}

	if exitosas != 1 {
		t.Errorf("se esperaba exactamente 1 reserva exitosa, hubo %d", exitosas)
	}
	if rechazadasPorSlotOcupado != n-1 {
		t.Errorf("se esperaban %d reservas rechazadas por slot ocupado, hubo %d", n-1, rechazadasPorSlotOcupado)
	}
}
