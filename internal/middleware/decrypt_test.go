package middleware

import (
	"crypto/rand"
	"crypto/rsa"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/rsacrypt"
)

func TestDecrypt(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		w.Write(body)
	})
	enc, err := rsacrypt.Encrypt(&key.PublicKey, []byte(`{"id":"x"}`))
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name       string
		key        *rsa.PrivateKey
		body       string
		header     bool
		wantStatus int
		wantBody   string
	}{
		{"valid", key, string(enc), true, http.StatusOK, `{"id":"x"}`},
		{"garbage", key, "garbage", true, http.StatusBadRequest, ""},
		{"missing header with key", key, `{"id":"x"}`, false, http.StatusBadRequest, ""},
		{"no key passes through", nil, `{"id":"x"}`, false, http.StatusOK, `{"id":"x"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/updates/", strings.NewReader(tt.body))
			if tt.header {
				req.Header.Set(rsacrypt.Header, rsacrypt.HeaderValue)
			}
			rec := httptest.NewRecorder()
			Decrypt(tt.key)(handler).ServeHTTP(rec, req)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status %d, want %d", rec.Code, tt.wantStatus)
			}
			if tt.wantStatus == http.StatusOK && rec.Body.String() != tt.wantBody {
				t.Fatalf("body %q, want %q", rec.Body.String(), tt.wantBody)
			}
		})
	}
}

func TestDecryptSkipsGet(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	Decrypt(key)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {})).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, want 200", rec.Code)
	}
}

func TestDecryptRejectsHugeBody(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/updates/", strings.NewReader(strings.Repeat("x", maxEncryptedBody+1)))
	req.Header.Set(rsacrypt.Header, rsacrypt.HeaderValue)
	rec := httptest.NewRecorder()
	Decrypt(key)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})).ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d, want 400", rec.Code)
	}
}
