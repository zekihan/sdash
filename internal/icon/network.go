package icon

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"time"
)

var errInvalidDestination = errors.New(
	"icon destination must be a public HTTP(S) address on its standard port",
)

const (
	defaultTimeout         = 5 * time.Second
	defaultIdleConnTimeout = 30 * time.Second
	maxResponseHeaderBytes = 16 << 10
)

// In addition to IsPrivate, exclude special-use networks that are not public destinations.
var blockedNetworks = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("192.88.99.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("240.0.0.0/4"),
	// Transition, documentation and special-purpose IPv6 ranges can encode non-public IPv4.
	netip.MustParsePrefix("2001::/23"),
	netip.MustParsePrefix("2001:db8::/32"),
	netip.MustParsePrefix("2002::/16"),
	netip.MustParsePrefix("3fff::/20"),
}

func publicAddress(address netip.Addr) bool {
	address = address.Unmap()
	if address.Zone() != "" || !address.IsGlobalUnicast() || address.IsPrivate() ||
		address.IsLoopback() || address.IsLinkLocalUnicast() {
		return false
	}

	// Only the currently allocated global unicast space is accepted for IPv6.
	if address.Is6() && !netip.MustParsePrefix("2000::/3").Contains(address) {
		return false
	}

	for _, network := range blockedNetworks {
		if network.Contains(address) {
			return false
		}
	}

	return true
}

func parseIconURL(rawURL string) (*url.URL, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Hostname() == "" || parsed.User != nil || parsed.Opaque != "" {
		return nil, errInvalidDestination
	}

	port := parsed.Port()
	if (parsed.Scheme != "http" || (port != "" && port != "80")) &&
		(parsed.Scheme != "https" || (port != "" && port != "443")) {
		return nil, errInvalidDestination
	}

	address, parseErr := netip.ParseAddr(parsed.Hostname())
	if parseErr == nil && !publicAddress(address) {
		return nil, errInvalidDestination
	}

	return parsed, nil
}

type (
	lookupFunc func(context.Context, string, string) ([]netip.Addr, error)
	dialFunc   func(context.Context, string, string) (net.Conn, error)
)

// Resolve once, reject the entire DNS answer if any address is unsafe, then dial
// literal validated addresses. The transport still uses the original hostname for TLS.
func publicDialer(lookup lookupFunc, dial dialFunc) dialFunc {
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil || (port != "80" && port != "443") {
			return nil, errInvalidDestination
		}

		addresses, err := lookup(ctx, "ip", host)
		if err != nil {
			return nil, fmt.Errorf("resolve icon host: %w", err)
		}

		if len(addresses) == 0 {
			return nil, errInvalidDestination
		}

		for _, resolved := range addresses {
			if !publicAddress(resolved) {
				return nil, errInvalidDestination
			}
		}

		for _, resolved := range addresses {
			connection, dialErr := dial(
				ctx,
				network,
				net.JoinHostPort(resolved.Unmap().String(), port),
			)
			if dialErr == nil {
				return connection, nil
			}

			err = dialErr
		}

		return nil, fmt.Errorf("dial icon host: %w", err)
	}
}

func getHTTPClient() *http.Client {
	dialer := new(net.Dialer)
	dialer.Timeout = defaultTimeout
	transport := new(http.Transport)
	// No environment proxy: all outbound connections must pass the public-address policy.
	transport.DialContext = publicDialer(net.DefaultResolver.LookupNetIP, dialer.DialContext)
	transport.DisableCompression = true
	transport.MaxIdleConns = maxConcurrentFetches
	transport.MaxConnsPerHost = maxConcurrentFetches
	transport.MaxIdleConnsPerHost = maxConcurrentFetches
	transport.IdleConnTimeout = defaultIdleConnTimeout
	transport.TLSHandshakeTimeout = defaultTimeout
	transport.ResponseHeaderTimeout = defaultTimeout
	transport.MaxResponseHeaderBytes = maxResponseHeaderBytes

	client := new(http.Client)
	client.Timeout = defaultTimeout
	client.Transport = transport
	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}

	return client
}
