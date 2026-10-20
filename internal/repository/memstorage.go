package repository

import (
	"sort"
	"sync"

	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/model"
)

type Storage interface {
	GetGauge(name string) (float64, bool)
	GetCounter(name string) (int64, bool)
	UpdateGauge(name string, value float64)
	UpdateCounter(name string, value int64) int64
	GetAllMetrics() (map[string]float64, map[string]int64)
}

type MemStorage struct {
	mu       sync.RWMutex
	gauges   map[string]float64
	counters map[string]int64
}

func NewMemStorage() *MemStorage {
	return &MemStorage{
		gauges:   make(map[string]float64),
		counters: make(map[string]int64),
	}
}

func (m *MemStorage) GetGauge(name string) (float64, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	value, ok := m.gauges[name]
	return value, ok
}

func (m *MemStorage) GetCounter(name string) (int64, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	value, ok := m.counters[name]
	return value, ok
}

func (m *MemStorage) UpdateGauge(name string, value float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.gauges[name] = value
}

// UpdateCounter прибавляет value к счётчику и возвращает новое накопленное значение.
func (m *MemStorage) UpdateCounter(name string, value int64) int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.counters[name] += value
	return m.counters[name]
}

// GetAllMetrics возвращает копии внутренних карт.
func (m *MemStorage) GetAllMetrics() (map[string]float64, map[string]int64) {
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
	return gauges, counters
}

// Snapshot возвращает все метрики списком, отсортированным по типу и имени.
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

// Load записывает метрики в хранилище, заменяя текущие значения.
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
