package routes

import (
	"github.com/gorilla/securecookie"
	"github.com/zekihan/sdash/config"
	"net/http"
)

var sc *securecookie.SecureCookie

func CreateNewCookie(name string, value interface{}, opts ...string) (*http.Cookie, error) {
	sameSite := http.SameSiteLaxMode
	path := "/"
	switch len(opts) {
	case 1:
		sameSiteStr := opts[0]
		switch sameSiteStr {
		case "lax":
			sameSite = http.SameSiteLaxMode
		case "strict":
			sameSite = http.SameSiteStrictMode
		case "none":
			sameSite = http.SameSiteNoneMode
		}
	case 2:
		sameSiteStr := opts[0]
		switch sameSiteStr {
		case "lax":
			sameSite = http.SameSiteLaxMode
		case "strict":
			sameSite = http.SameSiteStrictMode
		case "none":
			sameSite = http.SameSiteNoneMode
		}
		path = opts[1]
	}

	encode, err := getSecureCookie().Encode(name, value)
	if err != nil {
		return nil, err
	}

	c := &http.Cookie{
		Name:     name,
		Value:    encode,
		Path:     path,
		MaxAge:   int(config.GetConfig().WebConfig.SessionTimeout.Seconds()),
		Secure:   true,
		HttpOnly: true,
		SameSite: sameSite,
	}

	return c, nil
}

func getSecureCookie() *securecookie.SecureCookie {
	if sc == nil {
		sc = securecookie.New(config.GetConfig().WebConfig.GetHashKey(), config.GetConfig().WebConfig.GetBlockKey())
	}
	return sc
}

func SetCookie(w http.ResponseWriter, _ *http.Request, name string, value interface{}, opts ...string) {
	c, err := CreateNewCookie(name, value, opts...)
	if err != nil {
		return
	}

	http.SetCookie(w, c)
}

func GetCookie[T any](r *http.Request, name string, value *T) error {
	c, err := r.Cookie(name)
	if err != nil {
		return err
	}

	err = getSecureCookie().Decode(name, c.Value, value)
	if err != nil {
		return err
	}

	return nil
}
