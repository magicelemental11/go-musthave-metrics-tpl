package pool_test

import (
	"fmt"

	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/pool"
)

type counter struct {
	n int
}

func (c *counter) Reset() { c.n = 0 }

func ExamplePool() {
	p := pool.New(func() *counter { return &counter{} })

	c := p.Get()
	c.n = 5
	p.Put(c)

	fmt.Println(p.Get().n)
	// Output: 0
}
