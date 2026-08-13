'use strict';

/* ===================== estado y helpers de API ===================== */

const estado = { token: null, usuario: null, servicios: [] };

function cargarSesion() {
  const token = localStorage.getItem('token');
  const usuario = localStorage.getItem('usuario');
  if (token && usuario) {
    estado.token = token;
    estado.usuario = JSON.parse(usuario);
  }
}

function guardarSesion(token, usuario) {
  estado.token = token;
  estado.usuario = usuario;
  localStorage.setItem('token', token);
  localStorage.setItem('usuario', JSON.stringify(usuario));
}

function cerrarSesion() {
  estado.token = null;
  estado.usuario = null;
  localStorage.removeItem('token');
  localStorage.removeItem('usuario');
  mostrarAuth();
}

async function api(metodo, ruta, cuerpo) {
  const headers = { 'Content-Type': 'application/json' };
  if (estado.token) headers['Authorization'] = 'Bearer ' + estado.token;

  const resp = await fetch(ruta, {
    method: metodo,
    headers,
    body: cuerpo !== undefined ? JSON.stringify(cuerpo) : undefined,
  });

  const texto = await resp.text();
  let datos = null;
  if (texto) {
    try { datos = JSON.parse(texto); } catch { /* respuesta sin cuerpo JSON, ej. 204 */ }
  }

  if (!resp.ok) {
    const mensaje = (datos && datos.error) ? datos.error : `Error ${resp.status}`;
    throw new Error(mensaje);
  }
  return datos;
}

/* ===================== toast ===================== */

let toastTimeout;
function toast(mensaje, esError) {
  const el = document.getElementById('toast');
  el.textContent = mensaje;
  el.classList.toggle('error', !!esError);
  el.hidden = false;
  clearTimeout(toastTimeout);
  toastTimeout = setTimeout(() => { el.hidden = true; }, 3500);
}

/* ===================== formato de fecha/hora ===================== */

function formatearFechaHora(iso) {
  return new Date(iso).toLocaleString('es-AR', {
    weekday: 'short', day: '2-digit', month: '2-digit',
    hour: '2-digit', minute: '2-digit',
  });
}

function formatearPrecio(centavos) {
  return '$' + (centavos / 100).toFixed(2);
}

function nombreServicio(servicioId) {
  const s = estado.servicios.find((s) => s.id === servicioId);
  return s ? s.nombre : servicioId;
}

// formatearYMD/inicioDeHoy/fechaLocalYMD/lunesDeLaSemana son la base de
// todo el calendario y la agenda del admin: trabajan siempre con el
// calendario LOCAL del navegador (no UTC), porque es lo que el usuario
// ve en pantalla — el servidor sigue guardando y devolviendo todo en UTC
// (ver docs/CONCURRENCIA.md), la conversión pasa acá, al mostrar.

function formatearYMD(d) {
  const y = d.getFullYear();
  const m = String(d.getMonth() + 1).padStart(2, '0');
  const dia = String(d.getDate()).padStart(2, '0');
  return `${y}-${m}-${dia}`;
}

function inicioDeHoy() {
  const d = new Date();
  d.setHours(0, 0, 0, 0);
  return d;
}

function fechaLocalYMD(iso) {
  return formatearYMD(new Date(iso));
}

function lunesDeLaSemana(d) {
  const copia = new Date(d);
  copia.setHours(0, 0, 0, 0);
  const offset = (copia.getDay() + 6) % 7; // getDay(): 0=domingo..6=sábado; queremos 0=lunes
  copia.setDate(copia.getDate() - offset);
  return copia;
}

/* ===================== navegación de secciones ===================== */

function mostrarAuth() {
  document.getElementById('auth-section').hidden = false;
  document.getElementById('app-section').hidden = true;
}

function mostrarApp() {
  document.getElementById('auth-section').hidden = true;
  document.getElementById('app-section').hidden = false;

  const info = document.getElementById('user-info');
  info.innerHTML = `${estado.usuario.nombre} <span class="rol-badge">${estado.usuario.rol}</span>`;

  // El administrador no es un cliente del negocio: no tiene sentido que
  // "se reserve un turno a sí mismo" (el servidor ya lo rechaza con 403,
  // ver POST /reservas en cmd/api/main.go), así que directamente no le
  // mostramos esas pestañas — entra derecho al panel de administración.
  const esAdmin = estado.usuario.rol === 'administrador';
  document.querySelector('.admin-only').hidden = !esAdmin;
  document.querySelectorAll('.cliente-only').forEach((el) => { el.hidden = esAdmin; });

  cargarServicios().then(() => {
    poblarSelectServicios();
    cambiarVista(esAdmin ? 'admin' : 'reservar');
  });
}

function cambiarVista(nombre) {
  document.querySelectorAll('.nav-btn').forEach((b) => b.classList.toggle('active', b.dataset.view === nombre));
  document.querySelectorAll('.view').forEach((v) => v.classList.remove('active'));
  document.getElementById('view-' + nombre).classList.add('active');

  if (nombre === 'reservar') inicializarCalendario();
  if (nombre === 'mis-turnos') cargarMisTurnos();
  if (nombre === 'admin') cargarPanelAdmin();
}

document.querySelectorAll('.nav-btn').forEach((btn) => {
  btn.addEventListener('click', () => cambiarVista(btn.dataset.view));
});

document.querySelectorAll('.tab-btn').forEach((btn) => {
  btn.addEventListener('click', () => {
    document.querySelectorAll('.tab-btn').forEach((b) => b.classList.remove('active'));
    document.querySelectorAll('.tab-panel').forEach((p) => p.classList.remove('active'));
    btn.classList.add('active');
    document.getElementById(btn.dataset.tab + '-form').classList.add('active');
  });
});

// Los botones .sub-nav-btn se usan en dos niveles anidados (las pestañas
// del panel admin, y adentro de "Agenda" las pestañas Día/Semana/
// Filtros). Por eso el "apagado" de pestañas hermanas se limita a las
// que comparten el mismo <nav> padre — así clickear una pestaña interna
// no desactiva las externas, ni viceversa.
document.querySelectorAll('.sub-nav-btn').forEach((btn) => {
  btn.addEventListener('click', () => {
    const nav = btn.closest('nav');
    nav.querySelectorAll('.sub-nav-btn').forEach((b) => b.classList.remove('active'));
    btn.classList.add('active');

    const contenedor = nav.parentElement;
    Array.from(contenedor.children).forEach((hijo) => {
      if (hijo.classList.contains('sub-view')) hijo.classList.remove('active');
    });
    document.getElementById('sub-' + btn.dataset.sub).classList.add('active');

    if (btn.dataset.sub === 'dia') cargarAgendaDia();
    if (btn.dataset.sub === 'semana') cargarAgendaSemana();
    if (btn.dataset.sub === 'filtros') cargarAdminReservas({});
  });
});

/* ===================== autenticación ===================== */

document.getElementById('login-form').addEventListener('submit', async (e) => {
  e.preventDefault();
  const email = document.getElementById('login-email').value;
  const password = document.getElementById('login-password').value;
  try {
    const datos = await api('POST', '/auth/login', { email, password });
    guardarSesion(datos.token, datos.usuario);
    mostrarApp();
    toast(`¡Bienvenido, ${datos.usuario.nombre}!`);
  } catch (err) {
    mostrarMensajeAuth(err.message, true);
  }
});

document.getElementById('registro-form').addEventListener('submit', async (e) => {
  e.preventDefault();
  const nombre = document.getElementById('registro-nombre').value;
  const email = document.getElementById('registro-email').value;
  const password = document.getElementById('registro-password').value;
  try {
    await api('POST', '/auth/registro', { nombre, email, password });
    mostrarMensajeAuth('Cuenta creada. Ahora iniciá sesión.', false);
    document.querySelector('.tab-btn[data-tab="login"]').click();
    document.getElementById('login-email').value = email;
  } catch (err) {
    mostrarMensajeAuth(err.message, true);
  }
});

function mostrarMensajeAuth(texto, esError) {
  const el = document.getElementById('auth-msg');
  el.textContent = texto;
  el.className = 'msg ' + (esError ? 'error' : 'ok');
  el.hidden = false;
}

document.getElementById('logout-btn').addEventListener('click', cerrarSesion);

/* ===================== servicios (compartido) ===================== */

async function cargarServicios() {
  estado.servicios = await api('GET', '/servicios');
}

function poblarSelectServicios() {
  const opciones = estado.servicios
    .filter((s) => s.activo)
    .map((s) => `<option value="${s.id}">${s.nombre} — ${formatearPrecio(s.precio_centavos)}</option>`)
    .join('');
  document.getElementById('servicio-select').innerHTML = opciones;
  document.getElementById('wr-servicio').innerHTML = opciones;
}

/* ===================== reservar: calendario mensual ===================== */

let calMesActual;               // Date, día 1 del mes que se está mostrando
let calFechaSeleccionada = null; // "YYYY-MM-DD" o null
let diasConHorario = new Set();  // días de la semana (0-6) con horario configurado
let bloqueosCompletos = new Set(); // "YYYY-MM-DD" bloqueados el día entero

// La ventana de reserva del cliente son 28 días desde hoy (inclusive),
// el mismo límite que aplica el backend en
// aplicacion.validarAnticipacionCliente — se repite acá solo para poder
// dibujar el calendario, la regla real (la que no se puede saltear) vive
// en el servidor.
const DIAS_MAXIMO_ANTICIPACION = 28;

async function inicializarCalendario() {
  const hoy = inicioDeHoy();
  calMesActual = new Date(hoy.getFullYear(), hoy.getMonth(), 1);
  calFechaSeleccionada = null;
  document.getElementById('panel-horarios').hidden = true;

  const limite = new Date(hoy);
  limite.setDate(limite.getDate() + DIAS_MAXIMO_ANTICIPACION);

  try {
    const horarios = await api('GET', '/horarios');
    diasConHorario = new Set(horarios.map((h) => h.dia_semana));
  } catch {
    diasConHorario = new Set();
  }

  try {
    const bloqueos = await api('GET', `/dias-bloqueados?desde=${formatearYMD(hoy)}&hasta=${formatearYMD(limite)}`);
    bloqueosCompletos = new Set(bloqueos.filter((b) => !b.hora_desde).map((b) => b.fecha));
  } catch {
    bloqueosCompletos = new Set();
  }

  renderizarCalendario();
}

function renderizarCalendario() {
  const hoy = inicioDeHoy();
  const limite = new Date(hoy);
  limite.setDate(limite.getDate() + DIAS_MAXIMO_ANTICIPACION);

  document.getElementById('cal-mes-label').textContent =
    calMesActual.toLocaleDateString('es-AR', { month: 'long', year: 'numeric' });

  const mesHoy = new Date(hoy.getFullYear(), hoy.getMonth(), 1);
  const mesLimite = new Date(limite.getFullYear(), limite.getMonth(), 1);
  document.getElementById('cal-prev').disabled = calMesActual <= mesHoy;
  document.getElementById('cal-next').disabled = calMesActual >= mesLimite;

  const grid = document.getElementById('calendario-grid');
  grid.innerHTML = '';

  const primerDiaMes = new Date(calMesActual.getFullYear(), calMesActual.getMonth(), 1);
  const diasEnMes = new Date(calMesActual.getFullYear(), calMesActual.getMonth() + 1, 0).getDate();
  const offset = (primerDiaMes.getDay() + 6) % 7; // celdas vacías antes del día 1

  for (let i = 0; i < offset; i++) {
    const vacia = document.createElement('div');
    vacia.className = 'dia-celda vacia';
    grid.appendChild(vacia);
  }

  for (let dia = 1; dia <= diasEnMes; dia++) {
    const fecha = new Date(calMesActual.getFullYear(), calMesActual.getMonth(), dia);
    const ymd = formatearYMD(fecha);

    const celda = document.createElement('button');
    celda.type = 'button';
    celda.className = 'dia-celda';
    celda.textContent = String(dia);

    if (fecha.getTime() === hoy.getTime()) celda.classList.add('hoy');

    const dentroDeVentana = fecha >= hoy && fecha <= limite;
    const tieneHorario = diasConHorario.has(fecha.getDay());
    const bloqueado = bloqueosCompletos.has(ymd);

    if (dentroDeVentana && tieneHorario && !bloqueado) {
      celda.classList.add('disponible');
      if (ymd === calFechaSeleccionada) celda.classList.add('seleccionado');
      celda.addEventListener('click', () => seleccionarDia(ymd, fecha));
    } else {
      celda.disabled = true;
    }

    grid.appendChild(celda);
  }
}

document.getElementById('cal-prev').addEventListener('click', () => {
  calMesActual = new Date(calMesActual.getFullYear(), calMesActual.getMonth() - 1, 1);
  renderizarCalendario();
});
document.getElementById('cal-next').addEventListener('click', () => {
  calMesActual = new Date(calMesActual.getFullYear(), calMesActual.getMonth() + 1, 1);
  renderizarCalendario();
});

async function seleccionarDia(ymd, fechaObj) {
  calFechaSeleccionada = ymd;
  renderizarCalendario();

  const panel = document.getElementById('panel-horarios');
  const grid = document.getElementById('slots-grid');
  panel.hidden = false;
  document.getElementById('panel-horarios-fecha').textContent =
    fechaObj.toLocaleDateString('es-AR', { weekday: 'long', day: 'numeric', month: 'long' });
  grid.innerHTML = '<p class="vacio">Buscando disponibilidad…</p>';

  const servicioId = document.getElementById('servicio-select').value;
  if (!servicioId) { grid.innerHTML = '<p class="vacio">Elegí un servicio primero.</p>'; return; }

  try {
    const slots = await api('GET', `/slots?fecha=${ymd}&servicio_id=${servicioId}`);
    if (slots.length === 0) {
      grid.innerHTML = '<p class="vacio">No hay horarios ese día.</p>';
      return;
    }
    grid.innerHTML = '';
    slots.forEach((slot) => {
      const btn = document.createElement('button');
      btn.type = 'button';
      btn.className = 'slot-btn ' + (slot.disponible ? 'disponible' : 'ocupado');
      btn.textContent = new Date(slot.inicio).toLocaleTimeString('es-AR', { hour: '2-digit', minute: '2-digit' });
      btn.disabled = !slot.disponible;
      btn.addEventListener('click', () => reservarSlot(servicioId, slot.inicio));
      grid.appendChild(btn);
    });
  } catch (err) {
    toast(err.message, true);
    grid.innerHTML = '';
  }
}

document.getElementById('servicio-select').addEventListener('change', () => {
  if (!calFechaSeleccionada) return;
  const [y, m, d] = calFechaSeleccionada.split('-').map(Number);
  seleccionarDia(calFechaSeleccionada, new Date(y, m - 1, d));
});

async function reservarSlot(servicioId, inicioIso) {
  try {
    await api('POST', '/reservas', { servicio_id: servicioId, inicio: inicioIso });
    toast('¡Turno reservado!');
    if (calFechaSeleccionada) {
      const [y, m, d] = calFechaSeleccionada.split('-').map(Number);
      seleccionarDia(calFechaSeleccionada, new Date(y, m - 1, d));
    }
  } catch (err) {
    toast(err.message, true);
  }
}

/* ===================== mis turnos ===================== */

async function cargarMisTurnos() {
  const cont = document.getElementById('mis-turnos-lista');
  cont.innerHTML = '<p class="vacio">Cargando…</p>';
  try {
    const reservas = await api('GET', '/reservas/mias');
    renderizarTurnos(cont, reservas, true);
  } catch (err) {
    cont.innerHTML = '';
    toast(err.message, true);
  }
}

function renderizarTurnos(cont, reservas, permiteCancelar) {
  if (reservas.length === 0) {
    cont.innerHTML = '<p class="vacio">No hay turnos para mostrar.</p>';
    return;
  }
  reservas.sort((a, b) => new Date(b.inicio) - new Date(a.inicio));
  cont.innerHTML = '';
  reservas.forEach((r) => {
    const card = document.createElement('div');
    card.className = 'card-turno';
    card.innerHTML = `
      <div class="info">
        <span class="servicio">${nombreServicio(r.servicio_id)}</span>
        <span class="fecha">${formatearFechaHora(r.inicio)}</span>
      </div>
      <span class="estado-badge ${r.estado}">${r.estado}</span>
    `;
    if (permiteCancelar && r.estado === 'confirmada') {
      const btn = document.createElement('button');
      btn.className = 'btn btn--peligro';
      btn.textContent = 'Cancelar';
      btn.addEventListener('click', async () => {
        try {
          await api('POST', `/reservas/${r.id}/cancelar`, undefined);
          toast('Turno cancelado');
          cargarMisTurnos();
        } catch (err) {
          toast(err.message, true);
        }
      });
      card.appendChild(btn);
    }
    cont.appendChild(card);
  });
}

// renderizarTurnosAdmin es la versión para el panel de administración:
// las reservas vienen de GET /admin/reservas, que ya incluye el nombre y
// el email del cliente (ver dto.ReservaAdminResponse). Cada tarjeta es
// un <details> — colapsada por defecto, mostrando solo lo esencial
// (servicio, fecha, estado), y al hacer click se despliega el nombre y
// el email del cliente. <details>/<summary> es HTML nativo para "click
// para ver más": no hace falta ningún JS extra para abrir/cerrar, el
// navegador lo maneja solo.
function renderizarTurnosAdmin(cont, reservas) {
  if (reservas.length === 0) {
    cont.innerHTML = '<p class="vacio">No hay turnos para mostrar.</p>';
    return;
  }
  reservas.sort((a, b) => new Date(b.inicio) - new Date(a.inicio));
  cont.innerHTML = '';
  reservas.forEach((r) => {
    const card = document.createElement('details');
    card.className = 'card-turno';
    card.innerHTML = `
      <summary>
        <span class="info">
          <span class="servicio">${nombreServicio(r.servicio_id)}</span>
          <span class="fecha">${formatearFechaHora(r.inicio)}</span>
        </span>
        <span class="estado-badge ${r.estado}">${r.estado}</span>
      </summary>
      <div class="card-turno-detalle">
        <p><strong>Cliente:</strong> ${r.cliente_nombre}</p>
        <p><strong>Email:</strong> ${r.cliente_email}</p>
      </div>
    `;
    cont.appendChild(card);
  });
}

document.getElementById('refrescar-turnos-btn').addEventListener('click', cargarMisTurnos);

/* ===================== panel admin ===================== */

function cargarPanelAdmin() {
  cargarAdminServicios();
  cargarAdminHorarios();
  cargarAdminBloqueos();
  agendaDiaFecha = inicioDeHoy();
  agendaSemanaInicio = lunesDeLaSemana(new Date());
  cargarAgendaDia();
}

// --- servicios ---

document.getElementById('crear-servicio-form').addEventListener('submit', async (e) => {
  e.preventDefault();
  const nombre = document.getElementById('cs-nombre').value;
  const duracion = Number(document.getElementById('cs-duracion').value);
  const precio = Math.round(Number(document.getElementById('cs-precio').value) * 100);
  try {
    await api('POST', '/admin/servicios', { nombre, duracion_minutos: duracion, precio_centavos: precio });
    toast('Servicio creado');
    e.target.reset();
    await cargarServicios();
    poblarSelectServicios();
    cargarAdminServicios();
  } catch (err) {
    toast(err.message, true);
  }
});

async function cargarAdminServicios() {
  const cont = document.getElementById('admin-servicios-lista');
  try {
    const servicios = await api('GET', '/servicios');
    if (servicios.length === 0) { cont.innerHTML = '<p class="vacio">Sin servicios todavía.</p>'; return; }
    cont.innerHTML = '';
    servicios.forEach((s) => {
      const fila = document.createElement('div');
      fila.className = 'fila';
      fila.innerHTML = `
        <div class="info">
          <span class="principal">${s.nombre}</span>
          <span class="secundario">${s.duracion_minutos} min · ${formatearPrecio(s.precio_centavos)}</span>
        </div>
        <span class="estado-badge ${s.activo ? 'confirmada' : 'cancelada'}">${s.activo ? 'activo' : 'inactivo'}</span>
      `;
      const btn = document.createElement('button');
      btn.className = 'btn btn--outline';
      btn.textContent = s.activo ? 'Desactivar' : 'Activar';
      btn.addEventListener('click', async () => {
        try {
          await api('PUT', `/admin/servicios/${s.id}`, {
            nombre: s.nombre, duracion_minutos: s.duracion_minutos,
            precio_centavos: s.precio_centavos, activo: !s.activo,
          });
          toast('Servicio actualizado');
          await cargarServicios();
          poblarSelectServicios();
          cargarAdminServicios();
        } catch (err) {
          toast(err.message, true);
        }
      });
      fila.appendChild(btn);
      cont.appendChild(fila);
    });
  } catch (err) {
    toast(err.message, true);
  }
}

// --- horarios ---

document.getElementById('definir-horario-form').addEventListener('submit', async (e) => {
  e.preventDefault();
  const dia = Number(document.getElementById('dh-dia').value);
  const inicio = document.getElementById('dh-inicio').value;
  const fin = document.getElementById('dh-fin').value;
  try {
    await api('POST', '/admin/horarios', { dia_semana: dia, hora_inicio: inicio, hora_fin: fin });
    toast('Horario guardado');
    cargarAdminHorarios();
  } catch (err) {
    toast(err.message, true);
  }
});

const NOMBRES_DIA = ['Domingo', 'Lunes', 'Martes', 'Miércoles', 'Jueves', 'Viernes', 'Sábado'];

async function cargarAdminHorarios() {
  const cont = document.getElementById('admin-horarios-lista');
  try {
    const horarios = await api('GET', '/admin/horarios');
    if (horarios.length === 0) { cont.innerHTML = '<p class="vacio">Sin horarios configurados.</p>'; return; }
    horarios.sort((a, b) => a.dia_semana - b.dia_semana);
    cont.innerHTML = horarios.map((h) => `
      <div class="fila">
        <div class="info">
          <span class="principal">${NOMBRES_DIA[h.dia_semana]}</span>
          <span class="secundario">${h.hora_inicio} — ${h.hora_fin}</span>
        </div>
      </div>
    `).join('');
  } catch (err) {
    toast(err.message, true);
  }
}

// --- bloqueos ---

document.getElementById('crear-bloqueo-form').addEventListener('submit', async (e) => {
  e.preventDefault();
  const fecha = document.getElementById('cb-fecha').value;
  const horaDesde = document.getElementById('cb-hora-desde').value || null;
  const motivo = document.getElementById('cb-motivo').value;
  try {
    await api('POST', '/admin/dias-bloqueados', { fecha, hora_desde: horaDesde, motivo });
    toast('Día bloqueado');
    e.target.reset();
    cargarAdminBloqueos();
  } catch (err) {
    toast(err.message, true);
  }
});

async function cargarAdminBloqueos() {
  const cont = document.getElementById('admin-bloqueos-lista');
  const desde = formatearYMD(inicioDeHoy());
  const hastaObj = inicioDeHoy();
  hastaObj.setDate(hastaObj.getDate() + 60);
  const hasta = formatearYMD(hastaObj);
  try {
    const bloqueos = await api('GET', `/admin/dias-bloqueados?desde=${desde}&hasta=${hasta}`);
    if (bloqueos.length === 0) { cont.innerHTML = '<p class="vacio">Sin bloqueos en los próximos 60 días.</p>'; return; }
    cont.innerHTML = '';
    bloqueos.forEach((b) => {
      const fila = document.createElement('div');
      fila.className = 'fila';
      fila.innerHTML = `
        <div class="info">
          <span class="principal">${b.fecha}${b.hora_desde ? ' desde ' + b.hora_desde : ' (día completo)'}</span>
          <span class="secundario">${b.motivo}</span>
        </div>
      `;
      const btn = document.createElement('button');
      btn.className = 'btn btn--peligro';
      btn.textContent = 'Eliminar';
      btn.addEventListener('click', async () => {
        try {
          await api('DELETE', `/admin/dias-bloqueados/${b.id}`, undefined);
          toast('Bloqueo eliminado');
          cargarAdminBloqueos();
        } catch (err) {
          toast(err.message, true);
        }
      });
      fila.appendChild(btn);
      cont.appendChild(fila);
    });
  } catch (err) {
    toast(err.message, true);
  }
}

// --- walk-in ---

document.getElementById('crear-cliente-form').addEventListener('submit', async (e) => {
  e.preventDefault();
  const nombre = document.getElementById('wc-nombre').value;
  const email = document.getElementById('wc-email').value;
  try {
    const cliente = await api('POST', '/admin/clientes', { nombre, email });
    toast(`Cliente creado — ID: ${cliente.id}`);
    document.getElementById('wr-cliente-id').value = cliente.id;
    e.target.reset();
  } catch (err) {
    toast(err.message, true);
  }
});

document.getElementById('crear-reserva-admin-form').addEventListener('submit', async (e) => {
  e.preventDefault();
  const clienteId = document.getElementById('wr-cliente-id').value;
  const servicioId = document.getElementById('wr-servicio').value;
  const inicioLocal = document.getElementById('wr-inicio').value;
  if (!inicioLocal) { toast('Elegí fecha y hora', true); return; }
  const inicioIso = new Date(inicioLocal).toISOString();
  try {
    await api('POST', '/admin/reservas', { cliente_id: clienteId, servicio_id: servicioId, inicio: inicioIso });
    toast('Reserva creada para el cliente');
    e.target.reset();
  } catch (err) {
    toast(err.message, true);
  }
});

// --- agenda: por día ---

let agendaDiaFecha = inicioDeHoy();

async function cargarAgendaDia() {
  const cont = document.getElementById('agenda-dia-lista');
  document.getElementById('agenda-dia-label').textContent =
    agendaDiaFecha.toLocaleDateString('es-AR', { weekday: 'long', day: 'numeric', month: 'long' });

  const ymd = formatearYMD(agendaDiaFecha);
  cont.innerHTML = '<p class="vacio">Cargando…</p>';
  try {
    const reservas = await api('GET', `/admin/reservas?desde=${ymd}&hasta=${ymd}`);
    renderizarTurnosAdmin(cont, reservas);
  } catch (err) {
    cont.innerHTML = '';
    toast(err.message, true);
  }
}

document.getElementById('agenda-dia-prev').addEventListener('click', () => {
  agendaDiaFecha.setDate(agendaDiaFecha.getDate() - 1);
  cargarAgendaDia();
});
document.getElementById('agenda-dia-next').addEventListener('click', () => {
  agendaDiaFecha.setDate(agendaDiaFecha.getDate() + 1);
  cargarAgendaDia();
});

// --- agenda: semana completa ---

let agendaSemanaInicio = lunesDeLaSemana(new Date());

async function cargarAgendaSemana() {
  const cont = document.getElementById('agenda-semana-lista');
  const fin = new Date(agendaSemanaInicio);
  fin.setDate(fin.getDate() + 6);

  document.getElementById('agenda-semana-label').textContent =
    `${agendaSemanaInicio.toLocaleDateString('es-AR', { day: 'numeric', month: 'short' })} – ${fin.toLocaleDateString('es-AR', { day: 'numeric', month: 'short' })}`;

  cont.innerHTML = '<p class="vacio">Cargando…</p>';
  try {
    const reservas = await api('GET', `/admin/reservas?desde=${formatearYMD(agendaSemanaInicio)}&hasta=${formatearYMD(fin)}`);
    renderizarAgendaSemana(cont, reservas);
  } catch (err) {
    cont.innerHTML = '';
    toast(err.message, true);
  }
}

function renderizarAgendaSemana(cont, reservas) {
  cont.innerHTML = '';
  for (let i = 0; i < 7; i++) {
    const dia = new Date(agendaSemanaInicio);
    dia.setDate(dia.getDate() + i);
    const ymd = formatearYMD(dia);
    const reservasDelDia = reservas.filter((r) => fechaLocalYMD(r.inicio) === ymd);

    const bloque = document.createElement('div');
    bloque.className = 'agenda-semana-dia';

    const encabezado = document.createElement('h4');
    encabezado.textContent = dia.toLocaleDateString('es-AR', { weekday: 'long', day: 'numeric', month: 'short' });
    bloque.appendChild(encabezado);

    const listaDia = document.createElement('div');
    listaDia.className = 'tabla';
    bloque.appendChild(listaDia);

    cont.appendChild(bloque);
    renderizarTurnosAdmin(listaDia, reservasDelDia);
  }
}

document.getElementById('agenda-semana-prev').addEventListener('click', () => {
  agendaSemanaInicio.setDate(agendaSemanaInicio.getDate() - 7);
  cargarAgendaSemana();
});
document.getElementById('agenda-semana-next').addEventListener('click', () => {
  agendaSemanaInicio.setDate(agendaSemanaInicio.getDate() + 7);
  cargarAgendaSemana();
});

// --- agenda: filtros libres ---

document.getElementById('filtros-reservas-form').addEventListener('submit', (e) => {
  e.preventDefault();
  cargarAdminReservas({
    desde: document.getElementById('fr-desde').value,
    hasta: document.getElementById('fr-hasta').value,
    estado: document.getElementById('fr-estado').value,
  });
});

async function cargarAdminReservas(filtros) {
  const cont = document.getElementById('admin-reservas-lista');
  const params = new URLSearchParams();
  if (filtros.desde) params.set('desde', filtros.desde);
  if (filtros.hasta) params.set('hasta', filtros.hasta);
  if (filtros.estado) params.set('estado', filtros.estado);

  cont.innerHTML = '<p class="vacio">Cargando…</p>';
  try {
    const reservas = await api('GET', `/admin/reservas?${params.toString()}`);
    renderizarTurnosAdmin(cont, reservas);
  } catch (err) {
    cont.innerHTML = '';
    toast(err.message, true);
  }
}

/* ===================== arranque ===================== */

cargarSesion();
if (estado.token) {
  mostrarApp();
} else {
  mostrarAuth();
}
