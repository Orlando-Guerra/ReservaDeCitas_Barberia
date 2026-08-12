// Package entidades contiene los structs que modelan los conceptos del
// negocio (Usuario, Servicio, Reserva, etc.) junto con los constructores
// que validan sus datos. No importa nada fuera de la librería estándar de
// Go: es el corazón del dominio, y el dominio no depende de nadie.
package entidades

import (
	"crypto/rand"
	"fmt"
)

// ID identifica de forma única a cualquier entidad del dominio. Es un
// UUID versión 4: 16 bytes aleatorios criptográficamente seguros,
// formateados como texto (ej. "a1b2c3d4-e5f6-4a1b-8c2d-1234567890ab").
// Usamos un identificador no secuencial a propósito, para que nadie pueda
// adivinar "¿existirá la reserva siguiente a esta?" mirando una URL.
//
// El dominio genera sus propios IDs con crypto/rand en vez de depender de
// una librería externa de UUID, porque la regla del proyecto es que
// "dominio" solo puede importar la librería estándar de Go.
type ID string

// NuevoID genera un identificador único nuevo (UUID versión 4, variante
// RFC 4122).
func NuevoID() ID {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		// rand.Read solo falla si el sistema operativo no puede entregar
		// aleatoriedad — algo que en la práctica no ocurre en un sistema
		// operativo moderno. Si pasara, preferimos frenar en seco antes
		// que seguir generando IDs que podrían no ser realmente únicos.
		panic(fmt.Sprintf("no se pudo generar un ID aleatorio: %v", err))
	}

	b[6] = (b[6] & 0x0f) | 0x40 // fija la versión en 4
	b[8] = (b[8] & 0x3f) | 0x80 // fija la variante en RFC 4122

	return ID(fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]))
}
