package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/agent"
	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/config"
	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/subnet"
)

func TestBackendFor(t *testing.T) {
	tests := []struct {
		name string
		cfg  config.ServerConfig
		want string
	}{
		{"dsn wins over file", config.ServerConfig{DatabaseDSN: "dsn", FileStoragePath: "f.json"}, backendPostgres},
		{"dsn only", config.ServerConfig{DatabaseDSN: "dsn"}, backendPostgres},
		{"file without dsn", config.ServerConfig{FileStoragePath: "f.json"}, backendFile},
		{"nothing configured", config.ServerConfig{}, backendMemory},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := backendFor(tt.cfg); got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func freeAddr(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	return ln.Addr().String()
}

func startServer(t *testing.T, cfg config.ServerConfig) (cancel context.CancelFunc, done <-chan error) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() { errCh <- run(ctx, cfg) }()

	deadline := time.Now().Add(5 * time.Second)
	for {
		conn, err := net.Dial("tcp", cfg.Address)
		if err == nil {
			conn.Close()
			return cancel, errCh
		}
		if time.Now().After(deadline) {
			cancel()
			t.Fatalf("server did not start: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func waitRun(t *testing.T, done <-chan error) {
	t.Helper()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("run: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("run did not return")
	}
}

func TestRunSavesOnShutdown(t *testing.T) {
	cfg := config.ServerConfig{
		Address:         freeAddr(t),
		StoreInterval:   time.Hour,
		FileStoragePath: filepath.Join(t.TempDir(), "metrics.json"),
	}
	cancel, done := startServer(t, cfg)

	resp, err := http.Post("http://"+cfg.Address+"/update/gauge/shutdown_metric/1.5", "text/plain", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d", resp.StatusCode)
	}

	if _, statErr := os.Stat(cfg.FileStoragePath); statErr == nil {
		t.Fatal("file must not exist before the final save")
	}

	cancel()
	waitRun(t, done)

	data, err := os.ReadFile(cfg.FileStoragePath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "shutdown_metric") {
		t.Fatalf("metric is missing from %s", data)
	}
}

func TestRunFinishesInFlightRequest(t *testing.T) {
	cfg := config.ServerConfig{Address: freeAddr(t)}
	cancel, done := startServer(t, cfg)

	pr, pw := io.Pipe()
	req, err := http.NewRequest(http.MethodPost, "http://"+cfg.Address+"/update", pr)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")

	type result struct {
		status int
		err    error
	}
	resCh := make(chan result, 1)
	go func() {
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			resCh <- result{err: err}
			return
		}
		resp.Body.Close()
		resCh <- result{status: resp.StatusCode}
	}()

	if _, err := pw.Write([]byte(`{"id":"slow","type":"gauge",`)); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	cancel()
	deadline := time.Now().Add(5 * time.Second)
	for {
		conn, err := net.Dial("tcp", cfg.Address)
		if err != nil {
			break
		}
		conn.Close()
		if time.Now().After(deadline) {
			t.Fatal("server kept accepting connections after cancel")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if _, err := pw.Write([]byte(`"value":1}`)); err != nil {
		t.Fatal(err)
	}
	pw.Close()

	res := <-resCh
	if res.err != nil || res.status != http.StatusOK {
		t.Fatalf("got status %d, err %v", res.status, res.err)
	}
	waitRun(t, done)
}

func TestRunKeepsFileWhenServerFailsToStart(t *testing.T) {
	busy, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer busy.Close()

	path := filepath.Join(t.TempDir(), "metrics.json")
	const old = `[{"id":"old","type":"gauge","value":1}]`
	if err = os.WriteFile(path, []byte(old), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg := config.ServerConfig{Address: busy.Addr().String(), StoreInterval: time.Hour, FileStoragePath: path}
	if err = run(context.Background(), cfg); err == nil {
		t.Fatal("expected a listen error")
	}

	data, err := os.ReadFile(path)
	if err != nil || string(data) != old {
		t.Fatalf("file changed: %q, %v", data, err)
	}
}

func TestRunTrustedSubnet(t *testing.T) {
	tests := []struct {
		name      string
		subnet    string
		wantSaved bool
	}{
		{"agent inside subnet", "127.0.0.0/8", true},
		{"agent outside subnet", "10.0.0.0/8", false},
		{"empty subnet allows everyone", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.ServerConfig{Address: freeAddr(t), TrustedSubnet: tt.subnet}
			cancel, done := startServer(t, cfg)
			defer func() {
				cancel()
				waitRun(t, done)
			}()

			ip, err := agent.HostIP(cfg.Address)
			if err != nil {
				t.Fatal(err)
			}
			a := agent.NewAgent("", 1).WithRealIP(ip)
			a.CollectMetrics()
			a.ReportMetrics(context.Background(), "http://"+cfg.Address)

			resp, err := http.Get("http://" + cfg.Address + "/value/counter/PollCount")
			if err != nil {
				t.Fatal(err)
			}
			resp.Body.Close()
			if saved := resp.StatusCode == http.StatusOK; saved != tt.wantSaved {
				t.Fatalf("PollCount status %d, want saved=%v", resp.StatusCode, tt.wantSaved)
			}
		})
	}
}

func TestRunTrustedSubnetGuardsOnlyWrites(t *testing.T) {
	cfg := config.ServerConfig{Address: freeAddr(t), TrustedSubnet: "10.0.0.0/8"}
	cancel, done := startServer(t, cfg)
	defer func() {
		cancel()
		waitRun(t, done)
	}()
	base := "http://" + cfg.Address

	tests := []struct {
		method, path, ip string
		want             int
	}{
		{http.MethodPost, "/update/gauge/g/1", "", http.StatusForbidden},
		{http.MethodPost, "/update/gauge/g/1", "192.168.0.1", http.StatusForbidden},
		{http.MethodPost, "/update/gauge/g/1", "10.0.0.5", http.StatusOK},
		{http.MethodPost, "/update", "", http.StatusForbidden},
		{http.MethodPost, "/updates/", "", http.StatusForbidden},
		{http.MethodPost, "/update/gauge/g/", "", http.StatusNotFound}, // stubs stay 404 without the header
		{http.MethodPost, "/update/gauge/", "", http.StatusNotFound},
		{http.MethodPost, "/update/", "", http.StatusNotFound},
		{http.MethodGet, "/", "", http.StatusOK},
		{http.MethodGet, "/value/gauge/g", "", http.StatusOK},
		{http.MethodPost, "/value", "", http.StatusBadRequest}, // reaches the handler: empty body
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s %s ip=%q", tt.method, tt.path, tt.ip), func(t *testing.T) {
			req, err := http.NewRequest(tt.method, base+tt.path, nil)
			if err != nil {
				t.Fatal(err)
			}
			if tt.ip != "" {
				req.Header.Set(subnet.Header, tt.ip)
			}
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			resp.Body.Close()
			if resp.StatusCode != tt.want {
				t.Fatalf("status %d, want %d", resp.StatusCode, tt.want)
			}
		})
	}
}
