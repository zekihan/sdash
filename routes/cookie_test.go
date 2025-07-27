package routes

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateNewCookie(t *testing.T) {
	cookie, err := CreateNewCookie("testCookie", "testValue", "strict", "/testpath")
	assert.NoError(t, err)
	assert.NotNil(t, cookie)
	assert.Equal(t, "testCookie", cookie.Name)
	assert.Equal(t, "/testpath", cookie.Path)
	assert.Equal(t, http.SameSiteStrictMode, cookie.SameSite)
	assert.True(t, cookie.Secure)
	assert.True(t, cookie.HttpOnly)
}

func TestSetCookie(t *testing.T) {
	w := httptest.NewRecorder()
	SetCookie(w, nil, "testCookie", "testValue", "lax")

	resp := w.Result()
	cookie := resp.Cookies()[0]

	assert.Equal(t, "testCookie", cookie.Name)
	assert.Equal(t, http.SameSiteLaxMode, cookie.SameSite)
	assert.True(t, cookie.Secure)
	assert.True(t, cookie.HttpOnly)
}

func TestGetCookie(t *testing.T) {
	// Create a cookie
	w := httptest.NewRecorder()
	SetCookie(w, nil, "testCookie", "testValue", "lax")
	resp := w.Result()
	cookie := resp.Cookies()[0]

	// Mock a request with the cookie
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(cookie)

	var value string
	err := GetCookie(req, "testCookie", &value)
	assert.NoError(t, err)
	assert.Equal(t, "testValue", value)
}
