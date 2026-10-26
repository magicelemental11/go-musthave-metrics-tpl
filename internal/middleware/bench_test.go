package middleware

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/handler"
	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/model"
	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/repository"
	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/sign"
)

func seed(b *testing.B, r http.Handler) {
	b.Helper()
	body, _ := json.Marshal(benchMetrics(30))
	req := httptest.NewRequest(http.MethodPost, "/updates/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		b.Fatalf("seed: code=%d body=%s", rr.Code, rr.Body)
	}
}

func benchRouter(key string) http.Handler {
	h := handler.NewHandler(repository.NewMemStorage(), nil)
	r := chi.NewRouter()
	r.Use(Sign(key))
	r.Use(Gzip)
	r.Post("/updates/", h.UpdateMetricsBatch)
	r.Post("/update/{type}/{name}/{value}", h.UpdateMetric)
	r.Post("/update", h.UpdateMetricJSON)
	r.Post("/value", h.GetMetricJSON)
	r.Get("/", h.GetAllMetrics)
	return r
}

func benchMetrics(n int) []model.Metrics {
	batch := make([]model.Metrics, 0, n)
	for i := 0; i < n; i++ {
		v, d := float64(i)*1.5, int64(i)
		if i%5 == 0 {
			batch = append(batch, model.Metrics{ID: fmt.Sprintf("counter%d", i), MType: model.Counter, Delta: &d})
		} else {
			batch = append(batch, model.Metrics{ID: fmt.Sprintf("gauge%d", i), MType: model.Gauge, Value: &v})
		}
	}
	return batch
}

func gzipBytes(b *testing.B, data []byte) []byte {
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write(data); err != nil {
		b.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		b.Fatal(err)
	}
	return buf.Bytes()
}

func serve(b *testing.B, r http.Handler, req func() *http.Request) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req())
		if rr.Code != http.StatusOK {
			b.Fatalf("code=%d body=%s", rr.Code, rr.Body)
		}
	}
}

func BenchmarkUpdateMetricURL(b *testing.B) {
	log.SetOutput(io.Discard)
	defer log.SetOutput(os.Stderr)
	serve(b, benchRouter(""), func() *http.Request {
		return httptest.NewRequest(http.MethodPost, "/update/gauge/Alloc/1.5", nil)
	})
}

func BenchmarkUpdatesBatchPlain(b *testing.B) {
	body, _ := json.Marshal(benchMetrics(30))
	serve(b, benchRouter(""), func() *http.Request {
		req := httptest.NewRequest(http.MethodPost, "/updates/", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		return req
	})
}

func BenchmarkUpdatesBatchGzip(b *testing.B) {
	body, _ := json.Marshal(benchMetrics(30))
	zbody := gzipBytes(b, body)
	serve(b, benchRouter(""), func() *http.Request {
		req := httptest.NewRequest(http.MethodPost, "/updates/", bytes.NewReader(zbody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Content-Encoding", "gzip")
		return req
	})
}

func BenchmarkUpdatesBatchGzipSigned(b *testing.B) {
	const key = "bench-key"
	body, _ := json.Marshal(benchMetrics(30))
	zbody := gzipBytes(b, body)
	hash := sign.Sign(key, zbody)
	serve(b, benchRouter(key), func() *http.Request {
		req := httptest.NewRequest(http.MethodPost, "/updates/", bytes.NewReader(zbody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Content-Encoding", "gzip")
		req.Header.Set(sign.Header, hash)
		return req
	})
}

func BenchmarkValueJSONGzipResponse(b *testing.B) {
	r := benchRouter("")
	seed(b, r)
	query := []byte(`{"id":"gauge1","type":"gauge"}`)
	serve(b, r, func() *http.Request {
		req := httptest.NewRequest(http.MethodPost, "/value", bytes.NewReader(query))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept-Encoding", "gzip")
		return req
	})
}

func BenchmarkIndexPage(b *testing.B) {
	r := benchRouter("")
	seed(b, r)
	serve(b, r, func() *http.Request {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		return req
	})
}
