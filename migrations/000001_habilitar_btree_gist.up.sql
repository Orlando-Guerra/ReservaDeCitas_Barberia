-- btree_gist permite combinar, en un mismo índice GiST, columnas de
-- igualdad (como un uuid) con operadores de rango como "&&" (solapamiento).
-- Sin esta extensión, un índice GiST solo entiende operadores de rango.
-- La vamos a usar en 000006_crear_reservas para el constraint que impide
-- la doble reserva.
CREATE EXTENSION IF NOT EXISTS btree_gist;
