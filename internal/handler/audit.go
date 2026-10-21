package handler

import (
	"context"
	"net"
	"net/http"
	"time"

	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/audit"
)

type Publisher interface {
	Publish(ctx context.Context, e audit.Event)
}

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
