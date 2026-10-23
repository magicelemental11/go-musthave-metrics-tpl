package main

import "os"

type T struct{}

func (T) main() {
	os.Exit(1)
}

func main() {
	_ = os.Args
}
