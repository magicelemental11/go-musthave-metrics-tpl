package subnet_test

import (
	"fmt"

	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/subnet"
)

func ExampleChecker_Allowed() {
	c, err := subnet.New("192.168.1.0/24")
	if err != nil {
		panic(err)
	}
	fmt.Println(c.Allowed("192.168.1.10"))
	fmt.Println(c.Allowed("10.0.0.1"))

	var open *subnet.Checker // an empty trusted_subnet
	fmt.Println(open.Allowed("10.0.0.1"))
	// Output:
	// true
	// false
	// true
}
