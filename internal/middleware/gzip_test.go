package middleware

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func gzipped(t *testing.T, s string) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write([]byte(s)); err != nil {
		t.Fatal(err)
	}
	zw.Close()
	return &buf
}

func echoHandler(contentType string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		w.Header().Set("Content-Type", contentType)
		w.Write(body)
	})
}

func TestGzipCompressesResponse(t *testing.T) {
	for _, ct := range []string{"application/json", "text/html"} {
		req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{"a":1}`))
		req.Header.Set("Accept-Encoding", "gzip")
		rr := httptest.NewRecorder()
		Gzip(echoHandler(ct)).ServeHTTP(rr, req)

		if rr.Header().Get("Content-Encoding") != "gzip" {
			t.Fatalf("%s: response is not gzip-encoded", ct)
		}
		zr, err := gzip.NewReader(rr.Body)
		if err != nil {
			t.Fatal(err)
		}
		got, _ := io.ReadAll(zr)
		if string(got) != `{"a":1}` {
			t.Errorf("%s: unexpected body %q", ct, got)
		}
	}
}

func TestGzipSkipsOtherContentTypesAndClientsWithoutSupport(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString("hi"))
	req.Header.Set("Accept-Encoding", "gzip")
	rr := httptest.NewRecorder()
	Gzip(echoHandler("text/plain")).ServeHTTP(rr, req)
	if rr.Header().Get("Content-Encoding") != "" || rr.Body.String() != "hi" {
		t.Errorf("text/plain must not be compressed")
	}

	req = httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString("hi"))
	rr = httptest.NewRecorder()
	Gzip(echoHandler("application/json")).ServeHTTP(rr, req)
	if rr.Header().Get("Content-Encoding") != "" || rr.Body.String() != "hi" {
		t.Errorf("response must be plain without Accept-Encoding")
	}
}

func TestGzipDecompressesRequest(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", gzipped(t, `{"id":"x"}`))
	req.Header.Set("Content-Encoding", "gzip")
	rr := httptest.NewRecorder()
	Gzip(echoHandler("application/json")).ServeHTTP(rr, req)
	if rr.Body.String() != `{"id":"x"}` {
		t.Errorf("unexpected body %q", rr.Body.String())
	}
}

func TestGzipInvalidRequestBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString("not gzip"))
	req.Header.Set("Content-Encoding", "gzip")
	rr := httptest.NewRecorder()
	Gzip(echoHandler("application/json")).ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("got %d, want 400", rr.Code)
	}
}
