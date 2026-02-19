package handler

import (
	"net/http"
	"strconv"
	"strings"

	models "github.com/puzakov/watchdog/internal/model"
	"github.com/puzakov/watchdog/internal/service"
)

func NewHandler(store service.Storage) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/update/", func(w http.ResponseWriter, r *http.Request) {
		handleUpdate(store, w, r)
	})
	return mux
}

func handleUpdate(store service.Storage, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	parts := strings.Split(r.URL.Path, "/")
	// ["", "update", "<type>", "<name>", "<value>"]
	if len(parts) != 5 || parts[1] != "update" {
		http.NotFound(w, r)
		return
	}

	mType := parts[2]
	name := parts[3]
	value := parts[4]

	if name == "" {
		http.NotFound(w, r)
		return
	}

	ct := r.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "text/plain") {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	switch mType {
	case models.Gauge:
		v, err := strconv.ParseFloat(value, 64)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		store.UpdateGauge(name, v)
		w.WriteHeader(http.StatusOK)
	case models.Counter:
		delta, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		store.UpdateCounter(name, delta)
		w.WriteHeader(http.StatusOK)
	default:
		w.WriteHeader(http.StatusBadRequest)
	}
}
