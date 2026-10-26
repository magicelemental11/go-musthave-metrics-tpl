package main

import (
	"context"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/config"
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
