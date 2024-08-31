package errorRoutes

import (
	"github.com/a-h/templ"
	"github.com/zekihan/sdash/views"
	"net/http"
)

func Get5xx(w http.ResponseWriter, r *http.Request) {
	templ.Handler(views.Error(), templ.WithStatus(http.StatusInternalServerError)).ServeHTTP(w, r)
}
