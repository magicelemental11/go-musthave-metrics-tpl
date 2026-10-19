package sign

import "testing"

func TestSign(t *testing.T) {
	a := Sign("k", []byte("body"))
	if a != Sign("k", []byte("body")) {
		t.Fatal("not stable")
	}
	if a == Sign("other", []byte("body")) {
		t.Fatal("key ignored")
	}
	if Sign("k", nil) == "" {
		t.Fatal("empty body must still hash")
	}
}

func TestValid(t *testing.T) {
	h := Sign("k", []byte("body"))
	if !Valid("k", []byte("body"), h) {
		t.Fatal("valid hash rejected")
	}
	if Valid("k", []byte("tampered"), h) || Valid("x", []byte("body"), h) || Valid("k", []byte("body"), "zz") {
		t.Fatal("invalid hash accepted")
	}
}
