package handler

import (
	"html/template"
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/model"
	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/repository"
)

type Handler struct {
	storage repository.Storage
	pinger  Pinger
}

// NewHandler: pinger может быть nil — тогда /ping отвечает 500.
func NewHandler(s repository.Storage, pinger Pinger) *Handler {
	return &Handler{storage: s, pinger: pinger}
}

func (h *Handler) GetMetric(w http.ResponseWriter, r *http.Request) {
	metricType := chi.URLParam(r, "type")
	metricName := chi.URLParam(r, "name")

	if metricName == "" {
		http.Error(w, "metric name is required", http.StatusNotFound)
		return
	}

	switch metricType {
	case model.Gauge:
		value, ok := h.storage.GetGauge(metricName)
		if !ok {
			http.Error(w, "gauge not found", http.StatusNotFound)
			return
		}
		w.Write([]byte(strconv.FormatFloat(value, 'f', -1, 64)))
	case model.Counter:
		value, ok := h.storage.GetCounter(metricName)
		if !ok {
			http.Error(w, "counter not found", http.StatusNotFound)
			return
		}
		w.Write([]byte(strconv.FormatInt(value, 10)))

	default:
		http.Error(w, "unknown metric type", http.StatusBadRequest)
		return
	}
}

func (h *Handler) GetAllMetrics(w http.ResponseWriter, r *http.Request) {
	gauges, counters := h.storage.GetAllMetrics()
	w.Header().Set("Content-Type", "text/html")
	tmpl := template.Must(template.New("metrics").Parse(`
	<html><body><ul>
		{{range $name, $value := .Gauges}}<li>{{$name}}: {{$value}}</li>{{end}}
		{{range $name, $value := .Counters}}<li>{{$name}}: {{$value}}</li>{{end}}
	</ul></body></html>`))

	tmpl.Execute(w, struct {
		Gauges   map[string]float64
		Counters map[string]int64
	}{gauges, counters})
}

func (h *Handler) UpdateMetric(w http.ResponseWriter, r *http.Request) {
	metricType := chi.URLParam(r, "type")
	metricName := chi.URLParam(r, "name")
	metricValue := chi.URLParam(r, "value")

	if metricName == "" {
		http.Error(w, "metric name is required", http.StatusNotFound)
		return
	}

	switch metricType {
	case model.Gauge:
		value, err := strconv.ParseFloat(metricValue, 64)
		if err != nil {
			http.Error(w, "invalid gauge value", http.StatusBadRequest)
			return
		}
		h.storage.UpdateGauge(metricName, value)

	case model.Counter:
		value, err := strconv.ParseInt(metricValue, 10, 64)
		if err != nil {
			http.Error(w, "invalid counter value", http.StatusBadRequest)
			return
		}
		h.storage.UpdateCounter(metricName, value)

	default:
		http.Error(w, "unknown metric type", http.StatusBadRequest)
		return
	}

	log.Printf("received %s %s = %s\n", metricType, metricName, metricValue)
	w.WriteHeader(http.StatusOK)
}
