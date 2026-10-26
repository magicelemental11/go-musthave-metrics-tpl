// Package model defines the metric types shared by the server and the agent.
package model

import "errors"

//go:generate go run ../../cmd/reset

// Metric types.
const (
	Counter = "counter"
	Gauge   = "gauge"
)

// Metrics is the JSON representation of a metric.
// Delta is set for counters and Value for gauges; both are pointers
// so that zero can be told apart from an absent value.
//
// generate:reset
type Metrics struct {
	ID    string   `json:"id"`
	MType string   `json:"type"`
	Delta *int64   `json:"delta,omitempty"`
	Value *float64 `json:"value,omitempty"`
	Hash  string   `json:"hash,omitempty"`
}

// Validate reports whether the metric has an id, a known type and the value matching that type.
func (m Metrics) Validate() error {
	if m.ID == "" {
		return errors.New("metric id is required")
	}
	switch m.MType {
	case Gauge:
		if m.Value == nil {
			return errors.New("gauge value is required")
		}
	case Counter:
		if m.Delta == nil {
			return errors.New("counter delta is required")
		}
	default:
		return errors.New("unknown metric type")
	}
	return nil
}
