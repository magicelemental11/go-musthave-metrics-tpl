package model

import "errors"

const (
	Counter = "counter"
	Gauge   = "gauge"
)

// NOTE: Не усложняем пример, вводя иерархическую вложенность структур.
// Органичиваясь плоской моделью.
// Delta и Value объявлены через указатели,
// что бы отличать значение "0", от не заданного значения
// и соответственно не кодировать в структуру.
type Metrics struct {
	ID    string   `json:"id"`
	MType string   `json:"type"`
	Delta *int64   `json:"delta,omitempty"`
	Value *float64 `json:"value,omitempty"`
	Hash  string   `json:"hash,omitempty"`
}

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
