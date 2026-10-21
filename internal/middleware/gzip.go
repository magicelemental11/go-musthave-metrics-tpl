// Package middleware provides HTTP middleware for logging, gzip compression and request signing.
package middleware

import (
	"compress/gzip"
	"errors"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
)

var (
	writerPool = sync.Pool{New: func() any { return gzip.NewWriter(io.Discard) }}
	readerPool = sync.Pool{New: func() any { return new(gzip.Reader) }}
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
		w.Header().Del("Content-Length")
		w.gz = writerPool.Get().(*gzip.Writer)
		w.gz.Reset(w.ResponseWriter)
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
	if w.gz == nil {
		return nil
	}
	err := w.gz.Close()
	writerPool.Put(w.gz)
	w.gz = nil
	return err
}

// Gzip decompresses requests with Content-Encoding: gzip and compresses
// application/json and text/html responses if the client sent Accept-Encoding: gzip.
func Gzip(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.Header.Get("Content-Encoding"), "gzip") {
			zr := readerPool.Get().(*gzip.Reader)
			defer readerPool.Put(zr)
			err := zr.Reset(r.Body)
			switch {
			case err == nil:
				defer zr.Close()
				r.Body = zr
			case !errors.Is(err, io.EOF): // leave an empty body as is
				http.Error(w, "invalid gzip body", http.StatusBadRequest)
				return
			}
		}

		w.Header().Add("Vary", "Accept-Encoding")

		if strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			cw := &compressWriter{ResponseWriter: w}
			defer func() {
				if err := cw.close(); err != nil {
					log.Printf("gzip close: %v", err)
				}
			}()
			w = cw
		}

		next.ServeHTTP(w, r)
	})
}
