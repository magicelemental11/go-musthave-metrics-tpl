package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/subnet"
)

func TestTrustedSubnet(t *testing.T) {
	trusted, err := subnet.New("10.0.0.0/8")
	if err != nil {
		t.Fatal(err)
	}
	ok := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})

	tests := []struct {
		name       string
		checker    *subnet.Checker
		ip         string
		wantStatus int
	}{
		{"inside", trusted, "10.1.2.3", http.StatusOK},
		{"outside", trusted, "192.168.1.1", http.StatusForbidden},
		{"missing header", trusted, "", http.StatusForbidden},
		{"empty subnet passes without header", nil, "", http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/updates/", nil)
			if tt.ip != "" {
				req.Header.Set(subnet.Header, tt.ip)
			}
			rec := httptest.NewRecorder()
			TrustedSubnet(tt.checker)(ok).ServeHTTP(rec, req)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status %d, want %d", rec.Code, tt.wantStatus)
			}
		})
	}
}

func TestTrustedSubnetIgnoresRemoteAddr(t *testing.T) {
	trusted, err := subnet.New("192.0.2.0/24")
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/updates/", nil) // RemoteAddr is 192.0.2.1
	rec := httptest.NewRecorder()
	TrustedSubnet(trusted)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})).ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status %d, want 403: only %s is trusted", rec.Code, subnet.Header)
	}
}
