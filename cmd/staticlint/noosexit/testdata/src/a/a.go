package main

import (
	"os"
	osalias "os"
)

func helper() {
	os.Exit(2)
}

func main() {
	helper()
	os.Exit(1)      // want "direct call to os.Exit in main is forbidden"
	osalias.Exit(1) // want "direct call to os.Exit in main is forbidden"
	func() {
		os.Exit(3) // want "direct call to os.Exit in main is forbidden"
	}()
}
