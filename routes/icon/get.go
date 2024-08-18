package icon

import (
	log "github.com/sirupsen/logrus"
	"io"
	"net/http"
	"time"
)

func Proxy(w http.ResponseWriter, r *http.Request) {
	url := r.URL.Query().Get("url")
	w.Header().Set("Cache-Control", "public, max-age=86400")

	if icon, ok := cache.get(url); ok {
		w.Header().Set("Content-Type", icon.contentType)
		_, err := w.Write(icon.data)
		if err != nil {
			log.Error("Error writing icon: ", err)
			return
		}
		return
	}

	client := getHTTPClient()
	response, err := client.Get(url)
	if err != nil {
		log.Error("Error fetching icon: ", err)
		http.Error(w, "Proxy not implemented yet", http.StatusInternalServerError)
		return
	}

	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		log.Error("Error reading icon: ", err)
		http.Error(w, "Proxy not implemented yet", http.StatusInternalServerError)
		return
	}

	contentType := response.Header.Get("Content-Type")
	w.Header().Set("Content-Type", contentType)

	_, err = w.Write(body)
	if err != nil {
		log.Error("Error writing icon: ", err)
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
