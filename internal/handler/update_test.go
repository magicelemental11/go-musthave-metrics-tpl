package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/repository"
)

func TestUpdateMetric(t *testing.T) {
	storage := repository.NewMemStorage()
	h := NewHandler(storage, nil)

	r := chi.NewRouter()
	r.Post("/update/{type}/{name}/{value}", h.UpdateMetric)

	req := httptest.NewRequest("POST", "/update/gauge/Alloc/1.5", nil)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status code 200, got %d", rr.Code)
	}
}

func TestUpdateMetricInvalidType(t *testing.T) {
	storage := repository.NewMemStorage()
	h := NewHandler(storage, nil)

	r := chi.NewRouter()
	r.Post("/update/{type}/{name}/{value}", h.UpdateMetric)

	req := httptest.NewRequest("POST", "/update/unknown/Alloc/1.5", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status code 400, got %d", rr.Code)
	}

}

func TestUpdateMetricInvalidValue(t *testing.T) {
	storage := repository.NewMemStorage()
	h := NewHandler(storage, nil)

	r := chi.NewRouter()
	r.Post("/update/{type}/{name}/{value}", h.UpdateMetric)

	req := httptest.NewRequest("POST", "/update/gauge/Alloc/invalid", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status code 400, got %d", rr.Code)
	}
}

func TestUpdateCounter(t *testing.T) {
	storage := repository.NewMemStorage()
	h := NewHandler(storage, nil)

	r := chi.NewRouter()
	r.Post("/update/{type}/{name}/{value}", h.UpdateMetric)

	req := httptest.NewRequest("POST", "/update/counter/PollCount/1", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status code 200, got %d", rr.Code)
	}
}
