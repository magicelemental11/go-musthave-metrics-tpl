// Package agent collects runtime and system metrics and reports them to the server.
package agent

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"net/http"
	"runtime"
	"sync"
	"syscall"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/mem"

	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/model"
	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/retry"
	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/rsacrypt"
	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/sign"
	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/subnet"
)

const requestTimeout = 10 * time.Second

var (
	httpClient = &http.Client{Timeout: requestTimeout}
	gzipPool   = sync.Pool{New: func() any { return gzip.NewWriter(io.Discard) }}
)

// Agent collects metrics and sends them to the server. It is safe for concurrent use.
type Agent struct {
	mu        sync.Mutex
	pollCount int64
	stats     runtime.MemStats
	gauges    map[string]float64
	delays    []time.Duration
	key       string
	pubKey    *rsa.PublicKey
	realIP    string
	rateLimit int
}

// NewAgent creates an Agent. key signs requests (empty disables signing);
// rateLimit bounds the number of concurrent requests to the server.
func NewAgent(key string, rateLimit int) *Agent {
	return &Agent{
		key:       key,
		rateLimit: rateLimit,
		gauges:    make(map[string]float64),
		delays:    retry.DefaultDelays,
	}
}

func (a *Agent) WithPublicKey(pub *rsa.PublicKey) *Agent {
	a.pubKey = pub
	return a
}

// WithRealIP sets the IP sent in the X-Real-IP header; empty omits the header. It returns a.
func (a *Agent) WithRealIP(ip string) *Agent {
	a.realIP = ip
	return a
}

// HostIP returns the local IP address the host uses to reach serverAddr (host:port).
// Dialing UDP sends no packets: it only picks the route and the source address.
// IPv4 is tried first, so "localhost" gives 127.0.0.1 rather than ::1.
func HostIP(serverAddr string) (string, error) {
	conn, err := net.Dial("udp4", serverAddr)
	if err != nil {
		conn, err = net.Dial("udp", serverAddr)
	}
	if err != nil {
		return "", fmt.Errorf("host ip: %w", err)
	}
	defer conn.Close()
	return conn.LocalAddr().(*net.UDPAddr).IP.String(), nil
}

// CollectMetrics reads runtime.MemStats and increments PollCount.
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

func (a *Agent) snapshot() (map[string]float64, int64) {
	a.mu.Lock()
	defer a.mu.Unlock()
	gauges := make(map[string]float64, len(a.gauges))
	for name, value := range a.gauges {
		gauges[name] = value
	}
	return gauges, a.pollCount
}

// CollectSystemMetrics reads total and free memory and per-CPU utilization via gopsutil.
func (a *Agent) CollectSystemMetrics() error {
	vm, err := mem.VirtualMemory()
	if err != nil {
		return fmt.Errorf("virtual memory: %w", err)
	}
	cpus, err := cpu.Percent(0, true)
	if err != nil {
		return fmt.Errorf("cpu percent: %w", err)
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	a.gauges["TotalMemory"] = float64(vm.Total)
	a.gauges["FreeMemory"] = float64(vm.Free)
	for i, percent := range cpus {
		a.gauges[fmt.Sprintf("CPUutilization%d", i+1)] = percent
	}
	return nil
}

// ReportMetrics sends the collected metrics to serverAddr in batches through a bounded worker pool.
// The mutex is not held while sending, so collection is not blocked by a slow server.
func (a *Agent) ReportMetrics(ctx context.Context, serverAddr string) {
	gauges, pollCount := a.snapshot()

	batch := make([]model.Metrics, 0, len(gauges)+1)
	if pollCount > 0 {
		batch = append(batch, model.Metrics{ID: "PollCount", MType: model.Counter, Delta: &pollCount})
	}
	for name, value := range gauges {
		batch = append(batch, model.Metrics{ID: name, MType: model.Gauge, Value: &value})
	}
	if len(batch) == 0 {
		return
	}

	chunks := splitBatch(batch, a.rateLimit)
	runPool(a.rateLimit, len(chunks), func(i int) {
		var status int
		err := retry.Do(ctx, a.delays, func() (err error) {
			status, err = a.sendBatch(ctx, serverAddr+"/updates/", chunks[i])
			return err
		}, isTransientNetErr)
		if err != nil {
			log.Printf("error sending batch of %d metrics: %v", len(chunks[i]), err)
			return
		}
		log.Printf("sent batch of %d metrics → %d", len(chunks[i]), status)
		if i == 0 && status == http.StatusOK && pollCount > 0 {
			// Subtract only what was sent: polls made during sending are not lost.
			a.mu.Lock()
			a.pollCount -= pollCount
			a.mu.Unlock()
		}
	})
}

func splitBatch(batch []model.Metrics, parts int) [][]model.Metrics {
	size := (len(batch) + parts - 1) / parts
	chunks := make([][]model.Metrics, 0, parts)
	for start := 0; start < len(batch); start += size {
		chunks = append(chunks, batch[start:min(start+size, len(batch))])
	}
	return chunks
}

func runPool(limit, n int, job func(i int)) {
	jobs := make(chan int)
	var wg sync.WaitGroup
	for range min(limit, n) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				job(i)
			}
		}()
	}
	for i := range n {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
}

func isTransientNetErr(err error) bool {
	if errors.Is(err, syscall.ECONNREFUSED) || errors.Is(err, syscall.ECONNRESET) ||
		errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

func (a *Agent) sendBatch(ctx context.Context, url string, batch []model.Metrics) (int, error) {
	body, err := json.Marshal(batch)
	if err != nil {
		return 0, fmt.Errorf("marshal: %w", err)
	}
	var buf bytes.Buffer
	zw := gzipPool.Get().(*gzip.Writer)
	defer gzipPool.Put(zw)
	zw.Reset(&buf)
	if _, err = zw.Write(body); err != nil {
		return 0, fmt.Errorf("gzip: %w", err)
	}
	if err = zw.Close(); err != nil {
		return 0, fmt.Errorf("gzip: %w", err)
	}

	// The signature covers the gzip body before encryption: the server decrypts before verifying.
	var signature string
	if a.key != "" {
		signature = sign.Sign(a.key, buf.Bytes())
	}
	payload := buf.Bytes()
	if a.pubKey != nil {
		if payload, err = rsacrypt.Encrypt(a.pubKey, payload); err != nil {
			return 0, fmt.Errorf("encrypt: %w", err)
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return 0, fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "gzip")
	if signature != "" {
		req.Header.Set(sign.Header, signature)
	}
	if a.pubKey != nil {
		req.Header.Set(rsacrypt.Header, rsacrypt.HeaderValue)
	}
	if a.realIP != "" {
		req.Header.Set(subnet.Header, a.realIP)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("post: %w", err)
	}
	defer resp.Body.Close()
	return resp.StatusCode, nil
}
