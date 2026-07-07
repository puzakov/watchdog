package middleware

import (
	"bytes"
	"compress/gzip"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// ensure DecryptRSA satisfies the http.Handler signature.
var _ = DecryptRSA(nil, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

// generateTestPrivateKey creates an RSA private key for testing.
func generateTestPrivateKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

// encryptGzipPayload encrypts and then gzip-compresses the payload
// simulating the agent's encryption order: JSON -> gzip -> encrypt.
func encryptGzipPayload(t *testing.T, pub *rsa.PublicKey, payload []byte) []byte {
	t.Helper()

	// Compress.
	var buf bytes.Buffer
	gzw := gzip.NewWriter(&buf)
	if _, err := gzw.Write(payload); err != nil {
		t.Fatal(err)
	}
	if err := gzw.Close(); err != nil {
		t.Fatal(err)
	}

	// Encrypt.
	ciphertext, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, pub, buf.Bytes(), nil)
	if err != nil {
		t.Fatal(err)
	}
	return ciphertext
}

func TestDecryptRSA_ValidRequest(t *testing.T) {
	priv := generateTestPrivateKey(t)

	// Handler that echoes the received body.
	var receivedBody []byte
	handler := DecryptRSA(priv, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		receivedBody = body
		w.WriteHeader(http.StatusOK)
	}))

	original := []byte(`{"id":"test","type":"gauge","value":42.5}`)
	encrypted := encryptGzipPayload(t, &priv.PublicKey, original)

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(encrypted))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !bytes.Equal(receivedBody, original) {
		t.Fatalf("body mismatch:\ngot:  %s\nwant: %s", receivedBody, original)
	}
}

func TestDecryptRSA_NilKey_Passthrough(t *testing.T) {
	// When privateKey is nil, the middleware should be a no-op.
	var receivedBody []byte
	handler := DecryptRSA(nil, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		receivedBody = body
		w.WriteHeader(http.StatusOK)
	}))

	body := []byte(`plain text body`)
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !bytes.Equal(receivedBody, body) {
		t.Fatalf("body mismatch:\ngot:  %s\nwant: %s", receivedBody, body)
	}
}

func TestDecryptRSA_EmptyBody(t *testing.T) {
	priv := generateTestPrivateKey(t)

	handler := DecryptRSA(priv, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestDecryptRSA_NilRequestBody(t *testing.T) {
	priv := generateTestPrivateKey(t)
	called := false

	handler := DecryptRSA(priv, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	// Create a request where Body is nil.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if !called {
		t.Fatal("handler was not called")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestDecryptRSA_InvalidEncryptedData(t *testing.T) {
	priv := generateTestPrivateKey(t)

	handler := DecryptRSA(priv, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called on invalid data")
	}))

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte("not encrypted")))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestDecryptRSA_WrongKeyDecryption(t *testing.T) {
	// Encrypt with one key, decrypt with another.
	priv1 := generateTestPrivateKey(t)
	priv2 := generateTestPrivateKey(t)

	handler := DecryptRSA(priv2, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called on mismatched key")
	}))

	payload := []byte(`{"id":"test"}`)
	encrypted := encryptGzipPayload(t, &priv1.PublicKey, payload)

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(encrypted))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestDecryptRSA_NonGzipAfterDecrypt(t *testing.T) {
	// Test what happens when encrypted data is valid RSA but not gzip.
	priv := generateTestPrivateKey(t)

	// Encrypt plain non-gzip data.
	ciphertext, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, &priv.PublicKey, []byte("not gzip"), nil)
	if err != nil {
		t.Fatal(err)
	}

	handler := DecryptRSA(priv, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(ciphertext))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for non-gzip data, got %d", rec.Code)
	}
}

func TestDecryptRSA_MultipleRequests(t *testing.T) {
	priv := generateTestPrivateKey(t)
	callCount := 0

	handler := DecryptRSA(priv, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
	}))

	for i := range 5 {
		payload := []byte(`{"id":"test","n":` + string(rune('0'+i)) + `}`)
		encrypted := encryptGzipPayload(t, &priv.PublicKey, payload)

		req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(encrypted))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("iteration %d: expected 200, got %d", i, rec.Code)
		}
	}

	if callCount != 5 {
		t.Fatalf("expected 5 handler calls, got %d", callCount)
	}
}

func TestDecryptRSA_WithGzipMiddleware(t *testing.T) {
	// Integration test: chain Gzip before DecryptRSA.
	// The agent sends gzip(encrypt(JSON)) — but wait, our actual architecture
	// is: JSON -> gzip -> encrypt -> wire, and DecryptRSA handles both
	// decryption and decompression. Gzip middleware passes through because
	// there's no Content-Encoding: gzip header.
	priv := generateTestPrivateKey(t)

	var receivedBody []byte
	h := DecryptRSA(priv, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		receivedBody = body
		w.WriteHeader(http.StatusOK)
	}))
	h = Gzip(h)

	original := []byte(`{"id":"test","type":"gauge","value":3.14}`)
	encrypted := encryptGzipPayload(t, &priv.PublicKey, original)

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(encrypted))
	req.Header.Set("Content-Type", "application/json")
	// No Content-Encoding: gzip — the gzip layer is inside the encryption envelope.
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !bytes.Equal(receivedBody, original) {
		t.Fatalf("body mismatch:\ngot:  %s\nwant: %s", receivedBody, original)
	}
}

func TestDecryptRSA_WithFullMiddlewareChain(t *testing.T) {
	// Integration test with the full chain: LogRequest -> DecryptRSA -> Gzip -> HashSHA256 -> Handler
	priv := generateTestPrivateKey(t)

	var receivedBody []byte
	var h http.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		receivedBody = body
		w.WriteHeader(http.StatusOK)
	})
	h = HashSHA256("test-key", h)
	h = Gzip(h)
	h = DecryptRSA(priv, h)
	h = LogRequest(h)

	original := []byte(`{"id":"test","type":"counter","delta":10}`)
	encrypted := encryptGzipPayload(t, &priv.PublicKey, original)

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(encrypted))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !bytes.Equal(receivedBody, original) {
		t.Fatalf("body mismatch:\ngot:  %s\nwant: %s", receivedBody, original)
	}
}
