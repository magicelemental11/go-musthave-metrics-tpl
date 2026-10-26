package middleware

import (
	"bytes"
	"crypto/rsa"
	"io"
	"net/http"

	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/rsacrypt"
)

const maxEncryptedBody = 2 << 20 // twice the 1 MiB payload limit: room for the RSA-wrapped key, nonce and tag

// Decrypt decrypts request bodies before Sign sees them. GET and HEAD pass through;
// any other request that is not encrypted or cannot be decrypted gets 400. A nil key disables it.
func Decrypt(key *rsa.PrivateKey) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if key == nil {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet || r.Method == http.MethodHead {
				next.ServeHTTP(w, r)
				return
			}
			if r.Header.Get(rsacrypt.Header) != rsacrypt.HeaderValue {
				http.Error(w, "bad request", http.StatusBadRequest)
				return
			}
			body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxEncryptedBody))
			if err != nil {
				http.Error(w, "bad request", http.StatusBadRequest)
				return
			}
			plain, err := rsacrypt.Decrypt(key, body)
			if err != nil {
				http.Error(w, "bad request", http.StatusBadRequest)
				return
			}
			r.Body = io.NopCloser(bytes.NewReader(plain))
			r.ContentLength = int64(len(plain))
			r.Header.Del(rsacrypt.Header)
			next.ServeHTTP(w, r)
		})
	}
}
