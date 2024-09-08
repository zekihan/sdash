package routes

import (
	"github.com/gorilla/securecookie"
	"net/http"
	"time"
)

var sc = securecookie.New(securecookie.GenerateRandomKey(64), securecookie.GenerateRandomKey(32))

func SetCookie(w http.ResponseWriter, r *http.Request, name string, value interface{}, opts ...string) {
	path := "/"
	switch len(opts) {
	case 1:
		path = opts[0]
	}

	encode, err := sc.Encode(name, value)
	if err != nil {
		return
	}

	c := &http.Cookie{
		Name:     name,
		Value:    encode,
		Path:     path,
		MaxAge:   int(time.Hour.Seconds()),
		Secure:   r.TLS != nil,
		HttpOnly: true,
	}
	http.SetCookie(w, c)
}

func GetCookie[T any](r *http.Request, name string, value *T) error {
	c, err := r.Cookie(name)
	if err != nil {
		return err
	}

	err = sc.Decode(name, c.Value, value)
	if err != nil {
		return err
	}

	return nil
}
