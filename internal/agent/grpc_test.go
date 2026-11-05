package agent

import (
	"context"
	"errors"
	"net"
	"sync"
	"testing"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/grpcserver"
	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/handler"
	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/model"
	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/repository"
	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/subnet"
)

// recordingUpdater remembers the x-real-ip of every call before passing the batch on.
type recordingUpdater struct {
	grpcserver.BatchUpdater
	mu  sync.Mutex
	ips []string
}

func (r *recordingUpdater) UpdateBatch(ctx context.Context, ip string, metrics []model.Metrics) error {
	md, _ := metadata.FromIncomingContext(ctx)
	r.mu.Lock()
	r.ips = append(r.ips, md.Get(subnet.MetadataKey)...)
	r.mu.Unlock()
	return r.BatchUpdater.UpdateBatch(ctx, ip, metrics)
}

// dialBufconn connects a to srv over an in-memory listener.
func dialBufconn(t *testing.T, a *Agent, srv *grpc.Server) {
	t.Helper()
	lis := bufconn.Listen(1 << 20)
	go srv.Serve(lis)
	t.Cleanup(srv.Stop)
	conn, err := a.DialGRPC("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return lis.DialContext(ctx) }))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
}

func TestReportMetricsOverGRPC(t *testing.T) {
	storage := repository.NewMemStorage()
	updater := &recordingUpdater{BatchUpdater: handler.NewHandler(storage, nil)}
	checker, err := subnet.New("10.0.0.0/8")
	if err != nil {
		t.Fatal(err)
	}

	a := NewAgent("", 3).WithRealIP("10.1.2.3")
	dialBufconn(t, a, grpcserver.NewServer(updater, checker, zap.NewNop()))
	a.CollectMetrics()
	a.CollectMetrics()
	a.ReportMetrics(context.Background(), "")

	ctx := context.Background()
	if v, err := storage.GetCounter(ctx, "PollCount"); err != nil || v != 2 {
		t.Errorf("PollCount = %v, %v; want 2", v, err)
	}
	if _, err := storage.GetGauge(ctx, "Alloc"); err != nil {
		t.Errorf("Alloc: %v", err)
	}
	if len(updater.ips) != 3 {
		t.Errorf("calls = %d, want one per worker", len(updater.ips))
	}
	for _, ip := range updater.ips {
		if ip != "10.1.2.3" {
			t.Errorf("%s = %q", subnet.MetadataKey, ip)
		}
	}
	if a.pollCount != 0 {
		t.Errorf("pollCount = %d, want reset after success", a.pollCount)
	}
}

func TestReportMetricsOverGRPCOutsideSubnet(t *testing.T) {
	storage := repository.NewMemStorage()
	checker, err := subnet.New("10.0.0.0/8")
	if err != nil {
		t.Fatal(err)
	}
	a := newTestAgent().WithRealIP("192.168.0.1")
	dialBufconn(t, a, grpcserver.NewServer(handler.NewHandler(storage, nil), checker, zap.NewNop()))
	a.ReportMetrics(context.Background(), "")

	if gauges, counters, _ := storage.GetAllMetrics(context.Background()); len(gauges)+len(counters) != 0 {
		t.Errorf("metrics stored from outside the subnet: %v %v", gauges, counters)
	}
	if a.pollCount != 1 {
		t.Errorf("pollCount = %d, want kept after a rejected send", a.pollCount)
	}
}

func TestIsTransientGRPCErr(t *testing.T) {
	tests := []struct {
		err  error
		want bool
	}{
		{status.Error(codes.Unavailable, "down"), true},
		{status.Error(codes.DeadlineExceeded, "slow"), true},
		{status.Error(codes.PermissionDenied, "no"), false},
		{status.Error(codes.InvalidArgument, "bad"), false},
		{status.Error(codes.Internal, "db"), false},
		{errors.New("plain"), false},
		{nil, false},
	}
	for _, tt := range tests {
		if got := isTransientGRPCErr(tt.err); got != tt.want {
			t.Errorf("isTransientGRPCErr(%v) = %v, want %v", tt.err, got, tt.want)
		}
	}
}
