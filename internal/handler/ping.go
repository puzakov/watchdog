package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/puzakov/watchdog/internal/db"
)

func HandlePing(conn *db.DatabaseConnection, w http.ResponseWriter) {
	if conn == nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(1)*time.Second)
	defer cancel()

	err := conn.Ping(ctx)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if ctx.Err() != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
