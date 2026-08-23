package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"golive/internal/certs"
	"golive/internal/config"
	"golive/internal/netem"
	"golive/internal/room"
	"golive/internal/session"
)

func testAPI(t *testing.T) *API {
	t.Helper()
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	b, err := certs.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return New(cfg, b, netem.NewDual(1), session.NewRegistry(), room.NewHub(), func() bool { return true })
}

func TestHealthAndFingerprint(t *testing.T) {
	h := testAPI(t).Routes(nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/health", nil))
	if rec.Code != 200 {
		t.Fatalf("health %d", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["bbr_layer"] != "application-scheduler" {
		t.Fatal(body["bbr_layer"])
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/cert-fingerprint", nil))
	if rec.Code != 200 {
		t.Fatal(rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"sha-256"`) {
		t.Fatal(rec.Body.String())
	}
}

func TestNetemRejectsUnknown(t *testing.T) {
	h := testAPI(t).Routes(nil)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/netem", strings.NewReader(`{"preset":"99"}`))
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
}
