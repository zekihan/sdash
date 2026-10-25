package icon

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/zekihan/go-commons/web"
)

const (
	maxConcurrentFetches = 4
	maxURLBytes          = 4096
	maxRateCallers       = 1024
	requestsPerMinute    = 120
)

var (
	errBusy      = errors.New("icon fetch capacity exhausted")
	defaultProxy = newIconProxy(getHTTPClient())
)

type iconFlight struct {
	done chan struct{}
	icon *Icon
	err  error
}

type callerWindow struct {
	start time.Time
	count int
}

type iconProxy struct {
	client  *http.Client
	cache   *iconCache
	flights map[string]*iconFlight
	callers map[string]callerWindow
	now     func() time.Time
	lock    sync.Mutex
}

func newIconProxy(client *http.Client) *iconProxy {
	return &iconProxy{
		client:  client,
		cache:   newIconCache(),
		flights: make(map[string]*iconFlight),
		callers: make(map[string]callerWindow),
		now:     time.Now,
		lock:    sync.Mutex{},
	}
}

func Proxy(w http.ResponseWriter, r *http.Request) {
	defaultProxy.ServeHTTP(w, r)
}

func (proxy *iconProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; sandbox")
	w.Header().Set("Cache-Control", "no-store")

	if !proxy.allowCaller(r.RemoteAddr) {
		w.Header().Set("Retry-After", "60")
		http.Error(w, "too many icon requests", http.StatusTooManyRequests)

		return
	}

	rawURL := r.URL.Query().Get("url")
	if len(rawURL) > maxURLBytes {
		http.Error(w, "invalid icon URL", http.StatusBadRequest)

		return
	}

	parsed, err := parseIconURL(rawURL)
	if err != nil {
		http.Error(w, "invalid icon URL", http.StatusBadRequest)

		return
	}

	parsed.Fragment = ""
	key := parsed.String()

	icon, err := proxy.load(r.Context(), key)
	if err != nil {
		status := http.StatusBadGateway
		if errors.Is(err, errBusy) {
			status = http.StatusServiceUnavailable
		}

		http.Error(w, "unable to load icon", status)

		return
	}

	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=600")
	web.Write(r.Context(), w, icon.Data)
}

// RemoteAddr is the connection peer. Forwarded headers are deliberately not trusted.
func (proxy *iconProxy) allowCaller(remote string) bool {
	host, _, err := net.SplitHostPort(remote)
	if err != nil {
		host = remote
	}

	proxy.lock.Lock()
	defer proxy.lock.Unlock()

	now := proxy.now()
	for caller, window := range proxy.callers {
		if now.Sub(window.start) >= time.Minute {
			delete(proxy.callers, caller)
		}
	}

	window, exists := proxy.callers[host]
	if (!exists && len(proxy.callers) >= maxRateCallers) || window.count >= requestsPerMinute {
		return false
	}

	if !exists {
		window.start = now
	}

	window.count++
	proxy.callers[host] = window

	return true
}

func (proxy *iconProxy) load(ctx context.Context, key string) (*Icon, error) {
	proxy.lock.Lock()
	if icon, ok := proxy.cache.get(key); ok {
		proxy.lock.Unlock()

		return icon, nil
	}

	flight, exists := proxy.flights[key]
	if !exists {
		if len(proxy.flights) >= maxConcurrentFetches {
			proxy.lock.Unlock()

			return nil, errBusy
		}

		flight = &iconFlight{done: make(chan struct{}), icon: nil, err: nil}
		proxy.flights[key] = flight
		// One bounded worker owns each miss. A caller cancellation cannot cancel other waiters.
		go proxy.fetch(ctx, key, flight)
	}

	proxy.lock.Unlock()

	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("wait for icon: %w", ctx.Err())
	case <-flight.done:
		return flight.icon, flight.err
	}
}

func (proxy *iconProxy) fetch(parent context.Context, key string, flight *iconFlight) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(parent), defaultTimeout)
	defer cancel()

	flight.icon, flight.err = proxy.fetchIcon(ctx, key)

	proxy.lock.Lock()
	defer proxy.lock.Unlock()

	if flight.err == nil {
		proxy.cache.add(key, flight.icon)
	}

	delete(proxy.flights, key)
	close(flight.done)
}

func (proxy *iconProxy) fetchIcon(ctx context.Context, key string) (*Icon, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, key, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("create icon request: %w", err)
	}

	response, err := proxy.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch icon: %w", err)
	}
	defer response.Body.Close()

	return readIcon(response)
}
