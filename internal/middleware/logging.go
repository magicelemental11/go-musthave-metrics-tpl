package middleware

import (
	"net/http"
	"time"

	"go.uber.org/zap"
)

type responseData struct {
	status int
	size   int
}

type loggingResponseWriter struct {
	http.ResponseWriter
	data *responseData
}

func (w *loggingResponseWriter) Write(b []byte) (int, error) {
	if w.data.status == 0 {
		w.data.status = http.StatusOK
	}
	size, err := w.ResponseWriter.Write(b)
	w.data.size += size
	return size, err
}

func (w *loggingResponseWriter) WriteHeader(status int) {
	w.data.status = status
	w.ResponseWriter.WriteHeader(status)
}

// Logging логирует запрос (URI, метод, время выполнения) и ответ (статус, размер) на уровне Info.
func Logging(logger *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			data := &responseData{}
			next.ServeHTTP(&loggingResponseWriter{ResponseWriter: w, data: data}, r)

			status := data.status
			if status == 0 {
				status = http.StatusOK
			}
			logger.Info("request",
				zap.String("uri", r.RequestURI),
				zap.String("method", r.Method),
				zap.Duration("duration", time.Since(start)),
				zap.Int("status", status),
				zap.Int("size", data.size),
			)
		})
	}
}
