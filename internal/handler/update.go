// Package handler contains the HTTP handlers of the metrics server.
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

var indexTmpl = template.Must(template.New("metrics").Parse(`
	<html><body><ul>
		{{range $name, $value := .Gauges}}<li>{{$name}}: {{$value}}</li>{{end}}
		{{range $name, $value := .Counters}}<li>{{$name}}: {{$value}}</li>{{end}}
	</ul></body></html>`))

// Handler serves the metrics API on top of a repository.Storage.
type Handler struct {
	storage repository.Storage
	pinger  Pinger
	audit   Publisher
}

// NewHandler creates a Handler. pinger may be nil, in which case Ping always responds 500.
func NewHandler(s repository.Storage, pinger Pinger) *Handler {
	return &Handler{storage: s, pinger: pinger}
}

// GetMetric writes the plain-text value of the metric from URL params {type} and {name}.
// It responds 404 for an unknown metric and 400 for an unknown type.
func (h *Handler) GetMetric(w http.ResponseWriter, r *http.Request) {
	metricType := chi.URLParam(r, "type")
	metricName := chi.URLParam(r, "name")

	if metricName == "" {
		http.Error(w, "metric name is required", http.StatusNotFound)
		return
	}

	switch metricType {
	case model.Gauge:
		value, err := h.storage.GetGauge(r.Context(), metricName)
		if err != nil {
			storageError(w, plainError, "get gauge", err, "gauge not found")
			return
		}
		w.Write([]byte(strconv.FormatFloat(value, 'f', -1, 64)))
	case model.Counter:
		value, err := h.storage.GetCounter(r.Context(), metricName)
		if err != nil {
			storageError(w, plainError, "get counter", err, "counter not found")
			return
		}
		w.Write([]byte(strconv.FormatInt(value, 10)))

	default:
		http.Error(w, "unknown metric type", http.StatusBadRequest)
		return
	}
}

// GetAllMetrics renders all stored metrics as an HTML list.
func (h *Handler) GetAllMetrics(w http.ResponseWriter, r *http.Request) {
	gauges, counters, err := h.storage.GetAllMetrics(r.Context())
	if err != nil {
		storageError(w, plainError, "get all metrics", err, "")
		return
	}
	w.Header().Set("Content-Type", "text/html")
	indexTmpl.Execute(w, struct {
		Gauges   map[string]float64
		Counters map[string]int64
	}{gauges, counters})
}

// UpdateMetric stores the metric from URL params {type}, {name} and {value}.
// It responds 400 for an unknown type or an unparsable value and 404 for an empty name.
// Counter values are added to the stored total, gauge values replace it.
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
		if err := h.storage.UpdateGauge(r.Context(), metricName, value); err != nil {
			storageError(w, plainError, "update gauge", err, "")
			return
		}

	case model.Counter:
		value, err := strconv.ParseInt(metricValue, 10, 64)
		if err != nil {
			http.Error(w, "invalid counter value", http.StatusBadRequest)
			return
		}
		if _, err := h.storage.UpdateCounter(r.Context(), metricName, value); err != nil {
			storageError(w, plainError, "update counter", err, "")
			return
		}

	default:
		http.Error(w, "unknown metric type", http.StatusBadRequest)
		return
	}

	log.Printf("received %s %s = %s\n", metricType, metricName, metricValue)
	h.publishAudit(r.Context(), clientIP(r), metricName)
	w.WriteHeader(http.StatusOK)
}
