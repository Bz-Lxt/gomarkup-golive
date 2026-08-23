package config

import (
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("TCP_ADDR", "")
	t.Setenv("UDP_ADDR", "")
	t.Setenv("ALLOWED_ORIGINS", "")
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.PublicTCPPort != 19443 || c.PublicUDPPort != 19444 {
		t.Fatalf("ports %d/%d", c.PublicTCPPort, c.PublicUDPPort)
	}
	if len(c.AllowedOrigins) < 2 {
		t.Fatalf("origins=%v", c.AllowedOrigins)
	}
	if c.MetricsHz != 10 {
		t.Fatalf("hz=%d", c.MetricsHz)
	}
	if c.ReassemblyTTL != 200*time.Millisecond {
		t.Fatalf("ttl=%s", c.ReassemblyTTL)
	}
	if !c.BBR.Enabled {
		t.Fatal("BBR should default on")
	}
}

func TestValidateRejectsBadGain(t *testing.T) {
	t.Setenv("BBR_STARTUP_GAIN", "0.5")
	if _, err := Load(); err == nil {
		t.Fatal("expected error")
	}
}

func TestValidateRejectsEmptyOrigins(t *testing.T) {
	t.Setenv("ALLOWED_ORIGINS", "   ,  ")
	if _, err := Load(); err == nil {
		t.Fatal("expected empty origins error")
	}
}

func TestWebTransportURL(t *testing.T) {
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	want := "https://localhost:19444/webtransport"
	if c.WebTransportURL() != want {
		t.Fatalf("got %s", c.WebTransportURL())
	}
}

func TestEnvBool(t *testing.T) {
	t.Setenv("BBR_ENABLED", "off")
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.BBR.Enabled {
		t.Fatal("expected disabled")
	}
}
