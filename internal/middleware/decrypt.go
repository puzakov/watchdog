package middleware

import (
	"bytes"
	"compress/gzip"
	"crypto/rsa"
	"io"
	"net/http"

	"github.com/puzakov/watchdog/internal/crypto"
)

// DecryptRSA is HTTP middleware that decrypts incoming request bodies
// using the server's RSA private key.
//
// The agent encrypts its gzip-compressed JSON payload with the server's public key.
// This middleware reverses the operation: it reads the encrypted body, decrypts it
// with RSA-OAEP (SHA-256), decompresses the gzip layer, and replaces r.Body with
// the plain JSON so that downstream middleware (Gzip, HashSHA256) work as usual.
//
// When privateKey is nil, the middleware is a no-op and passes through immediately.
func DecryptRSA(privateKey *rsa.PrivateKey, next http.Handler) http.Handler {
	if privateKey == nil {
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body == nil {
			next.ServeHTTP(w, r)
			return
		}

		encrypted, err := io.ReadAll(r.Body)
		r.Body.Close()
		if err != nil {
			http.Error(w, "failed to read request body", http.StatusBadRequest)
			return
		}

		if len(encrypted) == 0 {
			next.ServeHTTP(w, r)
			return
		}

		// Decrypt the RSA-OAEP envelope.
		decrypted, err := crypto.DecryptOAEP(encrypted, privateKey)
		if err != nil {
			http.Error(w, "request body decryption failed", http.StatusBadRequest)
			return
		}

		// The decrypted payload is gzip-compressed JSON. Decompress it.
		gzr, err := gzip.NewReader(bytes.NewReader(decrypted))
		if err != nil {
			http.Error(w, "decompression of decrypted body failed", http.StatusBadRequest)
			return
		}
		defer gzr.Close()

		plain, err := io.ReadAll(gzr)
		if err != nil {
			http.Error(w, "failed to read decompressed body", http.StatusBadRequest)
			return
		}

		r.Body = io.NopCloser(bytes.NewReader(plain))
		next.ServeHTTP(w, r)
	})
}
