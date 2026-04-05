package handler

import (
	"encoding/json"
	"errors"
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

var errBadRequest = errors.New("bad request")

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
		writeUpdateError(w, err)
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
		writeUpdateError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	// Resty в автотестах может переиспользовать Request с SetResult,
	// поэтому ответ должен быть валидным JSON, даже если тест его не проверяет.
	enc := json.NewEncoder(w)
	if err := enc.Encode(&args); err != nil {
		logger.Log.Debug("error encoding response", zap.Error(err))
	}
}

func HandleUpdatesJSON(store service.Storage, w http.ResponseWriter, r *http.Request) {
	ct := r.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "application/json") {
		logger.Log.Debug("Invalid content type", zap.String("Content-Type", ct))
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	var args []models.Metrics
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&args); err != nil {
		logger.Log.Debug("cannot decode request JSON body", zap.Error(err))
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	batch, err := normalizeBatch(args)
	if err != nil {
		logger.Log.Debug(err.Error(), zap.Error(err))
		writeUpdateError(w, err)
		return
	}

	if err := store.UpdateBatch(batch); err != nil {
		logger.Log.Debug("batch update error", zap.Error(err))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	enc := json.NewEncoder(w)
	if err := enc.Encode(&args); err != nil {
		logger.Log.Debug("error encoding response", zap.Error(err))
	}
}

func updateInternal(args *models.Metrics, store service.Storage) error {
	switch args.MType {
	case models.Gauge:
		if args.Value == nil {
			return fmt.Errorf("%w: missing gauge value", errBadRequest)
		}
		if err := store.UpdateGauge(args.ID, *args.Value); err != nil {
			return fmt.Errorf("update gauge: %w", err)
		}
	case models.Counter:
		if args.Delta == nil {
			return fmt.Errorf("%w: missing counter delta", errBadRequest)
		}
		if err := store.UpdateCounter(args.ID, *args.Delta); err != nil {
			return fmt.Errorf("update counter: %w", err)
		}
	default:
		return fmt.Errorf("%w: unsupported metrics type: %s", errBadRequest, args.MType)
	}

	return nil
}

func normalizeBatch(in []models.Metrics) ([]models.Metrics, error) {
	if len(in) == 0 {
		return nil, nil
	}

	type key struct {
		id    string
		mtype string
	}

	// preserve order of first appearance
	order := make([]key, 0, len(in))
	gauges := make(map[key]float64, len(in))
	counters := make(map[key]int64, len(in))

	seen := make(map[key]struct{}, len(in))
	for _, m := range in {
		if m.ID == "" || m.MType == "" {
			return nil, fmt.Errorf("%w: invalid metric: empty id/type", errBadRequest)
		}

		k := key{id: m.ID, mtype: m.MType}
		if _, ok := seen[k]; !ok {
			seen[k] = struct{}{}
			order = append(order, k)
		}

		switch m.MType {
		case models.Gauge:
			if m.Value == nil {
				return nil, fmt.Errorf("%w: invalid gauge metric %q: missing value", errBadRequest, m.ID)
			}
			gauges[k] = *m.Value // last wins
		case models.Counter:
			if m.Delta == nil {
				return nil, fmt.Errorf("%w: invalid counter metric %q: missing delta", errBadRequest, m.ID)
			}
			counters[k] += *m.Delta
		default:
			return nil, fmt.Errorf("%w: unsupported metrics type: %s", errBadRequest, m.MType)
		}
	}

	out := make([]models.Metrics, 0, len(order))
	for _, k := range order {
		switch k.mtype {
		case models.Gauge:
			v := gauges[k]
			vv := v
			out = append(out, models.Metrics{ID: k.id, MType: models.Gauge, Value: &vv})
		case models.Counter:
			d := counters[k]
			dd := d
			out = append(out, models.Metrics{ID: k.id, MType: models.Counter, Delta: &dd})
		}
	}

	return out, nil
}

func writeUpdateError(w http.ResponseWriter, err error) {
	if errors.Is(err, errBadRequest) {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusInternalServerError)
}
