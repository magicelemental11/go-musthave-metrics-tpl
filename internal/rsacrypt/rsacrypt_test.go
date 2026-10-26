package rsacrypt

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newKey(t testing.TB) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func TestRoundTrip(t *testing.T) {
	key := newKey(t)
	for name, plain := range map[string][]byte{
		"short":       []byte("hello"),
		"empty":       {},
		"multi block": bytes.Repeat([]byte("metric,"), 5000),
	} {
		t.Run(name, func(t *testing.T) {
			enc, err := Encrypt(&key.PublicKey, plain)
			if err != nil {
				t.Fatal(err)
			}
			if len(plain) > 0 && bytes.Contains(enc, plain) {
				t.Fatal("ciphertext contains plaintext")
			}
			got, err := Decrypt(key, enc)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, plain) {
				t.Fatalf("got %d bytes, want %d", len(got), len(plain))
			}
		})
	}
}

func TestDecryptErrors(t *testing.T) {
	key := newKey(t)
	enc, err := Encrypt(&key.PublicKey, []byte("secret"))
	if err != nil {
		t.Fatal(err)
	}
	flip := func(i int) []byte {
		b := bytes.Clone(enc)
		b[i] ^= 0xff
		return b
	}

	tests := map[string]struct {
		priv *rsa.PrivateKey
		data []byte
	}{
		"wrong key":             {newKey(t), enc},
		"corrupted wrapped key": {key, flip(0)},
		"corrupted nonce":       {key, flip(key.Size())},
		"corrupted tag":         {key, flip(len(enc) - 1)},
		"too short":             {key, enc[:10]},
		"no payload":            {key, enc[:key.Size()]},
		"nil":                   {key, nil},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := Decrypt(tt.priv, tt.data); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestLoadKeys(t *testing.T) {
	key := newKey(t)
	dir := t.TempDir()

	privDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	pubDER, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	privPath := filepath.Join(dir, "private.pem")
	pubPath := filepath.Join(dir, "public.pem")
	badPath := filepath.Join(dir, "bad.pem")
	writePEM(t, privPath, "PRIVATE KEY", privDER)
	writePEM(t, pubPath, "PUBLIC KEY", pubDER)
	if err = os.WriteFile(badPath, []byte("not pem"), 0o600); err != nil {
		t.Fatal(err)
	}

	priv, err := LoadPrivateKey(privPath)
	if err != nil || !priv.Equal(key) {
		t.Fatalf("private key: %v", err)
	}
	pub, err := LoadPublicKey(pubPath)
	if err != nil || !pub.Equal(&key.PublicKey) {
		t.Fatalf("public key: %v", err)
	}

	for name, load := range map[string]func(string) error{
		"private": func(p string) error { _, e := LoadPrivateKey(p); return e },
		"public":  func(p string) error { _, e := LoadPublicKey(p); return e },
	} {
		for _, path := range []string{badPath, filepath.Join(dir, "missing.pem")} {
			if err = load(path); err == nil || !strings.Contains(err.Error(), path) {
				t.Errorf("%s %s: error %v does not mention path", name, path, err)
			}
		}
	}
	if _, err := LoadPublicKey(privPath); err == nil {
		t.Error("public loader accepted private key")
	}
}

func writePEM(t *testing.T, path, typ string, der []byte) {
	t.Helper()
	if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: typ, Bytes: der}), 0o600); err != nil {
		t.Fatal(err)
	}
}
