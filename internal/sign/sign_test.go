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

func TestSignIsBase64(t *testing.T) {
	if got := len(Sign("k", []byte("body"))); got != 44 {
		t.Fatalf("len %d, want 44", got)
	}
}

func TestValid(t *testing.T) {
	h := Sign("k", []byte("body"))
	if !Valid("k", []byte("body"), h) {
		t.Fatal("valid hash rejected")
	}
	if Valid("k", []byte("tampered"), h) || Valid("x", []byte("body"), h) || Valid("k", []byte("body"), "!!") {
		t.Fatal("invalid hash accepted")
	}
}
