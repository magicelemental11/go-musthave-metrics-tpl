package agent

import (
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"runtime"
)

type Agent struct {
	pollCount int64
	stats     runtime.MemStats
	gauges    map[string]float64
}

func NewAgent() *Agent {
	return &Agent{
		gauges: make(map[string]float64),
	}
}

func (a *Agent) CollectMetrics() {
	runtime.ReadMemStats(&a.stats)
	a.pollCount++
	a.gauges["Alloc"] = float64(a.stats.Alloc)
	a.gauges["BuckHashSys"] = float64(a.stats.BuckHashSys)
	a.gauges["Frees"] = float64(a.stats.Frees)
	a.gauges["GCCPUFraction"] = a.stats.GCCPUFraction
	a.gauges["GCSys"] = float64(a.stats.GCSys)
	a.gauges["HeapAlloc"] = float64(a.stats.HeapAlloc)
	a.gauges["HeapIdle"] = float64(a.stats.HeapIdle)
	a.gauges["HeapInuse"] = float64(a.stats.HeapInuse)
	a.gauges["HeapObjects"] = float64(a.stats.HeapObjects)
	a.gauges["HeapReleased"] = float64(a.stats.HeapReleased)
	a.gauges["HeapSys"] = float64(a.stats.HeapSys)
	a.gauges["LastGC"] = float64(a.stats.LastGC)
	a.gauges["Lookups"] = float64(a.stats.Lookups)
	a.gauges["MCacheInuse"] = float64(a.stats.MCacheInuse)
	a.gauges["MCacheSys"] = float64(a.stats.MCacheSys)
	a.gauges["MSpanInuse"] = float64(a.stats.MSpanInuse)
	a.gauges["MSpanSys"] = float64(a.stats.MSpanSys)
	a.gauges["Mallocs"] = float64(a.stats.Mallocs)
	a.gauges["NextGC"] = float64(a.stats.NextGC)
	a.gauges["NumForcedGC"] = float64(a.stats.NumForcedGC)
	a.gauges["NumGC"] = float64(a.stats.NumGC)
	a.gauges["OtherSys"] = float64(a.stats.OtherSys)
	a.gauges["PauseTotalNs"] = float64(a.stats.PauseTotalNs)
	a.gauges["StackInuse"] = float64(a.stats.StackInuse)
	a.gauges["StackSys"] = float64(a.stats.StackSys)
	a.gauges["Sys"] = float64(a.stats.Sys)
	a.gauges["TotalAlloc"] = float64(a.stats.TotalAlloc)
	a.gauges["RandomValue"] = rand.Float64()
}

func (a *Agent) ReportMetrics(serverAddr string) {
	for name, value := range a.gauges {
		url := fmt.Sprintf("%s/update/gauge/%s/%v", serverAddr, name, value)
		resp, err := http.Post(url, "text/plain", nil)
		if err != nil {
			log.Printf("error sending %s: %v\n", name, err)
			continue
		}
		resp.Body.Close()
		log.Printf("sent gauge %s = %v → %d\n", name, value, resp.StatusCode)
	}

	url := fmt.Sprintf("%s/update/counter/PollCount/%d", serverAddr, a.pollCount)
	resp, err := http.Post(url, "text/plain", nil)
	if err != nil {
		log.Printf("error sending PollCount: %v\n", err)
		return
	}
	resp.Body.Close()
	log.Printf("sent counter PollCount = %d → %d\n", a.pollCount, resp.StatusCode)
	if resp.StatusCode == http.StatusOK {
		a.pollCount = 0
	}
}
