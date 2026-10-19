package middleware

import (
	"compress/gzip"
	"net/http"
	"strings"
)

func shouldCompress(contentType string) bool {
	return strings.HasPrefix(contentType, "application/json") ||
		strings.HasPrefix(contentType, "text/html")
}

type compressWriter struct {
	http.ResponseWriter
	gz          *gzip.Writer
	wroteHeader bool
}

func (w *compressWriter) WriteHeader(status int) {
	if w.wroteHeader {
		return
	}
	w.wroteHeader = true
	if shouldCompress(w.Header().Get("Content-Type")) && w.Header().Get("Content-Encoding") == "" {
		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Add("Vary", "Accept-Encoding")
		w.Header().Del("Content-Length")
		w.gz = gzip.NewWriter(w.ResponseWriter)
	}
	w.ResponseWriter.WriteHeader(status)
}

func (w *compressWriter) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	if w.gz != nil {
		return w.gz.Write(b)
	}
	return w.ResponseWriter.Write(b)
}

func (w *compressWriter) close() error {
	if w.gz != nil {
		return w.gz.Close()
	}
	return nil
}

// Gzip распаковывает входящие запросы с Content-Encoding: gzip и сжимает ответы
// application/json и text/html для клиентов, которые прислали Accept-Encoding: gzip.
func Gzip(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.Header.Get("Content-Encoding"), "gzip") {
			zr, err := gzip.NewReader(r.Body)
			if err != nil {
				http.Error(w, "invalid gzip body", http.StatusBadRequest)
				return
			}
			defer zr.Close()
			r.Body = zr
		}

		if strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			cw := &compressWriter{ResponseWriter: w}
			defer cw.close()
			w = cw
		}

		next.ServeHTTP(w, r)
	})
}
