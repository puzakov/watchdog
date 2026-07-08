package db

import (
	"context"
	"testing"
)

func TestNewDatabaseConnection_EmptyString(t *testing.T) {
	conn, err := NewDatabaseConnection(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty connection string")
	}
	if conn != nil {
		t.Fatal("expected nil connection for empty connection string")
	}
}

func TestDatabaseConnection_Ping_NilReceiver(t *testing.T) {
	var conn *DatabaseConnection
	err := conn.Ping(context.Background())
	if err == nil {
		t.Fatal("expected error for nil receiver")
	}
}

func TestDatabaseConnection_Ping_NilPool(t *testing.T) {
	conn := &DatabaseConnection{Pool: nil}
	err := conn.Ping(context.Background())
	if err == nil {
		t.Fatal("expected error for nil pool")
	}
}

func TestDatabaseConnection_Close_NilReceiver(t *testing.T) {
	var conn *DatabaseConnection
	conn.Close() // should not panic
}

func TestDatabaseConnection_Close_NilPool(t *testing.T) {
	conn := &DatabaseConnection{Pool: nil}
	conn.Close() // should not panic
}
