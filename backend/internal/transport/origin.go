package transport

import (
	"net/http"
	"net/url"
	"strings"
)

// AllowOrigin returns a CheckOrigin that accepts only the configured list.
// Empty Origin is rejected (browsers always send it for WebTransport).
func AllowOrigin(allowed []string) func(*http.Request) bool {
	set := make(map[string]struct{}, len(allowed)*2)
	for _, o := range allowed {
		o = strings.TrimRight(strings.TrimSpace(o), "/")
		if o == "" {
			continue
		}
		set[o] = struct{}{}
		if u, err := url.Parse(o); err == nil {
			// accept host aliases already listed; also accept trailing-slash-stripped
			set[strings.TrimRight(u.String(), "/")] = struct{}{}
		}
	}
	return func(r *http.Request) bool {
		origin := strings.TrimRight(strings.TrimSpace(r.Header.Get("Origin")), "/")
		if origin == "" {
			return false
		}
		_, ok := set[origin]
		return ok
	}
}
