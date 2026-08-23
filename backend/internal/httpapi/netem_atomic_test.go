package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"golive/internal/netem"
)

func TestPutNetemRejectedUpdateIsAtomic(t *testing.T) {
	h := testAPI(t).Routes(nil)

	set := httptest.NewRecorder()
	h.ServeHTTP(set, httptest.NewRequest(http.MethodPut, "/api/v1/netem", strings.NewReader(`{
		"uplink":{"name":"baseline-up","loss_pct":12,"delay_ms":7},
		"downlink":{"name":"baseline-down","loss_pct":18,"delay_ms":9}
	}`)))
	if set.Code != http.StatusOK {
		t.Fatalf("setting baseline: code=%d body=%s", set.Code, set.Body.String())
	}

	rejected := httptest.NewRecorder()
	h.ServeHTTP(rejected, httptest.NewRequest(http.MethodPut, "/api/v1/netem", strings.NewReader(`{
		"uplink":{"name":"rejected-up","loss_pct":55,"delay_ms":70},
		"downlink":{"name":"rejected-down","loss_pct":101,"delay_ms":90}
	}`)))
	if rejected.Code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid update: code=%d body=%s", rejected.Code, rejected.Body.String())
	}

	got := httptest.NewRecorder()
	h.ServeHTTP(got, httptest.NewRequest(http.MethodGet, "/api/v1/netem", nil))
	if got.Code != http.StatusOK {
		t.Fatalf("reading config: code=%d body=%s", got.Code, got.Body.String())
	}
	var snapshot netem.Snapshot
	if err := json.Unmarshal(got.Body.Bytes(), &snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot.Uplink.Name != "baseline-up" || snapshot.Downlink.Name != "baseline-down" {
		t.Fatalf("rejected update changed active config: uplink=%+v downlink=%+v", snapshot.Uplink, snapshot.Downlink)
	}
}
