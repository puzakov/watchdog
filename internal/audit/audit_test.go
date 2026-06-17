package audit

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/puzakov/watchdog/internal/logger"
	"go.uber.org/zap"
)

func init() {
	_ = logger.Initialize("debug")
}

type captureObserver struct {
	events []Event
}

func (o *captureObserver) Notify(event Event) {
	o.events = append(o.events, event)
}

func TestSubject_NotifyAllObservers(t *testing.T) {
	var urlEvents []Event

	fileServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		var event Event
		if err := json.Unmarshal(body, &event); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		urlEvents = append(urlEvents, event)
		w.WriteHeader(http.StatusOK)
	}))
	defer fileServer.Close()

	auditFile := filepath.Join(t.TempDir(), "audit.log")
	fileObs := NewFileObserver(auditFile)
	subject := NewSubject(fileObs, NewURLObserver(fileServer.URL))
	subject.Notify([]string{"Alloc", "Frees"}, "192.168.0.42")
	subject.Shutdown()
	_ = fileObs.Close()

	data, err := os.ReadFile(auditFile)
	if err != nil {
		t.Fatalf("read audit file: %v", err)
	}
	var fileEvent Event
	if err := json.Unmarshal(data[:len(data)-1], &fileEvent); err != nil {
		t.Fatalf("unmarshal file event: %v", err)
	}
	if fileEvent.IPAddress != "192.168.0.42" {
		t.Fatalf("file ip = %q, want %q", fileEvent.IPAddress, "192.168.0.42")
	}
	if len(fileEvent.Metrics) != 2 || fileEvent.Metrics[0] != "Alloc" || fileEvent.Metrics[1] != "Frees" {
		t.Fatalf("file metrics = %v", fileEvent.Metrics)
	}
	if fileEvent.TS == 0 {
		t.Fatal("file ts must be set")
	}

	if len(urlEvents) != 1 {
		t.Fatalf("url events = %d, want 1", len(urlEvents))
	}
	if urlEvents[0].IPAddress != "192.168.0.42" {
		t.Fatalf("url ip = %q, want %q", urlEvents[0].IPAddress, "192.168.0.42")
	}
}

func TestSubject_NotifyNilSubject(t *testing.T) {
	var subject *Subject
	subject.Notify([]string{"Alloc"}, "127.0.0.1")
}

func TestCaptureObserver(t *testing.T) {
	capture := &captureObserver{}
	subject := NewSubject(capture)
	subject.Notify([]string{"Alloc"}, "10.0.0.1")
	subject.Shutdown()
	if len(capture.events) != 1 {
		t.Fatalf("events = %d, want 1", len(capture.events))
	}
}

func TestSubject_BufferOverflow(t *testing.T) {
	capture := &captureObserver{}
	subject := NewSubjectWithBuf(2, capture)

	// Fill the buffer
	subject.Notify([]string{"m1"}, "10.0.0.1")
	subject.Notify([]string{"m2"}, "10.0.0.1")
	// This one should be dropped
	subject.Notify([]string{"m3"}, "10.0.0.1")

	subject.Shutdown()

	if len(capture.events) > 2 {
		t.Fatalf("events = %d, want <= 2 (overflow should drop)", len(capture.events))
	}
	for _, e := range capture.events {
		if e.Metrics[0] == "m3" {
			t.Fatal("m3 should have been dropped due to buffer overflow")
		}
	}
}

func TestSubject_ShutdownIsIdempotent(t *testing.T) {
	logger.Log.Debug("test", zap.String("key", "val"))
	subject := NewSubject()
	subject.Shutdown()
	subject.Shutdown() // must not panic
}

func TestSubject_NoObservers(t *testing.T) {
	subject := NewSubject()
	subject.Notify([]string{"m1"}, "10.0.0.1")
	subject.Shutdown()
	// just must not panic
}
