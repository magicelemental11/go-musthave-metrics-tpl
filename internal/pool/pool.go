// Package pool provides a typed wrapper around sync.Pool that resets objects on return.
package pool

import "sync"

// Resetter is implemented by objects that can be returned to their initial state.
type Resetter interface {
	Reset()
}

// Pool is a typed sync.Pool that calls Reset on every object put back.
// Objects are stored as pointers: P is *T, so Put can check for nil without reflection.
type Pool[T any, P interface {
	*T
	Resetter
}] struct {
	inner sync.Pool
}

// New returns a Pool that creates objects with newFn when it is empty.
func New[T any, P interface {
	*T
	Resetter
}](newFn func() P) *Pool[T, P] {
	return &Pool[T, P]{inner: sync.Pool{New: func() any { return newFn() }}}
}

// Get returns a pooled object or a new one created by the constructor.
func (p *Pool[T, P]) Get() P {
	return p.inner.Get().(P)
}

// Put resets x and makes it available for reuse. x must not be used afterwards.
// A nil x is ignored.
func (p *Pool[T, P]) Put(x P) {
	if x == nil {
		return
	}
	x.Reset()
	p.inner.Put(x)
}
