package seguridad

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"reservas-go/internal/dominio/entidades"
)

// Claims son los datos que viajan adentro del JWT, además de los
// estándar (fecha de emisión, de expiración, etc. — jwt.RegisteredClaims
// ya los provee). Guardamos el ID del usuario y su Rol: el rol viaja dentro
// del token para que el middleware de autorización (Fase 4) pueda decidir
// si el pedido está permitido sin tener que ir a buscar el usuario a la
// base de datos en cada pedido.
type Claims struct {
	UsuarioID entidades.ID  `json:"usuario_id"`
	Rol       entidades.Rol `json:"rol"`
	jwt.RegisteredClaims
}

// GenerarToken crea y firma un JWT para un usuario. "ahora" y "duracion"
// se reciben como parámetros (en vez de que esta función llame a
// time.Now() por su cuenta) por la misma razón que en el dominio: queda
// determinística y fácil de testear con fechas fijas.
func GenerarToken(usuarioID entidades.ID, rol entidades.Rol, secreto []byte, ahora time.Time, duracion time.Duration) (string, error) {
	claims := Claims{
		UsuarioID: usuarioID,
		Rol:       rol,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(ahora),
			ExpiresAt: jwt.NewNumericDate(ahora.Add(duracion)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	firmado, err := token.SignedString(secreto)
	if err != nil {
		return "", fmt.Errorf("firmando token: %w", err)
	}
	return firmado, nil
}

// ValidarToken verifica la firma de un JWT y que no haya expirado, y
// devuelve sus Claims si es válido.
func ValidarToken(tokenString string, secreto []byte) (*Claims, error) {
	claims := &Claims{}

	token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (any, error) {
		// Verificamos a mano que el algoritmo de firma sea el que
		// esperamos (HMAC). Sin este chequeo, un atacante podría mandar
		// un token armado con otro algoritmo (por ejemplo "none", que no
		// firma nada) y hacer que la librería lo acepte como válido. Esto
		// se conoce como un ataque de "confusión de algoritmo" — es un
		// error de seguridad real y conocido en implementaciones de JWT
		// que confían ciegamente en el algoritmo que el propio token dice
		// usar.
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("método de firma inesperado: %v", t.Header["alg"])
		}
		return secreto, nil
	})
	if err != nil {
		return nil, fmt.Errorf("token inválido: %w", err)
	}
	if !token.Valid {
		return nil, fmt.Errorf("token inválido")
	}

	return claims, nil
}
