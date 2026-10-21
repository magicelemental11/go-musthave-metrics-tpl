// Package sign computes and verifies HMAC-SHA256 signatures of message bodies.
package sign

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
)

// Header is the HTTP header that carries the hex-encoded signature.
const Header = "HashSHA256"

func mac(key string, data []byte) []byte {
	h := hmac.New(sha256.New, []byte(key))
	h.Write(data)
	return h.Sum(nil)
}

// Sign returns the hex-encoded HMAC-SHA256 of data under key.
func Sign(key string, data []byte) string {
	return base64.StdEncoding.EncodeToString(mac(key, data))
}

// Valid reports whether got is the correct signature of data under key.
func Valid(key string, data []byte, got string) bool {
	gotBytes, err := base64.StdEncoding.DecodeString(got)
	if err != nil {
		return false
	}
	return hmac.Equal(gotBytes, mac(key, data))
}
