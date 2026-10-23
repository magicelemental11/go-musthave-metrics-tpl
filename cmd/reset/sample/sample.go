// Package sample holds structs whose Reset methods are produced by cmd/reset
// so the generated code is compiled and tested with the rest of the module.
package sample

//go:generate go run ../../../cmd/reset

// generate:reset
type Child struct {
	Hits int
	Tags []string
}

// generate:reset
type Buffer struct {
	Name    string
	Size    int
	Ratio   float64
	Ready   bool
	Data    []byte
	Index   map[string]int
	Counter *int64
	Child   Child
	Next    *Child
	Fixed   [3]int
	Any     any
}
