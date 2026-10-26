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

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	a := agent.NewAgent(cfg.Key, cfg.RateLimit)
	if cfg.CryptoKey != "" {
		pub, err := rsacrypt.LoadPublicKey(cfg.CryptoKey)
		if err != nil {
			log.Fatal(err)
		}
		a.WithPublicKey(pub)
	}

	var wg sync.WaitGroup
	defer wg.Wait()
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

	for {
		select {
		case <-ctx.Done():
			return
		case <-reportTicker.C:
			a.ReportMetrics(ctx, "http://"+cfg.Address)
		}
	}
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
