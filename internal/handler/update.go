package handler

import (
	"net/http"
	"strconv"

	models "github.com/puzakov/watchdog/internal/model"
	"github.com/puzakov/watchdog/internal/service"
)

type UpdateArgs struct {
	mType string
	name  string
	value string
}

func HandleUpdate(args *UpdateArgs, store service.Storage, w http.ResponseWriter, r *http.Request) {

	//ct := r.Header.Get("Content-Type")
	//if !strings.HasPrefix(ct, "text/plain") {
	//	w.WriteHeader(http.StatusBadRequest)
	//	return
	//}

	switch args.mType {
	case models.Gauge:
		v, err := strconv.ParseFloat(args.value, 64)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		store.UpdateGauge(args.name, v)
		w.WriteHeader(http.StatusOK)
	case models.Counter:
		delta, err := strconv.ParseInt(args.value, 10, 64)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		store.UpdateCounter(args.name, delta)
		w.WriteHeader(http.StatusOK)
	default:
		w.WriteHeader(http.StatusBadRequest)
	}
}
