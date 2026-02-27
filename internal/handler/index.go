package handler

import (
	"html"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/puzakov/watchdog/internal/service"
)

func HandleIndex(store service.Storage, w http.ResponseWriter) {
	gauges, counters := store.Snapshot()

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	var b strings.Builder
	b.WriteString("<!doctype html><html><head><meta charset=\"utf-8\"><title>Metrics</title></head><body>")
	b.WriteString("<h1>Metrics</h1>")

	dumpMetricsBlock[float64](&b, "Gauge", gauges, func(data float64) string {
		return strconv.FormatFloat(data, 'g', -1, 64)
	})
	dumpMetricsBlock[int64](&b, "Counter", counters, func(data int64) string {
		return strconv.FormatInt(data, 10)
	})

	b.WriteString("</body></html>")

	_, _ = w.Write([]byte(b.String()))
}

func dumpMetricsBlock[T int64 | float64](b *strings.Builder, blockName string, data map[string]T, formatMethod func(T) string) {
	b.WriteString("<h2>" + blockName + "</h2><ul>")
	cKeys := make([]string, 0, len(data))
	for k := range data {
		cKeys = append(cKeys, k)
	}
	// без сортировки порядок отражения на странице всегда разный, кажется нечитаемым
	sort.Strings(cKeys)
	for _, k := range cKeys {
		b.WriteString("<li><b>")
		b.WriteString(html.EscapeString(k))
		b.WriteString("</b>: ")
		b.WriteString(html.EscapeString(formatMethod(data[k])))
		b.WriteString("</li>")
	}
	b.WriteString("</ul>")

}
