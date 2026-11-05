package main

import (
	"context"
	"net"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/config"
	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/grpcserver"
	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/handler"
	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/repository"
	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/subnet"
)

func TestRunReportsOverGRPC(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	storage := repository.NewMemStorage()
	// The subnet check proves the agent sends x-real-ip derived from the gRPC address.
	checker, err := subnet.New("127.0.0.0/8")
	if err != nil {
		t.Fatal(err)
	}
	srv := grpcserver.NewServer(handler.NewHandler(storage, nil), checker, zap.NewNop())
	go srv.Serve(lis)
	defer srv.Stop()

	cfg := config.AgentConfig{
		Address:        "127.0.0.1:1", // must not be used
		GRPCAddress:    lis.Addr().String(),
		PollInterval:   20 * time.Millisecond,
		ReportInterval: time.Hour,
		RateLimit:      1,
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- run(ctx, cfg) }()
	time.Sleep(100 * time.Millisecond)
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}

	if v, err := storage.GetCounter(context.Background(), "PollCount"); err != nil || v == 0 {
		t.Fatalf("PollCount = %v, %v; want the final report stored", v, err)
	}
}
