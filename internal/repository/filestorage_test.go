package repository

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFileStorageSaveAndRestore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "metrics.json")

	src := NewFileStorage(NewMemStorage(), path, false)
	src.UpdateGauge("LastGC", 1.5)
	src.UpdateCounter("NumGC", 42)
	if err := src.Save(); err != nil {
		t.Fatal(err)
	}

	dst := NewFileStorage(NewMemStorage(), path, false)
	if err := dst.Restore(); err != nil {
		t.Fatal(err)
	}
	if v, ok := dst.GetGauge("LastGC"); !ok || v != 1.5 {
		t.Errorf("gauge = %v, %v", v, ok)
	}
	if v, ok := dst.GetCounter("NumGC"); !ok || v != 42 {
		t.Errorf("counter = %v, %v", v, ok)
	}
}

func TestFileStorageRestoreMissingFile(t *testing.T) {
	f := NewFileStorage(NewMemStorage(), filepath.Join(t.TempDir(), "none.json"), false)
	if err := f.Restore(); err != nil {
		t.Errorf("missing file must not be an error, got %v", err)
	}
}

func TestFileStorageRestoreInvalidFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(path, []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := NewFileStorage(NewMemStorage(), path, false).Restore(); err == nil {
		t.Error("expected error for invalid file")
	}
}

func TestFileStorageSyncSave(t *testing.T) {
	path := filepath.Join(t.TempDir(), "metrics.json")
	f := NewFileStorage(NewMemStorage(), path, true)

	f.UpdateCounter("PollCount", 3)
	f.UpdateCounter("PollCount", 4)

	restored := NewFileStorage(NewMemStorage(), path, false)
	if err := restored.Restore(); err != nil {
		t.Fatal(err)
	}
	if v, _ := restored.GetCounter("PollCount"); v != 7 {
		t.Errorf("counter = %d, want 7", v)
	}
}

func TestFileStorageNoSaveWithoutSync(t *testing.T) {
	path := filepath.Join(t.TempDir(), "metrics.json")
	NewFileStorage(NewMemStorage(), path, false).UpdateGauge("a", 1)
	if _, err := os.Stat(path); err == nil {
		t.Error("file must not be written when sync save is off")
	}
}

func TestFileStorageSaveCreatesDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "dir", "metrics.json")
	f := NewFileStorage(NewMemStorage(), path, false)
	f.UpdateGauge("a", 1)
	if err := f.Save(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("file was not created: %v", err)
	}
}

func TestFileStorageRestoreIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "metrics.json")
	src := NewFileStorage(NewMemStorage(), path, false)
	src.UpdateCounter("NumGC", 42)
	if err := src.Save(); err != nil {
		t.Fatal(err)
	}

	dst := NewFileStorage(NewMemStorage(), path, false)
	for range 2 {
		if err := dst.Restore(); err != nil {
			t.Fatal(err)
		}
	}
	if v, _ := dst.GetCounter("NumGC"); v != 42 {
		t.Errorf("counter = %d, want 42", v)
	}
}
