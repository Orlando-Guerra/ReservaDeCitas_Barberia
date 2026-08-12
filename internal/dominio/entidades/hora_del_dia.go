package entidades

import (
	"fmt"
	"time"
)

// HoraDelDia representa una hora del día sin fecha asociada (ej. "09:00"),
// para configurar horarios de atención. La librería estándar de Go no
// tiene un tipo "solo hora": time.Time siempre incluye una fecha completa.
// Por eso definimos el nuestro.
type HoraDelDia struct {
	Horas   int
	Minutos int
}

// NuevaHoraDelDia crea una HoraDelDia validando que horas y minutos sean
// valores reales (0-23 y 0-59).
func NuevaHoraDelDia(horas, minutos int) (HoraDelDia, error) {
	if horas < 0 || horas > 23 {
		return HoraDelDia{}, fmt.Errorf("la hora %d está fuera de rango (0-23)", horas)
	}
	if minutos < 0 || minutos > 59 {
		return HoraDelDia{}, fmt.Errorf("los minutos %d están fuera de rango (0-59)", minutos)
	}
	return HoraDelDia{Horas: horas, Minutos: minutos}, nil
}

// AntesDe indica si h ocurre antes que otra hora del mismo día.
func (h HoraDelDia) AntesDe(otra HoraDelDia) bool {
	if h.Horas != otra.Horas {
		return h.Horas < otra.Horas
	}
	return h.Minutos < otra.Minutos
}

// EnFecha combina esta hora del día con la fecha dada, devolviendo un
// time.Time completo en UTC.
//
// Guardamos y calculamos todo en UTC dentro del dominio; convertir a la
// zona horaria del usuario es responsabilidad de la capa de presentación,
// no del dominio (el detalle completo de por qué se explica en
// docs/CONCURRENCIA.md, a partir de la Fase 3).
func (h HoraDelDia) EnFecha(fecha time.Time) time.Time {
	return time.Date(fecha.Year(), fecha.Month(), fecha.Day(), h.Horas, h.Minutos, 0, 0, time.UTC)
}

// String implementa la interfaz fmt.Stringer, así que una HoraDelDia se
// imprime como "09:05" en vez de "{9 5}" al usarla con fmt.Println, %v,
// %s, etc. Go llama a este método automáticamente cada vez que el valor
// pasa por una función de fmt — no hace falta declarar en ningún lado
// "HoraDelDia implementa Stringer".
func (h HoraDelDia) String() string {
	return fmt.Sprintf("%02d:%02d", h.Horas, h.Minutos)
}
