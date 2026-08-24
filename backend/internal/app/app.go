package app

import (
	"context"
	"errors"
	"io/fs"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golive/internal/certs"
	"golive/internal/config"
	"golive/internal/httpapi"
	"golive/internal/logging"
	"golive/internal/netem"
	"golive/internal/room"
	"golive/internal/session"
	"golive/internal/transport"
)

type App struct {
	cfg    *config.Config
	bundle *certs.Bundle
	dual   *netem.Dual
	hub    *room.Hub
	reg    *session.Registry
	wt     *transport.Server
	http   *http.Server
	static fs.FS
}

func New(cfg *config.Config, static fs.FS) (*App, error) {
	bundle, err := certs.New(cfg.CertDir)
	if err != nil {
		return nil, err
	}
	return &App{
		cfg:    cfg,
		bundle: bundle,
		dual:   netem.NewDual(cfg.NetemSeed),
		hub:    room.NewHub(),
		reg:    session.NewRegistry(),
		static: static,
	}, nil
}

func (a *App) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	a.wt = transport.New(ctx, a.cfg, a.bundle, a.dual, a.hub, a.reg)
	go func() {
		if err := a.wt.ListenAndServe(); err != nil {
			logging.L().Error("webtransport stopped", "err", err)
			cancel()
		}
	}()

	// Give UDP a moment so /health can see the socket.
	time.Sleep(50 * time.Millisecond)

	api := httpapi.New(a.cfg, a.bundle, a.dual, a.reg, a.hub, func() bool {
		return a.wt != nil
	})
	_, srv, err := httpapi.Listen(ctx, a.cfg.TCPAddr, api.Routes(a.static))
	if err != nil {
		return err
	}
	a.http = srv

	go a.reap(ctx)
	go a.refreshCerts(ctx)

	<-ctx.Done()
	return a.shutdown()
}

func (a *App) WaitSignals(parent context.Context) error {
	ctx, stop := signal.NotifyContext(parent, os.Interrupt, syscall.SIGTERM)
	defer stop()
	return a.Run(ctx)
}

func (a *App) reap(ctx context.Context) {
	t := time.NewTicker(15 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if n := a.reg.ReapIdle(a.cfg.SessionIdle); n > 0 {
				logging.L().Info("reaped idle sessions", "n", n)
			}
		}
	}
}

func (a *App) refreshCerts(ctx context.Context) {
	t := time.NewTicker(time.Hour)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := a.bundle.EnsureFresh(); err != nil {
				logging.L().Error("cert refresh", "err", err)
			}
		}
	}
}

func (a *App) shutdown() error {
	logging.L().Info("graceful shutdown")
	ctx, cancel := context.WithTimeout(context.Background(), a.cfg.ShutdownTimeout)
	defer cancel()
	var first error
	if a.http != nil {
		if err := httpapi.Shutdown(ctx, a.http); err != nil && !errors.Is(err, http.ErrServerClosed) {
			first = err
		}
	}
	if a.wt != nil {
		if err := a.wt.Close(); err != nil && first == nil {
			first = err
		}
	}
	return first
}
