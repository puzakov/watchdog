package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/puzakov/watchdog/internal/logger"
	models "github.com/puzakov/watchdog/internal/model"
	"github.com/puzakov/watchdog/internal/service"
	"go.uber.org/zap"
)

type ValueArgs struct {
	mType string
	name  string
}

// HandleValue returns a metric value as plain text from URL parameters.
// Expected URL: /value/{type}/{name}. Returns 404 if the metric does not exist.
func HandleValue(store service.Storage, w http.ResponseWriter, r *http.Request) {
	var out string

	args := &ValueArgs{
		mType: chi.URLParam(r, "type"),
		name:  chi.URLParam(r, "name"),
	}

	switch args.mType {
	case models.Gauge:
		v, ok := store.GetGauge(r.Context(), args.name)
		if !ok {
			http.NotFound(w, r)
			return
		}
		out = strconv.FormatFloat(v, 'g', -1, 64)
	case models.Counter:
		v, ok := store.GetCounter(r.Context(), args.name)
		if !ok {
			http.NotFound(w, r)
			return
		}
		out = strconv.FormatInt(v, 10)
	default:
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(out))
}

// HandleValueJSON returns a metric value as JSON.
// Expected Content-Type: application/json. Returns 404 if the metric does not exist.
func HandleValueJSON(store service.Storage, w http.ResponseWriter, r *http.Request) {
	ct := r.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "application/json") {
		logger.Log.Debug("Invalid content type", zap.String("Content-Type", ct))
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	args := models.Metrics{}
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&args); err != nil {
		logger.Log.Debug("cannot decode request JSON body", zap.Error(err))
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	switch args.MType {
	case models.Gauge:
		v, ok := store.GetGauge(r.Context(), args.ID)
		if !ok {
			http.NotFound(w, r)
			return
		}
		args.Value = &v
	case models.Counter:
		v, ok := store.GetCounter(r.Context(), args.ID)
		if !ok {
			http.NotFound(w, r)
			return
		}
		args.Delta = &v
	default:
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	enc := json.NewEncoder(w)
	if err := enc.Encode(&args); err != nil {
		logger.Log.Debug("error encoding response", zap.Error(err))
	}
}
