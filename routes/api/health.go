package api

import (
	"github.com/zekihan/sdash/routes"
	"net/http"
)

func GetHealth(w http.ResponseWriter, r *http.Request) {
	routes.WriteString(w, "OK\n")
}
