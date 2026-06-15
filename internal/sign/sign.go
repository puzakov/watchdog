package sign

import (
	"crypto/sha256"
	"encoding/hex"
)

const HeaderHashSHA256 = "HashSHA256"

// SumSHA256 returns hex-encoded SHA256 over value+key.
func SumSHA256(value []byte, key string) string {
	buf := make([]byte, 0, len(value)+len(key))
	buf = append(buf, value...)
	buf = append(buf, key...)
	sum := sha256.Sum256(buf)
	return hex.EncodeToString(sum[:])
}
