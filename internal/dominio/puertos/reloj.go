// Package puertos define las interfaces (contratos) que el dominio y los
// casos de uso necesitan para hablar con el mundo exterior — persistencia,
// notificaciones, tiempo — sin saber cómo está implementado del otro lado.
// La infraestructura (Fase 3 en adelante) implementa estas interfaces.
package puertos

import "time"

// Reloj abstrae "qué hora es ahora mismo".
//
// El dominio y los casos de uso nunca llaman a time.Now() directamente:
// en su lugar, reciben un Reloj y le preguntan Ahora(). ¿Por qué importa
// esto? En los tests podemos usar una implementación falsa que siempre
// devuelve la misma hora fija, así los tests quedan 100% determinísticos
// y no dependen de "el momento justo" en que se corran. Por ejemplo, para
// probar la regla "no se puede cancelar con menos de 2 horas de
// anticipación" no hace falta esperar 2 horas de verdad, ni el test falla
// solo porque se ejecutó a las 23:00: simplemente le pasamos un reloj
// fijo con la hora que necesitemos para el caso que estemos probando.
type Reloj interface {
	// Ahora devuelve el instante actual, en UTC.
	Ahora() time.Time
}
