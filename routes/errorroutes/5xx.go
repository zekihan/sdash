package errorroutes

import (
	"github.com/a-h/templ"
	"github.com/zekihan/sdash/routes"
	"github.com/zekihan/sdash/views"
	"net/http"
)

func Get5xx(w http.ResponseWriter, r *http.Request) {
	switch routes.GetAcceptType(r) {
	case routes.AcceptTypeHTML:
		templ.Handler(views.Error(), templ.WithStatus(http.StatusInternalServerError)).ServeHTTP(w, r)
	default:
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}
