package agent

import (
	"net/http"
	"net/http/httptest"
	"testing"
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

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	a := NewAgent()
	a.CollectMetrics()
	a.ReportMetrics(ts.URL)

	if requestCount != 29 {
		t.Errorf("Expected 29 requests, got %d", requestCount)
	}

}
