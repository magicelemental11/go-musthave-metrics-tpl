// Package rsacrypt encrypts message bodies with AES-256-GCM and wraps the AES key with RSA-OAEP,
// because RSA alone cannot encrypt a body larger than one block.
package rsacrypt

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"errors"
	"fmt"
)

const aesKeySize = 32

// Encrypt returns RSA(aesKey) || nonce || AES-GCM(plaintext).
func Encrypt(pub *rsa.PublicKey, plaintext []byte) ([]byte, error) {
	aesKey := make([]byte, aesKeySize)
	if _, err := rand.Read(aesKey); err != nil {
		return nil, fmt.Errorf("aes key: %w", err)
	}
	wrapped, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, pub, aesKey, nil)
	if err != nil {
		return nil, fmt.Errorf("encrypt key: %w", err)
	}
	gcm, err := newGCM(aesKey)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("nonce: %w", err)
	}
	out := append(wrapped, nonce...)
	return gcm.Seal(out, nonce, plaintext, nil), nil
}

func Decrypt(priv *rsa.PrivateKey, data []byte) ([]byte, error) {
	if len(data) < priv.Size() {
		return nil, errors.New("ciphertext too short")
	}
	aesKey, err := rsa.DecryptOAEP(sha256.New(), nil, priv, data[:priv.Size()], nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt key: %w", err)
	}
	gcm, err := newGCM(aesKey)
	if err != nil {
		return nil, err
	}
	rest := data[priv.Size():]
	if len(rest) < gcm.NonceSize() {
		return nil, errors.New("ciphertext too short")
	}
	plain, err := gcm.Open(nil, rest[:gcm.NonceSize()], rest[gcm.NonceSize():], nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt body: %w", err)
	}
	return plain, nil
}

func newGCM(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("aes: %w", err)
	}
	return cipher.NewGCM(block)
}

const (
	Header      = "X-Encrypted"
	HeaderValue = "rsa"
)
