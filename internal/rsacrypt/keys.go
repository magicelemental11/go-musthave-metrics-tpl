package rsacrypt

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
)

func LoadPublicKey(path string) (*rsa.PublicKey, error) {
	der, err := readPEM(path)
	if err != nil {
		return nil, err
	}
	parsed, err := x509.ParsePKIXPublicKey(der)
	if err != nil {
		return nil, fmt.Errorf("parse public key %s: %w", path, err)
	}
	pub, ok := parsed.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("%s: not an RSA public key", path)
	}
	return pub, nil
}

// LoadPrivateKey accepts PKCS#8 (what keygen writes) and PKCS#1 (openssl genrsa).
func LoadPrivateKey(path string) (*rsa.PrivateKey, error) {
	der, err := readPEM(path)
	if err != nil {
		return nil, err
	}
	parsed, pkcs8Err := x509.ParsePKCS8PrivateKey(der)
	if pkcs8Err == nil {
		priv, ok := parsed.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("%s: not an RSA private key", path)
		}
		return priv, nil
	}
	priv, err := x509.ParsePKCS1PrivateKey(der)
	if err != nil {
		return nil, fmt.Errorf("parse private key %s: pkcs8: %v; pkcs1: %w", path, pkcs8Err, err)
	}
	return priv, nil
}

func readPEM(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read key %s: %w", path, err)
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, errors.New("no PEM data in " + path)
	}
	return block.Bytes, nil
}
