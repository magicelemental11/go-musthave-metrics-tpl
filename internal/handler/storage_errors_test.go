package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/repository"
)

type failingStorage struct{ err error }

func (f failingStorage) GetGauge(context.Context, string) (float64, error)  { return 0, f.err }
func (f failingStorage) GetCounter(context.Context, string) (int64, error)  { return 0, f.err }
func (f failingStorage) UpdateGauge(context.Context, string, float64) error { return f.err }
func (f failingStorage) UpdateCounter(context.Context, string, int64) (int64, error) {
	return 0, f.err
}
func (f failingStorage) GetAllMetrics(context.Context) (map[string]float64, map[string]int64, error) {
	return nil, nil, f.err
}

func TestStorageErrors(t *testing.T) {
	tests := []struct {
		name   string
		method string
		url    string
		body   string
		err    error
		want   int
	}{
		{"get gauge not found", http.MethodGet, "/value/gauge/x", "", repository.ErrNotFound, http.StatusNotFound},
		{"get gauge storage error", http.MethodGet, "/value/gauge/x", "", errors.New("db down"), http.StatusInternalServerError},
		{"get counter not found", http.MethodGet, "/value/counter/x", "", repository.ErrNotFound, http.StatusNotFound},
		{"get counter storage error", http.MethodGet, "/value/counter/x", "", errors.New("db down"), http.StatusInternalServerError},
		{"update gauge storage error", http.MethodPost, "/update/gauge/x/1", "", errors.New("db down"), http.StatusInternalServerError},
		{"update counter storage error", http.MethodPost, "/update/counter/x/1", "", errors.New("db down"), http.StatusInternalServerError},
		{"all metrics storage error", http.MethodGet, "/", "", errors.New("db down"), http.StatusInternalServerError},
		{"json value not found", http.MethodPost, "/value", `{"id":"x","type":"gauge"}`, repository.ErrNotFound, http.StatusNotFound},
		{"json value storage error", http.MethodPost, "/value", `{"id":"x","type":"counter"}`, errors.New("db down"), http.StatusInternalServerError},
		{"json update gauge storage error", http.MethodPost, "/update", `{"id":"x","type":"gauge","value":1}`, errors.New("db down"), http.StatusInternalServerError},
		{"json update counter storage error", http.MethodPost, "/update", `{"id":"x","type":"counter","delta":1}`, errors.New("db down"), http.StatusInternalServerError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewHandler(failingStorage{err: tt.err}, nil)
			r := chi.NewRouter()
			r.Post("/update/{type}/{name}/{value}", h.UpdateMetric)
			r.Get("/value/{type}/{name}", h.GetMetric)
			r.Post("/update", h.UpdateMetricJSON)
			r.Post("/value", h.GetMetricJSON)
			r.Get("/", h.GetAllMetrics)

			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, httptest.NewRequest(tt.method, tt.url, strings.NewReader(tt.body)))
			if rec.Code != tt.want {
				t.Fatalf("got %d, want %d (%s)", rec.Code, tt.want, rec.Body.String())
			}
		})
	}
}
