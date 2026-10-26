package sign_test

import (
	"fmt"

	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/sign"
)

func ExampleSign() {
	body := []byte(`{"id":"Alloc","type":"gauge","value":1.5}`)
	signature := sign.Sign("secret", body)
	fmt.Println(sign.Valid("secret", body, signature))
	fmt.Println(sign.Valid("other", body, signature))
	// Output:
	// true
	// false
}
