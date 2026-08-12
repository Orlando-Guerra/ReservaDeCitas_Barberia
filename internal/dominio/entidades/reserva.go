package entidades

import (
	"fmt"
	"time"
)

// EstadoReserva indica en qué situación está una Reserva.
type EstadoReserva string

const (
	// ReservaConfirmada es el estado inicial de toda reserva: no hay paso
	// de aprobación manual, se confirma automáticamente al crearse.
	ReservaConfirmada EstadoReserva = "confirmada"

	// ReservaCancelada indica que el cliente (o el administrador) canceló
	// el turno.
	ReservaCancelada EstadoReserva = "cancelada"
)

// DuracionSlot es la duración fija de todo bloque reservable en este
// proyecto: cada reserva ocupa siempre 1 hora completa, sin importar la
// duración configurada del Servicio (ver docs/CONTEXTO.md, decisión de
// Fase 0).
const DuracionSlot = time.Hour

// Reserva representa un turno reservado: un cliente, un servicio, y un
// bloque de tiempo de DuracionSlot.
type Reserva struct {
	ID         ID
	ClienteID  ID
	ServicioID ID
	Inicio     time.Time
	Fin        time.Time
	Estado     EstadoReserva
	CreadaEn   time.Time
	// CanceladaEn es nil mientras la reserva sigue confirmada. Usamos un
	// puntero (en vez de time.Time a secas) para poder representar "todavía
	// no tiene valor": con time.Time normal, el "valor cero" (01/01/0001)
	// sería ambiguo con una fecha real; con un puntero, nil no deja lugar
	// a dudas.
	CanceladaEn *time.Time
}

// NuevaReserva crea una Reserva confirmada que ocupa el bloque de
// DuracionSlot que empieza en "inicio".
func NuevaReserva(clienteID, servicioID ID, inicio, ahora time.Time) (Reserva, error) {
	if clienteID == "" {
		return Reserva{}, fmt.Errorf("la reserva necesita un cliente")
	}
	if servicioID == "" {
		return Reserva{}, fmt.Errorf("la reserva necesita un servicio")
	}

	return Reserva{
		ID:         NuevoID(),
		ClienteID:  clienteID,
		ServicioID: servicioID,
		Inicio:     inicio,
		Fin:        inicio.Add(DuracionSlot),
		Estado:     ReservaConfirmada,
		CreadaEn:   ahora,
	}, nil
}
