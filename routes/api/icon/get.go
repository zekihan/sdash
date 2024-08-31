package icon

import (
	"github.com/zekihan/sdash/routes"
	"io"
	"net/http"
	"time"
)

func Proxy(w http.ResponseWriter, r *http.Request) {
	url := r.URL.Query().Get("url")
	w.Header().Set("Cache-Control", "public, max-age=86400")

	if icon, ok := cache.get(url); ok {
		w.Header().Set("Content-Type", icon.contentType)
		routes.Write(w, icon.data)
		return
	}

	client := getHTTPClient()
	response, err := client.Get(url)
	if routes.CheckError(w, err, http.StatusInternalServerError, "failed to fetch icon") {
		return
	}

	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if routes.CheckError(w, err, http.StatusInternalServerError, "failed to read icon") {
		return
	}

	contentType := response.Header.Get("Content-Type")
	w.Header().Set("Content-Type", contentType)

	w.WriteHeader(response.StatusCode)
	if routes.Write(w, body) {
		return
	}

	cache.add(url, &icon{
		url:         url,
		data:        body,
		contentType: contentType,
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
