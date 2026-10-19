package handler

import (
	"context"
	"net/http"
	"time"
)

const pingTimeout = 3 * time.Second

// Pinger проверяет доступность БД; его реализует *sql.DB.
type Pinger interface {
	PingContext(ctx context.Context) error
}

// Ping отвечает 200, если БД доступна, иначе 500.
func (h *Handler) Ping(w http.ResponseWriter, r *http.Request) {
	if h.pinger == nil {
		http.Error(w, "database is not configured", http.StatusInternalServerError)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), pingTimeout)
	defer cancel()
	if err := h.pinger.PingContext(ctx); err != nil {
		http.Error(w, "database is unavailable", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}
