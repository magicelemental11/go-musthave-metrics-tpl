package agent

import (
	"compress/gzip"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/model"
	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/sign"
)

func TestCollectMetrics(t *testing.T) {
	a := NewAgent("")

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

	a := NewAgent("")
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

	a := NewAgent("")
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

	NewAgent("").ReportMetrics(ts.URL)
}

var shortDelays = []time.Duration{time.Millisecond, time.Millisecond, time.Millisecond}

func newTestAgent() *Agent {
	a := NewAgent("")
	a.delays = shortDelays
	a.CollectMetrics()
	return a
}

func dropConnection(t *testing.T, w http.ResponseWriter) {
	t.Helper()
	conn, _, err := w.(http.Hijacker).Hijack()
	if err != nil {
		t.Errorf("hijack: %v", err)
		return
	}
	conn.Close()
}

func TestReportMetricsRetriesConnectionErrors(t *testing.T) {
	var requests atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if requests.Add(1) <= 2 {
			dropConnection(t, w)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	a := newTestAgent()
	a.ReportMetrics(ts.URL)

	if requests.Load() != 3 {
		t.Errorf("expected 3 requests, got %d", requests.Load())
	}
	if a.pollCount != 0 {
		t.Errorf("expected pollCount reset after eventual success, got %d", a.pollCount)
	}
}

func TestReportMetricsGivesUpAfterThreeRetries(t *testing.T) {
	var requests atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		dropConnection(t, w)
	}))
	defer ts.Close()

	a := newTestAgent()
	a.ReportMetrics(ts.URL)

	if requests.Load() != 4 {
		t.Errorf("expected 4 requests (1 + 3 retries), got %d", requests.Load())
	}
	if a.pollCount != 1 {
		t.Errorf("expected pollCount kept after failure, got %d", a.pollCount)
	}
}

func TestReportMetricsDoesNotRetryHTTPErrors(t *testing.T) {
	for _, status := range []int{http.StatusBadRequest, http.StatusInternalServerError} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			var requests atomic.Int32
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				w.WriteHeader(status)
			}))
			defer ts.Close()

			a := newTestAgent()
			a.ReportMetrics(ts.URL)

			if requests.Load() != 1 {
				t.Errorf("expected 1 request, got %d", requests.Load())
			}
			if a.pollCount != 1 {
				t.Errorf("expected pollCount kept, got %d", a.pollCount)
			}
		})
	}
}

type countingTransport struct {
	calls atomic.Int32
	err   error
}

func (c *countingTransport) RoundTrip(*http.Request) (*http.Response, error) {
	c.calls.Add(1)
	return nil, c.err
}

func TestReportMetricsRetriesWhenServerIsDown(t *testing.T) {
	transport := &countingTransport{err: &net.OpError{Op: "dial", Err: os.NewSyscallError("connect", syscall.ECONNREFUSED)}}
	original := httpClient
	httpClient = &http.Client{Transport: transport}
	defer func() { httpClient = original }()

	a := newTestAgent()
	a.ReportMetrics("http://localhost:1")

	if transport.calls.Load() != 4 {
		t.Errorf("expected 4 attempts (1 + 3 retries), got %d", transport.calls.Load())
	}
	if a.pollCount != 1 {
		t.Errorf("expected pollCount kept, got %d", a.pollCount)
	}
}

type timeoutErr struct{}

func (timeoutErr) Error() string   { return "timeout" }
func (timeoutErr) Timeout() bool   { return true }
func (timeoutErr) Temporary() bool { return true }

func TestIsTransientNetErr(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"connection refused", &url.Error{Op: "Post", Err: &net.OpError{Op: "dial", Err: os.NewSyscallError("connect", syscall.ECONNREFUSED)}}, true},
		{"connection reset", fmt.Errorf("post: %w", syscall.ECONNRESET), true},
		{"dropped connection", &url.Error{Op: "Post", Err: io.EOF}, true},
		{"unexpected eof", fmt.Errorf("post: %w", io.ErrUnexpectedEOF), true},
		{"timeout", &url.Error{Op: "Post", Err: timeoutErr{}}, true},
		{"unsupported scheme", &url.Error{Op: "Post", Err: errors.New(`unsupported protocol scheme "localhost"`)}, false},
		{"tls certificate", &url.Error{Op: "Post", Err: x509.UnknownAuthorityError{}}, false},
		{"plain error", errors.New("boom"), false},
		{"nil", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isTransientNetErr(tt.err); got != tt.want {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestReportMetricsSignsBody(t *testing.T) {
	for _, key := range []string{"secret", ""} {
		var header string
		var ok bool
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			header = r.Header.Get(sign.Header)
			ok = sign.Valid(key, body, header)
		}))
		a := NewAgent(key)
		a.CollectMetrics()
		a.ReportMetrics(ts.URL)
		ts.Close()

		if key == "" && header != "" {
			t.Errorf("header must be absent without key, got %q", header)
		}
		if key != "" && !ok {
			t.Errorf("header %q does not match body", header)
		}
	}
}
