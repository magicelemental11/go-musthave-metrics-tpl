package repository

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

var ctx = context.Background()

func TestFileStorageSaveAndRestore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "metrics.json")

	src := NewFileStorage(NewMemStorage(), path, false)
	src.UpdateGauge(ctx, "LastGC", 1.5)
	src.UpdateCounter(ctx, "NumGC", 42)
	if err := src.Save(); err != nil {
		t.Fatal(err)
	}

	dst := NewFileStorage(NewMemStorage(), path, false)
	if err := dst.Restore(); err != nil {
		t.Fatal(err)
	}
	if v, err := dst.GetGauge(ctx, "LastGC"); err != nil || v != 1.5 {
		t.Errorf("gauge = %v, %v", v, err)
	}
	if v, err := dst.GetCounter(ctx, "NumGC"); err != nil || v != 42 {
		t.Errorf("counter = %v, %v", v, err)
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

	f.UpdateCounter(ctx, "PollCount", 3)
	f.UpdateCounter(ctx, "PollCount", 4)

	restored := NewFileStorage(NewMemStorage(), path, false)
	if err := restored.Restore(); err != nil {
		t.Fatal(err)
	}
	if v, _ := restored.GetCounter(ctx, "PollCount"); v != 7 {
		t.Errorf("counter = %d, want 7", v)
	}
}

func TestFileStorageNoSaveWithoutSync(t *testing.T) {
	path := filepath.Join(t.TempDir(), "metrics.json")
	NewFileStorage(NewMemStorage(), path, false).UpdateGauge(ctx, "a", 1)
	if _, err := os.Stat(path); err == nil {
		t.Error("file must not be written when sync save is off")
	}
}

func TestFileStorageSaveCreatesDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "dir", "metrics.json")
	f := NewFileStorage(NewMemStorage(), path, false)
	f.UpdateGauge(ctx, "a", 1)
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
	src.UpdateCounter(ctx, "NumGC", 42)
	if err := src.Save(); err != nil {
		t.Fatal(err)
	}

	dst := NewFileStorage(NewMemStorage(), path, false)
	for range 2 {
		if err := dst.Restore(); err != nil {
			t.Fatal(err)
		}
	}
	if v, _ := dst.GetCounter(ctx, "NumGC"); v != 42 {
		t.Errorf("counter = %d, want 42", v)
	}
}
