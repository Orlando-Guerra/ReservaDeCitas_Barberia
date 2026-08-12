package entidades

import (
	"fmt"
	"time"
)

// HorarioAtencion define, para un día de la semana, entre qué hora y qué
// hora atiende la barbería.
//
// Usamos time.Weekday de la librería estándar (time.Sunday = 0 ...
// time.Saturday = 6) en vez de inventar un enum propio de días: es un
// tipo que ya existe, ya sabe imprimirse ("Monday", etc.) y ya lo conoce
// cualquiera que use time.Time.
//
// Un día de la semana sin HorarioAtencion configurado es un día de
// descanso fijo (ej. domingo) — no se modela con DiaBloqueado, que queda
// reservado para excepciones puntuales.
type HorarioAtencion struct {
	ID         ID
	DiaSemana  time.Weekday
	HoraInicio HoraDelDia
	HoraFin    HoraDelDia
}

// NuevoHorarioAtencion crea un HorarioAtencion validando que la hora de
// inicio sea anterior a la hora de fin.
func NuevoHorarioAtencion(diaSemana time.Weekday, horaInicio, horaFin HoraDelDia) (HorarioAtencion, error) {
	if !horaInicio.AntesDe(horaFin) {
		return HorarioAtencion{}, fmt.Errorf("la hora de inicio (%s) debe ser anterior a la hora de fin (%s)", horaInicio, horaFin)
	}

	return HorarioAtencion{
		ID:         NuevoID(),
		DiaSemana:  diaSemana,
		HoraInicio: horaInicio,
		HoraFin:    horaFin,
	}, nil
}
