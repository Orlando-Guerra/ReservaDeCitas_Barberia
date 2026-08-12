CREATE TABLE reservas (
    id            uuid PRIMARY KEY,

    -- Identifica el "recurso" reservable (hoy siempre el mismo valor fijo,
    -- ver internal/infraestructura/postgres/recurso.go). El dominio de
    -- este proyecto modela un solo recurso a propósito (decisión de Fase
    -- 0); esta columna existe para que el constraint de más abajo use el
    -- patrón general de la industria, y para que soportar más de un
    -- recurso en el futuro no requiera romper el esquema, solo empezar a
    -- usar distintos valores acá.
    recurso_id    uuid NOT NULL DEFAULT '00000000-0000-0000-0000-000000000001',

    cliente_id    uuid NOT NULL REFERENCES usuarios(id),
    servicio_id   uuid NOT NULL REFERENCES servicios(id),

    inicio        timestamptz NOT NULL,
    fin           timestamptz NOT NULL,
    CONSTRAINT fin_despues_de_inicio CHECK (fin > inicio),

    estado        text NOT NULL CHECK (estado IN ('confirmada', 'cancelada')),
    creada_en     timestamptz NOT NULL DEFAULT now(),
    cancelada_en  timestamptz,

    -- Columna generada: Postgres la calcula sola a partir de inicio/fin y
    -- la guarda en disco (STORED). "[)" significa "inicio incluido, fin
    -- excluido" — así un turno de 9:00 a 10:00 no choca con uno de 10:00 a
    -- 11:00. La necesitamos como columna propia porque el constraint de
    -- exclusión de abajo trabaja sobre un tipo range, no sobre dos
    -- columnas timestamptz sueltas.
    rango tstzrange GENERATED ALWAYS AS (tstzrange(inicio, fin, '[)')) STORED
);

CREATE INDEX idx_reservas_inicio ON reservas (inicio);
CREATE INDEX idx_reservas_estado ON reservas (estado);

-- EL CONSTRAINT MÁS IMPORTANTE DEL PROYECTO.
--
-- Le dice a Postgres: "para el mismo recurso_id, nunca aceptes dos filas
-- cuyos rangos se solapen (&&), entre las que están 'confirmada'". No es
-- una validación que corre en Go antes de insertar — es una regla que el
-- propio motor de la base de datos hace cumplir en el momento exacto del
-- INSERT, a nivel de índice, sin importar cuántas conexiones intenten
-- insertar al mismo tiempo. Ver docs/CONCURRENCIA.md para la explicación
-- completa de por qué esto es imprescindible y un chequeo en Go (tipo
-- "if ya existe una reserva...") no alcanza.
ALTER TABLE reservas
    ADD CONSTRAINT reservas_no_solapadas
    EXCLUDE USING gist (
        recurso_id WITH =,
        rango WITH &&
    )
    WHERE (estado = 'confirmada');
