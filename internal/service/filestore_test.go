package service

import (
	"context"
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
	_ = s1.UpdateGauge(context.Background(), "g1", 1.25)
	_ = s1.UpdateCounter(context.Background(), "c1", 42)

	fs := NewFileStore(path)
	if err := fs.Save(s1.Snapshot(context.Background())); err != nil {
		t.Fatalf("save: %v", err)
	}

	s2 := NewMemStorage()
	if err := fs.Restore(context.Background(), s2); err != nil {
		t.Fatalf("restore: %v", err)
	}

	if v, ok := s2.GetGauge(context.Background(), "g1"); !ok || v != 1.25 {
		t.Fatalf("restored gauge g1 = (%v,%v), want (%v,true)", v, ok, 1.25)
	}
	if v, ok := s2.GetCounter(context.Background(), "c1"); !ok || v != 42 {
		t.Fatalf("restored counter c1 = (%v,%v), want (%v,true)", v, ok, int64(42))
	}
}

func TestFileStore_RestoreMissingFileIsOK(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "missing.json")

	s := NewMemStorage()
	fs := NewFileStore(path)
	if err := fs.Restore(context.Background(), s); err != nil {
		t.Fatalf("restore missing file: %v", err)
	}
}

func TestPersistingStorage_SavesOnUpdateWhenIntervalZeroMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "metrics.json")

	base := NewMemStorage()
	fs := NewFileStore(path)
	s := NewPersistingStorage(base, fs)

	_ = s.UpdateCounter(context.Background(), "c1", 7)

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

func TestPersistingStorage_UpdateGaugeSaves(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "metrics.json")
	base := NewMemStorage()
	fs := NewFileStore(path)
	s := NewPersistingStorage(base, fs)

	_ = s.UpdateGauge(context.Background(), "g1", 3.14)

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read saved file: %v", err)
	}
	var got []models.Metrics
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	found := false
	for _, m := range got {
		if m.ID == "g1" && m.MType == models.Gauge && m.Value != nil && *m.Value == 3.14 {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("saved file does not contain gauge g1=3.14, got=%+v", got)
	}
}

func TestPersistingStorage_UpdateBatchSaves(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "metrics.json")
	base := NewMemStorage()
	fs := NewFileStore(path)
	s := NewPersistingStorage(base, fs)

	gaugeVal := 2.71
	counterVal := int64(3)
	batch := []models.Metrics{
		{ID: "gx", MType: models.Gauge, Value: &gaugeVal},
		{ID: "cx", MType: models.Counter, Delta: &counterVal},
	}
	if err := s.UpdateBatch(context.Background(), batch); err != nil {
		t.Fatalf("UpdateBatch error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read saved file: %v", err)
	}
	var got []models.Metrics
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("saved file contains %d metrics, want 2", len(got))
	}
}

func TestFileStore_SaveNilReceiver(t *testing.T) {
	var fs *FileStore
	err := fs.Save(nil, nil)
	if err != nil {
		t.Fatalf("Save on nil receiver should return nil, got %v", err)
	}
}

func TestFileStore_RestoreNilReceiver(t *testing.T) {
	var fs *FileStore
	ctx := context.Background()
	storage := NewMemStorage()
	err := fs.Restore(ctx, storage)
	if err != nil {
		t.Fatalf("Restore on nil receiver should return nil, got %v", err)
	}
}

func TestFileStore_RestoreEmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.json")
	fs := NewFileStore(path)
	if err := os.WriteFile(path, []byte{}, 0666); err != nil {
		t.Fatalf("write empty file: %v", err)
	}
	storage := NewMemStorage()
	if err := fs.Restore(context.Background(), storage); err != nil {
		t.Fatalf("Restore empty file: %v", err)
	}
}

func TestFileStore_RestoreInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	fs := NewFileStore(path)
	if err := os.WriteFile(path, []byte("not json"), 0666); err != nil {
		t.Fatalf("write file: %v", err)
	}
	storage := NewMemStorage()
	if err := fs.Restore(context.Background(), storage); err == nil {
		t.Fatal("Restore invalid JSON should return error")
	}
}

func TestFileStore_RestoreWithEmptyPath(t *testing.T) {
	fs := NewFileStore("")
	err := fs.Restore(context.Background(), nil)
	if err != nil {
		t.Fatalf("Restore with empty path should return nil, got %v", err)
	}
}

func TestFileStore_SaveWithEmptyPath(t *testing.T) {
	fs := NewFileStore("")
	err := fs.Save(map[string]float64{"g1": 1.0}, map[string]int64{"c1": 1})
	if err != nil {
		t.Fatalf("Save with empty path should return nil, got %v", err)
	}
}

func TestFileStore_SaveUnwritableDir(t *testing.T) {
	// Use a path inside /dev/null which is a file, not a directory, so MkdirAll fails.
	fs := NewFileStore("/dev/null/subdir/metrics.json")
	err := fs.Save(map[string]float64{"g1": 1.0}, nil)
	if err == nil {
		t.Fatal("expected error when saving to unwritable location")
	}
}
