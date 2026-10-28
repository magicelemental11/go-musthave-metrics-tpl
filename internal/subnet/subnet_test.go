package subnet

import "testing"

func TestChecker(t *testing.T) {
	tests := []struct {
		name string
		cidr string
		ip   string
		want bool
	}{
		{"inside", "192.168.1.0/24", "192.168.1.10", true},
		{"outside", "192.168.1.0/24", "192.168.2.10", false},
		{"first address", "192.168.1.0/24", "192.168.1.0", true},
		{"last address", "192.168.1.0/24", "192.168.1.255", true},
		{"next after last", "192.168.1.0/24", "192.168.2.0", false},
		{"ipv4-mapped ipv6", "10.0.0.0/8", "::ffff:10.1.2.3", true},
		{"ipv4-mapped subnet", "::ffff:10.0.0.0/104", "10.1.2.3", true},
		{"ipv4-mapped subnet outside", "::ffff:10.0.0.0/104", "11.1.2.3", false},
		{"ipv6 inside", "2001:db8::/32", "2001:db8::1", true},
		{"ipv6 outside", "2001:db8::/32", "2001:db9::1", false},
		{"empty ip", "10.0.0.0/8", "", false},
		{"garbage ip", "10.0.0.0/8", "not-an-ip", false},
		{"ip with port", "10.0.0.0/8", "10.0.0.1:8080", false},
		{"empty cidr allows anything", "", "", true},
		{"empty cidr allows garbage", "", "not-an-ip", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := New(tt.cidr)
			if err != nil {
				t.Fatal(err)
			}
			if got := c.Allowed(tt.ip); got != tt.want {
				t.Fatalf("Allowed(%q) in %q = %v, want %v", tt.ip, tt.cidr, got, tt.want)
			}
		})
	}
}

func TestNewInvalid(t *testing.T) {
	for _, cidr := range []string{"10.0.0.1", "10.0.0.0/33", "garbage"} {
		if _, err := New(cidr); err == nil {
			t.Errorf("New(%q): want error", cidr)
		}
	}
}
