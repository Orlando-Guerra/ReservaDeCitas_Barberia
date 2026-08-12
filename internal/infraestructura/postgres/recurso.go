package postgres

// RecursoUnicoID identifica el único recurso reservable de este proyecto
// (la barbería / el único barbero). La tabla "reservas" ya está preparada
// para múltiples recursos (ver migración 000006_crear_reservas y el
// constraint de exclusión), pero el dominio de este proyecto modela un
// solo recurso a propósito (decisión de Fase 0).
//
// Por eso esta constante vive acá, en infraestructura, y no en el
// dominio: hoy no es una regla de negocio ("elegí este barbero entre
// varios"), es un detalle de cómo tuvo que modelarse la tabla para poder
// usar el constraint de exclusión estándar de la industria. Si el día de
// mañana se agrega soporte a múltiples barberos, ahí sí "Recurso" pasaría
// a ser un concepto del dominio.
const RecursoUnicoID = "00000000-0000-0000-0000-000000000001"
