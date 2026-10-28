package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/agent"
	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/buildinfo"
	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/config"
	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/rsacrypt"
)

var (
	buildVersion string
	buildDate    string
	buildCommit  string
)

func main() {
	buildinfo.Fprint(os.Stdout, buildVersion, buildDate, buildCommit)

	cfg, err := config.LoadAgent(os.Args[1:])
	if err != nil {
		log.Fatal(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	defer stop()
	// After the first signal the default behaviour is restored, so a second one kills a hung shutdown.
	go func() {
		<-ctx.Done()
		stop()
	}()

	if err := run(ctx, cfg); err != nil {
		log.Fatal(err)
	}
}

// reportTimeout bounds one report, including retries, and the final report on shutdown.
const reportTimeout = 10 * time.Second

// run collects and reports metrics until ctx is cancelled. Collection stops first, then the final
// report is sent, so the metrics do not change while they are being sent.
func run(ctx context.Context, cfg config.AgentConfig) error {
	a := agent.NewAgent(cfg.Key, cfg.RateLimit)
	if cfg.CryptoKey != "" {
		pub, err := rsacrypt.LoadPublicKey(cfg.CryptoKey)
		if err != nil {
			return err
		}
		a.WithPublicKey(pub)
	}
	// A gRPC address switches the transport, so the IP is the one used to reach the gRPC server.
	serverAddr := cfg.Address
	if cfg.GRPCAddress != "" {
		serverAddr = cfg.GRPCAddress
		conn, err := a.DialGRPC(cfg.GRPCAddress)
		if err != nil {
			return err
		}
		defer conn.Close()
	}
	// Without an IP the agent still works with a server that has no trusted subnet.
	if ip, err := agent.HostIP(serverAddr); err != nil {
		log.Printf("X-Real-IP disabled: %v", err)
	} else {
		a.WithRealIP(ip)
	}

	// A failed system metrics read is not critical: it is logged and the next tick tries again.
	g, collectCtx := errgroup.WithContext(ctx)
	g.Go(func() error {
		poll(collectCtx, cfg.PollInterval, a.CollectMetrics)
		return nil
	})
	g.Go(func() error {
		poll(collectCtx, cfg.PollInterval, func() {
			if err := a.CollectSystemMetrics(); err != nil {
				log.Printf("collect system metrics: %v", err)
			}
		})
		return nil
	})

	reportTicker := time.NewTicker(cfg.ReportInterval)
	defer reportTicker.Stop()

	// Ignored over gRPC.
	serverURL := "http://" + cfg.Address
	for {
		select {
		case <-collectCtx.Done():
			err := g.Wait()
			report(ctx, a, serverURL)
			return err
		case <-reportTicker.C:
			report(ctx, a, serverURL)
		}
	}
}

// report sends the metrics detached from ctx's cancellation, so a signal does not abort a send in progress.
func report(ctx context.Context, a *agent.Agent, serverURL string) {
	sendCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), reportTimeout)
	defer cancel()
	a.ReportMetrics(sendCtx, serverURL)
}

// poll collects once right away, so a shutdown before the first tick still has data to report,
// then collects on every tick until ctx is cancelled.
func poll(ctx context.Context, interval time.Duration, collect func()) {
	collect()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			collect()
		}
	}
}
