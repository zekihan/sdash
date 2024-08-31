package api

import (
	"github.com/zekihan/sdash/routes"
	"net/http"
)

func GetPing(w http.ResponseWriter, _ *http.Request) {
	routes.WriteString(w, "Pong\n")
}
