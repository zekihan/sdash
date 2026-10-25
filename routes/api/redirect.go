package api

import (
	"net/url"
	"strings"
	"unicode"
)

// localRedirectPath preserves local return URLs and falls back to the root for
// URLs that browsers could interpret as another origin. Query strings and
// fragments are kept verbatim; only the path is checked after repeated decoding.
func localRedirectPath(target string) string {
	parsed, err := url.Parse(target)
	if err != nil || parsed.IsAbs() || parsed.Host != "" || parsed.Opaque != "" {
		return "/"
	}

	path, _, _ := strings.Cut(target, "?")

	path, _, _ = strings.Cut(path, "#")
	for {
		if !strings.HasPrefix(path, "/") || strings.HasPrefix(path, "//") ||
			strings.Contains(path, `\`) || strings.ContainsFunc(path, unicode.IsControl) {
			return "/"
		}

		decoded, decodeErr := url.PathUnescape(path)
		if decodeErr != nil || decoded == path {
			return target
		}

		path = decoded
	}
}
