// Package middleware contiene los adaptadores de entrada que envuelven
// los handlers HTTP: autenticación (¿quién sos?) y autorización (¿podés
// hacer esto?). Son dos preocupaciones separadas a propósito, cada una en
// su propio middleware — ver docs/APRENDIZAJE.md para la diferencia.
package middleware

import (
	"context"
	"net/http"
	"strings"

	"reservas-go/internal/dominio/entidades"
	"reservas-go/internal/infraestructura/seguridad"
)

// claveContexto es un tipo propio (no "string") para las claves que
// guardamos en el context.Context de cada pedido. Si usáramos un string
// como clave (ej. "usuario"), cualquier otro paquete que también use
// context.WithValue con la clave "usuario" pisaría o leería por
// accidente nuestro valor. Con un tipo definido en este paquete, eso es
// imposible: ninguna otra clave, de ningún otro paquete, puede ser igual
// a claveUsuarioAutenticado, aunque por adentro sea "el mismo" entero.
type claveContexto int

const claveUsuarioAutenticado claveContexto = iota

// UsuarioAutenticado son los datos del usuario ya extraídos y validados
// del JWT, disponibles para cualquier handler más adelante en la cadena
// (vía UsuarioDesdeContexto).
type UsuarioAutenticado struct {
	ID  entidades.ID
	Rol entidades.Rol
}

// Autenticacion valida el header "Authorization: Bearer <token>". Si es
// válido, guarda los datos del usuario en el contexto del pedido y llama
// al siguiente handler. Si falta o es inválido, corta la cadena acá
// mismo con 401 — el siguiente handler ni se entera de que el pedido
// existió.
//
// Devuelve una función que envuelve un http.Handler (esto se llama
// "middleware" en el sentido de Go: no es una clase base ni una
// interfaz especial, es simplemente una función que recibe un handler y
// devuelve otro handler que hace algo extra antes o después de llamar al
// original).
func Autenticacion(secreto []byte) func(http.Handler) http.Handler {
	return func(siguiente http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			encabezado := r.Header.Get("Authorization")
			token, hayToken := strings.CutPrefix(encabezado, "Bearer ")
			if !hayToken || token == "" {
				http.Error(w, `{"error":"falta el header Authorization con un token Bearer"}`, http.StatusUnauthorized)
				return
			}

			claims, err := seguridad.ValidarToken(token, secreto)
			if err != nil {
				http.Error(w, `{"error":"token inválido o expirado"}`, http.StatusUnauthorized)
				return
			}

			usuario := UsuarioAutenticado{ID: claims.UsuarioID, Rol: claims.Rol}
			// r.WithContext devuelve una COPIA del *http.Request con el
			// nuevo contexto — no modifica "r" en el lugar. Por eso hay
			// que pasar explícitamente ese nuevo request a siguiente.ServeHTTP:
			// si le pasáramos "r" a secas, el handler siguiente no vería
			// el usuario que acabamos de guardar.
			ctx := context.WithValue(r.Context(), claveUsuarioAutenticado, usuario)
			siguiente.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// UsuarioDesdeContexto recupera el UsuarioAutenticado que Autenticacion
// dejó en el contexto. El segundo valor devuelto es false si
// Autenticacion no corrió antes en la cadena para este pedido — un error
// de programación (una ruta protegida armada sin pasar por el
// middleware), no algo que deba pasar en producción.
func UsuarioDesdeContexto(ctx context.Context) (UsuarioAutenticado, bool) {
	usuario, ok := ctx.Value(claveUsuarioAutenticado).(UsuarioAutenticado)
	return usuario, ok
}
