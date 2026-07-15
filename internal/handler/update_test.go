package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/puzakov/watchdog/internal/db"
	models "github.com/puzakov/watchdog/internal/model"
	"github.com/puzakov/watchdog/internal/service"
)

func TestHandleUpdate_BadContentType(t *testing.T) {
	store := service.NewMemStorage()
	h := NewHandler(store, &db.DatabaseConnection{}, nil)
	req := httptest.NewRequest(http.MethodPost, "/update/gauge/Alloc/1", nil)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandleUpdate_EmptyContentType_IsOK(t *testing.T) {
	store := service.NewMemStorage()
	h := NewHandler(store, &db.DatabaseConnection{}, nil)
	req := httptest.NewRequest(http.MethodPost, "/update/gauge/Alloc/1.5", nil)
	// Content-Type empty
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestHandleUpdate_UnknownType_BadRequest(t *testing.T) {
	store := service.NewMemStorage()
	h := NewHandler(store, &db.DatabaseConnection{}, nil)
	req := httptest.NewRequest(http.MethodPost, "/update/unknown/Alloc/1", nil)
	req.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandleUpdate_Gauge_OK(t *testing.T) {
	store := service.NewMemStorage()
	h := NewHandler(store, &db.DatabaseConnection{}, nil)
	req := httptest.NewRequest(http.MethodPost, "/update/gauge/Alloc/1.5", nil)
	req.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	g, _ := store.Snapshot(context.Background())
	if got := g["Alloc"]; got != 1.5 {
		t.Fatalf("Alloc = %v, want %v", got, 1.5)
	}
}

func TestHandleUpdate_Gauge_InvalidValue_BadRequest(t *testing.T) {
	store := service.NewMemStorage()
	h := NewHandler(store, &db.DatabaseConnection{}, nil)
	req := httptest.NewRequest(http.MethodPost, "/update/gauge/Alloc/nope", nil)
	req.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandleUpdate_Counter_OK(t *testing.T) {
	store := service.NewMemStorage()
	h := NewHandler(store, &db.DatabaseConnection{}, nil)
	req := httptest.NewRequest(http.MethodPost, "/update/counter/PollCount/10", nil)
	req.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	_, c := store.Snapshot(context.Background())
	if got := c["PollCount"]; got != 10 {
		t.Fatalf("PollCount = %v, want %v", got, 10)
	}
}

func TestHandleUpdate_Counter_InvalidValue_BadRequest(t *testing.T) {
	store := service.NewMemStorage()
	h := NewHandler(store, &db.DatabaseConnection{}, nil)
	req := httptest.NewRequest(http.MethodPost, "/update/counter/PollCount/nope", nil)
	req.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestUpdateJSON_ReturnsValidJSONBody(t *testing.T) {
	store := service.NewMemStorage()
	h := NewHandler(store, &db.DatabaseConnection{}, nil)

	d := int64(3)
	reqBody, _ := json.Marshal(&models.Metrics{ID: "c1", MType: models.Counter, Delta: &d})
	req := httptest.NewRequest(http.MethodPost, "/update/", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if ct := w.Header().Get("Content-Type"); ct == "" || ct[:16] != "application/json" {
		t.Fatalf("Content-Type = %q, want prefix %q", ct, "application/json")
	}

	var resp models.Metrics
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v, body=%q", err, w.Body.String())
	}
	if resp.ID != "c1" || resp.MType != models.Counter || resp.Delta == nil || *resp.Delta != 3 {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestHandleUpdateJSON_BadContentType(t *testing.T) {
	store := service.NewMemStorage()
	h := NewHandler(store, &db.DatabaseConnection{}, nil)

	req := httptest.NewRequest(http.MethodPost, "/update/", nil)
	req.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandleUpdateJSON_InvalidJSON(t *testing.T) {
	store := service.NewMemStorage()
	h := NewHandler(store, &db.DatabaseConnection{}, nil)

	req := httptest.NewRequest(http.MethodPost, "/update/", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandleUpdateJSON_MissingGaugeValue(t *testing.T) {
	store := service.NewMemStorage()
	h := NewHandler(store, &db.DatabaseConnection{}, nil)

	body, _ := json.Marshal(&models.Metrics{ID: "test", MType: models.Gauge, Value: nil})
	req := httptest.NewRequest(http.MethodPost, "/update/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandleUpdateJSON_MissingCounterDelta(t *testing.T) {
	store := service.NewMemStorage()
	h := NewHandler(store, &db.DatabaseConnection{}, nil)

	body, _ := json.Marshal(&models.Metrics{ID: "test", MType: models.Counter, Delta: nil})
	req := httptest.NewRequest(http.MethodPost, "/update/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandleUpdateJSON_UnknownType(t *testing.T) {
	store := service.NewMemStorage()
	h := NewHandler(store, &db.DatabaseConnection{}, nil)

	body, _ := json.Marshal(&models.Metrics{ID: "test", MType: "unknown"})
	req := httptest.NewRequest(http.MethodPost, "/update/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandleUpdateJSON_Gauge_OK(t *testing.T) {
	store := service.NewMemStorage()
	h := NewHandler(store, &db.DatabaseConnection{}, nil)

	v := 2.5
	body, _ := json.Marshal(&models.Metrics{ID: "Alloc", MType: models.Gauge, Value: &v})
	req := httptest.NewRequest(http.MethodPost, "/update/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	g, _ := store.Snapshot(context.Background())
	if got := g["Alloc"]; got != 2.5 {
		t.Fatalf("Alloc = %v, want %v", got, 2.5)
	}
}

func TestHandleUpdatesJSON_InvalidJSON(t *testing.T) {
	store := service.NewMemStorage()
	h := NewHandler(store, &db.DatabaseConnection{}, nil)

	req := httptest.NewRequest(http.MethodPost, "/updates/", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandleUpdatesJSON_EmptyID(t *testing.T) {
	store := service.NewMemStorage()
	h := NewHandler(store, &db.DatabaseConnection{}, nil)

	v := 1.0
	body, _ := json.Marshal([]models.Metrics{
		{ID: "", MType: models.Gauge, Value: &v},
	})
	req := httptest.NewRequest(http.MethodPost, "/updates/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestUpdateInternal_UnknownType(t *testing.T) {
	store := service.NewMemStorage()
	ctx := context.Background()
	args := &models.Metrics{ID: "test", MType: "unknown"}
	err := updateInternal(ctx, args, store)
	if err == nil {
		t.Fatal("expected error for unknown type")
	}
}

func TestMetricIDs_Empty(t *testing.T) {
	ids := metricIDs(nil)
	if ids != nil {
		t.Fatalf("metricIDs(nil) = %v, want nil", ids)
	}
	ids = metricIDs([]models.Metrics{})
	if ids != nil {
		t.Fatalf("metricIDs(empty) = %v, want nil", ids)
	}
}

func TestWriteUpdateError(t *testing.T) {
	w := httptest.NewRecorder()
	writeUpdateError(w, errBadRequest)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}

	w2 := httptest.NewRecorder()
	writeUpdateError(w2, errors.New("some other error"))
	if w2.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", w2.Code, http.StatusInternalServerError)
	}
}
