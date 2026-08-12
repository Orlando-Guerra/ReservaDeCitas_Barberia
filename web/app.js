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

/* ===================== formato ===================== */

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

  const esAdmin = estado.usuario.rol === 'administrador';
  document.querySelector('.admin-only').hidden = !esAdmin;

  cargarServicios().then(() => {
    poblarSelectServicios();
    cambiarVista('reservar');
  });
}

function cambiarVista(nombre) {
  document.querySelectorAll('.nav-btn').forEach((b) => b.classList.toggle('active', b.dataset.view === nombre));
  document.querySelectorAll('.view').forEach((v) => v.classList.remove('active'));
  document.getElementById('view-' + nombre).classList.add('active');

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

document.querySelectorAll('.sub-nav-btn').forEach((btn) => {
  btn.addEventListener('click', () => {
    document.querySelectorAll('.sub-nav-btn').forEach((b) => b.classList.remove('active'));
    document.querySelectorAll('.sub-view').forEach((p) => p.classList.remove('active'));
    btn.classList.add('active');
    document.getElementById('sub-' + btn.dataset.sub).classList.add('active');
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

/* ===================== reservar ===================== */

const fechaInput = document.getElementById('fecha-input');
fechaInput.value = new Date().toISOString().slice(0, 10);
fechaInput.min = new Date().toISOString().slice(0, 10);

document.getElementById('ver-slots-btn').addEventListener('click', async () => {
  const servicioId = document.getElementById('servicio-select').value;
  const fecha = fechaInput.value;
  const cont = document.getElementById('slots-grid');
  if (!servicioId || !fecha) { toast('Elegí un servicio y una fecha', true); return; }

  cont.innerHTML = '<p class="vacio">Buscando disponibilidad…</p>';
  try {
    const slots = await api('GET', `/slots?fecha=${fecha}&servicio_id=${servicioId}`);
    if (slots.length === 0) {
      cont.innerHTML = '<p class="vacio">No hay atención configurada para ese día.</p>';
      return;
    }
    cont.innerHTML = '';
    slots.forEach((slot) => {
      const btn = document.createElement('button');
      btn.type = 'button';
      btn.className = 'slot-btn ' + (slot.disponible ? 'disponible' : 'ocupado');
      btn.textContent = new Date(slot.inicio).toLocaleTimeString('es-AR', { hour: '2-digit', minute: '2-digit' });
      btn.disabled = !slot.disponible;
      btn.addEventListener('click', () => reservarSlot(servicioId, slot.inicio));
      cont.appendChild(btn);
    });
  } catch (err) {
    toast(err.message, true);
    cont.innerHTML = '';
  }
});

async function reservarSlot(servicioId, inicioIso) {
  try {
    await api('POST', '/reservas', { servicio_id: servicioId, inicio: inicioIso });
    toast('¡Turno reservado!');
    document.getElementById('ver-slots-btn').click();
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

document.getElementById('refrescar-turnos-btn').addEventListener('click', cargarMisTurnos);

/* ===================== panel admin ===================== */

function cargarPanelAdmin() {
  cargarAdminServicios();
  cargarAdminHorarios();
  cargarAdminBloqueos();
  cargarAdminReservas({});
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
  const desde = new Date().toISOString().slice(0, 10);
  const hasta = new Date(Date.now() + 60 * 24 * 3600 * 1000).toISOString().slice(0, 10);
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
    cargarAdminReservas({});
  } catch (err) {
    toast(err.message, true);
  }
});

// --- todas las reservas ---

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
    renderizarTurnos(cont, reservas, false);
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
