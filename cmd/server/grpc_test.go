package main

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/config"
	pb "github.com/magicelemental11/go-musthave-metrics-tpl/internal/proto"
	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/subnet"
)

func TestRunGRPC(t *testing.T) {
	cfg := config.ServerConfig{
		Address:         freeAddr(t),
		GRPCAddress:     freeAddr(t),
		StoreInterval:   time.Hour,
		FileStoragePath: filepath.Join(t.TempDir(), "metrics.json"),
		TrustedSubnet:   "10.0.0.0/8",
	}
	cancel, done := startServer(t, cfg)

	conn, err := grpc.NewClient(cfg.GRPCAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	client := pb.NewMetricsClient(conn)
	req := &pb.UpdateMetricsRequest{Metrics: []*pb.Metric{{Id: "grpc_metric", Value: 1.5}}}

	_, err = client.UpdateMetrics(context.Background(), req)
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("without x-real-ip: %v, want PermissionDenied", err)
	}
	ctx := metadata.AppendToOutgoingContext(context.Background(), subnet.MetadataKey, "10.0.0.1")
	if _, err = client.UpdateMetrics(ctx, req); err != nil {
		t.Fatal(err)
	}

	cancel()
	waitRun(t, done)

	data, err := os.ReadFile(cfg.FileStoragePath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "grpc_metric") {
		t.Fatalf("metric is missing from %s", data)
	}
}

func TestRunFailsOnInvalidGRPCAddress(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cfg := config.ServerConfig{Address: freeAddr(t), GRPCAddress: "127.0.0.1:-1"}
	if err := run(ctx, cfg); err == nil {
		t.Fatal("want error for an invalid gRPC address")
	}
}

func TestRunReleasesGRPCPortWhenHTTPFailsToStart(t *testing.T) {
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
	cfg := config.ServerConfig{
		Address:         busy.Addr().String(),
		GRPCAddress:     freeAddr(t),
		StoreInterval:   time.Hour,
		FileStoragePath: path,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err = run(ctx, cfg); err == nil {
		t.Fatal("expected a listen error")
	}

	if data, readErr := os.ReadFile(path); readErr != nil || string(data) != old {
		t.Fatalf("file changed: %q, %v", data, readErr)
	}
	// Nothing was served, so the gRPC port must be free again.
	ln, err := net.Listen("tcp", cfg.GRPCAddress)
	if err != nil {
		t.Fatalf("gRPC port still taken: %v", err)
	}
	ln.Close()
}
