package viewroutes

import (
	"github.com/zekihan/sdash/routes"
	"net/http"
)

func Root(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/" {
		http.Redirect(w, r, "/dashboard", http.StatusMovedPermanently)
		return
	}

	switch routes.GetAcceptType(r) {
	case routes.AcceptTypeHTML:
		http.Redirect(w, r, "/404", http.StatusMovedPermanently)
	default:
		http.Error(w, "Not Found", http.StatusNotFound)
	}
}
