package errorroutes

import (
	"github.com/a-h/templ"
	"github.com/zekihan/sdash/routes"
	"github.com/zekihan/sdash/views"
	"net/http"
)

func GetNotFound(w http.ResponseWriter, r *http.Request) {
	switch routes.GetAcceptType(r) {
	case routes.AcceptTypeHTML:
		templ.Handler(views.NotFound(), templ.WithStatus(http.StatusNotFound)).ServeHTTP(w, r)
	default:
		http.Error(w, "Not Found", http.StatusNotFound)
	}
}
