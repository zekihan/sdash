package middleware

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	log "github.com/sirupsen/logrus"
)

func TestLogging_HandlerCalled(t *testing.T) {
	var logBuffer bytes.Buffer
	log.SetOutput(&logBuffer) // capture logs in buffer

	called := false
	handler := Logging(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true

		w.WriteHeader(http.StatusCreated)
	}))

	req := httptest.NewRequest(http.MethodPost, "/test", http.NoBody)
	req.RemoteAddr = "192.0.2.100:12345" // simulate a request from a specific IP
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if !called {
		t.Error("expected wrapped handler to be called")
	}

	if rr.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", rr.Code)
	}

	logStr := logBuffer.String()
	checkLogs(t, logStr, http.MethodPost, "192.0.2.100", http.StatusCreated, "/test")
}

func TestLogging_HandlerCalled_IPUnknown(t *testing.T) {
	var logBuffer bytes.Buffer
	log.SetOutput(&logBuffer) // capture logs in buffer

	called := false
	handler := Logging(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true

		w.WriteHeader(http.StatusCreated)
	}))

	req := httptest.NewRequest(http.MethodPost, "/test", http.NoBody)
	req.RemoteAddr = "300.0.113.1:1234" // simulate invalid IP
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if !called {
		t.Error("expected wrapped handler to be called")
	}

	if rr.Code != http.StatusCreated {
		t.Errorf("expected status %d, got %d", http.StatusCreated, rr.Code)
	}

	logStr := logBuffer.String()
	checkLogs(t, logStr, http.MethodPost, "unknown", http.StatusCreated, "/test")
}

func checkLogs(t *testing.T, logStr, method, ip string, status int, path string) {
	t.Helper()

	if !strings.Contains(logStr, method) {
		t.Errorf("expected log to contain method '%s', got %s", method, logStr)
	}

	if !strings.Contains(logStr, ip) {
		t.Errorf("expected log to contain IP '%s', got %s", ip, logStr)
	}

	if !strings.Contains(logStr, strconv.Itoa(status)) {
		t.Errorf("expected log to contain status '%d', got %s", status, logStr)
	}

	if !strings.Contains(logStr, path) {
		t.Errorf("expected log to contain path '%s', got %s", path, logStr)
	}
}
