package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"

	"github.com/puzakov/watchdog/internal/db"
	"github.com/puzakov/watchdog/internal/handler"
	models "github.com/puzakov/watchdog/internal/model"
	"github.com/puzakov/watchdog/internal/service"
)

// Example: update a gauge metric via URL parameters and query its value.
func ExampleHandleUpdate_gauge() {
	store := service.NewMemStorage()
	h := handler.NewHandler(store, &db.DatabaseConnection{}, nil)

	// POST /update/gauge/Alloc/1.5
	req := httptest.NewRequest(http.MethodPost, "/update/gauge/Alloc/1.5", nil)
	req.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	fmt.Printf("update gauge: status=%d\n", w.Code)

	// GET /value/gauge/Alloc
	req = httptest.NewRequest(http.MethodGet, "/value/gauge/Alloc", nil)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	body, _ := io.ReadAll(w.Body)
	fmt.Printf("get gauge: status=%d, value=%s\n", w.Code, body)

	// Output:
	// update gauge: status=200
	// get gauge: status=200, value=1.5
}

// Example: update a counter via URL parameters — the value accumulates.
func ExampleHandleUpdate_counter() {
	store := service.NewMemStorage()
	h := handler.NewHandler(store, &db.DatabaseConnection{}, nil)

	// First update: PollCount = 10
	req := httptest.NewRequest(http.MethodPost, "/update/counter/PollCount/10", nil)
	req.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	fmt.Printf("first update: status=%d\n", w.Code)

	// Second update: PollCount = 10 → accumulates to 20
	req = httptest.NewRequest(http.MethodPost, "/update/counter/PollCount/10", nil)
	req.Header.Set("Content-Type", "text/plain")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	fmt.Printf("second update: status=%d\n", w.Code)

	// Verify: should be 20, not 10
	_, counters := store.Snapshot(context.Background())
	fmt.Printf("counter value: %d\n", counters["PollCount"])

	// Output:
	// first update: status=200
	// second update: status=200
	// counter value: 20
}

// Example: update a gauge metric via JSON and query it.
func ExampleHandleUpdateJSON_gauge() {
	store := service.NewMemStorage()
	h := handler.NewHandler(store, &db.DatabaseConnection{}, nil)

	v := 3.14
	metric := models.Metrics{ID: "CPU", MType: models.Gauge, Value: &v}
	body, _ := json.Marshal(metric)

	req := httptest.NewRequest(http.MethodPost, "/update/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	fmt.Printf("update JSON gauge: status=%d\n", w.Code)

	// Query via JSON
	q := models.Metrics{ID: "CPU", MType: models.Gauge}
	qBody, _ := json.Marshal(q)
	req = httptest.NewRequest(http.MethodPost, "/value/", bytes.NewReader(qBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)

	var resp models.Metrics
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	fmt.Printf("query JSON gauge: status=%d, value=%.2f\n", w.Code, *resp.Value)

	// Output:
	// update JSON gauge: status=200
	// query JSON gauge: status=200, value=3.14
}

// Example: batch update with multiple metrics.
func ExampleHandleUpdatesJSON_batch() {
	store := service.NewMemStorage()
	h := handler.NewHandler(store, &db.DatabaseConnection{}, nil)

	g1 := 1.0
	g2 := 2.0
	d1 := int64(1)
	d2 := int64(2)

	batch := []models.Metrics{
		{ID: "Alloc", MType: models.Gauge, Value: &g1},
		{ID: "Sys", MType: models.Gauge, Value: &g2},
		{ID: "PollCount", MType: models.Counter, Delta: &d1},
		{ID: "PollCount", MType: models.Counter, Delta: &d2}, // accumulates
	}
	body, _ := json.Marshal(batch)

	req := httptest.NewRequest(http.MethodPost, "/updates/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	fmt.Printf("batch update: status=%d\n", w.Code)

	gauges, counters := store.Snapshot(context.Background())
	fmt.Printf("gauges: Alloc=%.0f Sys=%.0f\n", gauges["Alloc"], gauges["Sys"])
	fmt.Printf("counters: PollCount=%d\n", counters["PollCount"])

	// Output:
	// batch update: status=200
	// gauges: Alloc=1 Sys=2
	// counters: PollCount=3
}

// Example: get the index page with all metrics as HTML.
func ExampleHandleIndex() {
	store := service.NewMemStorage()
	_ = store.UpdateGauge(context.Background(), "Alloc", 1.5)
	_ = store.UpdateGauge(context.Background(), "Sys", 256.0)
	_ = store.UpdateCounter(context.Background(), "PollCount", 42)

	h := handler.NewHandler(store, &db.DatabaseConnection{}, nil)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	fmt.Printf("index status: %d\n", w.Code)
	fmt.Printf("content type: %s\n", w.Header().Get("Content-Type"))
	fmt.Printf("body has Alloc: %t\n", bytes.Contains(w.Body.Bytes(), []byte("Alloc")))
	fmt.Printf("body has PollCount: %t\n", bytes.Contains(w.Body.Bytes(), []byte("42")))

	// Output:
	// index status: 200
	// content type: text/html; charset=utf-8
	// body has Alloc: true
	// body has PollCount: true
}
