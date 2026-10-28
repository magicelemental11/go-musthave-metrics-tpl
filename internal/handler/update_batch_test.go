package handler

import (
	"context"
	"errors"
	"testing"

	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/model"
	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/repository"
)

func TestUpdateBatch(t *testing.T) {
	s := repository.NewMemStorage()
	p := &fakePublisher{}
	h := NewHandler(s, nil).WithAudit(p)

	err := h.UpdateBatch(context.Background(), "10.0.0.1", []model.Metrics{
		{ID: "g", MType: model.Gauge, Value: new(1.5)},
		{ID: "c", MType: model.Counter, Delta: new(int64(2))},
	})
	if err != nil {
		t.Fatal(err)
	}
	if v, _ := s.GetGauge(context.Background(), "g"); v != 1.5 {
		t.Errorf("gauge = %v, want 1.5", v)
	}
	if v, _ := s.GetCounter(context.Background(), "c"); v != 2 {
		t.Errorf("counter = %v, want 2", v)
	}
	if len(p.events) != 1 || p.events[0].IPAddress != "10.0.0.1" || len(p.events[0].Metrics) != 2 {
		t.Errorf("audit events = %+v", p.events)
	}
}

func TestUpdateBatchInvalid(t *testing.T) {
	tests := map[string][]model.Metrics{
		"empty batch":    nil,
		"missing id":     {{MType: model.Gauge, Value: new(1.0)}},
		"missing value":  {{ID: "g", MType: model.Gauge}},
		"missing delta":  {{ID: "c", MType: model.Counter}},
		"unknown type":   {{ID: "x", MType: "histogram", Value: new(1.0)}},
		"one bad of two": {{ID: "g", MType: model.Gauge, Value: new(1.0)}, {ID: "c", MType: model.Counter}},
	}
	for name, batch := range tests {
		t.Run(name, func(t *testing.T) {
			s := repository.NewMemStorage()
			p := &fakePublisher{}
			err := NewHandler(s, nil).WithAudit(p).UpdateBatch(context.Background(), "", batch)
			if !errors.Is(err, ErrInvalidBatch) {
				t.Fatalf("err = %v, want ErrInvalidBatch", err)
			}
			if gauges, counters, _ := s.GetAllMetrics(context.Background()); len(gauges)+len(counters) != 0 {
				t.Errorf("invalid batch must not change storage, got %v %v", gauges, counters)
			}
			if len(p.events) != 0 {
				t.Errorf("invalid batch must not be audited, got %+v", p.events)
			}
		})
	}
}

func TestUpdateBatchStorageError(t *testing.T) {
	dbErr := errors.New("db down")
	err := NewHandler(failingStorage{err: dbErr}, nil).UpdateBatch(context.Background(), "",
		[]model.Metrics{{ID: "g", MType: model.Gauge, Value: new(1.0)}})
	if !errors.Is(err, dbErr) || errors.Is(err, ErrInvalidBatch) {
		t.Fatalf("err = %v, want the storage error", err)
	}
}
