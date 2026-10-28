// Package subnet checks that a client address belongs to the trusted subnet.
package subnet

import (
	"fmt"
	"net/netip"
)

// Header carries the agent's IP address in HTTP requests.
const Header = "X-Real-IP"

// Checker reports whether an IP address is inside the trusted subnet.
// A nil Checker trusts every address.
type Checker struct {
	prefix netip.Prefix
}

// New parses cidr such as "192.168.1.0/24". An empty cidr returns a nil Checker, which allows everything.
func New(cidr string) (*Checker, error) {
	if cidr == "" {
		return nil, nil
	}
	prefix, err := netip.ParsePrefix(cidr)
	if err != nil {
		return nil, fmt.Errorf("trusted subnet: %w", err)
	}
	// Allowed unmaps addresses, so ::ffff:10.0.0.0/104 must become 10.0.0.0/8 to match them.
	if prefix.Addr().Is4In6() && prefix.Bits() >= 96 {
		prefix = netip.PrefixFrom(prefix.Addr().Unmap(), prefix.Bits()-96)
	}
	return &Checker{prefix: prefix.Masked()}, nil
}

// Allowed reports whether ip is inside the subnet. An empty or malformed ip is not allowed.
func (c *Checker) Allowed(ip string) bool {
	if c == nil {
		return true
	}
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return false
	}
	return c.prefix.Contains(addr.Unmap())
}
