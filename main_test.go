package main

import (
	"net"
	"testing"
)

func TestListenerAddress(t *testing.T) {
	tests := map[string]struct {
		host string
		want string
	}{
		"hostname":      {host: "localhost", want: "localhost:8080"},
		"IPv4":          {host: "127.0.0.1", want: "127.0.0.1:8080"},
		"empty host":    {host: "", want: ":8080"},
		"IPv6 wildcard": {host: "::", want: "[::]:8080"},
		"IPv6 loopback": {host: "::1", want: "[::1]:8080"},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if got := listenerAddress(tt.host, 8080); got != tt.want {
				t.Errorf("listenerAddress(%q, 8080) = %q, want %q", tt.host, got, tt.want)
			}
		})
	}
}

func TestIPv6LoopbackListener(t *testing.T) {
	listenerConfig := new(net.ListenConfig)

	listener, err := listenerConfig.Listen(t.Context(), "tcp", listenerAddress("::1", 0))
	if err != nil {
		t.Skipf("IPv6 loopback is unavailable: %v", err)
	}
	defer listener.Close()

	dialer := new(net.Dialer)

	conn, err := dialer.DialContext(t.Context(), "tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("dial IPv6 loopback listener: %v", err)
	}

	err = conn.Close()
	if err != nil {
		t.Errorf("close IPv6 loopback connection: %v", err)
	}
}
