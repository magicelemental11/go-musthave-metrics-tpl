package rsacrypt

import (
	"bytes"
	"testing"
)

var benchBody = bytes.Repeat([]byte(`{"id":"Alloc","type":"gauge","value":1.5},`), 30)

func BenchmarkEncrypt(b *testing.B) {
	key := newKey(b)
	b.ReportAllocs()
	for b.Loop() {
		if _, err := Encrypt(&key.PublicKey, benchBody); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDecrypt(b *testing.B) {
	key := newKey(b)
	enc, err := Encrypt(&key.PublicKey, benchBody)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for b.Loop() {
		if _, err := Decrypt(key, enc); err != nil {
			b.Fatal(err)
		}
	}
}
