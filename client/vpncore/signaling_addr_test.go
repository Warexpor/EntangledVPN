package vpncore

import "testing"

func TestNormalizeWSAddr(t *testing.T) {
	cases := []struct {
		in, scheme, host string
	}{
		{"192.0.2.10:8080", "ws", "192.0.2.10:8080"},
		{"https://192.0.2.10:8080", "ws", "192.0.2.10:8080"},
		{"http://192.0.2.10:8080", "ws", "192.0.2.10:8080"},
		{"ws://192.0.2.10:8080", "ws", "192.0.2.10:8080"},
		{"wss://example.com", "wss", "example.com"},
		{"https://example.com", "wss", "example.com"},
		{"https://example.com:443", "wss", "example.com:443"},
		{"example.com:443", "wss", "example.com:443"},
		{"https://host:8080/path", "ws", "host:8080"},
	}
	for _, c := range cases {
		scheme, host := normalizeWSAddr(c.in)
		if scheme != c.scheme || host != c.host {
			t.Errorf("normalizeWSAddr(%q) = %q,%q; want %q,%q", c.in, scheme, host, c.scheme, c.host)
		}
	}
}
