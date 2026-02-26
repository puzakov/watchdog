package agent

import "testing"

func TestReadRuntimeGauges_ContainsAllRequiredKeys(t *testing.T) {
	got := ReadRuntimeGauges()

	required := []string{
		"Alloc",
		"BuckHashSys",
		"Frees",
		"GCCPUFraction",
		"GCSys",
		"HeapAlloc",
		"HeapIdle",
		"HeapInuse",
		"HeapObjects",
		"HeapReleased",
		"HeapSys",
		"LastGC",
		"Lookups",
		"MCacheInuse",
		"MCacheSys",
		"MSpanInuse",
		"MSpanSys",
		"Mallocs",
		"NextGC",
		"NumForcedGC",
		"NumGC",
		"OtherSys",
		"PauseTotalNs",
		"StackInuse",
		"StackSys",
		"Sys",
		"TotalAlloc",
	}

	if len(got) != len(required) {
		t.Fatalf("len = %d, want %d", len(got), len(required))
	}

	for _, k := range required {
		if _, ok := got[k]; !ok {
			t.Fatalf("missing key %q", k)
		}
	}
}
