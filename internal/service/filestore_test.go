package service

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	models "github.com/puzakov/watchdog/internal/model"
)

func TestFileStore_SaveAndRestore(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "metrics.json")

	s1 := NewMemStorage()
	s1.UpdateGauge("g1", 1.25)
	s1.UpdateCounter("c1", 42)

	fs := NewFileStore(path)
	if err := fs.Save(s1.Snapshot()); err != nil {
		t.Fatalf("save: %v", err)
	}

	s2 := NewMemStorage()
	if err := fs.Restore(s2); err != nil {
		t.Fatalf("restore: %v", err)
	}

	if v, ok := s2.GetGauge("g1"); !ok || v != 1.25 {
		t.Fatalf("restored gauge g1 = (%v,%v), want (%v,true)", v, ok, 1.25)
	}
	if v, ok := s2.GetCounter("c1"); !ok || v != 42 {
		t.Fatalf("restored counter c1 = (%v,%v), want (%v,true)", v, ok, int64(42))
	}
}

func TestFileStore_RestoreMissingFileIsOK(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "missing.json")

	s := NewMemStorage()
	fs := NewFileStore(path)
	if err := fs.Restore(s); err != nil {
		t.Fatalf("restore missing file: %v", err)
	}
}

func TestPersistingStorage_SavesOnUpdateWhenIntervalZeroMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "metrics.json")

	base := NewMemStorage()
	fs := NewFileStore(path)
	s := NewPersistingStorage(base, fs)

	s.UpdateCounter("c1", 7)

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read saved file: %v", err)
	}

	var got []models.Metrics
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal saved json: %v", err)
	}

	found := false
	for _, m := range got {
		if m.ID == "c1" && m.MType == models.Counter && m.Delta != nil && *m.Delta == 7 {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("saved file does not contain expected counter metric c1=7, got=%+v", got)
	}
}
