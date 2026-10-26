package handler

import (
	"context"
	"net"
	"net/http"
	"time"

	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/audit"
)

// Publisher receives an audit.Event for every successful update.
type Publisher interface {
	Publish(ctx context.Context, e audit.Event)
}

// WithAudit makes h publish an audit event after each successful update and returns h.
func (h *Handler) WithAudit(p Publisher) *Handler {
	h.audit = p
	return h
}

func (h *Handler) publishAudit(r *http.Request, metrics ...string) {
	if h.audit == nil {
		return
	}
	h.audit.Publish(context.WithoutCancel(r.Context()), audit.Event{
		TS:        time.Now().Unix(),
		Metrics:   metrics,
		IPAddress: clientIP(r),
	})
}

// clientIP prefers X-Real-IP, which a reverse proxy is expected to set, and falls back to the host of RemoteAddr.
// The header is not authenticated, so audit addresses are only as trustworthy as the proxy in front of the server.
func clientIP(r *http.Request) string {
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
