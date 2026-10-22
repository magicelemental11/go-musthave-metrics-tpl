package middleware

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/sign"
)

func echo(w http.ResponseWriter, r *http.Request) {
	b, _ := io.ReadAll(r.Body)
	w.Write(b)
}

func do(h http.Handler, body, hash string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	if hash != "" {
		req.Header.Set(sign.Header, hash)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestSignValidRequest(t *testing.T) {
	h := Sign("k")(http.HandlerFunc(echo))
	rec := do(h, "hello", sign.Sign("k", []byte("hello")))
	if rec.Code != http.StatusOK || rec.Body.String() != "hello" {
		t.Fatalf("got %d %q", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get(sign.Header); got != sign.Sign("k", []byte("hello")) {
		t.Fatalf("response hash %q", got)
	}
}

func TestSignInvalidRequest(t *testing.T) {
	called := false
	h := Sign("k")(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))
	rec := do(h, "hello", sign.Sign("other", []byte("hello")))
	if rec.Code != http.StatusBadRequest || called {
		t.Fatalf("got %d, handler called=%v", rec.Code, called)
	}
}

func TestSignWithoutHeaderPasses(t *testing.T) {
	h := Sign("k")(http.HandlerFunc(echo))
	rec := do(h, "hello", "")
	if rec.Code != http.StatusOK || rec.Body.String() != "hello" {
		t.Fatalf("got %d %q", rec.Code, rec.Body.String())
	}
}

func TestSignWithoutKeyDisabled(t *testing.T) {
	h := Sign("")(http.HandlerFunc(echo))
	rec := do(h, "hello", "bad")
	if rec.Code != http.StatusOK || rec.Header().Get(sign.Header) != "" {
		t.Fatalf("got %d, header %q", rec.Code, rec.Header().Get(sign.Header))
	}
}

func TestSignKeepsStatus(t *testing.T) {
	h := Sign("k")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNotFound) }))
	if rec := do(h, "", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("got %d", rec.Code)
	}
}

func TestSignCoversGzippedResponse(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
	})
	h := Sign("k")(Gzip(handler))

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Header().Get("Content-Encoding") != "gzip" {
		t.Fatal("response is not gzipped")
	}
	if got, want := rec.Header().Get(sign.Header), sign.Sign("k", rec.Body.Bytes()); got != want {
		t.Fatalf("hash %q does not cover gzipped bytes, want %q", got, want)
	}
}

func TestSignVerifiesGzippedRequest(t *testing.T) {
	var gz bytes.Buffer
	zw := gzip.NewWriter(&gz)
	zw.Write([]byte("hello"))
	zw.Close()
	wire := gz.Bytes()

	h := Sign("k")(Gzip(http.HandlerFunc(echo)))
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(wire))
	req.Header.Set("Content-Encoding", "gzip")
	req.Header.Set(sign.Header, sign.Sign("k", wire))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK || rec.Body.String() != "hello" {
		t.Fatalf("got %d %q", rec.Code, rec.Body.String())
	}
}
