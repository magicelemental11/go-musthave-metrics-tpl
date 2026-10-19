package handler

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/model"
)

const maxBodySize = 1 << 20

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("error encoding response: %v", err)
	}
}

func writeJSONError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func decodeMetrics(w http.ResponseWriter, r *http.Request) (model.Metrics, bool) {
	var m model.Metrics
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodySize)).Decode(&m); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid json")
		return m, false
	}
	return m, true
}

func (h *Handler) UpdateMetricJSON(w http.ResponseWriter, r *http.Request) {
	m, ok := decodeMetrics(w, r)
	if !ok {
		return
	}
	if m.ID == "" {
		writeJSONError(w, http.StatusNotFound, "metric id is required")
		return
	}

	switch m.MType {
	case model.Gauge:
		if m.Value == nil {
			writeJSONError(w, http.StatusBadRequest, "gauge value is required")
			return
		}
		h.storage.UpdateGauge(m.ID, *m.Value)
	case model.Counter:
		if m.Delta == nil {
			writeJSONError(w, http.StatusBadRequest, "counter delta is required")
			return
		}
		total := h.storage.UpdateCounter(m.ID, *m.Delta)
		m.Delta = &total
	default:
		writeJSONError(w, http.StatusBadRequest, "unknown metric type")
		return
	}

	writeJSON(w, http.StatusOK, m)
}

func (h *Handler) GetMetricJSON(w http.ResponseWriter, r *http.Request) {
	m, ok := decodeMetrics(w, r)
	if !ok {
		return
	}

	switch m.MType {
	case model.Gauge:
		value, ok := h.storage.GetGauge(m.ID)
		if !ok {
			writeJSONError(w, http.StatusNotFound, "gauge not found")
			return
		}
		m.Value = &value
	case model.Counter:
		delta, ok := h.storage.GetCounter(m.ID)
		if !ok {
			writeJSONError(w, http.StatusNotFound, "counter not found")
			return
		}
		m.Delta = &delta
	default:
		writeJSONError(w, http.StatusBadRequest, "unknown metric type")
		return
	}

	writeJSON(w, http.StatusOK, m)
}
