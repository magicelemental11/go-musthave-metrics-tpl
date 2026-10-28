package agent

import (
	"bytes"
	"compress/gzip"
	"context"
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

	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/model"
	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/retry"
	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/sign"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/mem"
)

const requestTimeout = 10 * time.Second

var httpClient = &http.Client{Timeout: requestTimeout}

type Agent struct {
	mu        sync.Mutex
	pollCount int64
	stats     runtime.MemStats
	gauges    map[string]float64
	delays    []time.Duration
	key       string
	rateLimit int
}

func NewAgent(key string, rateLimit int) *Agent {
	return &Agent{
		key:       key,
		rateLimit: max(rateLimit, 1),
		gauges:    make(map[string]float64),
		delays:    retry.DefaultDelays,
	}
}

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

// Во время отправки мьютекс не держим, чтобы сбор метрик не блокировался медленным сервером.
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
	runPool(ctx, a.rateLimit, len(chunks), func(i int) {
		var status int
		err := retry.Do(ctx, a.delays, func() (err error) {
			status, err = sendBatch(ctx, serverAddr+"/updates/", a.key, chunks[i])
			return err
		}, isTransientNetErr)
		if err != nil {
			log.Printf("error sending batch of %d metrics: %v", len(chunks[i]), err)
			return
		}
		log.Printf("sent batch of %d metrics → %d", len(chunks[i]), status)
		if status == http.StatusOK && hasPollCount(chunks[i]) {
			// Вычитаем только отправленное: опросы, сделанные во время отправки, не теряем.
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

func hasPollCount(batch []model.Metrics) bool {
	for _, m := range batch {
		if m.ID == "PollCount" && m.MType == model.Counter {
			return true
		}
	}
	return false
}

func runPool(ctx context.Context, limit, n int, job func(i int)) {
	jobs := make(chan int, min(limit, n))
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
dispatch:
	for i := range n {
		select {
		case <-ctx.Done():
			break dispatch
		case jobs <- i:
		}
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

func sendBatch(ctx context.Context, url, key string, batch []model.Metrics) (int, error) {
	body, err := json.Marshal(batch)
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

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &buf)
	if err != nil {
		return 0, fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "gzip")
	if key != "" {
		req.Header.Set(sign.Header, sign.Sign(key, body))
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("post: %w", err)
	}
	defer resp.Body.Close()
	return resp.StatusCode, nil
}
