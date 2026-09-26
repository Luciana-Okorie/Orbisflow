package database

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// NewPool opens a pgx connection pool against the given DSN (DATABASE_URL).
// Callers are responsible for calling pool.Close() on shutdown.
func NewPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return pool, nil
}
