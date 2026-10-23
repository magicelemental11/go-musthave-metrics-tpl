package pool

import "testing"

var sink *payload

type payload struct {
	items []int
}

func (x *payload) Reset() { x.items = x.items[:0] }

func BenchmarkPool(b *testing.B) {
	pl := New(func() *payload { return &payload{items: make([]int, 0, 1024)} })
	b.ReportAllocs()
	for b.Loop() {
		x := pl.Get()
		x.items = append(x.items, 1, 2, 3)
		pl.Put(x)
	}
}

func BenchmarkNoPool(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		x := &payload{items: make([]int, 0, 1024)}
		x.items = append(x.items, 1, 2, 3)
		sink = x
	}
}
