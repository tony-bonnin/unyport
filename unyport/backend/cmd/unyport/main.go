package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"unyport/config"
	"unyport/server"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	cfg, err := config.LoadConfig("settings/config.yaml")
	if err != nil {
		logger.Error("config load failed", "err", err)
		os.Exit(1)
	}

	settings, err := config.LoadSettings("settings/settings.yaml")
	if err != nil {
		logger.Error("settings load failed", "err", err)
		os.Exit(1)
	}

	logger = initLogger(settings.Paths.LogDir)
	logger.Info("startup", "os", settings.OS, "theme", settings.Theme)

	srv := server.New(cfg, settings, logger)

	go func() {
		if err := srv.ListenAndServe(); err != nil {
			logger.Error("server error", "err", err)
		}
	}()

	fmt.Fprintf(os.Stderr, "\n  🚀 UnyPort is up on %s\n\n", srv.Addr)
	logger.Info("unyport listening", "addr", srv.Addr)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	logger.Info("shutdown requested")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("shutdown error", "err", err)
		os.Exit(1)
	}
	fmt.Fprintln(os.Stderr, "  ✓ stopped gracefully")
	logger.Info("stopped gracefully")
}

// initLogger écrit sur stderr ET dans le fichier log simultanément.
func initLogger(logDir string) *slog.Logger {
	if err := os.MkdirAll(logDir, 0750); err != nil {
		return slog.New(slog.NewTextHandler(os.Stderr, nil))
	}
	f, err := os.OpenFile(filepath.Join(logDir, "unyport.log"),
		os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0640)
	if err != nil {
		return slog.New(slog.NewTextHandler(os.Stderr, nil))
	}
	return slog.New(slog.NewTextHandler(
		io.MultiWriter(os.Stderr, f),
		&slog.HandlerOptions{Level: slog.LevelInfo},
	))
}