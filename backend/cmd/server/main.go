package main

import (
	"context"
	"embed"
	"io/fs"
	"os"

	"golive/internal/app"
	"golive/internal/config"
	"golive/internal/logging"
)

//go:embed all:static
var staticRoot embed.FS

func main() {
	cfg, err := config.Load()
	if err != nil {
		logging.Setup("error")
		logging.L().Error("config", "err", err)
		os.Exit(2)
	}
	logging.Setup(cfg.LogLevel)

	sub, err := fs.Sub(staticRoot, "static")
	if err != nil {
		logging.L().Error("static fs", "err", err)
		os.Exit(2)
	}
	application, err := app.New(cfg, sub)
	if err != nil {
		logging.L().Error("app init", "err", err)
		os.Exit(1)
	}
	if err := application.WaitSignals(context.Background()); err != nil {
		logging.L().Error("exit", "err", err)
		os.Exit(1)
	}
}
