package httpapi

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"strings"

	"golive/internal/certs"
	"golive/internal/clock"
	"golive/internal/config"
	"golive/internal/netem"
	"golive/internal/room"
	"golive/internal/session"
)

type API struct {
	cfg    *config.Config
	bundle *certs.Bundle
	dual   *netem.Dual
	reg    *session.Registry
	hub    *room.Hub
	udpUp  func() bool
}

func New(cfg *config.Config, bundle *certs.Bundle, dual *netem.Dual, reg *session.Registry, hub *room.Hub, udpUp func() bool) *API {
	return &API{cfg: cfg, bundle: bundle, dual: dual, reg: reg, hub: hub, udpUp: udpUp}
}

func (a *API) Routes(static fs.FS) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/health", a.health)
	mux.HandleFunc("GET /api/v1/cert-fingerprint", a.fingerprint)
	mux.HandleFunc("GET /api/v1/config", a.showConfig)
	mux.HandleFunc("GET /api/v1/netem", a.getNetem)
	mux.HandleFunc("PUT /api/v1/netem", a.putNetem)
	mux.HandleFunc("GET /api/v1/sessions", a.sessions)
	if static != nil {
		file := http.FileServer(http.FS(static))
		mux.Handle("/", spa(file))
	} else {
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			writeError(w, http.StatusNotFound, "no_ui", "frontend not embedded")
		})
	}
	return wrap(mux)
}

func spa(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}
		h.ServeHTTP(w, r)
	})
}

type healthBody struct {
	Status    string `json:"status"`
	Time      string `json:"time"`
	UDP       bool   `json:"udp_listening"`
	CertHours int    `json:"cert_hours_remaining"`
	Sessions  int    `json:"sessions"`
	Rooms     int    `json:"rooms"`
	BBRLayer  string `json:"bbr_layer"`
}

func (a *API) health(w http.ResponseWriter, r *http.Request) {
	udp := a.udpUp != nil && a.udpUp()
	fp := a.bundle.Fingerprint()
	status := "ok"
	code := http.StatusOK
	if !udp || fp.ValidHours < 1 {
		status = "degraded"
		code = http.StatusServiceUnavailable
	}
	writeJSON(w, code, healthBody{
		Status: status, Time: clock.Format(clock.Now()),
		UDP: udp, CertHours: fp.ValidHours,
		Sessions: a.reg.Len(), Rooms: a.hub.Rooms(),
		BBRLayer: "application-scheduler",
	})
}

func (a *API) fingerprint(w http.ResponseWriter, r *http.Request) {
	fp := a.bundle.Fingerprint()
	writeJSON(w, http.StatusOK, map[string]any{
		"algorithm":   fp.Algorithm,
		"hex":         fp.Hex,
		"base64":      fp.Base64,
		"base64_raw":  fp.Base64Raw,
		"not_before":  fp.NotBefore,
		"not_after":   fp.NotAfter,
		"curve":       fp.Curve,
		"valid_hours": fp.ValidHours,
		"wt_url":      a.cfg.WebTransportURL(),
		"note":        "hash is SHA-256 of leaf DER, not SPKI; algorithm must stay lowercase sha-256",
	})
}

func (a *API) showConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"tcp":            a.cfg.PublicOrigin(),
		"wt_url":         a.cfg.WebTransportURL(),
		"allowed_origins": a.cfg.AllowedOrigins,
		"metrics_hz":     a.cfg.MetricsHz,
		"max_datagram":   a.cfg.MaxDatagram,
		"bbr_enabled":    a.cfg.BBR.Enabled,
		"bbr_layer":      "application-scheduler",
		"netem_seed":     a.cfg.NetemSeed,
		"presets":        netem.PresetNames(),
		"channels": []map[string]any{
			{"id": "signal", "kind": "bidi", "priority": 0, "reliable": true},
			{"id": "audio", "kind": "datagram", "priority": 1, "reliable": false},
			{"id": "cursor", "kind": "datagram", "priority": 1, "reliable": false},
			{"id": "video", "kind": "uni", "priority": 2, "reliable": true},
			{"id": "file", "kind": "bidi", "priority": 3, "reliable": true},
		},
	})
}

func (a *API) getNetem(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.dual.Snapshot())
}

func (a *API) putNetem(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Preset   string         `json:"preset"`
		Uplink   *netem.Profile `json:"uplink"`
		Downlink *netem.Profile `json:"downlink"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_json", err.Error())
		return
	}
	if body.Preset != "" {
		if err := a.dual.ApplyPreset(body.Preset); err != nil {
			writeError(w, http.StatusUnprocessableEntity, "bad_preset", err.Error())
			return
		}
	} else if body.Uplink != nil && body.Downlink != nil {
		if err := body.Uplink.Validate(); err != nil {
			writeError(w, http.StatusUnprocessableEntity, "bad_uplink", err.Error())
			return
		}
		if err := body.Downlink.Validate(); err != nil {
			writeError(w, http.StatusUnprocessableEntity, "bad_downlink", err.Error())
			return
		}
		a.dual.Apply(*body.Uplink, *body.Downlink)
	} else {
		writeError(w, http.StatusUnprocessableEntity, "missing", "preset or uplink+downlink required")
		return
	}
	writeJSON(w, http.StatusOK, a.dual.Snapshot())
}

func (a *API) sessions(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"items": a.reg.Snapshot(), "count": a.reg.Len()})
}
