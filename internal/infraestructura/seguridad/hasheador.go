package seguridad

// HasheadorBcrypt implementa puertos.HasheadorPasswords usando bcrypt.
// No tiene estado (no necesita campos), así que es un struct vacío: solo
// existe para tener un tipo sobre el que colgar los métodos que la
// interfaz pide.
type HasheadorBcrypt struct{}

// Hashear delega en la función libre HashearPassword de este mismo
// paquete.
func (HasheadorBcrypt) Hashear(password string) (string, error) {
	return HashearPassword(password)
}

// Verificar delega en la función libre VerificarPassword.
func (HasheadorBcrypt) Verificar(hashAlmacenado, passwordIngresada string) bool {
	return VerificarPassword(hashAlmacenado, passwordIngresada)
}
