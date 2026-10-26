package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/config"
)

func TestRunReportsOnShutdown(t *testing.T) {
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
	}))
	defer srv.Close()

	cfg := config.AgentConfig{
		Address:        strings.TrimPrefix(srv.URL, "http://"),
		PollInterval:   20 * time.Millisecond,
		ReportInterval: time.Hour,
		RateLimit:      1,
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- run(ctx, cfg) }()

	time.Sleep(300 * time.Millisecond)
	if n := requests.Load(); n != 0 {
		t.Fatalf("got %d requests before shutdown", n)
	}
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("run did not return")
	}
	if requests.Load() == 0 {
		t.Fatal("no final report was sent")
	}
}
