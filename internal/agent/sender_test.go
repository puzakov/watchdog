package agent

import (
	"bytes"
	"compress/gzip"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	models "github.com/puzakov/watchdog/internal/model"
	"github.com/puzakov/watchdog/internal/sign"
)

func TestSender_SendGauge_SendsExpectedRequest(t *testing.T) {
	var gotMethod, gotPath, gotCT, gotCE, gotHash string
	var got models.Metrics

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.EscapedPath()
		gotCT = r.Header.Get("Content-Type")
		gotCE = r.Header.Get("Content-Encoding")
		gotHash = r.Header.Get("HashSHA256")
		defer r.Body.Close()

		var body []byte
		if gotCE == "gzip" {
			gzr, err := gzip.NewReader(r.Body)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			body, _ = io.ReadAll(gzr)
			_ = gzr.Close()
		} else {
			body, _ = io.ReadAll(r.Body)
		}
		_ = json.Unmarshal(body, &got)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	s := NewSender(SenderConfig{
		ServerAddress: srv.URL,
		Client:        srv.Client(),
		Key:           "secret",
	})

	if err := s.SendGauge("Alloc", 1.5); err != nil {
		t.Fatalf("SendGauge error: %v", err)
	}

	if gotMethod != http.MethodPost {
		t.Fatalf("method = %q, want %q", gotMethod, http.MethodPost)
	}
	if gotCT != "application/json" {
		t.Fatalf("Content-Type = %q, want %q", gotCT, "application/json")
	}
	if gotCE != "gzip" {
		t.Fatalf("Content-Encoding = %q, want %q", gotCE, "gzip")
	}
	if gotPath != "/update" {
		t.Fatalf("path = %q, want %q", gotPath, "/update")
	}
	if gotHash == "" {
		t.Fatalf("HashSHA256 header is empty, want non-empty")
	}
	if got.ID != "Alloc" || got.MType != models.Gauge || got.Value == nil || *got.Value != 1.5 || got.Delta != nil {
		t.Fatalf("unexpected payload: %+v", got)
	}
}

func TestSender_SendCounter_SendsExpectedRequest(t *testing.T) {
	var gotPath string
	var gotCT string
	var gotCE string
	var got models.Metrics

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.EscapedPath()
		gotCT = r.Header.Get("Content-Type")
		gotCE = r.Header.Get("Content-Encoding")
		defer r.Body.Close()

		var body []byte
		if gotCE == "gzip" {
			gzr, err := gzip.NewReader(r.Body)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			body, _ = io.ReadAll(gzr)
			_ = gzr.Close()
		} else {
			body, _ = io.ReadAll(r.Body)
		}
		_ = json.Unmarshal(body, &got)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	s := NewSender(SenderConfig{
		ServerAddress: srv.URL,
		Client:        srv.Client(),
	})

	if err := s.SendCounter("PollCount", 42); err != nil {
		t.Fatalf("SendCounter error: %v", err)
	}
	if gotCT != "application/json" {
		t.Fatalf("Content-Type = %q, want %q", gotCT, "application/json")
	}
	if gotCE != "gzip" {
		t.Fatalf("Content-Encoding = %q, want %q", gotCE, "gzip")
	}
	if gotPath != "/update" {
		t.Fatalf("path = %q, want %q", gotPath, "/update")
	}
	if got.ID != "PollCount" || got.MType != models.Counter || got.Delta == nil || *got.Delta != 42 || got.Value != nil {
		t.Fatalf("unexpected payload: %+v", got)
	}
}

func TestSender_Non200_IsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	s := NewSender(SenderConfig{
		ServerAddress: srv.URL,
		Client:        srv.Client(),
	})

	if err := s.SendGauge("Alloc", 1); err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestSender_SendBatch_SendsExpectedRequest(t *testing.T) {
	var gotPath, gotCT, gotCE string
	var got []models.Metrics

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.EscapedPath()
		gotCT = r.Header.Get("Content-Type")
		gotCE = r.Header.Get("Content-Encoding")
		defer r.Body.Close()

		var body []byte
		if gotCE == "gzip" {
			gzr, err := gzip.NewReader(r.Body)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			body, _ = io.ReadAll(gzr)
			_ = gzr.Close()
		} else {
			body, _ = io.ReadAll(r.Body)
		}
		_ = json.Unmarshal(body, &got)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	s := NewSender(SenderConfig{
		ServerAddress: srv.URL,
		Client:        srv.Client(),
	})

	v := 1.5
	d := int64(42)
	batch := []models.Metrics{
		{ID: "Alloc", MType: models.Gauge, Value: &v},
		{ID: "PollCount", MType: models.Counter, Delta: &d},
	}

	if err := s.SendBatch(batch); err != nil {
		t.Fatalf("SendBatch error: %v", err)
	}
	if gotCT != "application/json" {
		t.Fatalf("Content-Type = %q, want %q", gotCT, "application/json")
	}
	if gotCE != "gzip" {
		t.Fatalf("Content-Encoding = %q, want %q", gotCE, "gzip")
	}
	if gotPath != "/updates" {
		t.Fatalf("path = %q, want %q", gotPath, "/updates")
	}
	if len(got) != 2 {
		t.Fatalf("payload len = %d, want %d", len(got), 2)
	}
	if got[0].ID != "Alloc" || got[0].MType != models.Gauge || got[0].Value == nil || *got[0].Value != 1.5 || got[0].Delta != nil {
		t.Fatalf("unexpected payload[0]: %+v", got[0])
	}
	if got[1].ID != "PollCount" || got[1].MType != models.Counter || got[1].Delta == nil || *got[1].Delta != 42 || got[1].Value != nil {
		t.Fatalf("unexpected payload[1]: %+v", got[1])
	}
}

func TestSenderPostJSON_WithCryptoKey_EncryptedBody(t *testing.T) {
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}

	var receivedBody []byte
	var receivedCE string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedCE = r.Header.Get("Content-Encoding")
		receivedBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	s := NewSender(SenderConfig{
		ServerAddress: srv.URL,
		Client:        srv.Client(),
		CryptoKey:     &privKey.PublicKey,
	})

	if err := s.SendGauge("Alloc", 42.5); err != nil {
		t.Fatal(err)
	}

	// When crypto is enabled, there is no Content-Encoding: gzip (gzip is inside the encryption envelope).
	if receivedCE != "" {
		t.Errorf("Content-Encoding = %q, want empty when crypto is enabled", receivedCE)
	}

	// The body is RSA-encrypted gzip data. Decrypt and decompress to verify.
	decrypted, err := rsa.DecryptOAEP(sha256.New(), rand.Reader, privKey, receivedBody, nil)
	if err != nil {
		t.Fatal("body is not valid RSA-encrypted:", err)
	}

	gzr, err := gzip.NewReader(bytes.NewReader(decrypted))
	if err != nil {
		t.Fatal("decrypted data is not valid gzip:", err)
	}
	plain, _ := io.ReadAll(gzr)
	gzr.Close()

	var m models.Metrics
	if err := json.Unmarshal(plain, &m); err != nil {
		t.Fatal("decrypted payload is not valid JSON:", err)
	}
	if m.ID != "Alloc" || m.MType != models.Gauge || m.Value == nil || *m.Value != 42.5 {
		t.Fatalf("unexpected metric: %+v", m)
	}
}

func TestSenderSendBatch_WithCryptoKey_EncryptedBody(t *testing.T) {
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}

	var receivedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	s := NewSender(SenderConfig{
		ServerAddress: srv.URL,
		Client:        srv.Client(),
		CryptoKey:     &privKey.PublicKey,
	})

	v := 1.5
	d := int64(10)
	batch := []models.Metrics{
		{ID: "Alloc", MType: models.Gauge, Value: &v},
		{ID: "PollCount", MType: models.Counter, Delta: &d},
	}
	if err := s.SendBatch(batch); err != nil {
		t.Fatal(err)
	}

	// Decrypt and verify the batch content.
	decrypted, err := rsa.DecryptOAEP(sha256.New(), rand.Reader, privKey, receivedBody, nil)
	if err != nil {
		t.Fatal("body is not RSA-encrypted:", err)
	}

	gzr, _ := gzip.NewReader(bytes.NewReader(decrypted))
	plain, _ := io.ReadAll(gzr)
	gzr.Close()

	var got []models.Metrics
	if err := json.Unmarshal(plain, &got); err != nil {
		t.Fatal("invalid JSON:", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d metrics, want 2", len(got))
	}
}

func TestSenderPostJSON_WithoutCryptoKey_GzipHeaderPreserved(t *testing.T) {
	var receivedCE string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedCE = r.Header.Get("Content-Encoding")
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	s := NewSender(SenderConfig{
		ServerAddress: srv.URL,
		Client:        srv.Client(),
	})

	if err := s.SendGauge("Test", 1.0); err != nil {
		t.Fatal(err)
	}

	if receivedCE != "gzip" {
		t.Errorf("Content-Encoding = %q, want gzip", receivedCE)
	}
}

func TestSenderPostJSON_CryptoAndHashTogether(t *testing.T) {
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}

	var receivedBody []byte
	var receivedHash, receivedCE string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHash = r.Header.Get("HashSHA256")
		receivedCE = r.Header.Get("Content-Encoding")
		receivedBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	s := NewSender(SenderConfig{
		ServerAddress: srv.URL,
		Client:        srv.Client(),
		CryptoKey:     &privKey.PublicKey,
		Key:           "my-secret",
	})

	if err := s.SendGauge("CPU", 0.99); err != nil {
		t.Fatal(err)
	}

	// Hash header must be present.
	if receivedHash == "" {
		t.Error("HashSHA256 header is empty when Key is set alongside CryptoKey")
	}

	// No Content-Encoding when crypto is enabled.
	if receivedCE != "" {
		t.Errorf("Content-Encoding = %q, want empty", receivedCE)
	}

	// Verify the hash is computed over the raw JSON, not the encrypted body.
	decrypted, err := rsa.DecryptOAEP(sha256.New(), rand.Reader, privKey, receivedBody, nil)
	if err != nil {
		t.Fatal("decryption failed:", err)
	}

	gzr, _ := gzip.NewReader(bytes.NewReader(decrypted))
	rawJSON, _ := io.ReadAll(gzr)
	gzr.Close()

	// Compute expected hash.
	expectedHash := sign.SumSHA256(rawJSON, "my-secret")
	if receivedHash != expectedHash {
		t.Errorf("HashSHA256 mismatch:\ngot:  %s\nwant: %s", receivedHash, expectedHash)
	}
}

func TestSenderConfig_CryptoKeyPassedThrough(t *testing.T) {
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}

	a := New(Config{
		ServerAddress:  "http://localhost:9999",
		PollInterval:   3600,
		ReportInterval: 3600,
		Timeout:        5,
		CryptoKey:      &privKey.PublicKey,
	})

	if a.sender.cfg.CryptoKey != &privKey.PublicKey {
		t.Error("CryptoKey not passed through from Config to Sender")
	}
}

func TestSenderPostJSON_CryptoKey_ContentTypePreserved(t *testing.T) {
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}

	var contentType string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		contentType = r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	s := NewSender(SenderConfig{
		ServerAddress: srv.URL,
		Client:        srv.Client(),
		CryptoKey:     &privKey.PublicKey,
	})

	if err := s.SendGauge("M", 1); err != nil {
		t.Fatal(err)
	}

	if contentType != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", contentType)
	}
}
