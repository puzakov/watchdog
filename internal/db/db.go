package db

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

type DatabaseConnection struct {
	*pgx.Conn
}

func NewDatabaseConnection(ctx context.Context, connString string) (*DatabaseConnection, error) {
	if connString == "" {
		return nil, errors.New("empty connection string")
	}

	conn, err := pgx.Connect(ctx, connString)
	return &DatabaseConnection{conn}, err
}
