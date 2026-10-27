package sign

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
)

const Header = "HashSHA256"

func mac(key string, data []byte) []byte {
	h := hmac.New(sha256.New, []byte(key))
	h.Write(data)
	return h.Sum(nil)
}

func Sign(key string, data []byte) string {
	return base64.StdEncoding.EncodeToString(mac(key, data))
}

func Valid(key string, data []byte, got string) bool {
	gotBytes, err := base64.StdEncoding.DecodeString(got)
	if err != nil {
		return false
	}
	return hmac.Equal(gotBytes, mac(key, data))
}
