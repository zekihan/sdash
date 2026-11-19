package main

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"testing"
	"testing/synctest"
)

func TestServeWithShutdownDrainsActiveRequest(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		serverConn, clientConn := net.Pipe()
		defer clientConn.Close()
		defer serverConn.Close()

		listener := &pipeListener{conn: serverConn, closed: make(chan struct{})}

		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		releaseCtx, releaseHandler := context.WithCancel(t.Context())
		defer releaseHandler()

		handlerStarted := make(chan struct{})
		httpServer := new(http.Server)
		httpServer.ReadHeaderTimeout = readHeaderTimeout
		httpServer.Handler = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			close(handlerStarted)
			<-releaseCtx.Done()
			w.WriteHeader(http.StatusNoContent)
		})

		shutdownDone := make(chan struct{})
		shutdown := func(ctx context.Context) error {
			defer close(shutdownDone)

			return httpServer.Shutdown(ctx)
		}
		done := make(chan struct{})

		var serveErr error

		go func() {
			defer close(done)

			serveErr = serveWithShutdown(
				ctx,
				func() error { return httpServer.Serve(listener) },
				shutdown,
			)
		}()

		_, err := io.WriteString(
			clientConn,
			"GET / HTTP/1.1\r\nHost: dashboard.example\r\nConnection: close\r\n\r\n",
		)
		if err != nil {
			t.Fatal(err)
		}

		<-handlerStarted

		cancel()
		synctest.Wait()

		select {
		case <-listener.closed:
		default:
			t.Error("shutdown did not stop accepting connections")
		}

		select {
		case <-done:
			t.Error("server returned before the active request finished")
		default:
		}

		releaseHandler()

		response, err := http.ReadResponse(bufio.NewReader(clientConn), nil)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()

		if response.StatusCode != http.StatusNoContent {
			t.Errorf("status = %d, want %d", response.StatusCode, http.StatusNoContent)
		}

		<-done
		<-shutdownDone

		if serveErr != nil {
			t.Errorf("serveWithShutdown: %v", serveErr)
		}
	})
}

func TestServeWithShutdownReturnsListenError(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		err := serveWithShutdown(
			ctx,
			func() error { return net.ErrClosed },
			func(context.Context) error {
				t.Error("shutdown ran after serving failed")

				return nil
			},
		)
		if !errors.Is(err, net.ErrClosed) {
			t.Errorf("serveWithShutdown = %v, want %v", err, net.ErrClosed)
		}

		cancel()
		synctest.Wait()
	})
}

type pipeListener struct {
	conn   net.Conn
	closed chan struct{}
}

func (listener *pipeListener) Accept() (net.Conn, error) {
	if listener.conn != nil {
		conn := listener.conn
		listener.conn = nil

		return conn, nil
	}

	<-listener.closed

	return nil, net.ErrClosed
}

func (listener *pipeListener) Close() error {
	close(listener.closed)

	return nil
}

func (*pipeListener) Addr() net.Addr { return new(net.TCPAddr) }

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
