package agent

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"runtime"
	"sync"
	"time"

	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/model"
)

// requestTimeout limits a single request to the server.
const requestTimeout = 10 * time.Second

var httpClient = &http.Client{Timeout: requestTimeout}

// Agent collects runtime metrics and reports them to the server.
type Agent struct {
	mu        sync.Mutex
	pollCount int64
	stats     runtime.MemStats
	gauges    map[string]float64
}

// NewAgent creates an agent with no collected metrics.
func NewAgent() *Agent {
	return &Agent{
		gauges: make(map[string]float64),
	}
}

// CollectMetrics reads runtime statistics and increments PollCount.
func (a *Agent) CollectMetrics() {
	a.mu.Lock()
	defer a.mu.Unlock()
	runtime.ReadMemStats(&a.stats)
	a.pollCount++
	a.gauges["Alloc"] = float64(a.stats.Alloc)
	a.gauges["BuckHashSys"] = float64(a.stats.BuckHashSys)
	a.gauges["Frees"] = float64(a.stats.Frees)
	a.gauges["GCCPUFraction"] = a.stats.GCCPUFraction
	a.gauges["GCSys"] = float64(a.stats.GCSys)
	a.gauges["HeapAlloc"] = float64(a.stats.HeapAlloc)
	a.gauges["HeapIdle"] = float64(a.stats.HeapIdle)
	a.gauges["HeapInuse"] = float64(a.stats.HeapInuse)
	a.gauges["HeapObjects"] = float64(a.stats.HeapObjects)
	a.gauges["HeapReleased"] = float64(a.stats.HeapReleased)
	a.gauges["HeapSys"] = float64(a.stats.HeapSys)
	a.gauges["LastGC"] = float64(a.stats.LastGC)
	a.gauges["Lookups"] = float64(a.stats.Lookups)
	a.gauges["MCacheInuse"] = float64(a.stats.MCacheInuse)
	a.gauges["MCacheSys"] = float64(a.stats.MCacheSys)
	a.gauges["MSpanInuse"] = float64(a.stats.MSpanInuse)
	a.gauges["MSpanSys"] = float64(a.stats.MSpanSys)
	a.gauges["Mallocs"] = float64(a.stats.Mallocs)
	a.gauges["NextGC"] = float64(a.stats.NextGC)
	a.gauges["NumForcedGC"] = float64(a.stats.NumForcedGC)
	a.gauges["NumGC"] = float64(a.stats.NumGC)
	a.gauges["OtherSys"] = float64(a.stats.OtherSys)
	a.gauges["PauseTotalNs"] = float64(a.stats.PauseTotalNs)
	a.gauges["StackInuse"] = float64(a.stats.StackInuse)
	a.gauges["StackSys"] = float64(a.stats.StackSys)
	a.gauges["Sys"] = float64(a.stats.Sys)
	a.gauges["TotalAlloc"] = float64(a.stats.TotalAlloc)
	a.gauges["RandomValue"] = rand.Float64()
}

// snapshot returns a copy of the collected gauges and the current PollCount.
func (a *Agent) snapshot() (map[string]float64, int64) {
	a.mu.Lock()
	defer a.mu.Unlock()
	gauges := make(map[string]float64, len(a.gauges))
	for name, value := range a.gauges {
		gauges[name] = value
	}
	return gauges, a.pollCount
}

// ReportMetrics sends all collected metrics to the server at serverAddr.
// The lock is not held while sending, so collecting continues during a slow report.
func (a *Agent) ReportMetrics(serverAddr string) {
	gauges, pollCount := a.snapshot()
	url := serverAddr + "/update"

	for name, value := range gauges {
		status, err := sendMetric(url, model.Metrics{ID: name, MType: model.Gauge, Value: &value})
		if err != nil {
			log.Printf("error sending %s: %v", name, err)
			continue
		}
		log.Printf("sent gauge %s = %v → %d", name, value, status)
	}

	status, err := sendMetric(url, model.Metrics{ID: "PollCount", MType: model.Counter, Delta: &pollCount})
	if err != nil {
		log.Printf("error sending PollCount: %v", err)
		return
	}
	log.Printf("sent counter PollCount = %d → %d", pollCount, status)
	if status == http.StatusOK {
		// Subtract only what was sent: polls made during the report are kept.
		a.mu.Lock()
		a.pollCount -= pollCount
		a.mu.Unlock()
	}
}

func sendMetric(url string, m model.Metrics) (int, error) {
	body, err := json.Marshal(m)
	if err != nil {
		return 0, fmt.Errorf("marshal: %w", err)
	}
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write(body); err != nil {
		return 0, fmt.Errorf("gzip: %w", err)
	}
	if err := zw.Close(); err != nil {
		return 0, fmt.Errorf("gzip: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, url, &buf)
	if err != nil {
		return 0, fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "gzip")
	resp, err := httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("post: %w", err)
	}
	defer resp.Body.Close()
	return resp.StatusCode, nil
}
