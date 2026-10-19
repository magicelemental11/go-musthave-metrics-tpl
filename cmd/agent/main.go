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
	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/config"
)

func main() {
	cfg, err := config.LoadAgent(os.Args[1:])
	if err != nil {
		log.Fatal(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	a := agent.NewAgent(cfg.Key, cfg.RateLimit)

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
