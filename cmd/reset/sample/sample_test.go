package sample

import "testing"

func filled() *Buffer {
	n := int64(7)
	return &Buffer{
		Name:    "buf",
		Size:    3,
		Ratio:   1.5,
		Ready:   true,
		Data:    append(make([]byte, 0, 64), 1, 2, 3),
		Index:   map[string]int{"a": 1},
		Counter: &n,
		Child:   Child{Hits: 2, Tags: append(make([]string, 0, 8), "x")},
		Next:    &Child{Hits: 5, Tags: []string{"y"}},
		Fixed:   [3]int{1, 2, 3},
		Any:     42,
	}
}

func TestReset(t *testing.T) {
	b := filled()
	counter, next := b.Counter, b.Next
	b.Reset()

	if b.Name != "" || b.Size != 0 || b.Ratio != 0 || b.Ready {
		t.Errorf("primitives not cleared: %+v", b)
	}
	if len(b.Data) != 0 || cap(b.Data) != 64 {
		t.Errorf("Data len=%d cap=%d, want 0 and 64", len(b.Data), cap(b.Data))
	}
	if len(b.Index) != 0 {
		t.Errorf("Index = %v, want empty", b.Index)
	}
	if b.Counter != counter || *b.Counter != 0 {
		t.Errorf("Counter pointer must be kept and zeroed, got %v", *b.Counter)
	}
	if b.Child.Hits != 0 || len(b.Child.Tags) != 0 || cap(b.Child.Tags) != 8 {
		t.Errorf("Child not reset: %+v", b.Child)
	}
	if b.Next != next || next.Hits != 0 || len(next.Tags) != 0 {
		t.Errorf("Next not reset: %+v", next)
	}
	if b.Fixed != [3]int{} {
		t.Errorf("Fixed = %v", b.Fixed)
	}
	if b.Any != nil {
		t.Errorf("Any = %v, want nil", b.Any)
	}
}

func TestResetNilPointers(t *testing.T) {
	b := &Buffer{Name: "x"}
	b.Reset()
	if b.Name != "" || b.Counter != nil || b.Next != nil {
		t.Errorf("unexpected state: %+v", b)
	}
}

func TestResetNilReceiver(t *testing.T) {
	var b *Buffer
	b.Reset()
}
