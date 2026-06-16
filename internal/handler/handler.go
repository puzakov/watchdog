package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/puzakov/watchdog/internal/audit"
	"github.com/puzakov/watchdog/internal/db"
	"github.com/puzakov/watchdog/internal/service"
)

// NewHandler creates the HTTP handler with the following routes:
//
//	GET  /                      — HTML page with all metrics
//	GET  /ping                  — database health check
//	POST /value                 — get metric value (JSON)
//	GET  /value/{type}/{name}   — get metric value (plain text)
//	POST /update                — update single metric (JSON)
//	POST /update/{type}/{name}/{value} — update single metric (text/plain)
//	POST /updates               — batch update (JSON)
func NewHandler(store service.Storage, conn *db.DatabaseConnection, auditor *audit.Subject) http.Handler {
	router := chi.NewRouter()

	router.Get("/", func(w http.ResponseWriter, r *http.Request) {
		HandleIndex(store, w, r)
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
			HandleUpdateJSON(store, auditor, w, r)
		})
		r.Post("/{type}/{name}/{value}", func(w http.ResponseWriter, r *http.Request) {
			HandleUpdate(store, auditor, w, r)
		})
	})

	router.Route("/updates", func(r chi.Router) {
		r.Post("/", func(w http.ResponseWriter, r *http.Request) {
			HandleUpdatesJSON(store, auditor, w, r)
		})
	})

	return router
}
