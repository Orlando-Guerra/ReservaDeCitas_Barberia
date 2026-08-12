-- Guardamos la hora de inicio/fin como "minutos desde medianoche" (un
-- entero simple, 0-1439) en vez de usar el tipo nativo `time` de Postgres.
-- Es una simplificación deliberada: evita la complejidad extra de mapear
-- el tipo `time` con pgx, y en este proyecto no perdemos nada porque
-- nunca necesitamos operar con estas horas en SQL directamente (todo el
-- cálculo de slots vive en Go, en el dominio).
CREATE TABLE horarios_atencion (
    id                  uuid PRIMARY KEY,
    dia_semana          smallint NOT NULL CHECK (dia_semana BETWEEN 0 AND 6), -- 0=domingo ... 6=sábado (igual que time.Weekday de Go)
    hora_inicio_minutos smallint NOT NULL CHECK (hora_inicio_minutos BETWEEN 0 AND 1439),
    hora_fin_minutos    smallint NOT NULL CHECK (hora_fin_minutos BETWEEN 0 AND 1439),
    CONSTRAINT horario_valido CHECK (hora_inicio_minutos < hora_fin_minutos),
    -- A lo sumo un horario por día de la semana (decisión de Fase 2: un
    -- solo rango por día, sin cortes de almuerzo).
    UNIQUE (dia_semana)
);
