// Package config loads all runtime knobs from the environment.
// Hard-coded ports / origins / BBR gains are forbidden; defaults
// exist only as documented fallbacks for local compose.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	TCPAddr         string
	UDPAddr         string
	PublicTCPHost   string
	PublicUDPHost   string
	PublicTCPPort   int
	PublicUDPPort   int
	AllowedOrigins  []string
	LogLevel        string
	CertDir         string
	NetemSeed       int64
	MetricsHz       int
	ReassemblyTTL   time.Duration
	SessionIdle     time.Duration
	ShutdownTimeout time.Duration
	MaxDatagram     int
	ALFMaxPayload   int
	QueueCap        QueueCap
	WFQ             WFQWeights
	BBR             BBRParams
}

type QueueCap struct {
	Signal int
	Audio  int
	Cursor int
	Video  int
	File   int
}

type WFQWeights struct {
	Audio  int
	Cursor int
	Video  int
	File   int
}

type BBRParams struct {
	Enabled          bool
	ProbeRTTInterval time.Duration
	ProbeRTTDuration time.Duration
	StartupGain      float64
	DrainGain        float64
	BtlBwWindow      int
	MinPacingBps     uint64
	MaxPacingBps     uint64
}

func Load() (*Config, error) {
	c := &Config{
		TCPAddr:         env("TCP_ADDR", ":19443"),
		UDPAddr:         env("UDP_ADDR", ":19444"),
		PublicTCPHost:   env("PUBLIC_TCP_HOST", "localhost"),
		PublicUDPHost:   env("PUBLIC_UDP_HOST", "localhost"),
		PublicTCPPort:   envInt("PUBLIC_TCP_PORT", 19443),
		PublicUDPPort:   envInt("PUBLIC_UDP_PORT", 19444),
		AllowedOrigins:  splitCSV(env("ALLOWED_ORIGINS", "http://localhost:19443,http://127.0.0.1:19443")),
		LogLevel:        env("LOG_LEVEL", "info"),
		CertDir:         env("CERT_DIR", "/var/lib/golive/certs"),
		NetemSeed:       envInt64("NETEM_SEED", 20260823),
		MetricsHz:       envInt("METRICS_HZ", 10),
		ReassemblyTTL:   envDur("REASSEMBLY_TTL", 200*time.Millisecond),
		SessionIdle:     envDur("SESSION_IDLE", 90*time.Second),
		ShutdownTimeout: envDur("SHUTDOWN_TIMEOUT", 8*time.Second),
		MaxDatagram:     envInt("MAX_DATAGRAM", 1024),
		ALFMaxPayload:   envInt("ALF_MAX_PAYLOAD", 1<<20),
		QueueCap: QueueCap{
			Signal: envInt("QUEUE_CAP_SIGNAL", 256),
			Audio:  envInt("QUEUE_CAP_AUDIO", 128),
			Cursor: envInt("QUEUE_CAP_CURSOR", 256),
			Video:  envInt("QUEUE_CAP_VIDEO", 64),
			File:   envInt("QUEUE_CAP_FILE", 32),
		},
		WFQ: WFQWeights{
			Audio:  envInt("WFQ_AUDIO", 4),
			Cursor: envInt("WFQ_CURSOR", 4),
			Video:  envInt("WFQ_VIDEO", 2),
			File:   envInt("WFQ_FILE", 1),
		},
		BBR: BBRParams{
			Enabled:          envBool("BBR_ENABLED", true),
			ProbeRTTInterval: envDur("BBR_PROBE_RTT_INTERVAL", 10*time.Second),
			ProbeRTTDuration: envDur("BBR_PROBE_RTT_DURATION", 200*time.Millisecond),
			StartupGain:      envFloat("BBR_STARTUP_GAIN", 2.885),
			DrainGain:        envFloat("BBR_DRAIN_GAIN", 0.346),
			BtlBwWindow:      envInt("BBR_BTLBW_WINDOW", 10),
			MinPacingBps:     uint64(envInt("BBR_MIN_PACING_BPS", 32*1024)),
			MaxPacingBps:     uint64(envInt("BBR_MAX_PACING_BPS", 80*1024*1024)),
		},
	}
	if err := c.validate(); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Config) validate() error {
	if c.PublicTCPPort < 1 || c.PublicTCPPort > 65535 {
		return fmt.Errorf("PUBLIC_TCP_PORT out of range: %d", c.PublicTCPPort)
	}
	if c.PublicUDPPort < 1 || c.PublicUDPPort > 65535 {
		return fmt.Errorf("PUBLIC_UDP_PORT out of range: %d", c.PublicUDPPort)
	}
	if len(c.AllowedOrigins) == 0 {
		return fmt.Errorf("ALLOWED_ORIGINS must not be empty")
	}
	if c.MetricsHz < 1 || c.MetricsHz > 50 {
		return fmt.Errorf("METRICS_HZ must be 1..50, got %d", c.MetricsHz)
	}
	if c.MaxDatagram < 256 || c.MaxDatagram > 2048 {
		return fmt.Errorf("MAX_DATAGRAM must be 256..2048 (Chrome hard-caps 1024)")
	}
	if c.BBR.StartupGain <= 1 {
		return fmt.Errorf("BBR_STARTUP_GAIN must be > 1")
	}
	if c.BBR.DrainGain <= 0 || c.BBR.DrainGain >= 1 {
		return fmt.Errorf("BBR_DRAIN_GAIN must be in (0, 1)")
	}
	if c.BBR.MinPacingBps == 0 || c.BBR.MaxPacingBps <= c.BBR.MinPacingBps {
		return fmt.Errorf("invalid BBR pacing bounds")
	}
	return nil
}

func (c *Config) WebTransportURL() string {
	return fmt.Sprintf("https://%s:%d/webtransport", c.PublicUDPHost, c.PublicUDPPort)
}

func (c *Config) PublicOrigin() string {
	return fmt.Sprintf("http://%s:%d", c.PublicTCPHost, c.PublicTCPPort)
}

func env(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func envInt64(key string, fallback int64) int64 {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return fallback
	}
	return n
}

func envFloat(key string, fallback float64) float64 {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	n, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return fallback
	}
	return n
}

func envBool(key string, fallback bool) bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	switch v {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

func envDur(key string, fallback time.Duration) time.Duration {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return d
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, strings.TrimRight(p, "/"))
		}
	}
	return out
}
