package httpapi

import (
	"context"
	"errors"
	"io/fs"
	"net"
	"net/http"
	"time"

	"golive/internal/logging"
)

func Listen(ctx context.Context, addr string, h http.Handler) (net.Listener, *http.Server, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, nil, err
	}
	srv := &http.Server{
		Handler:           h,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
		BaseContext:       func(net.Listener) context.Context { return ctx },
	}
	go func() {
		logging.L().Info("http listening", "addr", addr)
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logging.L().Error("http serve", "err", err)
		}
	}()
	return ln, srv, nil
}

func Shutdown(ctx context.Context, srv *http.Server) error {
	return srv.Shutdown(ctx)
}

func StaticFS(root fs.FS) fs.FS { return root }
