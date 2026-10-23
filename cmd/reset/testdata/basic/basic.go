package basic

import (
	"sync"
	"time"
)

type Inner struct {
	N int
}

// generate:reset
type Leaf struct {
	Count int
}

type Plain struct {
	A int
}

type Celsius float64

type Runner interface {
	Run()
	Reset()
}

// generate:reset
type Sample struct {
	Int     int
	Uint8   uint8
	Float   float64
	Complex complex128
	Flag    bool
	Text    string
	Rune    rune
	Byte    byte
	Temp    Celsius
	Wait    time.Duration
	Start   time.Time
	Mu      sync.Mutex
	Items   []string
	Index   map[string]int
	Ptr     *int
	PtrPtr  **int
	Leaf    Leaf
	LeafPtr *Leaf
	Plain   Plain
	Inner   Inner
	Fixed   [4]byte
	Any     any
	Runner  Runner
	Ch      chan int
	Fn      func()
	Anon    struct{ X int }
	Leaf2
	_ int
}

type (
	// generate:reset
	Leaf2 struct {
		Tags []string
	}

	NotMarked struct {
		S []int
	}
)

// generate:reset
type NotAStruct int

// generate:reset
type Node struct {
	Val  int
	Next *Node
}
