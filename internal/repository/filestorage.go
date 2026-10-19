package repository

import (
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

// FileStorage — MemStorage с сохранением в JSON-файл.
// При syncSave каждое обновление сразу пишется на диск.
type FileStorage struct {
	*MemStorage
	path     string
	syncSave bool
	saveMu   sync.Mutex
}

func NewFileStorage(mem *MemStorage, path string, syncSave bool) *FileStorage {
	return &FileStorage{MemStorage: mem, path: path, syncSave: syncSave}
}

func (f *FileStorage) UpdateGauge(name string, value float64) {
	f.MemStorage.UpdateGauge(name, value)
	f.saveIfSync()
}

func (f *FileStorage) UpdateCounter(name string, value int64) int64 {
	total := f.MemStorage.UpdateCounter(name, value)
	f.saveIfSync()
	return total
}

func (f *FileStorage) saveIfSync() {
	if !f.syncSave {
		return
	}
	if err := f.Save(); err != nil {
		log.Printf("error saving metrics: %v", err)
	}
}

// Save записывает метрики во временный файл и переименовывает его, чтобы не испортить старый файл при сбое.
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

// Restore загружает метрики из файла. Отсутствие файла не считается ошибкой.
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
