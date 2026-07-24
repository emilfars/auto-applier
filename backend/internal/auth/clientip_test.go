package auth

import (
	"net/http"
	"testing"
)

func reqWith(remote, xff string) *http.Request {
	r := &http.Request{RemoteAddr: remote, Header: http.Header{}}
	if xff != "" {
		r.Header.Set("X-Forwarded-For", xff)
	}
	return r
}

func TestClientIP_TrustedProxyHops(t *testing.T) {
	tests := []struct {
		name   string
		hops   int
		remote string
		xff    string
		want   string
	}{
		{
			name:   "default (0 hops) ignores XFF and uses peer",
			hops:   0,
			remote: "10.0.0.9:53344",
			xff:    "1.2.3.4, 5.6.7.8",
			want:   "10.0.0.9",
		},
		{
			name:   "0 hops: spoofed XFF cannot change the key",
			hops:   0,
			remote: "203.0.113.5:40000",
			xff:    "attacker-spoofed",
			want:   "203.0.113.5",
		},
		{
			name:   "1 hop returns the address the LB saw (rightmost XFF)",
			hops:   1,
			remote: "10.0.0.1:8080", // the load balancer
			xff:    "198.51.100.7, 172.16.0.2",
			want:   "172.16.0.2",
		},
		{
			name:   "2 hops walks one further left",
			hops:   2,
			remote: "10.0.0.1:8080",
			xff:    "198.51.100.7, 172.16.0.2",
			want:   "198.51.100.7",
		},
		{
			name:   "1 hop with a single client entry",
			hops:   1,
			remote: "10.0.0.1:8080",
			xff:    "198.51.100.7",
			want:   "198.51.100.7",
		},
		{
			name:   "hops beyond the chain clamps to the claimed client",
			hops:   5,
			remote: "10.0.0.1:8080",
			xff:    "198.51.100.7, 172.16.0.2",
			want:   "198.51.100.7",
		},
		{
			name:   "trusted hops but no XFF falls back to peer",
			hops:   1,
			remote: "203.0.113.9:1234",
			xff:    "",
			want:   "203.0.113.9",
		},
		{
			name:   "IPv6 XFF entry is preserved",
			hops:   1,
			remote: "10.0.0.1:8080",
			xff:    "2001:db8::1",
			want:   "2001:db8::1",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := &Service{cfg: Config{TrustedProxyHops: tc.hops}}
			if got := s.clientIP(reqWith(tc.remote, tc.xff)); got != tc.want {
				t.Fatalf("clientIP = %q, want %q", got, tc.want)
			}
		})
	}
}

// A spoofed XFF must not let an attacker rotate the limiter key when no proxy is
// trusted — otherwise password-spray/lockout protections are trivially bypassed.
func TestClientIP_SpoofResistantByDefault(t *testing.T) {
	s := &Service{cfg: Config{TrustedProxyHops: 0}}
	a := s.clientIP(reqWith("203.0.113.5:5000", "9.9.9.9"))
	b := s.clientIP(reqWith("203.0.113.5:5001", "8.8.8.8"))
	if a != b {
		t.Fatalf("same peer produced different keys (%q vs %q); XFF was trusted when it should not be", a, b)
	}
}
