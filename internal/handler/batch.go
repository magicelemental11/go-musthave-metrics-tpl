package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/model"
)

// ErrInvalidBatch matches, via errors.Is, the error for a batch rejected before reaching the storage.
var ErrInvalidBatch = errors.New("invalid batch")

// invalidBatchError carries only the reason, so the HTTP response text stays as before.
type invalidBatchError string

func (e invalidBatchError) Error() string        { return string(e) }
func (e invalidBatchError) Is(target error) bool { return target == ErrInvalidBatch }

// UpdateBatch validates metrics, stores them in one storage call and publishes an audit event with ip.
// It is shared by the HTTP and gRPC transports. A rejected batch returns an error wrapping ErrInvalidBatch
// and stores nothing; any other error comes from the storage.
func (h *Handler) UpdateBatch(ctx context.Context, ip string, metrics []model.Metrics) error {
	if len(metrics) == 0 {
		return invalidBatchError("empty batch")
	}
	for i, m := range metrics {
		if err := m.Validate(); err != nil {
			return invalidBatchError(fmt.Sprintf("metric %d: %v", i, err))
		}
	}

	if err := h.storage.UpdateBatch(ctx, metrics); err != nil {
		return fmt.Errorf("update batch: %w", err)
	}
	names := make([]string, len(metrics))
	for i, m := range metrics {
		names[i] = m.ID
	}
	h.publishAudit(ctx, ip, names...)
	return nil
}

// UpdateMetricsBatch stores a JSON array of model.Metrics in one storage call.
// The batch is validated first: a malformed, empty or invalid batch responds 400 and nothing is stored.
func (h *Handler) UpdateMetricsBatch(w http.ResponseWriter, r *http.Request) {
	var metrics []model.Metrics
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodySize)).Decode(&metrics); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid json")
		return
	}

	err := h.UpdateBatch(r.Context(), clientIP(r), metrics)
	switch {
	case errors.Is(err, ErrInvalidBatch):
		writeJSONError(w, http.StatusBadRequest, err.Error())
	case err != nil:
		log.Print(err)
		writeJSONError(w, http.StatusInternalServerError, "storage error")
	default:
		w.WriteHeader(http.StatusOK)
	}
}
