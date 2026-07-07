// Package crypto provides RSA-OAEP encryption and decryption helpers
// for asymmetric encryption of metrics payloads between agent and server.
package crypto

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
)

// LoadPublicKey reads a PEM-encoded RSA public key from the given file path.
func LoadPublicKey(path string) (*rsa.PublicKey, error) {
	pemBytes, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read public key file: %w", err)
	}

	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, errors.New("no PEM block found in public key file")
	}

	key, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse public key: %w", err)
	}

	pub, ok := key.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("key is not an RSA public key")
	}

	return pub, nil
}

// LoadPrivateKey reads a PEM-encoded RSA private key from the given file path.
func LoadPrivateKey(path string) (*rsa.PrivateKey, error) {
	pemBytes, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read private key file: %w", err)
	}

	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, errors.New("no PEM block found in private key file")
	}

	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		// Fallback: try PKCS1 format.
		key, err = x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return nil, errors.New("private key is not in PKCS8 or PKCS1 format")
		}
	}

	priv, ok := key.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("key is not an RSA private key")
	}

	return priv, nil
}

// EncryptOAEP encrypts plaintext using RSA-OAEP with SHA-256.
// The label is empty. Returns the ciphertext.
func EncryptOAEP(plaintext []byte, pub *rsa.PublicKey) ([]byte, error) {
	// RSA-OAEP can only encrypt data up to keySize - 2*hashSize - 2 bytes.
	// For a 2048-bit key with SHA-256 (32 bytes): 2048/8 - 2*32 - 2 = 190 bytes.
	// The gzip-compressed metrics payload is typically well under this limit.
	ciphertext, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, pub, plaintext, nil)
	if err != nil {
		return nil, fmt.Errorf("rsa encrypt: %w", err)
	}
	return ciphertext, nil
}

// DecryptOAEP decrypts ciphertext using RSA-OAEP with SHA-256.
// The label is empty. Returns the plaintext.
func DecryptOAEP(ciphertext []byte, priv *rsa.PrivateKey) ([]byte, error) {
	plaintext, err := rsa.DecryptOAEP(sha256.New(), rand.Reader, priv, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("rsa decrypt: %w", err)
	}
	return plaintext, nil
}
