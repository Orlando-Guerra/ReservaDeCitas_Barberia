package dominio_test

import (
	"testing"
	"time"

	"reservas-go/internal/dominio"
	"reservas-go/internal/dominio/entidades"
)

// fecha es el lunes usado como "día bajo prueba" en todos los casos de la
// tabla, salvo que un caso puntual necesite otra cosa.
var fecha = time.Date(2026, 8, 17, 0, 0, 0, 0, time.UTC)

// ayer es un instante bien anterior a "fecha": usarlo como "ahora" hace
// que todos los slots del día bajo prueba queden en el futuro, que es la
// situación por defecto que la mayoría de los casos quiere.
var ayer = fecha.AddDate(0, 0, -1)

func hora(t *testing.T, horas, minutos int) entidades.HoraDelDia {
	t.Helper()
	h, err := entidades.NuevaHoraDelDia(horas, minutos)
	if err != nil {
		t.Fatalf("no se esperaba error construyendo HoraDelDia(%d,%d): %v", horas, minutos, err)
	}
	return h
}

func horario(t *testing.T, horaInicio, horaFin entidades.HoraDelDia) *entidades.HorarioAtencion {
	t.Helper()
	h, err := entidades.NuevoHorarioAtencion(fecha.Weekday(), horaInicio, horaFin)
	if err != nil {
		t.Fatalf("no se esperaba error construyendo HorarioAtencion: %v", err)
	}
	return &h
}

func reservaEn(t *testing.T, inicio time.Time, fin time.Time, estado entidades.EstadoReserva) entidades.Reserva {
	t.Helper()
	r, err := entidades.NuevaReserva("cliente-1", "servicio-1", inicio, ayer)
	if err != nil {
		t.Fatalf("no se esperaba error construyendo Reserva: %v", err)
	}
	r.Fin = fin // NuevaReserva siempre usa DuracionSlot; lo pisamos para probar solapamientos no alineados a la grilla.
	r.Estado = estado
	return r
}

func bloqueoCompleto(t *testing.T, enFecha time.Time, motivo string) entidades.DiaBloqueado {
	t.Helper()
	b, err := entidades.NuevoDiaBloqueado(enFecha, nil, motivo)
	if err != nil {
		t.Fatalf("no se esperaba error construyendo DiaBloqueado: %v", err)
	}
	return b
}

func bloqueoParcial(t *testing.T, enFecha time.Time, desde entidades.HoraDelDia, motivo string) entidades.DiaBloqueado {
	t.Helper()
	b, err := entidades.NuevoDiaBloqueado(enFecha, &desde, motivo)
	if err != nil {
		t.Fatalf("no se esperaba error construyendo DiaBloqueado: %v", err)
	}
	return b
}

// TestCalcularSlotsDisponibles es la batería de tests de tabla del
// corazón del dominio (ver docs/CONTEXTO.md y docs/CONCURRENCIA.md).
// Cada caso arma su propio horario/bloqueos/reservas/ahora y describe qué
// slots espera, en orden, como una lista de "¿disponible?" — la cantidad
// de bools esperados también fija cuántos slots tienen que generarse.
func TestCalcularSlotsDisponibles(t *testing.T) {
	casos := []struct {
		nombre                 string
		horario                func(t *testing.T) *entidades.HorarioAtencion
		bloqueos               func(t *testing.T) []entidades.DiaBloqueado
		reservas               func(t *testing.T) []entidades.Reserva
		ahora                  time.Time
		disponibilidadEsperada []bool // una entrada por slot esperado, en orden
	}{
		{
			nombre:                 "sin horario ese día, no hay slots",
			horario:                func(t *testing.T) *entidades.HorarioAtencion { return nil },
			ahora:                  ayer,
			disponibilidadEsperada: nil,
		},
		{
			nombre: "sin reservas ni bloqueos, todo disponible",
			horario: func(t *testing.T) *entidades.HorarioAtencion {
				return horario(t, hora(t, 9, 0), hora(t, 12, 0))
			},
			ahora:                  ayer,
			disponibilidadEsperada: []bool{true, true, true}, // 9-10, 10-11, 11-12
		},
		{
			nombre: "una reserva confirmada ocupa exactamente su slot",
			horario: func(t *testing.T) *entidades.HorarioAtencion {
				return horario(t, hora(t, 9, 0), hora(t, 12, 0))
			},
			reservas: func(t *testing.T) []entidades.Reserva {
				inicio := hora(t, 10, 0).EnFecha(fecha)
				return []entidades.Reserva{reservaEn(t, inicio, inicio.Add(time.Hour), entidades.ReservaConfirmada)}
			},
			ahora:                  ayer,
			disponibilidadEsperada: []bool{true, false, true},
		},
		{
			nombre: "una reserva cancelada NO ocupa su slot",
			horario: func(t *testing.T) *entidades.HorarioAtencion {
				return horario(t, hora(t, 9, 0), hora(t, 12, 0))
			},
			reservas: func(t *testing.T) []entidades.Reserva {
				inicio := hora(t, 10, 0).EnFecha(fecha)
				return []entidades.Reserva{reservaEn(t, inicio, inicio.Add(time.Hour), entidades.ReservaCancelada)}
			},
			ahora:                  ayer,
			disponibilidadEsperada: []bool{true, true, true},
		},
		{
			nombre: "una reserva con solapamiento parcial (no alineada a la grilla) ocupa el slot que toca",
			horario: func(t *testing.T) *entidades.HorarioAtencion {
				return horario(t, hora(t, 9, 0), hora(t, 12, 0))
			},
			reservas: func(t *testing.T) []entidades.Reserva {
				// 9:30 a 10:00 se solapa con el slot 9-10, pero NO con el
				// 10-11 (10:00 es el fin de la reserva, y los rangos son
				// [inicio, fin): terminar justo cuando empieza el otro no
				// es solaparse).
				inicio := hora(t, 9, 30).EnFecha(fecha)
				fin := hora(t, 10, 0).EnFecha(fecha)
				return []entidades.Reserva{reservaEn(t, inicio, fin, entidades.ReservaConfirmada)}
			},
			ahora:                  ayer,
			disponibilidadEsperada: []bool{false, true, true},
		},
		{
			nombre: "día completo bloqueado, no hay slots",
			horario: func(t *testing.T) *entidades.HorarioAtencion {
				return horario(t, hora(t, 9, 0), hora(t, 12, 0))
			},
			bloqueos: func(t *testing.T) []entidades.DiaBloqueado {
				return []entidades.DiaBloqueado{bloqueoCompleto(t, fecha, "vacaciones")}
			},
			ahora:                  ayer,
			disponibilidadEsperada: nil,
		},
		{
			nombre: "bloqueo parcial recorta el resto del día",
			horario: func(t *testing.T) *entidades.HorarioAtencion {
				return horario(t, hora(t, 9, 0), hora(t, 12, 0))
			},
			bloqueos: func(t *testing.T) []entidades.DiaBloqueado {
				return []entidades.DiaBloqueado{bloqueoParcial(t, fecha, hora(t, 10, 0), "emergencia")}
			},
			ahora:                  ayer,
			disponibilidadEsperada: []bool{true}, // solo 9-10
		},
		{
			nombre: "bloqueo de otro día no afecta",
			horario: func(t *testing.T) *entidades.HorarioAtencion {
				return horario(t, hora(t, 9, 0), hora(t, 12, 0))
			},
			bloqueos: func(t *testing.T) []entidades.DiaBloqueado {
				otroDia := fecha.AddDate(0, 0, 1)
				return []entidades.DiaBloqueado{bloqueoCompleto(t, otroDia, "vacaciones")}
			},
			ahora:                  ayer,
			disponibilidadEsperada: []bool{true, true, true},
		},
		{
			nombre: "entre varios bloqueos el mismo día, un bloqueo completo gana sin importar el orden",
			horario: func(t *testing.T) *entidades.HorarioAtencion {
				return horario(t, hora(t, 9, 0), hora(t, 12, 0))
			},
			bloqueos: func(t *testing.T) []entidades.DiaBloqueado {
				return []entidades.DiaBloqueado{
					bloqueoParcial(t, fecha, hora(t, 11, 0), "emergencia parcial"),
					bloqueoCompleto(t, fecha, "en realidad se toma todo el día"),
				}
			},
			ahora:                  ayer,
			disponibilidadEsperada: nil,
		},
		{
			nombre: "un slot que ya empezó no está disponible",
			horario: func(t *testing.T) *entidades.HorarioAtencion {
				return horario(t, hora(t, 9, 0), hora(t, 12, 0))
			},
			ahora:                  hora(t, 9, 30).EnFecha(fecha), // 30 min después de que arrancó el primer slot
			disponibilidadEsperada: []bool{false, true, true},
		},
		{
			nombre: "un slot que empieza justo ahora tampoco está disponible (límite)",
			horario: func(t *testing.T) *entidades.HorarioAtencion {
				return horario(t, hora(t, 9, 0), hora(t, 12, 0))
			},
			ahora:                  hora(t, 10, 0).EnFecha(fecha), // exactamente el inicio del segundo slot
			disponibilidadEsperada: []bool{false, false, true},
		},
		{
			nombre: "un horario más chico que un slot no genera ninguno",
			horario: func(t *testing.T) *entidades.HorarioAtencion {
				return horario(t, hora(t, 9, 0), hora(t, 9, 30))
			},
			ahora:                  ayer,
			disponibilidadEsperada: nil,
		},
	}

	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			var h *entidades.HorarioAtencion
			if caso.horario != nil {
				h = caso.horario(t)
			}
			var bloqueos []entidades.DiaBloqueado
			if caso.bloqueos != nil {
				bloqueos = caso.bloqueos(t)
			}
			var reservas []entidades.Reserva
			if caso.reservas != nil {
				reservas = caso.reservas(t)
			}

			slots := dominio.CalcularSlotsDisponibles(fecha, h, bloqueos, reservas, caso.ahora)

			if len(slots) != len(caso.disponibilidadEsperada) {
				t.Fatalf("se esperaban %d slots, se obtuvieron %d: %+v", len(caso.disponibilidadEsperada), len(slots), slots)
			}
			for i, esperado := range caso.disponibilidadEsperada {
				if slots[i].Disponible != esperado {
					t.Errorf("slot %d (inicio %s): Disponible = %v, se esperaba %v", i, slots[i].Inicio, slots[i].Disponible, esperado)
				}
			}
		})
	}
}
