package seguridad

import (
	"time"

	"reservas-go/internal/dominio/entidades"
)

// GeneradorTokensJWT implementa puertos.GeneradorTokens usando JWT.
type GeneradorTokensJWT struct {
	secreto  []byte
	duracion time.Duration
}

// NuevoGeneradorTokensJWT crea un GeneradorTokensJWT. "duracion" es
// cuánto tiempo va a durar cada token emitido antes de expirar.
func NuevoGeneradorTokensJWT(secreto []byte, duracion time.Duration) *GeneradorTokensJWT {
	return &GeneradorTokensJWT{secreto: secreto, duracion: duracion}
}

// Generar crea un token nuevo para el usuario y rol dados, usando la hora
// real del sistema como momento de emisión.
func (g *GeneradorTokensJWT) Generar(usuarioID entidades.ID, rol entidades.Rol) (string, error) {
	return GenerarToken(usuarioID, rol, g.secreto, time.Now().UTC(), g.duracion)
}
