package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/model"
	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/repository"
)

func newJSONRouter() http.Handler {
	h := NewHandler(repository.NewMemStorage(), nil)
	r := chi.NewRouter()
	r.Post("/update", h.UpdateMetricJSON)
	r.Post("/value", h.GetMetricJSON)
	return r
}

func post(r http.Handler, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	return rr
}

func TestUpdateAndValueJSON(t *testing.T) {
	r := newJSONRouter()

	rr := post(r, "/update", `{"id":"LastGC","type":"gauge","value":1744184459}`)
	if rr.Code != http.StatusOK || rr.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("update gauge: code=%d ct=%q", rr.Code, rr.Header().Get("Content-Type"))
	}

	rr = post(r, "/value", `{"id":"LastGC","type":"gauge"}`)
	var m model.Metrics
	if err := json.Unmarshal(rr.Body.Bytes(), &m); err != nil {
		t.Fatal(err)
	}
	if rr.Code != http.StatusOK || m.Value == nil || *m.Value != 1744184459 {
		t.Fatalf("value gauge: code=%d body=%s", rr.Code, rr.Body)
	}
}

func TestCounterAccumulatesJSON(t *testing.T) {
	r := newJSONRouter()
	post(r, "/update", `{"id":"PollCount","type":"counter","delta":5}`)
	rr := post(r, "/update", `{"id":"PollCount","type":"counter","delta":3}`)

	var m model.Metrics
	if err := json.Unmarshal(rr.Body.Bytes(), &m); err != nil {
		t.Fatal(err)
	}
	if m.Delta == nil || *m.Delta != 8 {
		t.Fatalf("expected delta 8, got body %s", rr.Body)
	}

	rr = post(r, "/value", `{"id":"PollCount","type":"counter"}`)
	if err := json.Unmarshal(rr.Body.Bytes(), &m); err != nil || m.Delta == nil || *m.Delta != 8 {
		t.Fatalf("value counter: body %s", rr.Body)
	}
}

func TestJSONErrors(t *testing.T) {
	r := newJSONRouter()
	tests := []struct {
		name, path, body string
		want             int
	}{
		{"invalid json", "/update", `{`, http.StatusBadRequest},
		{"unknown type", "/update", `{"id":"a","type":"x","value":1}`, http.StatusBadRequest},
		{"gauge without value", "/update", `{"id":"a","type":"gauge"}`, http.StatusBadRequest},
		{"counter without delta", "/update", `{"id":"a","type":"counter"}`, http.StatusBadRequest},
		{"no id", "/update", `{"type":"gauge","value":1}`, http.StatusNotFound},
		{"value not found", "/value", `{"id":"nope","type":"gauge"}`, http.StatusNotFound},
		{"value unknown type", "/value", `{"id":"a","type":"x"}`, http.StatusBadRequest},
		{"value invalid json", "/value", `nope`, http.StatusBadRequest},
		{"body too large", "/update", `{"id":"` + strings.Repeat("a", 2<<20) + `"}`, http.StatusBadRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rr := post(r, tt.path, tt.body)
			if rr.Code != tt.want {
				t.Errorf("got %d, want %d", rr.Code, tt.want)
			}
			if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
				t.Errorf("Content-Type = %q, want application/json", ct)
			}
		})
	}
}
