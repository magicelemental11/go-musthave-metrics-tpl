package model

import "testing"

func TestMetricsReset(t *testing.T) {
	delta, value := int64(4), 2.5
	m := &Metrics{ID: "id", MType: Counter, Delta: &delta, Value: &value, Hash: "h"}
	m.Reset()

	if m.ID != "" || m.MType != "" || m.Hash != "" {
		t.Errorf("strings not cleared: %+v", m)
	}
	if m.Delta != &delta || delta != 0 || m.Value != &value || value != 0 {
		t.Errorf("pointers must be kept and zeroed: delta=%d value=%v", delta, value)
	}
}

func TestMetricsResetNil(t *testing.T) {
	var m *Metrics
	m.Reset()
	(&Metrics{}).Reset()
}
