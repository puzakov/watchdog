package service

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"

	models "github.com/puzakov/watchdog/internal/model"
)

// FileStore persists metrics to a JSON file on disk.
// It uses atomic file writes (write to .tmp, then rename).
type FileStore struct {
	path string
	mu   sync.Mutex
}

// NewFileStore creates a FileStore that writes to the given path.
func NewFileStore(path string) *FileStore {
	return &FileStore{path: path}
}

// Save writes all gauges and counters to the file as a JSON array of Metrics.
// Writes to a .tmp file first, then renames atomically.
func (fs *FileStore) Save(gauges map[string]float64, counters map[string]int64) error {
	if fs == nil || fs.path == "" {
		return nil
	}

	metrics := make([]models.Metrics, 0, len(gauges)+len(counters))

	for id, v := range gauges {
		vv := v
		metrics = append(metrics, models.Metrics{
			ID:    id,
			MType: models.Gauge,
			Value: &vv,
		})
	}
	for id, d := range counters {
		dd := d
		metrics = append(metrics, models.Metrics{
			ID:    id,
			MType: models.Counter,
			Delta: &dd,
		})
	}

	fs.mu.Lock()
	defer fs.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(fs.path), 0755); err != nil && filepath.Dir(fs.path) != "." {
		return err
	}

	tmp := fs.path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	enc := json.NewEncoder(f)
	if err := enc.Encode(metrics); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}

	return os.Rename(tmp, fs.path)
}

// Restore reads metrics from the file and loads them into the given Storage.
// If the file is empty or does not exist, it returns nil.
func (fs *FileStore) Restore(ctx context.Context, storage Storage) error {
	if fs == nil || fs.path == "" {
		return nil
	}

	fs.mu.Lock()
	defer fs.mu.Unlock()

	f, err := os.OpenFile(fs.path, os.O_RDONLY|os.O_CREATE, 0666)
	if err != nil {
		return err
	}
	defer f.Close()

	var metrics []models.Metrics
	dec := json.NewDecoder(f)
	if err := dec.Decode(&metrics); err != nil {
		if errors.Is(err, io.EOF) {
			return nil
		}
		return err
	}

	for _, m := range metrics {
		switch m.MType {
		case models.Gauge:
			if m.Value == nil {
				continue
			}
			_ = storage.UpdateGauge(ctx, m.ID, *m.Value)
		case models.Counter:
			if m.Delta == nil {
				continue
			}
			_ = storage.UpdateCounter(ctx, m.ID, *m.Delta)
		}
	}

	return nil
}

// PersistingStorage wraps any Storage with a FileStore, writing to file
// on every mutation (UpdateGauge, UpdateCounter, UpdateBatch).
type PersistingStorage struct {
	Storage
	fs *FileStore
}

// NewPersistingStorage wraps inner Storage so every write is also saved to the FileStore.
func NewPersistingStorage(inner Storage, fs *FileStore) *PersistingStorage {
	return &PersistingStorage{Storage: inner, fs: fs}
}

func (s *PersistingStorage) UpdateGauge(ctx context.Context, name string, value float64) error {
	if err := s.Storage.UpdateGauge(ctx, name, value); err != nil {
		return err
	}
	return s.fs.Save(s.Snapshot(ctx))
}

func (s *PersistingStorage) UpdateCounter(ctx context.Context, name string, delta int64) error {
	if err := s.Storage.UpdateCounter(ctx, name, delta); err != nil {
		return err
	}
	return s.fs.Save(s.Snapshot(ctx))
}

func (s *PersistingStorage) UpdateBatch(ctx context.Context, metrics []models.Metrics) error {
	if err := s.Storage.UpdateBatch(ctx, metrics); err != nil {
		return err
	}
	return s.fs.Save(s.Snapshot(ctx))
}
