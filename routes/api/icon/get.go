package icon

import (
	"errors"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/zekihan/go-commons/web"
)

var errMissingURL = errors.New("missing url parameter")

var errInvalidURLScheme = errors.New("invalid url scheme")

const (
	defaultTimeout             = 5 * time.Second
	defaultIdleConns           = 10
	defaultIdleConnTimeout     = 30 * time.Second
	defaultMaxIdleConnsPerHost = 10
)

func Proxy(w http.ResponseWriter, r *http.Request) {
	rawURL := r.URL.Query().Get("url")
	if rawURL == "" {
		web.CheckError(
			r.Context(),
			w,
			errMissingURL,
			http.StatusBadRequest,
			"missing url parameter",
		)

		return
	}

	parsedURL, err := url.Parse(rawURL)
	if err != nil || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") {
		web.CheckError(
			r.Context(),
			w,
			errInvalidURLScheme,
			http.StatusBadRequest,
			"invalid url",
		)

		return
	}

	w.Header().Set("Cache-Control", "public, max-age=86400")

	if icon, ok := cache.get(rawURL); ok {
		w.Header().Set("Content-Type", icon.ContentType)
		web.Write(r.Context(), w, icon.Data)

		return
	}

	client := getHTTPClient()

	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, rawURL, http.NoBody)
	if web.CheckError(
		r.Context(),
		w,
		err,
		http.StatusInternalServerError,
		"failed to create request",
	) {
		return
	}

	response, err := client.Do(req)
	if web.CheckError(r.Context(), w, err, http.StatusInternalServerError, "failed to fetch icon") {
		return
	}

	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if web.CheckError(r.Context(), w, err, http.StatusInternalServerError, "failed to read icon") {
		return
	}

	contentType := response.Header.Get("Content-Type")
	w.Header().Set("Content-Type", contentType)

	w.WriteHeader(response.StatusCode)

	if web.Write(r.Context(), w, body) {
		return
	}

	cache.add(rawURL, &Icon{
		URL:         rawURL,
		Data:        body,
		ContentType: contentType,
	})
}

func getHTTPClient() *http.Client {
	transport := new(http.Transport)
	transport.MaxIdleConns = defaultIdleConns
	transport.IdleConnTimeout = defaultIdleConnTimeout
	transport.DisableCompression = true
	transport.DisableKeepAlives = false
	transport.MaxIdleConnsPerHost = defaultMaxIdleConnsPerHost

	client := new(http.Client)
	client.Timeout = defaultTimeout
	client.Transport = transport

	return client
}
