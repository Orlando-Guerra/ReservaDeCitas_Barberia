package main

// Test de integración de los endpoints: arma la aplicación completa
// (construirAplicacion) contra una base de datos Postgres real — la
// misma que docker-compose.yml levanta para desarrollo — y la ejercita
// por HTTP de punta a punta con httptest, tal como se probó a mano con
// curl durante las Fases 5 y 6. Se salta solo si no hay una base
// disponible, con el mismo patrón que
// internal/infraestructura/postgres/reservas_concurrencia_test.go.

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"reservas-go/internal/dominio/entidades"
	"reservas-go/internal/infraestructura/postgres"
	"reservas-go/internal/infraestructura/seguridad"
)

func configDePrueba() Config {
	return Config{
		AppPort:     "0",
		DBHost:      envODefault("DB_HOST", "localhost"),
		DBPort:      envODefault("DB_PORT", "5434"),
		DBUser:      envODefault("DB_USER", "reservas_user"),
		DBPassword:  envODefault("DB_PASSWORD", "reservas_pass"),
		DBName:      envODefault("DB_NAME", "reservas_db"),
		DBSSLMode:   envODefault("DB_SSLMODE", "disable"),
		SMTPHost:    envODefault("SMTP_HOST", "localhost"),
		SMTPPort:    envODefault("SMTP_PORT", "1025"),
		SMTPFrom:    "no-reply@barberia.local",
		JWTSecret:   "secreto-de-integracion",
		ZonaHoraria: "America/Argentina/Buenos_Aires",
	}
}

// levantarServidorDePrueba arma la aplicación completa y la sirve con
// httptest.NewServer. Deja las tablas limpias y con un administrador ya
// creado (simulando lo que cmd/seed-admin hace en desarrollo real).
func levantarServidorDePrueba(t *testing.T) (url string, adminEmail, adminPassword string) {
	t.Helper()
	if testing.Short() {
		t.Skip("test de integración: se omite con -short")
	}

	cfg := configDePrueba()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	handler, pool, err := construirAplicacion(ctx, cfg)
	if err != nil {
		t.Skipf("no se pudo conectar a Postgres para el test de integración (¿está corriendo 'docker compose up -d'?): %v", err)
	}
	t.Cleanup(pool.Close)

	_, err = pool.Exec(ctx, `TRUNCATE reservas, usuarios, servicios, horarios_atencion, dias_bloqueados CASCADE`)
	if err != nil {
		t.Fatalf("limpiando tablas antes del test: %v", err)
	}

	adminEmail = "admin-integracion@prueba.com"
	adminPassword = "adminPassword123"
	hasheador := seguridad.HasheadorBcrypt{}
	hash, err := hasheador.Hashear(adminPassword)
	if err != nil {
		t.Fatalf("no se esperaba error hasheando: %v", err)
	}
	admin, err := entidades.NuevoUsuario("Admin de prueba", adminEmail, hash, entidades.RolAdministrador, time.Now().UTC())
	if err != nil {
		t.Fatalf("no se esperaba error creando el admin: %v", err)
	}
	if err := postgres.NuevoRepositorioUsuarios(pool).Guardar(ctx, admin); err != nil {
		t.Fatalf("no se esperaba error guardando el admin: %v", err)
	}

	servidor := httptest.NewServer(handler)
	t.Cleanup(servidor.Close)
	return servidor.URL, adminEmail, adminPassword
}

// peticion es un helper mínimo para no repetir el armado de
// http.NewRequest + Content-Type + Authorization en cada llamada.
func peticion(t *testing.T, metodo, url, token string, cuerpo any) *http.Response {
	t.Helper()

	var lector *bytes.Reader
	if cuerpo != nil {
		datos, err := json.Marshal(cuerpo)
		if err != nil {
			t.Fatalf("no se esperaba error codificando el cuerpo: %v", err)
		}
		lector = bytes.NewReader(datos)
	} else {
		lector = bytes.NewReader(nil)
	}

	req, err := http.NewRequest(metodo, url, lector)
	if err != nil {
		t.Fatalf("no se esperaba error armando el pedido: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("no se esperaba error haciendo el pedido: %v", err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	return resp
}

func decodificar(t *testing.T, resp *http.Response, destino any) {
	t.Helper()
	if err := json.NewDecoder(resp.Body).Decode(destino); err != nil {
		t.Fatalf("no se esperaba error decodificando la respuesta: %v", err)
	}
}

// TestIntegracion_FlujoCompleto sigue, de punta a punta, el mismo camino
// que se validó a mano con curl en las Fases 5 y 6: registro, login,
// alta de servicio y horario por el admin, consulta de slots, creación y
// rechazo de doble reserva, y las reglas de cancelación.
func TestIntegracion_FlujoCompleto(t *testing.T) {
	url, adminEmail, adminPassword := levantarServidorDePrueba(t)

	// --- login del admin ---
	var loginAdmin struct {
		Token string `json:"token"`
	}
	resp := peticion(t, http.MethodPost, url+"/auth/login", "", map[string]string{
		"email": adminEmail, "password": adminPassword,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login admin: código = %d, se esperaba 200", resp.StatusCode)
	}
	decodificar(t, resp, &loginAdmin)
	tokenAdmin := loginAdmin.Token

	// --- registro público nunca crea administradores ---
	var registroResp struct {
		Rol string `json:"rol"`
	}
	resp = peticion(t, http.MethodPost, url+"/auth/registro", "", map[string]string{
		"nombre": "Cliente Integración", "email": "cliente-integracion@prueba.com",
		"password": "clientePassword123", "rol": "administrador",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("registro cliente: código = %d, se esperaba 201", resp.StatusCode)
	}
	decodificar(t, resp, &registroResp)
	if registroResp.Rol != "cliente" {
		t.Errorf("rol tras registro = %q, se esperaba \"cliente\" (el registro nunca debe poder crear administradores)", registroResp.Rol)
	}

	// --- login del cliente ---
	var loginCliente struct {
		Token string `json:"token"`
	}
	resp = peticion(t, http.MethodPost, url+"/auth/login", "", map[string]string{
		"email": "cliente-integracion@prueba.com", "password": "clientePassword123",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login cliente: código = %d, se esperaba 200", resp.StatusCode)
	}
	decodificar(t, resp, &loginCliente)
	tokenCliente := loginCliente.Token

	// --- rutas protegidas ---
	t.Run("sin token, 401", func(t *testing.T) {
		resp := peticion(t, http.MethodGet, url+"/servicios", "", nil)
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("código = %d, se esperaba 401", resp.StatusCode)
		}
	})
	t.Run("cliente en ruta de admin, 403", func(t *testing.T) {
		resp := peticion(t, http.MethodPost, url+"/admin/servicios", tokenCliente, map[string]any{
			"nombre": "x", "duracion_minutos": 30, "precio_centavos": 100,
		})
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("código = %d, se esperaba 403", resp.StatusCode)
		}
	})

	// --- el admin crea un servicio y define el horario de hoy y mañana ---
	var servicioResp struct {
		ID string `json:"id"`
	}
	resp = peticion(t, http.MethodPost, url+"/admin/servicios", tokenAdmin, map[string]any{
		"nombre": "Corte de integración", "duracion_minutos": 30, "precio_centavos": 150000,
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("crear servicio: código = %d, se esperaba 201", resp.StatusCode)
	}
	decodificar(t, resp, &servicioResp)

	ahora := time.Now().UTC()
	for _, dia := range []time.Weekday{ahora.Weekday(), ahora.AddDate(0, 0, 1).Weekday()} {
		resp = peticion(t, http.MethodPost, url+"/admin/horarios", tokenAdmin, map[string]any{
			"dia_semana": int(dia), "hora_inicio": "00:00", "hora_fin": "23:59",
		})
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("definir horario: código = %d, se esperaba 200", resp.StatusCode)
		}
	}

	// --- el cliente consulta slots de mañana y reserva el primero disponible ---
	manana := ahora.AddDate(0, 0, 1).Format("2006-01-02")
	resp = peticion(t, http.MethodGet, url+"/slots?fecha="+manana+"&servicio_id="+servicioResp.ID, tokenCliente, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("consultar slots: código = %d, se esperaba 200", resp.StatusCode)
	}
	var slots []struct {
		Inicio     time.Time `json:"inicio"`
		Disponible bool      `json:"disponible"`
	}
	decodificar(t, resp, &slots)

	var inicioElegido time.Time
	for _, s := range slots {
		if s.Disponible {
			inicioElegido = s.Inicio
			break
		}
	}
	if inicioElegido.IsZero() {
		t.Fatal("no se encontró ningún slot disponible para mañana")
	}

	var reservaResp struct {
		ID string `json:"id"`
	}
	resp = peticion(t, http.MethodPost, url+"/reservas", tokenCliente, map[string]string{
		"servicio_id": servicioResp.ID,
		"inicio":      inicioElegido.Format(time.RFC3339),
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("crear reserva: código = %d, se esperaba 201", resp.StatusCode)
	}
	decodificar(t, resp, &reservaResp)

	// --- reservar el mismo slot de nuevo choca contra el constraint de la Fase 3 ---
	t.Run("doble reserva sobre el mismo slot, 409", func(t *testing.T) {
		resp := peticion(t, http.MethodPost, url+"/reservas", tokenCliente, map[string]string{
			"servicio_id": servicioResp.ID,
			"inicio":      inicioElegido.Format(time.RFC3339),
		})
		if resp.StatusCode != http.StatusConflict {
			t.Errorf("código = %d, se esperaba 409", resp.StatusCode)
		}
	})

	// --- cancelar: depende de qué tan cerca está el turno ---
	t.Run("cancelar la reserva", func(t *testing.T) {
		resp := peticion(t, http.MethodPost, url+"/reservas/"+reservaResp.ID+"/cancelar", tokenCliente, nil)
		faltaMenosDe2Horas := time.Until(inicioElegido) < 2*time.Hour

		switch {
		case faltaMenosDe2Horas && resp.StatusCode != http.StatusUnprocessableEntity:
			t.Errorf("faltan menos de 2hs: código = %d, se esperaba 422", resp.StatusCode)
		case !faltaMenosDe2Horas && resp.StatusCode != http.StatusNoContent:
			t.Errorf("faltan más de 2hs: código = %d, se esperaba 204", resp.StatusCode)
		}
	})

	// --- el admin siempre puede cancelar, sin importar el plazo ---
	t.Run("el admin cancela sin restricción de plazo", func(t *testing.T) {
		resp := peticion(t, http.MethodPost, url+"/reservas/"+reservaResp.ID+"/cancelar", tokenAdmin, nil)
		// Puede ser 204 (si el cliente no la había cancelado ya en el
		// subtest anterior) o 409 ErrReservaYaCancelada (si sí) — las dos
		// son respuestas correctas según el orden en que corrieron los
		// subtests; lo que NO puede pasar es 422 (al admin no le aplica
		// el plazo de 2 horas).
		if resp.StatusCode == http.StatusUnprocessableEntity {
			t.Errorf("código = %d: al admin no debería aplicarle el plazo de 2 horas", resp.StatusCode)
		}
	})
}
