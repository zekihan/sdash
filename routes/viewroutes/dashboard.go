package viewroutes

import (
	"context"
	"errors"
	"net/http"

	"github.com/zekihan/go-commons/web"

	"github.com/zekihan/sdash/config/dashboard"
	"github.com/zekihan/sdash/views"
)

func GetDashboard(w http.ResponseWriter, r *http.Request) {
	config := dashboard.GetConfigWithContext(r.Context())

	err := views.Root(config).Render(r.Context(), w)
	if errors.Is(err, context.Canceled) {
		return
	}

	web.CheckError(
		r.Context(),
		w,
		err,
		http.StatusInternalServerError,
		"failed to render dashboard",
	)
}
