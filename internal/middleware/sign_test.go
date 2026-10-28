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

func TestSignKeepsStatus(t *testing.T) {
	h := Sign("k")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNotFound) }))
	if rec := do(h, "", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("got %d", rec.Code)
	}
}

func TestSignCoversUncompressedResponse(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
	})
	h := Gzip(Sign("k")(handler))

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Header().Get("Content-Encoding") != "gzip" {
		t.Fatal("response is not gzipped")
	}
	zr, err := gzip.NewReader(rec.Body)
	if err != nil {
		t.Fatal(err)
	}
	plain, err := io.ReadAll(zr)
	if err != nil {
		t.Fatal(err)
	}
	if !sign.Valid("k", plain, rec.Header().Get(sign.Header)) {
		t.Fatalf("hash does not cover uncompressed body %q", plain)
	}
}

func TestSignVerifiesUncompressedRequest(t *testing.T) {
	var gz bytes.Buffer
	zw := gzip.NewWriter(&gz)
	zw.Write([]byte("hello"))
	zw.Close()

	h := Gzip(Sign("k")(http.HandlerFunc(echo)))
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(gz.Bytes()))
	req.Header.Set("Content-Encoding", "gzip")
	req.Header.Set(sign.Header, sign.Sign("k", []byte("hello")))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK || rec.Body.String() != "hello" {
		t.Fatalf("got %d %q", rec.Code, rec.Body.String())
	}
}

func TestSignRejectsTooLargeBody(t *testing.T) {
	body := strings.Repeat("a", maxSignedBody+1)
	h := Sign("k")(http.HandlerFunc(echo))
	if rec := do(h, body, sign.Sign("k", []byte(body))); rec.Code != http.StatusBadRequest {
		t.Fatalf("got %d", rec.Code)
	}
}
