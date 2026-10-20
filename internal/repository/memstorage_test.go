package repository

import (
	"errors"
	"sync"
	"testing"

	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/model"
)

func TestMemStorageUpdateBatch(t *testing.T) {
	m := NewMemStorage()
	g1, g2 := 1.5, 2.5
	c1, c2 := int64(3), int64(4)
	err := m.UpdateBatch(ctx, []model.Metrics{
		{ID: "g", MType: model.Gauge, Value: &g1},
		{ID: "g", MType: model.Gauge, Value: &g2},
		{ID: "c", MType: model.Counter, Delta: &c1},
		{ID: "c", MType: model.Counter, Delta: &c2},
	})
	if err != nil {
		t.Fatal(err)
	}
	if v, _ := m.GetGauge(ctx, "g"); v != 2.5 {
		t.Errorf("gauge = %v", v)
	}
	if v, _ := m.GetCounter(ctx, "c"); v != 7 {
		t.Errorf("counter = %v", v)
	}
}

func TestMemStorageConcurrentBatches(t *testing.T) {
	m := NewMemStorage()
	one := int64(1)
	const workers, batches = 8, 100

	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range batches {
				m.UpdateBatch(ctx, []model.Metrics{{ID: "c", MType: model.Counter, Delta: &one}})
			}
		}()
	}
	wg.Wait()

	if v, _ := m.GetCounter(ctx, "c"); v != workers*batches {
		t.Fatalf("counter = %d, want %d", v, workers*batches)
	}
}

func TestMemStorageUpdateBatchRejectsInvalid(t *testing.T) {
	m := NewMemStorage()
	g := 1.5
	err := m.UpdateBatch(ctx, []model.Metrics{
		{ID: "g", MType: model.Gauge, Value: &g},
		{ID: "c", MType: model.Counter},
	})
	if err == nil {
		t.Fatal("expected error for counter without delta")
	}
	if _, err := m.GetGauge(ctx, "g"); !errors.Is(err, ErrNotFound) {
		t.Errorf("invalid batch must not be applied, got %v", err)
	}
}
