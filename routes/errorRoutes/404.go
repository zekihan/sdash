package errorRoutes

import (
	"github.com/a-h/templ"
	"github.com/zekihan/sdash/views"
	"net/http"
)

func GetNotFound(w http.ResponseWriter, r *http.Request) {
	templ.Handler(views.NotFound(), templ.WithStatus(http.StatusNotFound)).ServeHTTP(w, r)
}
