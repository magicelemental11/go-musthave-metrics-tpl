package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

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
	// Without an IP the agent still works with a server that has no trusted subnet.
	if ip, err := agent.HostIP(cfg.Address); err != nil {
		log.Printf("X-Real-IP disabled: %v", err)
	} else {
		a.WithRealIP(ip)
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		poll(ctx, cfg.PollInterval, a.CollectMetrics)
	}()
	go func() {
		defer wg.Done()
		poll(ctx, cfg.PollInterval, func() {
			if err := a.CollectSystemMetrics(); err != nil {
				log.Printf("collect system metrics: %v", err)
			}
		})
	}()

	reportTicker := time.NewTicker(cfg.ReportInterval)
	defer reportTicker.Stop()

	serverURL := "http://" + cfg.Address
	for {
		select {
		case <-ctx.Done():
			wg.Wait()
			report(ctx, a, serverURL)
			return nil
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

func poll(ctx context.Context, interval time.Duration, collect func()) {
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
