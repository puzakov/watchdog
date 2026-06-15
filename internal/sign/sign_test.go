package sign

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestSumSHA256(t *testing.T) {
	value := []byte("payload")
	key := "secret"

	got := SumSHA256(value, key)

	h := sha256.New()
	_, _ = h.Write(value)
	_, _ = h.Write([]byte(key))
	want := hex.EncodeToString(h.Sum(nil))

	if got != want {
		t.Fatalf("SumSHA256() = %q, want %q", got, want)
	}
}

func TestSumSHA256_Empty(t *testing.T) {
	got := SumSHA256(nil, "")
	if len(got) != 64 {
		t.Fatalf("hash length = %d, want 64", len(got))
	}
}
