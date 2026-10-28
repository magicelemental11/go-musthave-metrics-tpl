package middleware

import (
	"bytes"
	"io"
	"net/http"

	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/sign"
)

type signWriter struct {
	http.ResponseWriter
	status int
	buf    bytes.Buffer
}

func (w *signWriter) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
}

func (w *signWriter) Write(b []byte) (int, error) {
	return w.buf.Write(b)
}

const maxSignedBody = 1 << 20

// Sign ставится после Gzip, поэтому проверяет и подписывает несжатые данные.
func Sign(key string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if got := r.Header.Get(sign.Header); got != "" {
				body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxSignedBody))
				if err != nil || !sign.Valid(key, body, got) {
					http.Error(w, "invalid signature", http.StatusBadRequest)
					return
				}
				r.Body = io.NopCloser(bytes.NewReader(body))
			}

			sw := &signWriter{ResponseWriter: w}
			next.ServeHTTP(sw, r)

			if sw.status == 0 {
				sw.status = http.StatusOK
			}
			w.Header().Set(sign.Header, sign.Sign(key, sw.buf.Bytes()))
			w.WriteHeader(sw.status)
			w.Write(sw.buf.Bytes())
		})
	}
}
