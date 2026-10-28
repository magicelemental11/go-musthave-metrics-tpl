package grpcserver

import (
	"context"
	"errors"
	"net"
	"testing"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/handler"
	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/model"
	pb "github.com/magicelemental11/go-musthave-metrics-tpl/internal/proto"
	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/repository"
	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/subnet"
)

func TestTrustedSubnetInterceptor(t *testing.T) {
	checker, err := subnet.New("10.0.0.0/8")
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name    string
		checker *subnet.Checker
		md      metadata.MD
		want    codes.Code
	}{
		{"inside", checker, metadata.Pairs(subnet.MetadataKey, "10.1.2.3"), codes.OK},
		{"outside", checker, metadata.Pairs(subnet.MetadataKey, "192.168.0.1"), codes.PermissionDenied},
		{"no ip", checker, metadata.Pairs("other", "x"), codes.PermissionDenied},
		{"no metadata", checker, nil, codes.PermissionDenied},
		{"no subnet", nil, nil, codes.OK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			if tt.md != nil {
				ctx = metadata.NewIncomingContext(ctx, tt.md)
			}
			called := false
			_, err := TrustedSubnet(tt.checker)(ctx, nil, &grpc.UnaryServerInfo{}, func(context.Context, any) (any, error) {
				called = true
				return nil, nil
			})
			if status.Code(err) != tt.want || called != (tt.want == codes.OK) {
				t.Fatalf("code = %v, handler called = %v; want %v", status.Code(err), called, tt.want)
			}
		})
	}
}

func TestClientIP(t *testing.T) {
	peerCtx := peer.NewContext(context.Background(), &peer.Peer{Addr: &net.TCPAddr{IP: net.IPv4(127, 0, 0, 2), Port: 5000}})
	if got := clientIP(peerCtx); got != "127.0.0.2" {
		t.Errorf("from peer: %q", got)
	}
	mdCtx := metadata.NewIncomingContext(peerCtx, metadata.Pairs(subnet.MetadataKey, "10.0.0.1"))
	if got := clientIP(mdCtx); got != "10.0.0.1" {
		t.Errorf("from metadata: %q", got)
	}
	if got := clientIP(context.Background()); got != "" {
		t.Errorf("without peer: %q", got)
	}
}

// dial starts srv on an in-memory listener and returns a client connected to it.
func dial(t *testing.T, srv *grpc.Server) pb.MetricsClient {
	t.Helper()
	lis := bufconn.Listen(1 << 20)
	go srv.Serve(lis)
	t.Cleanup(srv.Stop)
	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return lis.DialContext(ctx) }),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	return pb.NewMetricsClient(conn)
}

func TestUpdateMetrics(t *testing.T) {
	storage := repository.NewMemStorage()
	client := dial(t, NewServer(handler.NewHandler(storage, nil), nil, zap.NewNop()))
	ctx := context.Background()

	_, err := client.UpdateMetrics(ctx, &pb.UpdateMetricsRequest{Metrics: []*pb.Metric{
		{Id: "g", Type: pb.Metric_GAUGE, Value: 1.5},
		{Id: "c", Type: pb.Metric_COUNTER, Delta: 2},
		{Id: "c", Type: pb.Metric_COUNTER, Delta: 3},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if v, _ := storage.GetGauge(ctx, "g"); v != 1.5 {
		t.Errorf("gauge = %v, want 1.5", v)
	}
	if v, _ := storage.GetCounter(ctx, "c"); v != 5 {
		t.Errorf("counter = %v, want 5", v)
	}
}

func TestUpdateMetricsErrors(t *testing.T) {
	checker, err := subnet.New("10.0.0.0/8")
	if err != nil {
		t.Fatal(err)
	}
	valid := []*pb.Metric{{Id: "g", Value: 1}}
	tests := []struct {
		name    string
		storage repository.Storage
		checker *subnet.Checker
		ip      string
		metrics []*pb.Metric
		want    codes.Code
	}{
		{"empty batch", repository.NewMemStorage(), nil, "", nil, codes.InvalidArgument},
		{"missing id", repository.NewMemStorage(), nil, "", []*pb.Metric{{Value: 1}}, codes.InvalidArgument},
		{"unknown type", repository.NewMemStorage(), nil, "", []*pb.Metric{{Id: "x", Type: 7}}, codes.InvalidArgument},
		{"storage error", failingStorage{}, nil, "", valid, codes.Internal},
		{"outside subnet", repository.NewMemStorage(), checker, "192.168.0.1", valid, codes.PermissionDenied},
		{"no ip", repository.NewMemStorage(), checker, "", valid, codes.PermissionDenied},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := dial(t, NewServer(handler.NewHandler(tt.storage, nil), tt.checker, zap.NewNop()))
			ctx := context.Background()
			if tt.ip != "" {
				ctx = metadata.AppendToOutgoingContext(ctx, subnet.MetadataKey, tt.ip)
			}
			_, err := client.UpdateMetrics(ctx, &pb.UpdateMetricsRequest{Metrics: tt.metrics})
			if status.Code(err) != tt.want {
				t.Fatalf("code = %v (%v), want %v", status.Code(err), err, tt.want)
			}
			if mem, ok := tt.storage.(*repository.MemStorage); ok {
				if gauges, counters, _ := mem.GetAllMetrics(ctx); len(gauges)+len(counters) != 0 {
					t.Errorf("rejected batch changed storage: %v %v", gauges, counters)
				}
			}
		})
	}
}

type failingStorage struct{ repository.Storage }

func (failingStorage) UpdateBatch(context.Context, []model.Metrics) error {
	return errors.New("db down")
}
