package handler

import (
	"testing"

	models "github.com/puzakov/watchdog/internal/model"
)

func TestNormalizeBatch_Empty(t *testing.T) {
	out, err := normalizeBatch(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != nil {
		t.Fatalf("out = %v, want nil", out)
	}
}

func TestNormalizeBatch_MergesDuplicates(t *testing.T) {
	v1, v2 := 1.0, 2.0
	d1, d2 := int64(3), int64(4)

	out, err := normalizeBatch([]models.Metrics{
		{ID: "g", MType: models.Gauge, Value: &v1},
		{ID: "g", MType: models.Gauge, Value: &v2},
		{ID: "c", MType: models.Counter, Delta: &d1},
		{ID: "c", MType: models.Counter, Delta: &d2},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("len(out) = %d, want 2", len(out))
	}

	gauges := map[string]float64{}
	counters := map[string]int64{}
	for _, m := range out {
		switch m.MType {
		case models.Gauge:
			gauges[m.ID] = *m.Value
		case models.Counter:
			counters[m.ID] = *m.Delta
		}
	}
	if gauges["g"] != 2.0 {
		t.Fatalf("gauge g = %v, want 2.0", gauges["g"])
	}
	if counters["c"] != 7 {
		t.Fatalf("counter c = %v, want 7", counters["c"])
	}
}

func TestNormalizeBatch_InvalidMetric(t *testing.T) {
	v := 1.0
	_, err := normalizeBatch([]models.Metrics{{ID: "", MType: models.Gauge, Value: &v}})
	if err == nil {
		t.Fatal("expected error for empty id")
	}
}
