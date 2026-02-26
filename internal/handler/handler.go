package handler

import (
	"net/http"

	"github.com/puzakov/watchdog/internal/service"
)

func NewHandler(store service.Storage) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/update/", func(w http.ResponseWriter, r *http.Request) {
		HandleUpdate(store, w, r)
	})
	return mux
}
