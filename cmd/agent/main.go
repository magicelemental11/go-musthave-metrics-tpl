package main

import (
	"log"
	"os"
	"time"

	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/agent"
	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/config"
)

func main() {
	cfg, err := config.LoadAgent(os.Args[1:])
	if err != nil {
		log.Fatal(err)
	}

	a := agent.NewAgent()

	pollTicker := time.NewTicker(cfg.PollInterval)
	defer pollTicker.Stop()

	go func() {
		for range pollTicker.C {
			a.CollectMetrics()
		}
	}()

	reportTicker := time.NewTicker(cfg.ReportInterval)
	defer reportTicker.Stop()

	for range reportTicker.C {
		a.ReportMetrics("http://" + cfg.Address)
	}
}
