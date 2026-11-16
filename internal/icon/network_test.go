package icon

import (
	"bufio"
	"context"
	"errors"
	"net"
	"net/http"
	"net/netip"
	"testing"
)

var errTestDial = errors.New("test dial failed")

func TestPublicAddress(t *testing.T) {
	blocked := []string{
		"127.0.0.1", "127.9.8.7", "10.0.0.1", "172.16.0.1", "192.168.1.1",
		"169.254.169.254", "169.254.1.1", "100.100.100.200", "0.0.0.0", "0.1.2.3",
		"224.0.0.1", "255.255.255.255", "198.18.0.1", "192.0.2.1", "240.0.0.1",
		"::1", "::", "fc00::1", "fe80::1", "ff02::1", "::ffff:127.0.0.1",
		"::ffff:10.0.0.1", "64:ff9b::a00:1", "2002:7f00:1::", "2001::1",
		"2001:db8::1", "3fff::1", "2606:4700::1%en0",
	}
	for _, raw := range blocked {
		t.Run(raw, func(t *testing.T) {
			if publicAddress(netip.MustParseAddr(raw)) {
				t.Errorf("allowed prohibited address %s", raw)
			}
		})
	}

	for _, raw := range []string{"1.1.1.1", "8.8.8.8", "::ffff:8.8.8.8", "2606:4700:4700::1111"} {
		if !publicAddress(netip.MustParseAddr(raw)) {
			t.Errorf("blocked public address %s", raw)
		}
	}
}

func TestParseIconURL(t *testing.T) {
	for _, raw := range []string{
		"", "invalid-url", "file:///etc/passwd", "ftp://example.com/icon", "http://",
		"http://127.0.0.1/x", "https://[::1]/x", "http://169.254.169.254/latest/meta-data",
		"http://[::ffff:127.0.0.1]/", "http://user:password@example.com/x",
		"http://example.com:8080/x", "http://example.com:443/x", "https://example.com:80/x",
		"https://example.com:abc/x", "https://[fe80::1%25en0]/x", "%",
	} {
		t.Run(raw, func(t *testing.T) {
			_, err := parseIconURL(raw)
			if !errors.Is(err, errInvalidDestination) {
				t.Errorf("URL %q: expected policy error, got %v", raw, err)
			}
		})
	}

	for _, raw := range []string{
		"http://example.com/icon.png", "https://example.com:443/icon.png",
		"http://8.8.8.8:80/x", "https://[2606:4700:4700::1111]/x",
	} {
		_, err := parseIconURL(raw)
		if err != nil {
			t.Errorf("public URL %q: %v", raw, err)
		}
	}
}

func TestPublicDialerRejectsDNSAnswers(t *testing.T) {
	for _, answer := range [][]netip.Addr{
		nil,
		{netip.MustParseAddr("127.0.0.1")},
		{netip.MustParseAddr("10.0.0.1")},
		{netip.MustParseAddr("169.254.169.254")},
		{netip.MustParseAddr("::1")},
		{netip.MustParseAddr("fd00::1")},
		{netip.MustParseAddr("1.1.1.1"), netip.MustParseAddr("192.168.1.1")},
	} {
		dial := publicDialer(
			func(context.Context, string, string) ([]netip.Addr, error) { return answer, nil },
			func(context.Context, string, string) (net.Conn, error) {
				t.Error("prohibited DNS answer reached dial")

				return nil, errTestDial
			},
		)

		_, err := dial(t.Context(), "tcp", "public.example:443")
		if !errors.Is(err, errInvalidDestination) {
			t.Errorf("answer %v: expected policy error, got %v", answer, err)
		}
	}
}

func TestPublicDialerPinsResolvedAddresses(t *testing.T) {
	lookups := 0

	var dialed []string

	dial := publicDialer(
		func(_ context.Context, network, host string) ([]netip.Addr, error) {
			lookups++

			if network != "ip" || host != "public.example" {
				t.Errorf("unexpected lookup %s %s", network, host)
			}

			return []netip.Addr{
				netip.MustParseAddr("1.1.1.1"),
				netip.MustParseAddr("2606:4700::1111"),
			}, nil
		},
		func(_ context.Context, _, address string) (net.Conn, error) {
			dialed = append(dialed, address)

			return nil, errTestDial
		},
	)

	_, err := dial(t.Context(), "tcp", "public.example:443")
	if !errors.Is(err, errTestDial) {
		t.Fatalf("unexpected error: %v", err)
	}

	if lookups != 1 || len(dialed) != 2 || dialed[0] != "1.1.1.1:443" ||
		dialed[1] != "[2606:4700::1111]:443" {
		t.Fatalf("expected one DNS lookup and literal pinned dials, got %d / %v", lookups, dialed)
	}
}

func TestHTTPClientPolicy(t *testing.T) {
	client := getHTTPClient()

	transport, ok := client.Transport.(*http.Transport)
	if !ok || transport.Proxy != nil || transport.DialContext == nil ||
		!transport.DisableCompression {
		t.Fatal(
			"transport must pin public addresses, bypass environment proxies and disable compression",
		)
	}

	if client.Timeout != defaultTimeout || transport.MaxConnsPerHost != maxConcurrentFetches ||
		transport.TLSClientConfig != nil || transport.MaxResponseHeaderBytes == 0 {
		t.Fatal("missing timeout/connection/header budgets or modified TLS defaults")
	}
}

// Exercise the actual HTTP transport against an in-memory peer. No DNS or external sockets are
// used.
func TestPublicIconThroughPinnedTransport(t *testing.T) {
	data := pngFixture(t)
	client := getHTTPClient()

	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatal("unexpected transport")
	}

	t.Cleanup(transport.CloseIdleConnections)

	serverResult := make(chan error, 1)
	transport.DialContext = publicDialer(
		func(context.Context, string, string) ([]netip.Addr, error) {
			return []netip.Addr{netip.MustParseAddr("1.1.1.1")}, nil
		},
		func(_ context.Context, _, address string) (net.Conn, error) {
			if address != "1.1.1.1:80" {
				t.Errorf("unvalidated dial: %s", address)
			}

			local, peer := net.Pipe()
			go func() {
				defer peer.Close()

				request, err := http.ReadRequest(bufio.NewReader(peer))
				if err != nil {
					serverResult <- err

					return
				}
				defer request.Body.Close()

				if request.Host != "public.example" {
					t.Errorf("original host lost: %s", request.Host)

					serverResult <- errTestDial

					return
				}

				response := imageResponse(data, "image/png")
				defer response.Body.Close()

				serverResult <- response.Write(peer)
			}()

			return local, nil
		},
	)

	response := iconRequest(t, newIconProxy(client), "http://public.example/icon.png")
	if response.Code != http.StatusOK || response.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("public icon failed through pinned transport: %d", response.Code)
	}

	err := <-serverResult
	if err != nil {
		t.Fatal(err)
	}
}
