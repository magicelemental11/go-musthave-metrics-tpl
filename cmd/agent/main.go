package main

import (
	"flag"
	"time"

	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/agent"
)

func main() {
	address := flag.String("a", "localhost:8080", "server address")
	pollInterval := flag.Int("p", 2, "poll interval in seconds")
	reportInterval := flag.Int("r", 10, "report interval in seconds")
	flag.Parse()

	a := agent.NewAgent()

	pollTicker := time.NewTicker(time.Duration(*pollInterval) * time.Second)
	defer pollTicker.Stop()

	go func() {
		for range pollTicker.C {
			a.CollectMetrics()
		}
	}()

	reportTicker := time.NewTicker(time.Duration(*reportInterval) * time.Second)
	defer reportTicker.Stop()

	for range reportTicker.C {
		a.ReportMetrics("http://" + *address)
	}
}
