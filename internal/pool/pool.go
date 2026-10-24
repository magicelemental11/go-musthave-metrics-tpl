// Package pool provides a typed wrapper around sync.Pool that resets objects on return.
package pool

import (
	"reflect"
	"sync"
)

// Resetter is implemented by objects that can be returned to their initial state.
type Resetter interface {
	Reset()
}

// Pool is a typed sync.Pool that calls Reset on every object put back.
type Pool[T Resetter] struct {
	inner sync.Pool
}

// New returns a Pool that creates objects with newFn when it is empty.
func New[T Resetter](newFn func() T) *Pool[T] {
	return &Pool[T]{inner: sync.Pool{New: func() any { return newFn() }}}
}

// Get returns a pooled object or a new one created by the constructor.
func (p *Pool[T]) Get() T {
	return p.inner.Get().(T)
}

// Put resets x and makes it available for reuse. x must not be used afterwards.
// A nil x is ignored.
func (p *Pool[T]) Put(x T) {
	if isNil(x) {
		return
	}
	x.Reset()
	p.inner.Put(x)
}

func isNil(x any) bool {
	v := reflect.ValueOf(x)
	return !v.IsValid() || v.Kind() == reflect.Pointer && v.IsNil()
}
