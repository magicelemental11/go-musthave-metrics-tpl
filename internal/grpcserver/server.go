// Package grpcserver serves the Metrics gRPC service described in api/proto/metrics.proto.
package grpcserver

import (
	"context"
	"errors"
	"log"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"

	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/handler"
	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/model"
	pb "github.com/magicelemental11/go-musthave-metrics-tpl/internal/proto"
	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/subnet"
)

// BatchUpdater validates and stores a batch; *handler.Handler implements it.
// Errors matching handler.ErrInvalidBatch are the client's fault, the rest are storage errors.
type BatchUpdater interface {
	UpdateBatch(ctx context.Context, ip string, metrics []model.Metrics) error
}

// NewServer returns a grpc.Server with the Metrics service registered behind the logging
// and trusted subnet interceptors. A nil checker disables the subnet check.
func NewServer(u BatchUpdater, c *subnet.Checker, logger *zap.Logger) *grpc.Server {
	srv := grpc.NewServer(grpc.ChainUnaryInterceptor(Logging(logger), TrustedSubnet(c)))
	pb.RegisterMetricsServer(srv, &metricsServer{updater: u})
	return srv
}

// metricsServer implements pb.MetricsServer on top of a BatchUpdater.
type metricsServer struct {
	pb.UnimplementedMetricsServer
	updater BatchUpdater
}

// UpdateMetrics stores the batch. An invalid batch returns codes.InvalidArgument, a storage error codes.Internal.
func (s *metricsServer) UpdateMetrics(ctx context.Context, req *pb.UpdateMetricsRequest) (*pb.UpdateMetricsResponse, error) {
	err := s.updater.UpdateBatch(ctx, clientIP(ctx), pb.ToModel(req.GetMetrics()))
	switch {
	case errors.Is(err, handler.ErrInvalidBatch):
		return nil, status.Error(codes.InvalidArgument, err.Error())
	case err != nil:
		log.Print(err)
		return nil, status.Error(codes.Internal, "storage error")
	}
	return &pb.UpdateMetricsResponse{}, nil
}

// TrustedSubnet rejects calls with codes.PermissionDenied unless the x-real-ip metadata holds an address
// inside the subnet. As with the HTTP middleware, there is no fallback to the peer address.
// A nil checker disables the check.
func TrustedSubnet(c *subnet.Checker) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, next grpc.UnaryHandler) (any, error) {
		if c != nil && !c.Allowed(realIP(ctx)) {
			return nil, status.Error(codes.PermissionDenied, "forbidden")
		}
		return next(ctx, req)
	}
}

// Logging logs the method, status code and duration of every call.
func Logging(logger *zap.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, next grpc.UnaryHandler) (any, error) {
		start := time.Now()
		resp, err := next(ctx, req)
		logger.Info("grpc request",
			zap.String("method", info.FullMethod),
			zap.Duration("duration", time.Since(start)),
			zap.String("code", status.Code(err).String()),
		)
		return resp, err
	}
}

func realIP(ctx context.Context) string {
	md, _ := metadata.FromIncomingContext(ctx)
	if ips := md.Get(subnet.MetadataKey); len(ips) > 0 {
		return ips[0]
	}
	return ""
}

// clientIP is the audit address: x-real-ip metadata, then the host of the peer address.
func clientIP(ctx context.Context) string {
	if ip := realIP(ctx); ip != "" {
		return ip
	}
	if p, ok := peer.FromContext(ctx); ok {
		return subnet.HostOf(p.Addr.String())
	}
	return ""
}
