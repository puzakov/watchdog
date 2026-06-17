// Package models defines the Metrics data structure used across the application
// for representing gauge and counter metrics with optional hashing.
package models

// Metric type constants.
const (
	// Counter is a metric type whose value accumulates (adds to previous).
	Counter = "counter"
	// Gauge is a metric type whose value is a float64 that replaces the previous.
	Gauge = "gauge"
)

// Metrics represents a single metric with either a gauge (float64) or counter (int64) value.
//
// Delta and Value are pointers to distinguish "zero" from "not set",
// preventing them from being encoded when nil.
type Metrics struct {
	ID    string   `json:"id"`
	MType string   `json:"type"`
	Delta *int64   `json:"delta,omitempty"`
	Value *float64 `json:"value,omitempty"`
	Hash  string   `json:"hash,omitempty"`
}
