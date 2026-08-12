// Package seguridad implementa los detalles criptográficos concretos que
// el dominio no puede tener: hasheo de contraseñas (bcrypt) y JWT. Es
// infraestructura — puede importar librerías de terceros libremente.
package seguridad

import (
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// HashearPassword calcula el hash bcrypt de una contraseña en texto
// plano.
//
// bcrypt incluye un "salt" aleatorio distinto en cada hash que calcula —
// por eso hashear la misma contraseña dos veces da como resultado dos
// strings distintos (lo comprobamos en el test). Y es deliberadamente
// lento: a diferencia de algo como SHA-256 (rápido a propósito, y por eso
// NO sirve para contraseñas: si se filtra la base, permite probar miles
// de millones de combinaciones por segundo), bcrypt hace que cada intento
// de verificación cueste un tiempo perceptible, lo que vuelve inviable un
// ataque de fuerza bruta masivo contra contraseñas filtradas.
func HashearPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hasheando contraseña: %w", err)
	}
	return string(hash), nil
}

// VerificarPassword compara una contraseña en texto plano contra un hash
// ya calculado (el que se guardó en Usuario.PasswordHash). Devuelve true
// si coinciden.
func VerificarPassword(hashAlmacenado, passwordIngresada string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hashAlmacenado), []byte(passwordIngresada))
	return err == nil
}
