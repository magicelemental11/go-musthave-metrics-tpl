package handler

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/model"
)

func (h *Handler) UpdateMetricsBatch(w http.ResponseWriter, r *http.Request) {
	var metrics []model.Metrics
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodySize)).Decode(&metrics); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if len(metrics) == 0 {
		writeJSONError(w, http.StatusBadRequest, "empty batch")
		return
	}
	for i, m := range metrics {
		if err := m.Validate(); err != nil {
			writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("metric %d: %v", i, err))
			return
		}
	}

	if err := h.storage.UpdateBatch(r.Context(), metrics); err != nil {
		log.Printf("update batch: %v", err)
		writeJSONError(w, http.StatusInternalServerError, "storage error")
		return
	}
	names := make([]string, len(metrics))
	for i, m := range metrics {
		names[i] = m.ID
	}
	h.publishAudit(r, names...)
	w.WriteHeader(http.StatusOK)
}
