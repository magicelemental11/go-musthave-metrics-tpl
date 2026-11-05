package proto

import "github.com/magicelemental11/go-musthave-metrics-tpl/internal/model"

// FromModel converts metrics for UpdateMetricsRequest: Delta of a counter, Value of a gauge.
// A missing value is sent as zero; the agent never builds such metrics.
func FromModel(batch []model.Metrics) []*Metric {
	out := make([]*Metric, len(batch))
	for i, m := range batch {
		pm := &Metric{Id: m.ID}
		switch m.MType {
		case model.Counter:
			pm.Type = Metric_COUNTER
			if m.Delta != nil {
				pm.Delta = *m.Delta
			}
		case model.Gauge:
			pm.Type = Metric_GAUGE
			if m.Value != nil {
				pm.Value = *m.Value
			}
		}
		out[i] = pm
	}
	return out
}

// ToModel takes Delta for a counter and Value for a gauge and ignores the other field.
// GAUGE is the zero value, so a metric without a type is a gauge. An unknown type is kept
// as its number for model.Metrics.Validate to reject.
func ToModel(metrics []*Metric) []model.Metrics {
	out := make([]model.Metrics, len(metrics))
	for i, m := range metrics {
		mm := model.Metrics{ID: m.GetId()}
		switch m.GetType() {
		case Metric_COUNTER:
			mm.MType = model.Counter
			mm.Delta = new(m.GetDelta())
		case Metric_GAUGE:
			mm.MType = model.Gauge
			mm.Value = new(m.GetValue())
		default:
			mm.MType = m.GetType().String()
		}
		out[i] = mm
	}
	return out
}
