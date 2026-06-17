package sign

import (
	"testing"
)

func BenchmarkSumSHA256(b *testing.B) {
	value := []byte(`{"id":"Alloc","type":"gauge","value":123.45}`)
	key := "secret-key"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = SumSHA256(value, key)
	}
}
