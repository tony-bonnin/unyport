package server

import (
	"log/slog"
	"net/http"
	"time"

	"unyport/auth"
	"unyport/config"
	"unyport/middleware"
	"unyport/sse"
)

// New assemble les services, enregistre les routes et retourne le *http.Server.
func New(cfg config.Config, settings *config.Settings, logger *slog.Logger) *http.Server {
	// ---- Services ----
	jwt, err := auth.NewJWTService(settings.Security.JWTSecret)
	if err != nil {
		logger.Error("jwt init failed", "err", err)
		panic(err)
	}

	users, err := auth.NewUserStore(settings.Paths.UsersFile)
	if err != nil {
		logger.Error("userstore init failed", "err", err)
		panic(err)
	}

	authHandler := auth.NewHandler(users, jwt, logger)
	oauthSvc := auth.NewOAuthService(cfg.Auth, users, jwt)
	broker := sse.NewBroker(logger)
	authMW := middleware.Auth(jwt, logger)

	// ---- Routes ----
	mux := setupRoutes(cfg, settings, authHandler, oauthSvc, broker, authMW, logger)

	// ---- Middleware chain ----
	// SecureHeaders → CSRFBypass → CSRFProtect → mux
	trustedOrigins := []string{
		"http://localhost:8800",
		"http://127.0.0.1:8800",
	}
	handler := middleware.Secure(
		middleware.CSRFBypass(
			middleware.CSRFProtect(settings, trustedOrigins)(mux),
		),
	)

	logger.Info("middleware chain ready", "chain", "Secure→CSRFBypass→CSRFProtect→mux")
	logger.Info("listening", "addr", ":8800")

	return &http.Server{
		Addr:              ":8800",
		Handler:           handler,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      0,
		IdleTimeout:       120 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
	}
}