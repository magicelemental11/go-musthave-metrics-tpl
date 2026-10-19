package agent

import (
	"compress/gzip"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/model"
)

func TestCollectMetrics(t *testing.T) {
	a := NewAgent()

	a.CollectMetrics()

	if a.pollCount != 1 {
		t.Errorf("Expected pollCount to be 1, got %d", a.pollCount)
	}

	if a.gauges["Alloc"] == 0 {
		t.Errorf("Expected gauge 'Alloc' to be non-zero, got 0")
	}

	a.CollectMetrics()

	if a.pollCount != 2 {
		t.Errorf("Expected pollCount to be 2, got %d", a.pollCount)
	}

}

func TestReportMetrics(t *testing.T) {
	var requestCount int
	var batch []model.Metrics

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if r.URL.Path != "/updates/" || r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("unexpected request: %s, content-type %q", r.URL.Path, r.Header.Get("Content-Type"))
		}
		if r.Header.Get("Content-Encoding") != "gzip" {
			t.Errorf("request body is not gzip-encoded")
		}
		zr, err := gzip.NewReader(r.Body)
		if err != nil {
			t.Errorf("invalid gzip body: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if err := json.NewDecoder(zr).Decode(&batch); err != nil {
			t.Errorf("invalid json body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	a := NewAgent()
	a.CollectMetrics()
	a.ReportMetrics(ts.URL)

	if requestCount != 1 {
		t.Fatalf("expected 1 request, got %d", requestCount)
	}
	if len(batch) != 29 {
		t.Errorf("expected 29 metrics in batch, got %d", len(batch))
	}
	if a.pollCount != 0 {
		t.Errorf("expected pollCount to be reset after a successful report, got %d", a.pollCount)
	}
}

func TestReportMetricsKeepsPollCountOnFailure(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	a := NewAgent()
	a.CollectMetrics()
	a.ReportMetrics(ts.URL)

	if a.pollCount != 1 {
		t.Errorf("expected pollCount to stay 1 after a failed report, got %d", a.pollCount)
	}
}

func TestReportMetricsSkipsEmptyBatch(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("unexpected request for an empty batch")
	}))
	defer ts.Close()

	NewAgent().ReportMetrics(ts.URL)
}
