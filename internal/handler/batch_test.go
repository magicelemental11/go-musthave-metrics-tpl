package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/repository"
)

func newBatchRouter(s repository.Storage) http.Handler {
	h := NewHandler(s, nil)
	r := chi.NewRouter()
	r.Post("/updates/", h.UpdateMetricsBatch)
	r.Get("/value/{type}/{name}", h.GetMetric)
	return r
}

func httpGet(r http.Handler, path string) *httptest.ResponseRecorder {
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, path, nil))
	return rr
}

func TestUpdateMetricsBatch(t *testing.T) {
	r := newBatchRouter(repository.NewMemStorage())

	rr := post(r, "/updates/", `[
		{"id":"g","type":"gauge","value":1.5},
		{"id":"c","type":"counter","delta":2},
		{"id":"c","type":"counter","delta":3}
	]`)
	if rr.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rr.Code, rr.Body)
	}

	for path, want := range map[string]string{"/value/gauge/g": "1.5", "/value/counter/c": "5"} {
		rr := httpGet(r, path)
		if rr.Code != http.StatusOK || rr.Body.String() != want {
			t.Errorf("%s: code=%d body=%q, want %q", path, rr.Code, rr.Body, want)
		}
	}
}

func TestUpdateMetricsBatchInvalid(t *testing.T) {
	tests := map[string]string{
		"empty batch":    `[]`,
		"not a list":     `{"id":"g","type":"gauge","value":1}`,
		"broken json":    `[{"id":`,
		"missing id":     `[{"type":"gauge","value":1}]`,
		"missing value":  `[{"id":"g","type":"gauge"}]`,
		"missing delta":  `[{"id":"c","type":"counter"}]`,
		"unknown type":   `[{"id":"x","type":"histogram","value":1}]`,
		"one bad of two": `[{"id":"g","type":"gauge","value":1},{"id":"c","type":"counter"}]`,
	}
	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			s := repository.NewMemStorage()
			rr := post(newBatchRouter(s), "/updates/", body)
			if rr.Code != http.StatusBadRequest {
				t.Fatalf("code=%d, want 400", rr.Code)
			}
			if gauges, counters, _ := s.GetAllMetrics(context.Background()); len(gauges)+len(counters) != 0 {
				t.Errorf("invalid batch must not change storage, got %v %v", gauges, counters)
			}
		})
	}
}

func TestBatchKeepsLegacyEndpoints(t *testing.T) {
	storage := repository.NewMemStorage()
	h := NewHandler(storage, nil)
	r := chi.NewRouter()
	r.Post("/update/{type}/{name}/{value}", h.UpdateMetric)
	r.Post("/update", h.UpdateMetricJSON)
	r.Post("/updates/", h.UpdateMetricsBatch)
	r.Get("/value/{type}/{name}", h.GetMetric)

	if rr := post(r, "/update/counter/c/1", ""); rr.Code != http.StatusOK {
		t.Fatalf("plain update: code=%d", rr.Code)
	}
	if rr := post(r, "/update", `{"id":"c","type":"counter","delta":2}`); rr.Code != http.StatusOK {
		t.Fatalf("json update: code=%d", rr.Code)
	}
	if rr := post(r, "/updates/", `[{"id":"c","type":"counter","delta":4}]`); rr.Code != http.StatusOK {
		t.Fatalf("batch update: code=%d", rr.Code)
	}
	if rr := httpGet(r, "/value/counter/c"); rr.Body.String() != "7" {
		t.Errorf("counter = %q, want 7", rr.Body)
	}
	if rr := post(r, "/update", `{"type":"gauge","value":1}`); rr.Code != http.StatusNotFound {
		t.Errorf("json update without id: code=%d, want 404", rr.Code)
	}
}
