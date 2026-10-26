package main

import (
	"path/filepath"
	"testing"

	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/rsacrypt"
)

func TestRun(t *testing.T) {
	dir := t.TempDir()
	priv := filepath.Join(dir, "private.pem")
	pub := filepath.Join(dir, "public.pem")
	if err := run(priv, pub, 2048); err != nil {
		t.Fatal(err)
	}
	privKey, err := rsacrypt.LoadPrivateKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	pubKey, err := rsacrypt.LoadPublicKey(pub)
	if err != nil {
		t.Fatal(err)
	}
	enc, err := rsacrypt.Encrypt(pubKey, []byte("ping"))
	if err != nil {
		t.Fatal(err)
	}
	if got, err := rsacrypt.Decrypt(privKey, enc); err != nil || string(got) != "ping" {
		t.Fatalf("got %q, %v", got, err)
	}
}
