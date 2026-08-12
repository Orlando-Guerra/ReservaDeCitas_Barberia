package entidades

import "fmt"

// Servicio es algo que la barbería ofrece (ej. "Corte de pelo", "Barba").
//
// El precio se guarda en centavos, como entero, para evitar por completo
// los problemas de precisión de los números de punto flotante al trabajar
// con dinero (ej. 0.1 + 0.2 no da exactamente 0.3 en float64). Se formatea
// a moneda recién al mostrarlo, nunca antes.
type Servicio struct {
	ID              ID
	Nombre          string
	DuracionMinutos int
	PrecioCentavos  int64
	Activo          bool
}

// NuevoServicio crea un Servicio validando sus datos.
//
// La duración se guarda como dato informativo (y de referencia para el
// precio) — no afecta el cálculo de slots, porque en este proyecto cada
// reserva ocupa siempre un bloque fijo de 1 hora, sin importar la
// duración configurada del servicio (ver docs/CONTEXTO.md, decisión de
// Fase 0).
func NuevoServicio(nombre string, duracionMinutos int, precioCentavos int64) (Servicio, error) {
	if nombre == "" {
		return Servicio{}, fmt.Errorf("el nombre del servicio no puede estar vacío")
	}
	if duracionMinutos <= 0 {
		return Servicio{}, fmt.Errorf("la duración debe ser mayor a 0 minutos")
	}
	if precioCentavos < 0 {
		return Servicio{}, fmt.Errorf("el precio no puede ser negativo")
	}

	return Servicio{
		ID:              NuevoID(),
		Nombre:          nombre,
		DuracionMinutos: duracionMinutos,
		PrecioCentavos:  precioCentavos,
		Activo:          true,
	}, nil
}
