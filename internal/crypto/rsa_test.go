package crypto

import (
	"bytes"
	"compress/gzip"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// generateTestKeyPair creates a temporary RSA key pair for testing.
// Returns paths to public and private key PEM files, plus the parsed keys.
// generateTestKeyPair creates a temporary RSA key pair for testing.
// Returns: publicKeyPath, privateKeyPath, publicKey, privateKey.
func generateTestKeyPair(t *testing.T) (string, string, *rsa.PublicKey, *rsa.PrivateKey) {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	pub := &key.PublicKey

	dir := t.TempDir()

	// Write private key in PKCS8 format.
	privBytes, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	privPath := filepath.Join(dir, "private.pem")
	if err := os.WriteFile(privPath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privBytes}), 0600); err != nil {
		t.Fatal(err)
	}

	// Write public key in PKCS1 format (traditional RSA public key).
	pubBytes := x509.MarshalPKCS1PublicKey(pub)
	pubPath := filepath.Join(dir, "public.pem")
	if err := os.WriteFile(pubPath, pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubBytes}), 0600); err != nil {
		t.Fatal(err)
	}

	return pubPath, privPath, pub, key
}

// gzipData compresses data for use in encrypt/decrypt tests.
func gzipData(t *testing.T, data []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gzw := gzip.NewWriter(&buf)
	if _, err := gzw.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := gzw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// TestLoadPublicKey_Success verifies loading a valid PEM public key.
func TestLoadPublicKey_Success(t *testing.T) {
	pubPath, _, _, _ := generateTestKeyPair(t)

	key, err := LoadPublicKey(pubPath)
	if err != nil {
		t.Fatal(err)
	}
	if key == nil {
		t.Fatal("expected non-nil key")
	}
}

// TestLoadPublicKey_FileNotFound verifies error on missing file.
func TestLoadPublicKey_FileNotFound(t *testing.T) {
	_, err := LoadPublicKey("/nonexistent/key.pem")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

// TestLoadPublicKey_InvalidPEM verifies error on non-PEM content.
func TestLoadPublicKey_InvalidPEM(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.pem")
	if err := os.WriteFile(path, []byte("not pem data"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadPublicKey(path)
	if err == nil {
		t.Fatal("expected error for invalid PEM")
	}
}

// TestLoadPublicKey_WrongKeyType verifies error when loading a private key as public.
func TestLoadPublicKey_WrongKeyType(t *testing.T) {
	_, privPath, _, _ := generateTestKeyPair(t)

	_, err := LoadPublicKey(privPath)
	if err == nil {
		t.Fatal("expected error when loading private key as public")
	}
}

// TestLoadPrivateKey_Success verifies loading a valid PEM private key.
func TestLoadPrivateKey_Success(t *testing.T) {
	_, privPath, _, _ := generateTestKeyPair(t)

	key, err := LoadPrivateKey(privPath)
	if err != nil {
		t.Fatal(err)
	}
	if key == nil {
		t.Fatal("expected non-nil key")
	}
}

// TestLoadPrivateKey_FileNotFound verifies error on missing file.
func TestLoadPrivateKey_FileNotFound(t *testing.T) {
	_, err := LoadPrivateKey("/nonexistent/key.pem")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

// TestLoadPrivateKey_InvalidPEM verifies error on non-PEM content.
func TestLoadPrivateKey_InvalidPEM(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.pem")
	if err := os.WriteFile(path, []byte("not pem"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadPrivateKey(path)
	if err == nil {
		t.Fatal("expected error for invalid PEM")
	}
}

// TestLoadPrivateKey_WrongKeyType verifies error when loading a public key as private.
func TestLoadPrivateKey_WrongKeyType(t *testing.T) {
	pubPath, _, _, _ := generateTestKeyPair(t)

	_, err := LoadPrivateKey(pubPath)
	if err == nil {
		t.Fatal("expected error when loading public key as private")
	}
}

// TestLoadPrivateKey_PKCS1Format verifies loading a private key in PKCS1 format.
func TestLoadPrivateKey_PKCS1Format(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(t.TempDir(), "pkcs1.pem")
	block := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)}
	if err := os.WriteFile(path, pem.EncodeToMemory(block), 0600); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadPrivateKey(path)
	if err != nil {
		t.Fatal("expected PKCS1 to be loadable:", err)
	}
	if loaded == nil {
		t.Fatal("expected non-nil key")
	}
}

// TestEncryptDecrypt_RoundTrip verifies that data survives encrypt->decrypt.
func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	_, _, pub, priv := generateTestKeyPair(t)

	plaintext := []byte(`{"id":"test","type":"gauge","value":42.5}`)

	ciphertext, err := EncryptOAEP(plaintext, pub)
	if err != nil {
		t.Fatal("EncryptOAEP:", err)
	}

	result, err := DecryptOAEP(ciphertext, priv)
	if err != nil {
		t.Fatal("DecryptOAEP:", err)
	}

	if !bytes.Equal(result, plaintext) {
		t.Fatalf("data mismatch:\ngot:  %s\nwant: %s", result, plaintext)
	}
}

// TestEncryptDecrypt_GzippedPayload simulates the actual agent->server flow:
// JSON -> gzip -> encrypt -> decrypt -> gzip decompress -> original.
func TestEncryptDecrypt_GzippedPayload(t *testing.T) {
	_, _, pub, priv := generateTestKeyPair(t)

	original := []byte(`[{"id":"test","type":"counter","delta":1}]`)
	gzipped := gzipData(t, original)

	encrypted, err := EncryptOAEP(gzipped, pub)
	if err != nil {
		t.Fatal("EncryptOAEP:", err)
	}

	decrypted, err := DecryptOAEP(encrypted, priv)
	if err != nil {
		t.Fatal("DecryptOAEP:", err)
	}

	gzr, err := gzip.NewReader(bytes.NewReader(decrypted))
	if err != nil {
		t.Fatal("gzip.NewReader:", err)
	}
	plain, err := io.ReadAll(gzr)
	if err != nil {
		t.Fatal("gzip read:", err)
	}
	gzr.Close()

	if !bytes.Equal(plain, original) {
		t.Fatalf("data mismatch:\ngot:  %s\nwant: %s", plain, original)
	}
}

// TestEncryptDecrypt_LargePayload verifies that payloads near the
// maximum RSA-OAEP size work correctly.
func TestEncryptDecrypt_LargePayload(t *testing.T) {
	_, _, pub, priv := generateTestKeyPair(t)

	// Max OAEP payload for 2048-bit key with SHA-256: 2048/8 - 2*32 - 2 = 190 bytes.
	plaintext := make([]byte, 190)
	for i := range plaintext {
		plaintext[i] = byte(i % 256)
	}

	ciphertext, err := EncryptOAEP(plaintext, pub)
	if err != nil {
		t.Fatal("EncryptOAEP:", err)
	}

	result, err := DecryptOAEP(ciphertext, priv)
	if err != nil {
		t.Fatal("DecryptOAEP:", err)
	}

	if !bytes.Equal(result, plaintext) {
		t.Fatal("data mismatch for large payload")
	}
}

// TestEncryptDecrypt_WrongKey verifies that decryption with a different
// private key fails.
func TestEncryptDecrypt_WrongKey(t *testing.T) {
	_, _, pub, _ := generateTestKeyPair(t)
	_, _, _, wrongPriv := generateTestKeyPair(t)

	plaintext := []byte("test data")
	ciphertext, err := EncryptOAEP(plaintext, pub)
	if err != nil {
		t.Fatal("EncryptOAEP:", err)
	}

	_, err = DecryptOAEP(ciphertext, wrongPriv)
	if err == nil {
		t.Fatal("expected error when decrypting with wrong key")
	}
}

// TestEncryptDecrypt_EmptyPlaintext verifies encrypt/decrypt of empty data.
func TestEncryptDecrypt_EmptyPlaintext(t *testing.T) {
	_, _, pub, priv := generateTestKeyPair(t)

	ciphertext, err := EncryptOAEP([]byte{}, pub)
	if err != nil {
		t.Fatal("EncryptOAEP empty:", err)
	}

	result, err := DecryptOAEP(ciphertext, priv)
	if err != nil {
		t.Fatal("DecryptOAEP empty:", err)
	}

	if len(result) != 0 {
		t.Fatal("expected empty result")
	}
}

// TestDecryptOAEP_InvalidCiphertext verifies error on garbage input.
func TestDecryptOAEP_InvalidCiphertext(t *testing.T) {
	_, _, _, priv := generateTestKeyPair(t)

	_, err := DecryptOAEP([]byte("not encrypted"), priv)
	if err == nil {
		t.Fatal("expected error for invalid ciphertext")
	}
}

// TestEncryptOAEP_NilPublicKey verifies panic-safe behavior (nil key).
func TestEncryptOAEP_NilPublicKey(t *testing.T) {
	_, err := EncryptOAEP([]byte("data"), nil)
	if err == nil {
		t.Fatal("expected error for nil public key")
	}
}

// TestDecryptOAEP_NilPrivateKey verifies panic-safe behavior (nil key).
func TestDecryptOAEP_NilPrivateKey(t *testing.T) {
	_, err := DecryptOAEP([]byte("data"), nil)
	if err == nil {
		t.Fatal("expected error for nil private key")
	}
}

// TestLoadPrivateKey_NonRSAPrivateKey verifies that non-RSA keys are rejected.
func TestLoadPrivateKey_NonRSAPrivateKey(t *testing.T) {
	// Generate an ECDSA key and write it as PEM.
	ecKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	// Use a dummy x509 certificate template to create non-RSA content.
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test"},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(time.Hour),
	}
	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &ecKey.PublicKey, ecKey)
	if err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(t.TempDir(), "cert.pem")
	if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER}), 0644); err != nil {
		t.Fatal(err)
	}

	// Loading a certificate as a private key should fail.
	_, err = LoadPrivateKey(path)
	if err == nil {
		t.Fatal("expected error for non-key PEM")
	}
}

// TestLoadPublicKey_NonRSAPublicKey verifies that non-RSA public keys are rejected.
func TestLoadPublicKey_NonRSAPublicKey(t *testing.T) {
	// Generate an RSA key and write its PKCS8 private key as "PUBLIC KEY" — should fail.
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}

	privBytes, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(t.TempDir(), "fake_pub.pem")
	if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: privBytes}), 0644); err != nil {
		t.Fatal(err)
	}

	_, err = LoadPublicKey(path)
	if err == nil {
		t.Fatal("expected error for private key bytes in PUBLIC KEY block")
	}
}
