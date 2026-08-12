package entidades

import (
	"fmt"
	"time"
)

// DiaBloqueado representa un día (o parte de un día) en el que la
// barbería no atiende: un feriado, vacaciones, o un bloqueo de emergencia
// de último momento.
//
// Si HoraDesde es nil, se bloquea el día completo. Si tiene un valor, se
// bloquea desde esa hora hasta el fin del horario de atención de ese día
// (ej. el barbero tiene una emergencia a las 15:00 y ya no puede atender
// el resto del día). Usamos un puntero acá, en vez de una HoraDelDia
// normal, porque necesitamos representar "no se especificó ninguna hora"
// — y con un valor normal no habría forma de distinguir eso de "se
// especificó la hora 00:00".
type DiaBloqueado struct {
	ID        ID
	Fecha     time.Time
	HoraDesde *HoraDelDia
	Motivo    string
}

// NuevoDiaBloqueado crea un DiaBloqueado. horaDesde puede ser nil para
// bloquear el día completo.
func NuevoDiaBloqueado(fecha time.Time, horaDesde *HoraDelDia, motivo string) (DiaBloqueado, error) {
	if motivo == "" {
		return DiaBloqueado{}, fmt.Errorf("el motivo del bloqueo no puede estar vacío")
	}

	// Nos quedamos solo con la fecha (año/mes/día): la hora exacta de
	// "Fecha" no tiene significado propio, el que sí lo tiene es HoraDesde.
	fechaSinHora := time.Date(fecha.Year(), fecha.Month(), fecha.Day(), 0, 0, 0, 0, time.UTC)

	return DiaBloqueado{
		ID:        NuevoID(),
		Fecha:     fechaSinHora,
		HoraDesde: horaDesde,
		Motivo:    motivo,
	}, nil
}
