package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/repository"
)

type fakePinger struct{ err error }

func (f fakePinger) PingContext(context.Context) error { return f.err }

func TestPing(t *testing.T) {
	tests := []struct {
		name   string
		pinger Pinger
		want   int
	}{
		{"ok", fakePinger{}, http.StatusOK},
		{"db error", fakePinger{err: errors.New("down")}, http.StatusInternalServerError},
		{"no database", nil, http.StatusInternalServerError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewHandler(repository.NewMemStorage(), tt.pinger)
			rec := httptest.NewRecorder()
			h.Ping(rec, httptest.NewRequest(http.MethodGet, "/ping", nil))
			if rec.Code != tt.want {
				t.Fatalf("got %d, want %d", rec.Code, tt.want)
			}
		})
	}
}
