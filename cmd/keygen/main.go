// Command keygen writes an RSA key pair: a PKCS#8 private key and a PKIX public key, both PEM-encoded.
package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"flag"
	"log"
	"os"
)

func main() {
	private := flag.String("private", "private.pem", "private key output path")
	public := flag.String("public", "public.pem", "public key output path")
	bits := flag.Int("bits", 4096, "RSA key size")
	flag.Parse()

	if err := run(*private, *public, *bits); err != nil {
		log.Fatal(err)
	}
}

func run(privatePath, publicPath string, bits int) error {
	key, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		return err
	}
	privDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return err
	}
	pubDER, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		return err
	}
	if err = writePEM(privatePath, "PRIVATE KEY", privDER, 0o600); err != nil {
		return err
	}
	return writePEM(publicPath, "PUBLIC KEY", pubDER, 0o644)
}

func writePEM(path, blockType string, der []byte, perm os.FileMode) error {
	return os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: blockType, Bytes: der}), perm)
}
