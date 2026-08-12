package aplicacion_test

// Implementaciones falsas (en memoria) de cada puerto del dominio, para
// testear los casos de uso de aplicacion/ sin tocar una base de datos
// real ni un servidor SMTP real. Como cada puerto es una interfaz
// pequeña (Fase 2), alcanza con que estos structs tengan los métodos
// correctos — el compilador conecta todo solo (interfaces implícitas).
//
// Viven en su propio archivo _test.go, no en un paquete de producción:
// nada de esto tiene sentido fuera de los tests.

import (
	"context"
	"fmt"
	"sync"
	"time"

	"reservas-go/internal/dominio"
	"reservas-go/internal/dominio/entidades"
	"reservas-go/internal/dominio/puertos"
)

// relojFijo implementa puertos.Reloj devolviendo siempre el mismo
// instante — así los tests son determinísticos (ver docs/APRENDIZAJE.md,
// Fase 2).
//
// Ahora() tiene receptor puntero (*relojFijo) a propósito, no de valor:
// algunos tests necesitan cambiar "momento" después de haber construido
// el caso de uso (para simular "pasó el tiempo" entre dos llamadas). Si
// el receptor fuera de valor, el caso de uso ya habría recibido su
// propia copia del reloj en el momento de construirse, y cambiar el
// original no tendría ningún efecto sobre esa copia.
type relojFijo struct{ momento time.Time }

func (r *relojFijo) Ahora() time.Time { return r.momento }

// hasheadorFalso implementa puertos.HasheadorPasswords sin usar bcrypt de
// verdad — bcrypt es deliberadamente lento, y no queremos que los tests
// de casos de uso paguen ese costo por algo que ya se testea aparte
// (internal/infraestructura/seguridad/password_test.go).
type hasheadorFalso struct{}

func (hasheadorFalso) Hashear(password string) (string, error) {
	return "hash:" + password, nil
}

func (hasheadorFalso) Verificar(hashAlmacenado, passwordIngresada string) bool {
	return hashAlmacenado == "hash:"+passwordIngresada
}

// generadorTokensFalso implementa puertos.GeneradorTokens sin JWT real.
type generadorTokensFalso struct{}

func (generadorTokensFalso) Generar(usuarioID entidades.ID, rol entidades.Rol) (string, error) {
	return fmt.Sprintf("token-de-prueba:%s:%s", usuarioID, rol), nil
}

// notificadorFalso implementa puertos.Notificador registrando cada envío
// en un canal, en vez de mandar un correo real. Los casos de uso que
// notifican lo hacen desde una goroutine aparte (ver Fase 6) — por eso
// los tests que quieran comprobar "¿se mandó el correo?" tienen que leer
// de estos canales con un timeout, no revisar un campo a secas
// (leer un campo normal desde el test, mientras otra goroutine lo
// escribe al mismo tiempo, sería exactamente la clase de condición de
// carrera que docs/CONCURRENCIA.md explica para la base de datos — acá
// pasa lo mismo, pero entre dos goroutines de Go en vez de dos
// transacciones de Postgres).
type notificadorFalso struct {
	confirmaciones chan entidades.Reserva
	cancelaciones  chan entidades.Reserva
}

func nuevoNotificadorFalso() *notificadorFalso {
	return &notificadorFalso{
		confirmaciones: make(chan entidades.Reserva, 1),
		cancelaciones:  make(chan entidades.Reserva, 1),
	}
}

func (n *notificadorFalso) EnviarConfirmacionReserva(ctx context.Context, reserva entidades.Reserva, cliente entidades.Usuario, servicio entidades.Servicio) error {
	n.confirmaciones <- reserva
	return nil
}

func (n *notificadorFalso) EnviarCancelacionReserva(ctx context.Context, reserva entidades.Reserva, cliente entidades.Usuario, servicio entidades.Servicio) error {
	n.cancelaciones <- reserva
	return nil
}

// repositorioUsuariosMemoria implementa puertos.RepositorioUsuarios
// guardando todo en un map protegido por un mutex — hace falta el mutex
// porque las notificaciones asíncronas (Fase 6) pueden leer este
// repositorio desde una goroutine distinta a la del test.
type repositorioUsuariosMemoria struct {
	mu       sync.Mutex
	porID    map[entidades.ID]entidades.Usuario
	porEmail map[string]entidades.ID
}

func nuevoRepositorioUsuariosMemoria() *repositorioUsuariosMemoria {
	return &repositorioUsuariosMemoria{
		porID:    make(map[entidades.ID]entidades.Usuario),
		porEmail: make(map[string]entidades.ID),
	}
}

func (r *repositorioUsuariosMemoria) Guardar(ctx context.Context, usuario entidades.Usuario) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, existe := r.porEmail[usuario.Email]; existe {
		return fmt.Errorf("guardando usuario: %w", dominio.ErrEmailYaRegistrado)
	}
	r.porID[usuario.ID] = usuario
	r.porEmail[usuario.Email] = usuario.ID
	return nil
}

func (r *repositorioUsuariosMemoria) BuscarPorEmail(ctx context.Context, email string) (entidades.Usuario, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	id, existe := r.porEmail[email]
	if !existe {
		return entidades.Usuario{}, dominio.ErrNoEncontrado
	}
	return r.porID[id], nil
}

func (r *repositorioUsuariosMemoria) BuscarPorID(ctx context.Context, id entidades.ID) (entidades.Usuario, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	usuario, existe := r.porID[id]
	if !existe {
		return entidades.Usuario{}, dominio.ErrNoEncontrado
	}
	return usuario, nil
}

func (r *repositorioUsuariosMemoria) ActualizarPassword(ctx context.Context, id entidades.ID, passwordHash string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	usuario, existe := r.porID[id]
	if !existe {
		return dominio.ErrNoEncontrado
	}
	usuario.PasswordHash = passwordHash
	r.porID[id] = usuario
	return nil
}

// repositorioServiciosMemoria implementa puertos.RepositorioServicios.
type repositorioServiciosMemoria struct {
	mu    sync.Mutex
	porID map[entidades.ID]entidades.Servicio
}

func nuevoRepositorioServiciosMemoria() *repositorioServiciosMemoria {
	return &repositorioServiciosMemoria{porID: make(map[entidades.ID]entidades.Servicio)}
}

func (r *repositorioServiciosMemoria) Guardar(ctx context.Context, servicio entidades.Servicio) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.porID[servicio.ID] = servicio
	return nil
}

func (r *repositorioServiciosMemoria) Actualizar(ctx context.Context, servicio entidades.Servicio) error {
	return r.Guardar(ctx, servicio)
}

func (r *repositorioServiciosMemoria) BuscarPorID(ctx context.Context, id entidades.ID) (entidades.Servicio, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	servicio, existe := r.porID[id]
	if !existe {
		return entidades.Servicio{}, dominio.ErrNoEncontrado
	}
	return servicio, nil
}

func (r *repositorioServiciosMemoria) Listar(ctx context.Context) ([]entidades.Servicio, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var servicios []entidades.Servicio
	for _, s := range r.porID {
		servicios = append(servicios, s)
	}
	return servicios, nil
}

// repositorioHorariosMemoria implementa puertos.RepositorioHorarios.
type repositorioHorariosMemoria struct {
	mu     sync.Mutex
	porDia map[time.Weekday]entidades.HorarioAtencion
}

func nuevoRepositorioHorariosMemoria() *repositorioHorariosMemoria {
	return &repositorioHorariosMemoria{porDia: make(map[time.Weekday]entidades.HorarioAtencion)}
}

func (r *repositorioHorariosMemoria) Guardar(ctx context.Context, horario entidades.HorarioAtencion) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.porDia[horario.DiaSemana] = horario
	return nil
}

func (r *repositorioHorariosMemoria) Actualizar(ctx context.Context, horario entidades.HorarioAtencion) error {
	return r.Guardar(ctx, horario)
}

func (r *repositorioHorariosMemoria) BuscarPorDia(ctx context.Context, dia time.Weekday) (*entidades.HorarioAtencion, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	horario, existe := r.porDia[dia]
	if !existe {
		return nil, nil
	}
	return &horario, nil
}

func (r *repositorioHorariosMemoria) Listar(ctx context.Context) ([]entidades.HorarioAtencion, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var horarios []entidades.HorarioAtencion
	for _, h := range r.porDia {
		horarios = append(horarios, h)
	}
	return horarios, nil
}

// repositorioDiasBloqueadosMemoria implementa puertos.RepositorioDiasBloqueados.
type repositorioDiasBloqueadosMemoria struct {
	mu       sync.Mutex
	bloqueos []entidades.DiaBloqueado
}

func (r *repositorioDiasBloqueadosMemoria) Guardar(ctx context.Context, bloqueo entidades.DiaBloqueado) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.bloqueos = append(r.bloqueos, bloqueo)
	return nil
}

func (r *repositorioDiasBloqueadosMemoria) Eliminar(ctx context.Context, id entidades.ID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i, b := range r.bloqueos {
		if b.ID == id {
			r.bloqueos = append(r.bloqueos[:i], r.bloqueos[i+1:]...)
			return nil
		}
	}
	return dominio.ErrNoEncontrado
}

func (r *repositorioDiasBloqueadosMemoria) ListarEnRango(ctx context.Context, desde, hasta time.Time) ([]entidades.DiaBloqueado, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var resultado []entidades.DiaBloqueado
	for _, b := range r.bloqueos {
		if !b.Fecha.Before(desde) && !b.Fecha.After(hasta) {
			resultado = append(resultado, b)
		}
	}
	return resultado, nil
}

// repositorioReservasMemoria implementa puertos.RepositorioReservas. No
// reproduce el constraint de exclusión de Postgres (Fase 3) — eso está
// probado aparte, contra la base real, en
// internal/infraestructura/postgres/reservas_concurrencia_test.go. Acá
// solo necesitamos guardar y consultar reservas para poder testear las
// reglas de negocio de ServicioReservas.
type repositorioReservasMemoria struct {
	mu    sync.Mutex
	porID map[entidades.ID]entidades.Reserva
}

func nuevoRepositorioReservasMemoria() *repositorioReservasMemoria {
	return &repositorioReservasMemoria{porID: make(map[entidades.ID]entidades.Reserva)}
}

func (r *repositorioReservasMemoria) Guardar(ctx context.Context, reserva entidades.Reserva) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, existente := range r.porID {
		if existente.Estado != entidades.ReservaConfirmada {
			continue
		}
		if reserva.Inicio.Before(existente.Fin) && existente.Inicio.Before(reserva.Fin) {
			return fmt.Errorf("guardando reserva: %w", dominio.ErrSlotNoDisponible)
		}
	}
	r.porID[reserva.ID] = reserva
	return nil
}

func (r *repositorioReservasMemoria) Cancelar(ctx context.Context, id entidades.ID, canceladaEn time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	reserva, existe := r.porID[id]
	if !existe {
		return dominio.ErrNoEncontrado
	}
	reserva.Estado = entidades.ReservaCancelada
	reserva.CanceladaEn = &canceladaEn
	r.porID[id] = reserva
	return nil
}

func (r *repositorioReservasMemoria) BuscarPorID(ctx context.Context, id entidades.ID) (entidades.Reserva, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	reserva, existe := r.porID[id]
	if !existe {
		return entidades.Reserva{}, dominio.ErrNoEncontrado
	}
	return reserva, nil
}

func (r *repositorioReservasMemoria) ListarPorFecha(ctx context.Context, fecha time.Time) ([]entidades.Reserva, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var resultado []entidades.Reserva
	for _, reserva := range r.porID {
		ay, am, ad := reserva.Inicio.Date()
		by, bm, bd := fecha.Date()
		if ay == by && am == bm && ad == bd {
			resultado = append(resultado, reserva)
		}
	}
	return resultado, nil
}

func (r *repositorioReservasMemoria) ListarConFiltros(ctx context.Context, filtros puertos.FiltrosReservas) ([]entidades.Reserva, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var resultado []entidades.Reserva
	for _, reserva := range r.porID {
		if filtros.ClienteID != nil && reserva.ClienteID != *filtros.ClienteID {
			continue
		}
		if filtros.Estado != nil && reserva.Estado != *filtros.Estado {
			continue
		}
		resultado = append(resultado, reserva)
	}
	return resultado, nil
}
