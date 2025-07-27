package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetSearch_NoQuery(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/search", nil)
	rr := httptest.NewRecorder()

	GetSearch(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}
	if rr.Body.Len() == 0 {
		t.Error("expected non-empty response body")
	}
}

func TestGetSearch_WithQuery(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/search?search=test", nil)
	rr := httptest.NewRecorder()

	GetSearch(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}
	if rr.Body.Len() == 0 {
		t.Error("expected non-empty response body")
	}
}
