package main

import (
	"net"
	"testing"
)

func TestListenerAddress(t *testing.T) {
	tests := []struct {
		name string
		host string
		port int
		want string
	}{
		{name: "hostname", host: "localhost", port: 8080, want: "localhost:8080"},
		{name: "IPv4", host: "127.0.0.1", port: 8080, want: "127.0.0.1:8080"},
		{name: "empty host", host: "", port: 8080, want: ":8080"},
		{name: "IPv6 wildcard", host: "::", port: 8080, want: "[::]:8080"},
		{name: "IPv6 loopback", host: "::1", port: 8080, want: "[::1]:8080"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := listenerAddress(tt.host, tt.port); got != tt.want {
				t.Errorf("listenerAddress(%q, %d) = %q, want %q", tt.host, tt.port, got, tt.want)
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
