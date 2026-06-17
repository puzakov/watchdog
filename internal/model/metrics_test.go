package models

import (
	"encoding/json"
	"testing"
)

func TestMetricsMarshal(t *testing.T) {
	gaugeVal := 42.5
	m := Metrics{
		ID:    "test",
		MType: Gauge,
		Value: &gaugeVal,
	}
	data, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded Metrics
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.ID != "test" {
		t.Errorf("ID = %q, want %q", decoded.ID, "test")
	}
	if decoded.MType != Gauge {
		t.Errorf("MType = %q, want %q", decoded.MType, Gauge)
	}
	if decoded.Value == nil || *decoded.Value != 42.5 {
		t.Errorf("Value = %v, want 42.5", decoded.Value)
	}
}

func TestMetricsMarshalCounter(t *testing.T) {
	delta := int64(100)
	m := Metrics{
		ID:    "counter1",
		MType: Counter,
		Delta: &delta,
	}
	data, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded Metrics
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.ID != "counter1" {
		t.Errorf("ID = %q, want %q", decoded.ID, "counter1")
	}
	if decoded.MType != Counter {
		t.Errorf("MType = %q, want %q", decoded.MType, Counter)
	}
	if decoded.Delta == nil || *decoded.Delta != 100 {
		t.Errorf("Delta = %v, want 100", decoded.Delta)
	}
}

func TestMetricsEmptyValues(t *testing.T) {
	m := Metrics{
		ID:    "empty",
		MType: Gauge,
	}
	data, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("marshal returned empty data")
	}
}

func TestMetricsZeroValues(t *testing.T) {
	delta := int64(0)
	gaugeVal := 0.0
	m := Metrics{
		ID:    "zero",
		MType: Counter,
		Delta: &delta,
		Value: &gaugeVal,
	}
	data, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal zero values: %v", err)
	}
	var decoded Metrics
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.Delta == nil || *decoded.Delta != 0 {
		t.Errorf("Delta = %v, want 0", decoded.Delta)
	}
}

func TestGaugeConstant(t *testing.T) {
	if Gauge != "gauge" {
		t.Errorf("Gauge = %q, want %q", Gauge, "gauge")
	}
}

func TestCounterConstant(t *testing.T) {
	if Counter != "counter" {
		t.Errorf("Counter = %q, want %q", Counter, "counter")
	}
}
