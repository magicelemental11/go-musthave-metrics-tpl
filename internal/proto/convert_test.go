package proto

import (
	"testing"

	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/model"
)

func TestToModel(t *testing.T) {
	tests := []struct {
		name string
		in   *Metric
		want model.Metrics
	}{
		{"counter ignores value", &Metric{Id: "c", Type: Metric_COUNTER, Delta: 3, Value: 9}, model.Metrics{ID: "c", MType: model.Counter, Delta: new(int64(3))}},
		{"gauge ignores delta", &Metric{Id: "g", Type: Metric_GAUGE, Delta: 9, Value: 1.5}, model.Metrics{ID: "g", MType: model.Gauge, Value: new(1.5)}},
		{"type defaults to gauge", &Metric{Id: "g"}, model.Metrics{ID: "g", MType: model.Gauge, Value: new(0.0)}},
		{"unknown type", &Metric{Id: "x", Type: 7}, model.Metrics{ID: "x", MType: "7"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ToModel([]*Metric{tt.in})
			if len(got) != 1 {
				t.Fatalf("got %d metrics", len(got))
			}
			if g := got[0]; g.ID != tt.want.ID || g.MType != tt.want.MType ||
				!equalPtr(g.Delta, tt.want.Delta) || !equalPtr(g.Value, tt.want.Value) {
				t.Fatalf("got %+v, want %+v", g, tt.want)
			}
		})
	}
	if got := ToModel(nil); len(got) != 0 {
		t.Errorf("empty request gave %+v", got)
	}
}

func equalPtr[T comparable](a, b *T) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func TestFromModel(t *testing.T) {
	got := FromModel([]model.Metrics{
		{ID: "c", MType: model.Counter, Delta: new(int64(3))},
		{ID: "g", MType: model.Gauge, Value: new(1.5)},
		{ID: "nil", MType: model.Counter},
	})
	if len(got) != 3 {
		t.Fatalf("got %d metrics", len(got))
	}
	if c := got[0]; c.GetId() != "c" || c.GetType() != Metric_COUNTER || c.GetDelta() != 3 || c.GetValue() != 0 {
		t.Errorf("counter = %v", c)
	}
	if g := got[1]; g.GetId() != "g" || g.GetType() != Metric_GAUGE || g.GetValue() != 1.5 || g.GetDelta() != 0 {
		t.Errorf("gauge = %v", g)
	}
	if n := got[2]; n.GetType() != Metric_COUNTER || n.GetDelta() != 0 {
		t.Errorf("counter without delta = %v", n)
	}
	if empty := FromModel(nil); len(empty) != 0 {
		t.Errorf("empty batch gave %v", empty)
	}
}
