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

// UpdateMetricJSON stores a single model.Metrics from the JSON body and echoes it back.
// For counters the echoed Delta is the accumulated total.
// It responds 400 for malformed JSON, an unknown type or a missing value, and 404 for an empty id.
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
		if err := h.storage.UpdateGauge(r.Context(), m.ID, *m.Value); err != nil {
			storageError(w, writeJSONError, "update gauge", err, "")
			return
		}
	case model.Counter:
		if m.Delta == nil {
			writeJSONError(w, http.StatusBadRequest, "counter delta is required")
			return
		}
		total, err := h.storage.UpdateCounter(r.Context(), m.ID, *m.Delta)
		if err != nil {
			storageError(w, writeJSONError, "update counter", err, "")
			return
		}
		m.Delta = &total
	default:
		writeJSONError(w, http.StatusBadRequest, "unknown metric type")
		return
	}

	h.publishAudit(r.Context(), clientIP(r), m.ID)
	writeJSON(w, http.StatusOK, m)
}

// GetMetricJSON fills in the value of the model.Metrics identified by the JSON body.
// It responds 404 if the metric does not exist and 400 for malformed JSON or an unknown type.
func (h *Handler) GetMetricJSON(w http.ResponseWriter, r *http.Request) {
	m, ok := decodeMetrics(w, r)
	if !ok {
		return
	}

	switch m.MType {
	case model.Gauge:
		value, err := h.storage.GetGauge(r.Context(), m.ID)
		if err != nil {
			storageError(w, writeJSONError, "get gauge", err, "gauge not found")
			return
		}
		m.Value = &value
	case model.Counter:
		delta, err := h.storage.GetCounter(r.Context(), m.ID)
		if err != nil {
			storageError(w, writeJSONError, "get counter", err, "counter not found")
			return
		}
		m.Delta = &delta
	default:
		writeJSONError(w, http.StatusBadRequest, "unknown metric type")
		return
	}

	writeJSON(w, http.StatusOK, m)
}
