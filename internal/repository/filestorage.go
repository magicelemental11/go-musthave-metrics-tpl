package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/model"
)

// FileStorage is a MemStorage persisted to a JSON file.
// With syncSave every update is written to disk immediately.
type FileStorage struct {
	*MemStorage
	path     string
	syncSave bool
	saveMu   sync.Mutex
}

// NewFileStorage wraps mem so that its metrics are saved to path.
func NewFileStorage(mem *MemStorage, path string, syncSave bool) *FileStorage {
	return &FileStorage{MemStorage: mem, path: path, syncSave: syncSave}
}

// UpdateGauge sets the gauge and saves the file if syncSave is on.
func (f *FileStorage) UpdateGauge(ctx context.Context, name string, value float64) error {
	if err := f.MemStorage.UpdateGauge(ctx, name, value); err != nil {
		return err
	}
	f.saveIfSync()
	return nil
}

// UpdateCounter adds to the counter, returns the new total and saves the file if syncSave is on.
func (f *FileStorage) UpdateCounter(ctx context.Context, name string, value int64) (int64, error) {
	total, err := f.MemStorage.UpdateCounter(ctx, name, value)
	if err != nil {
		return 0, err
	}
	f.saveIfSync()
	return total, nil
}

// UpdateBatch applies all metrics and saves the file if syncSave is on.
func (f *FileStorage) UpdateBatch(ctx context.Context, metrics []model.Metrics) error {
	if err := f.MemStorage.UpdateBatch(ctx, metrics); err != nil {
		return err
	}
	f.saveIfSync()
	return nil
}

func (f *FileStorage) saveIfSync() {
	if !f.syncSave {
		return
	}
	if err := f.Save(); err != nil {
		log.Printf("error saving metrics: %v", err)
	}
}

// Save writes the metrics to a temporary file and renames it, so a failure cannot corrupt the old file.
func (f *FileStorage) Save() error {
	f.saveMu.Lock()
	defer f.saveMu.Unlock()

	data, err := json.MarshalIndent(f.MemStorage.Snapshot(), "", "  ")
	if err != nil {
		return fmt.Errorf("marshal metrics: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(f.path), 0o755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(f.path), filepath.Base(f.path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	defer os.Remove(tmp.Name())

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("sync temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Rename(tmp.Name(), f.path); err != nil {
		return fmt.Errorf("replace %s: %w", f.path, err)
	}
	return nil
}

// Restore loads metrics from the file. A missing file is not an error.
func (f *FileStorage) Restore() error {
	data, err := os.ReadFile(f.path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read %s: %w", f.path, err)
	}
	var metrics []model.Metrics
	if err := json.Unmarshal(data, &metrics); err != nil {
		return fmt.Errorf("parse %s: %w", f.path, err)
	}
	f.MemStorage.Load(metrics)
	return nil
}
