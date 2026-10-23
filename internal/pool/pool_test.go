package pool

import (
	"sync"
	"sync/atomic"
	"testing"
)

type buf struct {
	data   []byte
	resets int
}

func (b *buf) Reset() {
	b.data = b.data[:0]
	b.resets++
}

func newBuf() *buf { return &buf{} }

func TestPutResetsObject(t *testing.T) {
	p := New(newBuf)
	b := p.Get()
	b.data = append(b.data, 1, 2, 3)

	p.Put(b)

	if b.resets != 1 || len(b.data) != 0 {
		t.Errorf("after Put: resets=%d len=%d, want 1 and 0", b.resets, len(b.data))
	}
}

func TestGetReturnsCleanObject(t *testing.T) {
	p := New(newBuf)
	b := p.Get()
	b.data = append(b.data, 1, 2, 3)
	p.Put(b)

	if got := p.Get(); len(got.data) != 0 {
		t.Errorf("Get returned dirty object: %v", got.data)
	}
}

func TestGetCallsConstructorWhenEmpty(t *testing.T) {
	var calls int
	p := New(func() *buf {
		calls++
		return &buf{}
	})

	p.Get()

	if calls != 1 {
		t.Errorf("constructor calls = %d, want 1", calls)
	}
}

func TestConcurrentUse(t *testing.T) {
	var created atomic.Int64
	p := New(func() *buf {
		created.Add(1)
		return &buf{data: make([]byte, 0, 16)}
	})

	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 1000 {
				b := p.Get()
				if len(b.data) != 0 {
					t.Error("dirty object from pool")
				}
				b.data = append(b.data, 1)
				p.Put(b)
			}
		}()
	}
	wg.Wait()

	if created.Load() == 0 {
		t.Error("constructor never called")
	}
}
