// Package handlers contiene los adaptadores HTTP: traducen pedidos HTTP
// a llamadas a casos de uso de aplicacion/, y sus resultados de vuelta a
// JSON. No tienen lógica de negocio propia — si un handler empieza a
// tener un "if" que decide algo sobre reservas o slots, esa lógica está
// en el lugar equivocado y debería vivir en aplicacion/ o dominio/.
package handlers

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"reservas-go/internal/dominio"
)

// responderJSON escribe "cuerpo" como JSON con el código de estado dado.
func responderJSON(w http.ResponseWriter, status int, cuerpo any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if cuerpo != nil {
		if err := json.NewEncoder(w).Encode(cuerpo); err != nil {
			log.Printf("error codificando respuesta JSON: %v", err)
		}
	}
}

type respuestaError struct {
	Error string `json:"error"`
}

// responderError escribe un error como {"error": "..."} con el código de
// estado dado.
func responderError(w http.ResponseWriter, status int, mensaje string) {
	responderJSON(w, status, respuestaError{Error: mensaje})
}

// manejarError centraliza la traducción de errores de los casos de uso a
// códigos de estado HTTP. Es el único lugar del proyecto que conoce la
// relación completa entre "qué salió mal" y "qué código HTTP
// corresponde" — cada handler llama a esto para cualquier error que
// venga de aplicacion/, en vez de decidir el código por su cuenta. Así,
// si mañana cambiamos qué código le corresponde a un error, se cambia en
// un solo lugar.
func manejarError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, dominio.ErrSlotNoDisponible),
		errors.Is(err, dominio.ErrDiaBloqueadoConReservas),
		errors.Is(err, dominio.ErrEmailYaRegistrado),
		errors.Is(err, dominio.ErrReservaYaCancelada):
		responderError(w, http.StatusConflict, err.Error())

	case errors.Is(err, dominio.ErrCancelacionFueraDePlazo),
		errors.Is(err, dominio.ErrFechaFueraDeAnticipacion):
		responderError(w, http.StatusUnprocessableEntity, err.Error())

	case errors.Is(err, dominio.ErrCredencialesInvalidas):
		responderError(w, http.StatusUnauthorized, err.Error())

	case errors.Is(err, dominio.ErrNoAutorizado):
		responderError(w, http.StatusForbidden, err.Error())

	case errors.Is(err, dominio.ErrNoEncontrado):
		responderError(w, http.StatusNotFound, err.Error())

	default:
		// Acá NO devolvemos err.Error() al cliente: puede contener
		// detalles internos (qué tabla, qué driver, qué connection
		// string) que no queremos exponer por HTTP. Lo dejamos en el log
		// del servidor, que es donde alguien con acceso al backend puede
		// investigarlo, y le devolvemos al cliente un mensaje genérico.
		log.Printf("error interno no manejado: %v", err)
		responderError(w, http.StatusInternalServerError, "error interno")
	}
}
