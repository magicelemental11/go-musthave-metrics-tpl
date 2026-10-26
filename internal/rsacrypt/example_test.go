package rsacrypt_test

import (
	"crypto/rand"
	"crypto/rsa"
	"fmt"

	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/rsacrypt"
)

func ExampleEncrypt() {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)

	enc, _ := rsacrypt.Encrypt(&key.PublicKey, []byte("hello"))
	plain, _ := rsacrypt.Decrypt(key, enc)

	fmt.Println(string(plain))
	// Output: hello
}
