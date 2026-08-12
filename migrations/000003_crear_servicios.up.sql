CREATE TABLE servicios (
    id               uuid PRIMARY KEY,
    nombre           text NOT NULL,
    duracion_minutos smallint NOT NULL CHECK (duracion_minutos > 0),
    precio_centavos  bigint NOT NULL CHECK (precio_centavos >= 0),
    activo           boolean NOT NULL DEFAULT true
);
