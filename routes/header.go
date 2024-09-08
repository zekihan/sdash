package routes

import (
	"net/http"
	"strings"
)

type AcceptType string

const (
	AcceptTypeJSON  AcceptType = "json"
	AcceptTypeHTML  AcceptType = "html"
	AcceptTypePlain AcceptType = "plain"
	AcceptTypeNone  AcceptType = "none"
)

func GetAcceptType(r *http.Request) AcceptType {
	accept := r.Header.Get("Accept")
	if strings.Contains(accept, "application/json") {
		return AcceptTypeJSON
	} else if strings.Contains(accept, "text/html") {
		return AcceptTypeHTML
	} else if strings.Contains(accept, "text/plain") {
		return AcceptTypePlain
	}
	return AcceptTypeNone
}
