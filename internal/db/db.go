package db

import (
	"context"

	"github.com/jackc/pgx/v5"
)

type DatabaseConnection struct {
	*pgx.Conn
}

func NewDatabaseConnection(ctx context.Context, connString string) (DatabaseConnection, error) {
	conn, err := pgx.Connect(ctx, connString)
	return DatabaseConnection{conn}, err
}
