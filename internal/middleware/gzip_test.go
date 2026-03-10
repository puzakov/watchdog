package middleware

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func gzipBytes(t *testing.T, b []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	_, err := zw.Write(b)
	if err != nil {
		t.Fatalf("gzip write: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}
	return buf.Bytes()
}

func gunzipBytes(t *testing.T, b []byte) []byte {
	t.Helper()
	zr, err := gzip.NewReader(bytes.NewReader(b))
	if err != nil {
		t.Fatalf("gzip reader: %v", err)
	}
	defer zr.Close()
	out, err := io.ReadAll(zr)
	if err != nil {
		t.Fatalf("gzip read: %v", err)
	}
	return out
}

func TestGzip_CompressesJSONAndSetsContentEncodingEvenWithWriteHeader(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/any", nil)
	req.Header.Set("Accept-Encoding", "gzip")

	Gzip(h).ServeHTTP(rr, req)

	if got := rr.Header().Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("Content-Encoding = %q, want %q", got, "gzip")
	}
	if got := rr.Header().Get("Vary"); got == "" {
		t.Fatalf("expected Vary header to be set")
	}
	raw := rr.Body.Bytes()
	if len(raw) < 2 || raw[0] != 0x1f || raw[1] != 0x8b {
		t.Fatalf("response does not look gzipped: first bytes=%v", raw[:min(len(raw), 4)])
	}
	if got := string(gunzipBytes(t, raw)); got != `{"ok":true}` {
		t.Fatalf("unzipped body = %q, want %q", got, `{"ok":true}`)
	}
}

func TestGzip_SkipsWhenClientDoesNotAcceptGzip(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/any", nil)

	Gzip(h).ServeHTTP(rr, req)

	if got := rr.Header().Get("Content-Encoding"); got != "" {
		t.Fatalf("Content-Encoding = %q, want empty", got)
	}
	if got := rr.Body.String(); got != `{"ok":true}` {
		t.Fatalf("body = %q, want %q", got, `{"ok":true}`)
	}
}

func TestGzip_SkipsWhenContentTypeNotCompressible(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("hello"))
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/any", nil)
	req.Header.Set("Accept-Encoding", "gzip")

	Gzip(h).ServeHTTP(rr, req)

	if got := rr.Header().Get("Content-Encoding"); got != "" {
		t.Fatalf("Content-Encoding = %q, want empty", got)
	}
	if got := rr.Body.String(); got != "hello" {
		t.Fatalf("body = %q, want %q", got, "hello")
	}
}

func TestGzip_DecompressesGzippedJSONRequestBody(t *testing.T) {
	var (
		seenContentEncoding string
		seenBody            []byte
	)

	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenContentEncoding = r.Header.Get("Content-Encoding")
		b, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body in handler: %v", err)
		}
		seenBody = b

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(b)
	})

	plain := []byte(`{"id":"x","type":"counter","delta":1}`)
	reqBody := gzipBytes(t, plain)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/value/", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "gzip")

	Gzip(h).ServeHTTP(rr, req)

	if seenContentEncoding != "" {
		t.Fatalf("handler saw Content-Encoding = %q, want empty", seenContentEncoding)
	}
	if string(seenBody) != string(plain) {
		t.Fatalf("handler saw body = %q, want %q", string(seenBody), string(plain))
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	if got := rr.Body.String(); got != string(plain) {
		t.Fatalf("response body = %q, want %q", got, string(plain))
	}
}

func TestGzip_InvalidGzipBodyReturnsBadRequest(t *testing.T) {
	nextCalled := false
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/value/", bytes.NewReader([]byte("not-a-gzip-stream")))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "gzip")

	Gzip(h).ServeHTTP(rr, req)

	if nextCalled {
		t.Fatalf("next handler was called, expected early return on bad gzip")
	}
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
