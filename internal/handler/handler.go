package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/puzakov/watchdog/internal/db"
	"github.com/puzakov/watchdog/internal/service"
)

func NewHandler(store service.Storage, conn db.DatabaseConnection) http.Handler {
	router := chi.NewRouter()

	router.Get("/", func(w http.ResponseWriter, r *http.Request) {
		HandleIndex(store, w)
	})

	router.Get("/ping", func(w http.ResponseWriter, r *http.Request) {
		HandlePing(conn, w)
	})

	router.Route("/value", func(r chi.Router) {
		r.Post("/", func(w http.ResponseWriter, r *http.Request) {
			HandleValueJSON(store, w, r)
		})
		r.Get("/{type}/{name}", func(w http.ResponseWriter, r *http.Request) {
			HandleValue(store, w, r)
		})
	})

	router.Route("/update", func(r chi.Router) {
		r.Post("/", func(w http.ResponseWriter, r *http.Request) {
			HandleUpdateJSON(store, w, r)
		})
		r.Post("/{type}/{name}/{value}", func(w http.ResponseWriter, r *http.Request) {
			HandleUpdate(store, w, r)
		})
	})

	return router
}
