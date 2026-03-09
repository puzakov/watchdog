package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/puzakov/watchdog/internal/logger"
	models "github.com/puzakov/watchdog/internal/model"
	"github.com/puzakov/watchdog/internal/service"
	"go.uber.org/zap"
)

func HandleUpdate(store service.Storage, w http.ResponseWriter, r *http.Request) {
	args := models.Metrics{
		MType: chi.URLParam(r, "type"),
		ID:    chi.URLParam(r, "name"),
	}
	value := chi.URLParam(r, "value")

	ct := r.Header.Get("Content-Type")
	if ct != "" && !strings.HasPrefix(ct, "text/plain") {
		logger.Log.Debug("Invalid content type", zap.String("Content-Type", ct))
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	switch args.MType {
	case models.Gauge:
		v, err := strconv.ParseFloat(value, 64)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		args.Value = &v
	case models.Counter:
		delta, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		args.Delta = &delta
	}

	if err := updateInternal(&args, store); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func HandleUpdateJSON(store service.Storage, w http.ResponseWriter, r *http.Request) {
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

	if err := updateInternal(&args, store); err != nil {
		logger.Log.Debug(err.Error(), zap.Error(err))
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
}

func updateInternal(args *models.Metrics, store service.Storage) error {
	switch args.MType {
	case models.Gauge:
		store.UpdateGauge(args.ID, *args.Value)
	case models.Counter:
		store.UpdateCounter(args.ID, *args.Delta)
	default:
		return fmt.Errorf("unsupported metrics type: %s", args.MType)
	}

	return nil
}
