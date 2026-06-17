// Package sign provides SHA256 request/response signing utilities
// used by the HashSHA256 middleware for integrity verification.
package sign

import (
	"crypto/sha256"
	"encoding/hex"
)

// HeaderHashSHA256 is the header name used to transmit the SHA256 body hash.
const HeaderHashSHA256 = "HashSHA256"

// SumSHA256 returns a hex-encoded SHA256 hash of value concatenated with key.
// Used for request/response integrity verification in HashSHA256 middleware.
func SumSHA256(value []byte, key string) string {
	buf := make([]byte, 0, len(value)+len(key))
	buf = append(buf, value...)
	buf = append(buf, key...)
	sum := sha256.Sum256(buf)
	return hex.EncodeToString(sum[:])
}
