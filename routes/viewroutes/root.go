package viewroutes

import (
	"net/http"
)

func Root(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/" {
		http.Redirect(w, r, "/dashboard", http.StatusMovedPermanently)

		return
	}

	http.Redirect(w, r, "/404", http.StatusMovedPermanently)
}
