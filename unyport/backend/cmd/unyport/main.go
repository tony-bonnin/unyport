package main

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
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
	logger.Info("startup", "platform", "alpine-xen", "theme", settings.Theme)
	recordStartupEvent(filepath.Join(settings.Paths.LogDir, "startup-history.jsonl"), settings.Theme, logger)

	srv := server.New(cfg, settings, logger)

	go func() {
		var err error
		if settings.HTTP3.Enabled && settings.HTTP3.CertFile != "" {
			err = srv.ListenAndServe(logger)
		} else {
			err = srv.ListenAndServeHTTP()
		}
		if err != nil {
			logger.Error("server error", "err", err)
		}
	}()

	logger.Info("unyport listening", "addr", srv.Addr())

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
	logger.Info("stopped gracefully")
}

// initLogger écrit sur stderr ET dans le fichier log simultanément.
// Le format slog texte conserve un timestamp lisible pour la console et le fichier.
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

type startupHistoryEvent struct {
	Timestamp  string `json:"timestamp"`
	Event      string `json:"event"`
	Theme      string `json:"theme,omitempty"`
	BootID     string `json:"boot_id,omitempty"`
	RebootedAt string `json:"rebooted_at,omitempty"`
}

func recordStartupEvent(path string, theme string, logger *slog.Logger) {
	now := time.Now().UTC()
	bootID := strings.TrimSpace(readFirstLine("/proc/sys/kernel/random/boot_id"))
	rebootedAt := estimateLastBootRFC3339(now)
	if startupHistoryContains(path, bootID, rebootedAt) {
		return
	}

	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		logger.Warn("startup history mkdir failed", "path", path, "err", err)
		return
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0640)
	if err != nil {
		logger.Warn("startup history open failed", "path", path, "err", err)
		return
	}
	defer f.Close()

	event := startupHistoryEvent{
		Timestamp:  now.Format(time.RFC3339Nano),
		Event:      "startup",
		Theme:      theme,
		BootID:     bootID,
		RebootedAt: rebootedAt,
	}
	if err := json.NewEncoder(f).Encode(event); err != nil {
		logger.Warn("startup history write failed", "path", path, "err", err)
	}
}

func readFirstLine(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func estimateLastBootRFC3339(now time.Time) string {
	data, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return ""
	}
	fields := strings.Fields(string(data))
	if len(fields) == 0 {
		return ""
	}
	secs, err := strconv.ParseFloat(fields[0], 64)
	if err != nil || secs < 0 {
		return ""
	}
	return now.Add(-time.Duration(secs * float64(time.Second))).UTC().Format(time.RFC3339Nano)
}

func startupHistoryContains(path string, bootID string, rebootedAt string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var rec startupHistoryEvent
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			continue
		}
		if bootID != "" && rec.BootID == bootID {
			return true
		}
		if rebootedAt != "" && rec.RebootedAt == rebootedAt {
			return true
		}
	}
	return false
}
