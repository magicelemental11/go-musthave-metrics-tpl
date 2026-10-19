package handler

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/model"
)

func writeJSON(w http.ResponseWriter, m model.Metrics) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(m); err != nil {
		log.Printf("error encoding response: %v\n", err)
	}
}

// UpdateMetricJSON обрабатывает POST /update: принимает метрику в JSON и возвращает её актуальное значение.
func (h *Handler) UpdateMetricJSON(w http.ResponseWriter, r *http.Request) {
	var m model.Metrics
	if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if m.ID == "" {
		http.Error(w, "metric id is required", http.StatusNotFound)
		return
	}

	switch m.MType {
	case model.Gauge:
		if m.Value == nil {
			http.Error(w, "gauge value is required", http.StatusBadRequest)
			return
		}
		h.storage.UpdateGauge(m.ID, *m.Value)
	case model.Counter:
		if m.Delta == nil {
			http.Error(w, "counter delta is required", http.StatusBadRequest)
			return
		}
		total := h.storage.UpdateCounter(m.ID, *m.Delta)
		m.Delta = &total
	default:
		http.Error(w, "unknown metric type", http.StatusBadRequest)
		return
	}

	writeJSON(w, m)
}

// GetMetricJSON обрабатывает POST /value: по id и type возвращает метрику с заполненным значением.
func (h *Handler) GetMetricJSON(w http.ResponseWriter, r *http.Request) {
	var m model.Metrics
	if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	switch m.MType {
	case model.Gauge:
		value, ok := h.storage.GetGauge(m.ID)
		if !ok {
			http.Error(w, "gauge not found", http.StatusNotFound)
			return
		}
		m.Value = &value
	case model.Counter:
		delta, ok := h.storage.GetCounter(m.ID)
		if !ok {
			http.Error(w, "counter not found", http.StatusNotFound)
			return
		}
		m.Delta = &delta
	default:
		http.Error(w, "unknown metric type", http.StatusBadRequest)
		return
	}

	writeJSON(w, m)
}
