package agent

import (
	"compress/gzip"
	"context"
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
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/model"
	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/sign"
	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/subnet"
)

func TestCollectMetrics(t *testing.T) {
	a := NewAgent("", 1)

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

	a := NewAgent("", 1)
	a.CollectMetrics()
	a.ReportMetrics(context.Background(), ts.URL)

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

	a := NewAgent("", 1)
	a.CollectMetrics()
	a.ReportMetrics(context.Background(), ts.URL)

	if a.pollCount != 1 {
		t.Errorf("expected pollCount to stay 1 after a failed report, got %d", a.pollCount)
	}
}

func TestReportMetricsSkipsEmptyBatch(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("unexpected request for an empty batch")
	}))
	defer ts.Close()

	NewAgent("", 1).ReportMetrics(context.Background(), ts.URL)
}

var shortDelays = []time.Duration{time.Millisecond, time.Millisecond, time.Millisecond}

func newTestAgent() *Agent {
	a := NewAgent("", 1)
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
	a.ReportMetrics(context.Background(), ts.URL)

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
	a.ReportMetrics(context.Background(), ts.URL)

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
			a.ReportMetrics(context.Background(), ts.URL)

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
	a.ReportMetrics(context.Background(), "http://localhost:1")

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
		a := NewAgent(key, 1)
		a.CollectMetrics()
		a.ReportMetrics(context.Background(), ts.URL)
		ts.Close()

		if key == "" && header != "" {
			t.Errorf("header must be absent without key, got %q", header)
		}
		if key != "" && !ok {
			t.Errorf("header %q does not match body", header)
		}
	}
}

func TestRunPoolLimitsConcurrency(t *testing.T) {
	var current, peak, done atomic.Int32
	runPool(3, 12, func(int) {
		n := current.Add(1)
		for {
			p := peak.Load()
			if n <= p || peak.CompareAndSwap(p, n) {
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
		current.Add(-1)
		done.Add(1)
	})
	if done.Load() != 12 {
		t.Fatalf("ran %d jobs, want 12", done.Load())
	}
	if peak.Load() > 3 {
		t.Fatalf("peak concurrency %d, want at most 3", peak.Load())
	}
}

func TestReportMetricsSplitsAcrossRateLimit(t *testing.T) {
	var requests, total, polls atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		zr, err := gzip.NewReader(r.Body)
		if err != nil {
			t.Errorf("gzip: %v", err)
			return
		}
		var batch []model.Metrics
		if err := json.NewDecoder(zr).Decode(&batch); err != nil {
			t.Errorf("json: %v", err)
		}
		requests.Add(1)
		total.Add(int32(len(batch)))
		for _, m := range batch {
			if m.ID == "PollCount" {
				polls.Add(1)
			}
		}
	}))
	defer ts.Close()

	a := NewAgent("", 3)
	a.CollectMetrics()
	gauges, _ := a.snapshot()
	a.ReportMetrics(context.Background(), ts.URL)

	if requests.Load() != 3 || int(total.Load()) != len(gauges)+1 || polls.Load() != 1 {
		t.Fatalf("requests=%d metrics=%d pollCount sent=%d", requests.Load(), total.Load(), polls.Load())
	}
	if a.pollCount != 0 {
		t.Errorf("pollCount not reset: %d", a.pollCount)
	}
}

func TestCollectSystemMetrics(t *testing.T) {
	a := NewAgent("", 1)
	if err := a.CollectSystemMetrics(); err != nil {
		t.Fatal(err)
	}
	gauges, _ := a.snapshot()
	for _, name := range []string{"TotalMemory", "FreeMemory", "CPUutilization1"} {
		if _, ok := gauges[name]; !ok {
			t.Errorf("missing %s", name)
		}
	}
	if gauges["TotalMemory"] <= 0 {
		t.Errorf("TotalMemory = %v", gauges["TotalMemory"])
	}
}

func TestReportMetricsLimitsConcurrentRequests(t *testing.T) {
	var current, peak, requests atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := current.Add(1)
		for {
			p := peak.Load()
			if n <= p || peak.CompareAndSwap(p, n) {
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
		current.Add(-1)
		requests.Add(1)
	}))
	defer ts.Close()

	a := NewAgent("", 2)
	a.CollectMetrics()
	a.ReportMetrics(context.Background(), ts.URL)

	if requests.Load() != 2 || peak.Load() > 2 {
		t.Fatalf("requests=%d peak=%d, want 2 requests and peak <= 2", requests.Load(), peak.Load())
	}
}

func TestReportMetricsIncludesSystemMetrics(t *testing.T) {
	seen := make(map[string]bool)
	var mu sync.Mutex
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		zr, err := gzip.NewReader(r.Body)
		if err != nil {
			t.Errorf("gzip: %v", err)
			return
		}
		var batch []model.Metrics
		if err := json.NewDecoder(zr).Decode(&batch); err != nil {
			t.Errorf("json: %v", err)
		}
		mu.Lock()
		defer mu.Unlock()
		for _, m := range batch {
			seen[m.ID] = true
		}
	}))
	defer ts.Close()

	a := NewAgent("", 1)
	a.CollectMetrics()
	if err := a.CollectSystemMetrics(); err != nil {
		t.Fatal(err)
	}
	a.ReportMetrics(context.Background(), ts.URL)

	for _, name := range []string{"TotalMemory", "FreeMemory", "CPUutilization1"} {
		if !seen[name] {
			t.Errorf("%s was not sent", name)
		}
	}
}

func TestReportMetricsSendsRealIP(t *testing.T) {
	tests := []struct {
		name string
		ip   string
	}{
		{"header set", "10.1.2.3"},
		{"empty omits header", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got []string
			var mu sync.Mutex
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				got = append(got, r.Header.Get(subnet.Header))
				mu.Unlock()
			}))
			a := NewAgent("", 1).WithRealIP(tt.ip)
			a.CollectMetrics()
			a.ReportMetrics(context.Background(), ts.URL)
			ts.Close()

			if len(got) == 0 {
				t.Fatal("no requests")
			}
			for _, h := range got {
				if h != tt.ip {
					t.Errorf("%s = %q, want %q", subnet.Header, h, tt.ip)
				}
			}
		})
	}
}

func TestHostIP(t *testing.T) {
	tests := []struct {
		name    string
		addr    string
		want    string
		wantErr bool
	}{
		{"ipv4 loopback", "127.0.0.1:8080", "127.0.0.1", false},
		// localhost resolves to both ::1 and 127.0.0.1; a trusted 127.0.0.0/8 must keep working.
		{"localhost prefers ipv4", "localhost:8080", "127.0.0.1", false},
		{"no port", "no-port", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip, err := HostIP(tt.addr)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("want error, got %q", ip)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if ip != tt.want {
				t.Fatalf("HostIP(%q) = %q, want %q", tt.addr, ip, tt.want)
			}
		})
	}
}

func TestHostIPFallsBackToIPv6(t *testing.T) {
	ip, err := HostIP("[::1]:8080")
	if err != nil {
		t.Skipf("no IPv6 loopback: %v", err)
	}
	if ip != "::1" {
		t.Fatalf("HostIP = %q, want ::1", ip)
	}
}
