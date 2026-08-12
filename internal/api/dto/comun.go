// Package dto define las formas de entrada y salida de la API HTTP
// (los "Data Transfer Objects"): lo que llega en el cuerpo de un pedido
// JSON y lo que se devuelve en la respuesta. No son entidades del
// dominio — son su traducción de/hacia JSON, con su propia validación de
// forma (formato de fecha, campos requeridos), separada de las reglas de
// negocio que ya validan los constructores de entidades/ y los casos de
// uso de aplicacion/.
package dto

import (
	"fmt"
	"strconv"
	"strings"

	"reservas-go/internal/dominio/entidades"
)

// ParsearHoraDelDia interpreta un string "HH:MM" (ej. "09:30") como una
// entidades.HoraDelDia. Se usa tanto para horarios de atención como para
// la hora de un bloqueo parcial.
func ParsearHoraDelDia(s string) (entidades.HoraDelDia, error) {
	partes := strings.Split(s, ":")
	if len(partes) != 2 {
		return entidades.HoraDelDia{}, fmt.Errorf("formato esperado HH:MM (ej. 09:30)")
	}

	horas, err := strconv.Atoi(partes[0])
	if err != nil {
		return entidades.HoraDelDia{}, fmt.Errorf("formato esperado HH:MM (ej. 09:30)")
	}
	minutos, err := strconv.Atoi(partes[1])
	if err != nil {
		return entidades.HoraDelDia{}, fmt.Errorf("formato esperado HH:MM (ej. 09:30)")
	}

	return entidades.NuevaHoraDelDia(horas, minutos)
}
