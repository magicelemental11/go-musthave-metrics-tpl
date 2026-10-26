package agent

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/handler"
	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/middleware"
	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/repository"
)

func TestReportMetricsEncrypted(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	storage := repository.NewMemStorage()
	h := handler.NewHandler(storage, nil)

	var wire []byte
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			body, _ := io.ReadAll(req.Body)
			wire = body
			req.Body = io.NopCloser(bytes.NewReader(body))
			next.ServeHTTP(w, req)
		})
	})
	r.Use(middleware.Decrypt(key))
	r.Use(middleware.Sign("secret"))
	r.Use(middleware.Gzip)
	r.Post("/updates/", h.UpdateMetricsBatch)
	srv := httptest.NewServer(r)
	defer srv.Close()

	a := NewAgent("secret", 1).WithPublicKey(&key.PublicKey)
	a.CollectMetrics()
	a.ReportMetrics(context.Background(), srv.URL)

	if got, err := storage.GetCounter(context.Background(), "PollCount"); err != nil || got != 1 {
		t.Fatalf("PollCount = %d, %v", got, err)
	}
	if got, err := storage.GetGauge(context.Background(), "Alloc"); err != nil || got == 0 {
		t.Fatalf("Alloc = %v, %v", got, err)
	}
	if bytes.Contains(wire, []byte("PollCount")) {
		t.Fatal("request body is not encrypted")
	}
}
