package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/puzakov/watchdog/internal/service"
)

func NewHandler(store service.Storage) http.Handler {
	router := chi.NewRouter()

	router.Get("/", func(w http.ResponseWriter, r *http.Request) {
		HandleIndex(store, w)
	})

	router.Get("/value/{type}/{name}", func(w http.ResponseWriter, r *http.Request) {
		HandleValue(&ValueArgs{
			mType: chi.URLParam(r, "type"),
			name:  chi.URLParam(r, "name"),
		}, store, w, r)
	})

	router.Post("/update/{type}/{name}/{value}", func(w http.ResponseWriter, r *http.Request) {
		HandleUpdate(&UpdateArgs{
			mType: chi.URLParam(r, "type"),
			name:  chi.URLParam(r, "name"),
			value: chi.URLParam(r, "value"),
		}, store, w, r)

	})

	return router
}
