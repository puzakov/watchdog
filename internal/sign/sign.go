package sign

import (
	"crypto/sha256"
	"encoding/hex"
)

const HeaderHashSHA256 = "HashSHA256"

// SumSHA256 returns hex-encoded SHA256 over value+key.
func SumSHA256(value []byte, key string) string {
	h := sha256.New()
	_, _ = h.Write(value)
	_, _ = h.Write([]byte(key))
	return hex.EncodeToString(h.Sum(nil))
}
