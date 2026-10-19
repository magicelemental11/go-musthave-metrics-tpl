package middleware

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/handler"
	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/model"
	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/repository"
)

func gzipped(t *testing.T, s string) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write([]byte(s)); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return &buf
}

func echoHandler(t *testing.T, contentType string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
		}
		w.Header().Set("Content-Type", contentType)
		if _, err := w.Write(body); err != nil {
			t.Error(err)
		}
	})
}

func TestGzipCompressesResponse(t *testing.T) {
	for _, ct := range []string{"application/json", "text/html"} {
		req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{"a":1}`))
		req.Header.Set("Accept-Encoding", "gzip")
		rr := httptest.NewRecorder()
		Gzip(echoHandler(t, ct)).ServeHTTP(rr, req)

		if rr.Header().Get("Content-Encoding") != "gzip" {
			t.Fatalf("%s: response is not gzip-encoded", ct)
		}
		zr, err := gzip.NewReader(rr.Body)
		if err != nil {
			t.Fatal(err)
		}
		got, err := io.ReadAll(zr)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != `{"a":1}` {
			t.Errorf("%s: unexpected body %q", ct, got)
		}
	}
}

func TestGzipSkipsOtherContentTypesAndClientsWithoutSupport(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString("hi"))
	req.Header.Set("Accept-Encoding", "gzip")
	rr := httptest.NewRecorder()
	Gzip(echoHandler(t, "text/plain")).ServeHTTP(rr, req)
	if rr.Header().Get("Content-Encoding") != "" || rr.Body.String() != "hi" {
		t.Errorf("text/plain must not be compressed")
	}

	req = httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString("hi"))
	rr = httptest.NewRecorder()
	Gzip(echoHandler(t, "application/json")).ServeHTTP(rr, req)
	if rr.Header().Get("Content-Encoding") != "" || rr.Body.String() != "hi" {
		t.Errorf("response must be plain without Accept-Encoding")
	}
}

func TestGzipDecompressesRequest(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", gzipped(t, `{"id":"x"}`))
	req.Header.Set("Content-Encoding", "gzip")
	rr := httptest.NewRecorder()
	Gzip(echoHandler(t, "application/json")).ServeHTTP(rr, req)
	if rr.Body.String() != `{"id":"x"}` {
		t.Errorf("unexpected body %q", rr.Body.String())
	}
}

func TestGzipInvalidRequestBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString("not gzip"))
	req.Header.Set("Content-Encoding", "gzip")
	rr := httptest.NewRecorder()
	Gzip(echoHandler(t, "application/json")).ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("got %d, want 400", rr.Code)
	}
}

func TestGzipEmptyRequestBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", http.NoBody)
	req.Header.Set("Content-Encoding", "gzip")
	rr := httptest.NewRecorder()
	Gzip(echoHandler(t, "application/json")).ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("got %d, want 200", rr.Code)
	}
}

// Сжатый запрос и сжатый ответ через настоящий JSON-хендлер.
func TestGzipWithJSONHandler(t *testing.T) {
	h := handler.NewHandler(repository.NewMemStorage(), nil)
	r := chi.NewRouter()
	r.Use(Gzip)
	r.Post("/update", h.UpdateMetricJSON)

	req := httptest.NewRequest(http.MethodPost, "/update", gzipped(t, `{"id":"PollCount","type":"counter","delta":4}`))
	req.Header.Set("Content-Encoding", "gzip")
	req.Header.Set("Accept-Encoding", "gzip")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK || rr.Header().Get("Content-Encoding") != "gzip" {
		t.Fatalf("code=%d encoding=%q", rr.Code, rr.Header().Get("Content-Encoding"))
	}
	zr, err := gzip.NewReader(rr.Body)
	if err != nil {
		t.Fatal(err)
	}
	var m model.Metrics
	if err := json.NewDecoder(zr).Decode(&m); err != nil {
		t.Fatal(err)
	}
	if m.Delta == nil || *m.Delta != 4 {
		t.Errorf("unexpected metric: %+v", m)
	}
}

func TestGzipWithBatchHandler(t *testing.T) {
	storage := repository.NewMemStorage()
	h := handler.NewHandler(storage, nil)
	r := chi.NewRouter()
	r.Use(Gzip)
	r.Post("/updates/", h.UpdateMetricsBatch)

	body := `[{"id":"g","type":"gauge","value":1.5},{"id":"c","type":"counter","delta":2},{"id":"c","type":"counter","delta":3}]`
	req := httptest.NewRequest(http.MethodPost, "/updates/", gzipped(t, body))
	req.Header.Set("Content-Encoding", "gzip")
	req.Header.Set("Accept-Encoding", "gzip")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rr.Code, rr.Body)
	}
	if v, err := storage.GetCounter(context.Background(), "c"); err != nil || v != 5 {
		t.Errorf("counter = %v, %v", v, err)
	}
	if v, err := storage.GetGauge(context.Background(), "g"); err != nil || v != 1.5 {
		t.Errorf("gauge = %v, %v", v, err)
	}
}
