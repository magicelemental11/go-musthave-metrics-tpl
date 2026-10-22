package agent

import (
	"context"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func BenchmarkCollectMetrics(b *testing.B) {
	a := NewAgent("", 1)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		a.CollectMetrics()
	}
}

func BenchmarkReportMetrics(b *testing.B) {
	log.SetOutput(io.Discard)
	defer log.SetOutput(os.Stderr)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()
	a := NewAgent("bench-key", 2)
	a.CollectMetrics()
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		a.ReportMetrics(ctx, srv.URL)
	}
}
