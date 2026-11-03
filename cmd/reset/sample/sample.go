// Package sample holds structs whose Reset methods are produced by cmd/reset
// so the generated code is compiled and tested with the rest of the module.
package sample

//go:generate go run ../../../cmd/reset

// generate:reset
type Child struct {
	Hits int
	Tags []string
}

// CounterRef is a named pointer type: Reset must zero the value it points to.
type CounterRef *int64

// ChildRef is a named pointer to a struct with a Reset method.
type ChildRef *Child

// generate:reset
type Buffer struct {
	Name    string
	Size    int
	Ratio   float64
	Ready   bool
	Data    []byte
	Index   map[string]int
	Counter *int64
	Total   CounterRef
	Last    ChildRef
	Child   Child
	Next    *Child
	Fixed   [3]int
	Any     any
}
