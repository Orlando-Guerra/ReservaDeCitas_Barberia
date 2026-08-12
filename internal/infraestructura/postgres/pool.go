// Package postgres implementa los puertos del dominio (repositorios)
// contra PostgreSQL, usando pgx/v5. Es infraestructura: puede importar
// librerías de terceros libremente, algo que internal/dominio tiene
// prohibido.
package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// NuevoPool crea un pool de conexiones a PostgreSQL.
//
// Un pool mantiene varias conexiones ya abiertas y las va prestando a
// cada consulta que las necesita, en vez de abrir una conexión TCP nueva
// (con su handshake, autenticación, etc.) cada vez — eso sería
// muchísimo más lento bajo cualquier carga real. pgxpool además maneja
// solo la cola de espera cuando todas las conexiones del pool están
// ocupadas.
//
// Verificamos la conexión con Ping antes de devolver el pool, para que
// un problema de configuración (host mal escrito, credenciales
// inválidas) se detecte al arrancar la aplicación, no en el primer
// pedido HTTP que la necesite.
func NuevoPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("creando pool de conexiones: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("verificando conexión a la base de datos: %w", err)
	}

	return pool, nil
}
