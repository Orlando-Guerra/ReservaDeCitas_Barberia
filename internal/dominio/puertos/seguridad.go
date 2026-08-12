package puertos

import "reservas-go/internal/dominio/entidades"

// HasheadorPasswords abstrae el hasheo y verificación de contraseñas. La
// infraestructura lo implementa con bcrypt (Fase 4).
//
// ¿Por qué un puerto para esto, si ya usamos las funciones de
// infraestructura/seguridad directamente en la Fase 4? Porque los casos
// de uso de aplicación (Fase 5) necesitan poder testearse sin depender
// de bcrypt real: bcrypt es deliberadamente lento (por diseño, para
// dificultar ataques de fuerza bruta), así que un test que llame a
// bcrypt de verdad cientos de veces sería notablemente más lento que uno
// que use una implementación falsa y rápida. Con este puerto, los tests
// de los casos de uso pueden usar un hasheador falso instantáneo.
type HasheadorPasswords interface {
	Hashear(password string) (string, error)
	Verificar(hashAlmacenado, passwordIngresada string) bool
}

// GeneradorTokens abstrae la creación de tokens de sesión. La
// infraestructura lo implementa con JWT (Fase 4), pero los casos de uso
// que necesitan emitir un token (como el login) no dependen de esa
// librería concreta — solo de este contrato.
type GeneradorTokens interface {
	Generar(usuarioID entidades.ID, rol entidades.Rol) (string, error)
}
