CREATE TABLE dias_bloqueados (
    id                  uuid PRIMARY KEY,
    fecha               date NOT NULL,
    -- NULL = se bloquea el día completo. Con valor = se bloquea desde esa
    -- hora (en minutos desde medianoche) hasta el fin del día.
    hora_desde_minutos  smallint CHECK (hora_desde_minutos BETWEEN 0 AND 1439),
    motivo              text NOT NULL,
    -- A lo sumo un bloqueo por fecha.
    UNIQUE (fecha)
);
