package sign

import (
	"bytes"
	"testing"
)

func BenchmarkSign(b *testing.B) {
	data := bytes.Repeat([]byte("x"), 2048)
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	for i := 0; i < b.N; i++ {
		Sign("key", data)
	}
}

func BenchmarkValid(b *testing.B) {
	data := bytes.Repeat([]byte("x"), 2048)
	got := Sign("key", data)
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	for i := 0; i < b.N; i++ {
		Valid("key", data, got)
	}
}
