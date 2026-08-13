package dominio

import (
	"time"

	"reservas-go/internal/dominio/entidades"
)

// Slot es un bloque de DuracionSlot dentro de un día, con su estado de
// disponibilidad ya calculado.
type Slot struct {
	Inicio     time.Time
	Fin        time.Time
	Disponible bool
}

// CalcularSlotsDisponibles calcula, para una fecha dada, todos los slots
// del horario de atención de ese día de la semana, marcando cada uno como
// disponible u ocupado.
//
// Es una función pura: no toca la base de datos ni pregunta la hora por
// su cuenta. Todo lo que necesita para decidir se lo pasan como
// parámetros:
//
//   - horario: el horario de atención del día de la semana de "fecha". Si
//     es nil, la barbería no atiende ese día (día de descanso) y no hay
//     slots.
//   - bloqueos: los días bloqueados que podrían aplicar a "fecha" (la
//     función descarta sola los que no correspondan a ese día).
//   - reservas: las reservas ya existentes ese día, para saber qué slots
//     están ocupados.
//   - ahora: el instante actual, para no ofrecer como disponible un slot
//     que ya empezó o que ya pasó.
//
// Al ser pura, se puede testear con total determinismo: se le pasa
// cualquier "ahora" fijo y siempre da el mismo resultado, sin necesidad de
// levantar una base de datos ni de esperar a que pase el tiempo real.
func CalcularSlotsDisponibles(
	fecha time.Time,
	horario *entidades.HorarioAtencion,
	bloqueos []entidades.DiaBloqueado,
	reservas []entidades.Reserva,
	ahora time.Time,
) []Slot {
	if horario == nil {
		return nil
	}

	rangoInicio := horario.HoraInicio.EnFecha(fecha)
	rangoFin := horario.HoraFin.EnFecha(fecha)

	for _, bloqueo := range bloqueos {
		if !mismoDia(bloqueo.Fecha, fecha) {
			continue
		}
		if bloqueo.HoraDesde == nil {
			// Día completo bloqueado: no hay nada que generar.
			return nil
		}
		inicioBloqueo := bloqueo.HoraDesde.EnFecha(fecha)
		if inicioBloqueo.Before(rangoFin) {
			rangoFin = inicioBloqueo
		}
	}

	var slots []Slot
	for inicio := rangoInicio; !inicio.Add(entidades.DuracionSlot).After(rangoFin); inicio = inicio.Add(entidades.DuracionSlot) {
		fin := inicio.Add(entidades.DuracionSlot)
		slots = append(slots, Slot{
			Inicio:     inicio,
			Fin:        fin,
			Disponible: inicio.After(ahora) && !SolapaConReservaConfirmada(inicio, fin, reservas),
		})
	}

	return slots
}

// mismoDia compara solo año/mes/día, ignorando la hora — así una Fecha de
// bloqueo guardada a medianoche se puede comparar contra una "fecha" que
// venga con cualquier hora del día.
func mismoDia(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}

// SolapaConReservaConfirmada indica si el rango [inicio, fin) se cruza con
// alguna reserva confirmada. Las reservas canceladas no ocupan el slot.
//
// Exportada (además de usarla CalcularSlotsDisponibles acá adentro) para
// que ServicioReservas.CrearReserva la reutilice en el caso especial de
// una reserva de administrador en un día sin horario configurado, donde
// no hay una grilla de slots contra la cual comparar — ver
// docs/CONTEXTO.md.
func SolapaConReservaConfirmada(inicio, fin time.Time, reservas []entidades.Reserva) bool {
	for _, r := range reservas {
		if r.Estado != entidades.ReservaConfirmada {
			continue
		}
		if inicio.Before(r.Fin) && r.Inicio.Before(fin) {
			return true
		}
	}
	return false
}
