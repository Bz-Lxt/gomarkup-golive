// Package logging provides the project-wide structured logger.
// Production (LOG_LEVEL!=debug) never emits debug records.
// Callers must not use fmt.Println or the default log package.
package logging

import (
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
)

var (
	mu     sync.RWMutex
	global *slog.Logger
)

func init() {
	global = newLogger(os.Stdout, "info")
}

func newLogger(w io.Writer, level string) *slog.Logger {
	var lv slog.Level
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		lv = slog.LevelDebug
	case "warn", "warning":
		lv = slog.LevelWarn
	case "error":
		lv = slog.LevelError
	default:
		lv = slog.LevelInfo
	}
	h := slog.NewJSONHandler(w, &slog.HandlerOptions{
		Level: lv,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				if t, ok := a.Value.Any().(interface{ Format(string) string }); ok {
					_ = t
				}
			}
			return a
		},
	})
	return slog.New(h)
}

// Setup replaces the global logger. Safe to call once at process start.
func Setup(level string) *slog.Logger {
	l := newLogger(os.Stdout, level)
	mu.Lock()
	global = l
	mu.Unlock()
	slog.SetDefault(l)
	return l
}

func L() *slog.Logger {
	mu.RLock()
	defer mu.RUnlock()
	return global
}

func With(args ...any) *slog.Logger {
	return L().With(args...)
}
