package middleware

import (
	"net/http"

	"github.com/magicelemental11/go-musthave-metrics-tpl/internal/subnet"
)

// TrustedSubnet responds 403 unless the X-Real-IP header holds an address inside the trusted subnet.
// Unlike the audit's clientIP there is no fallback to RemoteAddr: the agent must state its address.
// The header is set by the client, so this guards a deployment behind a proxy, not against a forged header.
// A nil checker disables the check.
func TrustedSubnet(c *subnet.Checker) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if c == nil {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !c.Allowed(r.Header.Get(subnet.Header)) {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
