// Command reset generates Reset methods for structs marked with a
// "// generate:reset" comment and writes them to reset.gen.go next to the
// marked types. It is meant to be run through go:generate.
//
// Fields are reset as follows: numbers, strings and bools become zero; slices
// are truncated to length zero and maps are cleared, keeping their storage;
// non-nil pointers are dereferenced and reset in place; values and pointers
// whose type has a Reset method get it called; arrays and structs without
// Reset are replaced by their zero literal; interfaces, channels and funcs
// become nil, and so does a pointer to the struct being generated.
package main
