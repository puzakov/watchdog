package handler

import (
	"net/http"
	"strconv"

	models "github.com/puzakov/watchdog/internal/model"
	"github.com/puzakov/watchdog/internal/service"
)

type ValueArgs struct {
	mType string
	name  string
}

func HandleValue(args *ValueArgs, store service.Storage, w http.ResponseWriter, r *http.Request) {
	var out string

	switch args.mType {
	case models.Gauge:
		v, ok := store.GetGauge(args.name)
		if !ok {
			http.NotFound(w, r)
			return
		}
		out = strconv.FormatFloat(v, 'g', -1, 64)
	case models.Counter:
		v, ok := store.GetCounter(args.name)
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
