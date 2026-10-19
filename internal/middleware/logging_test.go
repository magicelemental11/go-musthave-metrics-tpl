package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestLogging(t *testing.T) {
	core, logs := observer.New(zap.InfoLevel)
	h := Logging(zap.New(core))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		w.Write([]byte("hello"))
	}))

	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/update/gauge/a/1", nil))

	entries := logs.All()
	if len(entries) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(entries))
	}
	e := entries[0]
	if e.Level != zap.InfoLevel {
		t.Errorf("level = %v", e.Level)
	}
	f := e.ContextMap()
	if f["uri"] != "/update/gauge/a/1" || f["method"] != "POST" || f["status"] != int64(418) || f["size"] != int64(5) {
		t.Errorf("unexpected fields: %v", f)
	}
	if _, ok := f["duration"]; !ok {
		t.Error("duration field missing")
	}
}

func TestLoggingDefaultStatus(t *testing.T) {
	core, logs := observer.New(zap.InfoLevel)
	h := Logging(zap.New(core))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	if got := logs.All()[0].ContextMap()["status"]; got != int64(200) {
		t.Errorf("status = %v", got)
	}
}
