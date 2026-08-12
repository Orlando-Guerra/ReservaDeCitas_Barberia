package middleware

import (
	"net/http"

	"reservas-go/internal/dominio/entidades"
)

// RequiereRol exige que el usuario autenticado tenga alguno de los roles
// permitidos. Tiene que correr DESPUÉS de Autenticacion en la cadena
// (necesita que ya haya un UsuarioAutenticado en el contexto): si no lo
// encuentra, corta con 401; si lo encuentra pero el rol no está
// permitido, corta con 403.
//
// Por qué es un middleware separado de Autenticacion, en vez de una sola
// función que hace las dos cosas: son preguntas distintas.
// Autenticacion responde "¿quién sos, y sos de verdad quien decís ser?"
// (identidad). RequiereRol responde "vos, que ya sé quién sos, ¿podés
// hacer ESTO puntual?" (permiso). Separarlos permite, por ejemplo, tener
// rutas que solo necesitan autenticación (cualquier usuario logueado)
// sin exigir ningún rol en particular, y reutilizar RequiereRol con
// distintas combinaciones de roles en cada ruta que sí lo necesite.
func RequiereRol(rolesPermitidos ...entidades.Rol) func(http.Handler) http.Handler {
	return func(siguiente http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			usuario, ok := UsuarioDesdeContexto(r.Context())
			if !ok {
				http.Error(w, `{"error":"no autenticado"}`, http.StatusUnauthorized)
				return
			}

			for _, rol := range rolesPermitidos {
				if usuario.Rol == rol {
					siguiente.ServeHTTP(w, r)
					return
				}
			}

			http.Error(w, `{"error":"no autorizado para este recurso"}`, http.StatusForbidden)
		})
	}
}
