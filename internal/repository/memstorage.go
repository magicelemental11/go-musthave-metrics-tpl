package repository

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/model"
)

// MemStorage is a Storage that keeps metrics in memory. It is safe for concurrent use.
type MemStorage struct {
	mu       sync.RWMutex
	gauges   map[string]float64
	counters map[string]int64
}

// NewMemStorage creates an empty MemStorage.
func NewMemStorage() *MemStorage {
	return &MemStorage{
		gauges:   make(map[string]float64),
		counters: make(map[string]int64),
	}
}

// GetGauge returns the gauge value or ErrNotFound.
func (m *MemStorage) GetGauge(_ context.Context, name string) (float64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	value, ok := m.gauges[name]
	if !ok {
		return 0, ErrNotFound
	}
	return value, nil
}

// GetCounter returns the counter value or ErrNotFound.
func (m *MemStorage) GetCounter(_ context.Context, name string) (int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	value, ok := m.counters[name]
	if !ok {
		return 0, ErrNotFound
	}
	return value, nil
}

// UpdateGauge sets the gauge to value.
func (m *MemStorage) UpdateGauge(_ context.Context, name string, value float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.gauges[name] = value
	return nil
}

// UpdateCounter adds value to the counter and returns the new total.
func (m *MemStorage) UpdateCounter(_ context.Context, name string, value int64) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.counters[name] += value
	return m.counters[name], nil
}

// UpdateBatch validates all metrics and then applies them under one lock.
func (m *MemStorage) UpdateBatch(_ context.Context, metrics []model.Metrics) error {
	for _, m := range metrics {
		if err := m.Validate(); err != nil {
			return fmt.Errorf("invalid metric %q: %w", m.ID, err)
		}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, metric := range metrics {
		switch metric.MType {
		case model.Gauge:
			m.gauges[metric.ID] = *metric.Value
		case model.Counter:
			m.counters[metric.ID] += *metric.Delta
		}
	}
	return nil
}

// GetAllMetrics returns copies of the gauge and counter maps.
func (m *MemStorage) GetAllMetrics(_ context.Context) (map[string]float64, map[string]int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	gauges := make(map[string]float64, len(m.gauges))
	for k, v := range m.gauges {
		gauges[k] = v
	}
	counters := make(map[string]int64, len(m.counters))
	for k, v := range m.counters {
		counters[k] = v
	}
	return gauges, counters, nil
}

// Snapshot returns all metrics as a list sorted by type and name.
func (m *MemStorage) Snapshot() []model.Metrics {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]model.Metrics, 0, len(m.gauges)+len(m.counters))
	for name, value := range m.gauges {
		result = append(result, model.Metrics{ID: name, MType: model.Gauge, Value: &value})
	}
	for name, value := range m.counters {
		result = append(result, model.Metrics{ID: name, MType: model.Counter, Delta: &value})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].MType != result[j].MType {
			return result[i].MType < result[j].MType
		}
		return result[i].ID < result[j].ID
	})
	return result
}

// Load stores metrics, replacing the current values.
func (m *MemStorage) Load(metrics []model.Metrics) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, metric := range metrics {
		switch {
		case metric.MType == model.Gauge && metric.Value != nil:
			m.gauges[metric.ID] = *metric.Value
		case metric.MType == model.Counter && metric.Delta != nil:
			m.counters[metric.ID] = *metric.Delta
		}
	}
}
