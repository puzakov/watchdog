package db

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"
)

type DatabaseConnection struct {
	Pool *pgxpool.Pool
}

func NewDatabaseConnection(ctx context.Context, connString string) (*DatabaseConnection, error) {
	if connString == "" {
		return nil, errors.New("empty connection string")
	}

	pool, err := pgxpool.New(ctx, connString)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}

	return &DatabaseConnection{Pool: pool}, nil
}

func (c *DatabaseConnection) Ping(ctx context.Context) error {
	if c == nil || c.Pool == nil {
		return errors.New("database pool is nil")
	}
	return c.Pool.Ping(ctx)
}

func (c *DatabaseConnection) Close() {
	if c == nil || c.Pool == nil {
		return
	}
	c.Pool.Close()
}
