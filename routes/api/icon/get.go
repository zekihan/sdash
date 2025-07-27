package icon

import (
	"errors"
	"github.com/zekihan/go-commons/web"
	"io"
	"net/http"
	"time"
)

var errMissingURL = errors.New("missing url parameter")

func Proxy(w http.ResponseWriter, r *http.Request) {
	url := r.URL.Query().Get("url")
	if url == "" {
		web.CheckError(w, errMissingURL, http.StatusBadRequest, "missing url parameter")
		return
	}

	w.Header().Set("Cache-Control", "public, max-age=86400")

	if icon, ok := cache.get(url); ok {
		w.Header().Set("Content-Type", icon.ContentType)
		web.Write(w, icon.Data)
		return
	}

	client := getHTTPClient()
	response, err := client.Get(url)
	if web.CheckError(w, err, http.StatusInternalServerError, "failed to fetch icon") {
		return
	}

	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if web.CheckError(w, err, http.StatusInternalServerError, "failed to read icon") {
		return
	}

	contentType := response.Header.Get("Content-Type")
	w.Header().Set("Content-Type", contentType)

	w.WriteHeader(response.StatusCode)
	if web.Write(w, body) {
		return
	}

	cache.add(url, &Icon{
		URL:         url,
		Data:        body,
		ContentType: contentType,
	})
}

func getHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        10,
			IdleConnTimeout:     30 * time.Second,
			DisableCompression:  true,
			DisableKeepAlives:   false,
			MaxIdleConnsPerHost: 10,
		},
	}
}
