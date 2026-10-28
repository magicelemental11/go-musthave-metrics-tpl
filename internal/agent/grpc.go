package agent

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/model"
	pb "github.com/magicelemental11/go-musthave-metrics-tpl/internal/proto"
	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/subnet"
)

// DialGRPC creates a client for the gRPC server at addr and switches ReportMetrics to it.
// The connection is plaintext and is established lazily on the first call. The caller closes
// the returned connection when the agent stops. opts are added after the defaults.
// Signing and encryption are HTTP-only and are not applied over gRPC.
func (a *Agent) DialGRPC(addr string, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
	opts = append([]grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(a.realIPInterceptor),
	}, opts...)
	conn, err := grpc.NewClient(addr, opts...)
	if err != nil {
		return nil, fmt.Errorf("grpc client: %w", err)
	}
	a.sender = grpcSender{client: pb.NewMetricsClient(conn)}
	return conn, nil
}

// realIPInterceptor adds the WithRealIP address to the outgoing metadata under subnet.MetadataKey,
// like the X-Real-IP header over HTTP; an empty address adds nothing.
func (a *Agent) realIPInterceptor(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
	if a.realIP != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, subnet.MetadataKey, a.realIP)
	}
	return invoker(ctx, method, req, reply, cc, opts...)
}

type grpcSender struct {
	client pb.MetricsClient
}

// Send bounds each call by requestTimeout, like the HTTP client, so a timeout can be retried.
func (s grpcSender) Send(ctx context.Context, batch []model.Metrics) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	_, err := s.client.UpdateMetrics(ctx, &pb.UpdateMetricsRequest{Metrics: pb.FromModel(batch)})
	return err
}

// isTransientGRPCErr reports whether the call may succeed if repeated: the server is unreachable
// or did not answer in time. Rejections such as PermissionDenied or InvalidArgument are final.
func isTransientGRPCErr(err error) bool {
	switch status.Code(err) {
	case codes.Unavailable, codes.DeadlineExceeded:
		return true
	default:
		return false
	}
}
