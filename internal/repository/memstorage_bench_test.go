package repository

import (
	"context"
	"fmt"
	"testing"

	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/model"
)

func benchBatch(n int) []model.Metrics {
	batch := make([]model.Metrics, 0, n)
	for i := 0; i < n; i++ {
		v, d := float64(i)*1.5, int64(i)
		if i%5 == 0 {
			batch = append(batch, model.Metrics{ID: fmt.Sprintf("counter%d", i), MType: model.Counter, Delta: &d})
		} else {
			batch = append(batch, model.Metrics{ID: fmt.Sprintf("gauge%d", i), MType: model.Gauge, Value: &v})
		}
	}
	return batch
}

func BenchmarkMemStorageUpdateGauge(b *testing.B) {
	s := NewMemStorage()
	ctx := context.Background()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			s.UpdateGauge(ctx, "Alloc", 1.5)
		}
	})
}

func BenchmarkMemStorageGetGauge(b *testing.B) {
	s := NewMemStorage()
	ctx := context.Background()
	s.UpdateGauge(ctx, "Alloc", 1.5)
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if _, err := s.GetGauge(ctx, "Alloc"); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkMemStorageUpdateBatch(b *testing.B) {
	s := NewMemStorage()
	batch := benchBatch(30)
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := s.UpdateBatch(ctx, batch); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMemStorageGetAllMetrics(b *testing.B) {
	s := NewMemStorage()
	if err := s.UpdateBatch(context.Background(), benchBatch(30)); err != nil {
		b.Fatal(err)
	}
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, _, err := s.GetAllMetrics(ctx); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMemStorageSnapshot(b *testing.B) {
	s := NewMemStorage()
	if err := s.UpdateBatch(context.Background(), benchBatch(30)); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Snapshot()
	}
}
