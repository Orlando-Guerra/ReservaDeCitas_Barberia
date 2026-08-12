CREATE TABLE usuarios (
    id            uuid PRIMARY KEY,
    nombre        text NOT NULL,
    email         text NOT NULL UNIQUE,
    password_hash text NOT NULL,
    rol           text NOT NULL CHECK (rol IN ('cliente', 'administrador')),
    creado_en     timestamptz NOT NULL DEFAULT now()
);
