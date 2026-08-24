package transport

import (
	"net/http"
	"testing"
)

func TestAllowOrigin(t *testing.T) {
	fn := AllowOrigin([]string{"http://localhost:19443", "http://127.0.0.1:19443"})
	req := func(o string) *http.Request {
		r, _ := http.NewRequest(http.MethodConnect, "https://localhost/webtransport", nil)
		if o != "" {
			r.Header.Set("Origin", o)
		}
		return r
	}
	if !fn(req("http://localhost:19443")) {
		t.Fatal("localhost should pass")
	}
	if !fn(req("http://127.0.0.1:19443")) {
		t.Fatal("127 should pass")
	}
	if fn(req("https://evil.example")) {
		t.Fatal("evil should fail")
	}
	if fn(req("")) {
		t.Fatal("empty origin should fail")
	}
}
