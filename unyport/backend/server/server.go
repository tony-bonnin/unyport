package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/quic-go/quic-go/http3"
	"unyport/auth"
	"unyport/config"
	"unyport/middleware"
	"unyport/sse"
)

// Server wraps http.Server et le serveur HTTP/3 optionnel.
type Server struct {
	HTTP  *http.Server
	HTTP3 *http3.Server // nil si HTTP/3 désactivé
	addr  string
}

// Addr retourne l'adresse d'écoute principale.
func (s *Server) Addr() string { return s.addr }

func normalizeTrustedOrigins(origins []string) []string {
	seen := make(map[string]struct{}, len(origins))
	out := make([]string, 0, len(origins))
	for _, origin := range origins {
		origin = strings.TrimSpace(origin)
		if origin == "" {
			continue
		}
		host := origin
		if u, err := url.Parse(origin); err == nil && u.Host != "" {
			host = u.Host
		}
		if _, ok := seen[host]; ok {
			continue
		}
		seen[host] = struct{}{}
		out = append(out, host)
	}
	return out
}

func defaultTrustedOrigins(port string) []string {
	hosts := []string{"localhost", "127.0.0.1", "::1"}
	ifaces, err := net.Interfaces()
	if err == nil {
		for _, iface := range ifaces {
			if iface.Flags&net.FlagUp == 0 {
				continue
			}
			addrs, err := iface.Addrs()
			if err != nil {
				continue
			}
			for _, addr := range addrs {
				ip, _, err := net.ParseCIDR(addr.String())
				if err != nil || ip == nil || ip.IsLoopback() {
					continue
				}
				hosts = append(hosts, ip.String())
			}
		}
	}

	seen := make(map[string]struct{}, len(hosts))
	out := make([]string, 0, len(hosts))
	for _, host := range hosts {
		origin := net.JoinHostPort(host, port)
		if _, ok := seen[origin]; ok {
			continue
		}
		seen[origin] = struct{}{}
		out = append(out, origin)
	}
	return out
}

// ListenAndServe démarre HTTP/1.1+2 (TLS) et HTTP/3 (QUIC) si activé.
func (s *Server) ListenAndServe(logger *slog.Logger) error {
	if s.HTTP3 != nil {
		go func() {
			logger.Info("http3 listening", "addr", s.HTTP3.Addr)
			if err := s.HTTP3.ListenAndServeTLS("", ""); err != nil {
				logger.Error("http3 error", "err", err)
			}
		}()
	}
	return s.HTTP.ListenAndServeTLS("", "")
}

// ListenAndServeHTTP démarre en HTTP simple (sans TLS).
func (s *Server) ListenAndServeHTTP() error {
	return s.HTTP.ListenAndServe()
}

// Shutdown arrête proprement HTTP/1.1+2 et HTTP/3.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.HTTP3 != nil {
		_ = s.HTTP3.Close()
	}
	return s.HTTP.Shutdown(ctx)
}

// New assemble les services, enregistre les routes et retourne un *Server.
func New(cfg config.Config, settings *config.Settings, logger *slog.Logger) *Server {
	// ---- JWT ----
	jwtTTL := time.Duration(settings.Security2.SessionTimeoutMins) * time.Minute
	if jwtTTL <= 0 {
		jwtTTL = time.Hour
	}
	jwt, err := auth.NewJWTServiceWithOpts(
		settings.Security.JWTSecret,
		jwtTTL,
		settings.Security2.HTTPS,
	)
	if err != nil {
		logger.Error("jwt init failed", "err", err)
		panic(err)
	}

	// ---- UserStore ----
	users, err := auth.NewUserStore(settings.Paths.UsersFile)
	if err != nil {
		logger.Error("userstore init failed", "err", err)
		panic(err)
	}

	// ---- BrandingStore ----
	brandingStore, err := config.NewBrandingStore("settings/branding.yaml")
	if err != nil {
		logger.Error("branding store init failed", "err", err)
		panic(err)
	}
	brandingHandler := auth.NewBrandingHandler(brandingStore, logger)
	mailer := auth.NewMailer(settings, logger)

	// ---- Services ----
	authHandler := auth.NewHandler(users, jwt, mailer, logger)
	oauthSvc := auth.NewOAuthService(cfg.Auth, users, jwt, mailer, settings.Security2.HTTPS)
	broker := sse.NewBroker(
		logger,
		filepath.Join(settings.Paths.LogDir, "unyport.log"),
		filepath.Join(settings.Paths.LogDir, "startup-history.jsonl"),
	)
	authMW := middleware.Auth(jwt, users, logger)

	// ---- Rate limiter login ----
	maxAttempts := settings.Security2.RateLimitLogin
	if maxAttempts <= 0 {
		maxAttempts = 5
	}
	loginRL := middleware.NewLoginRateLimiter(maxAttempts, logger)

	// ---- Trusted origins ----
	trustedOrigins := normalizeTrustedOrigins(settings.Security2.TrustedOrigins)
	if len(trustedOrigins) == 0 {
		trustedOrigins = defaultTrustedOrigins("8800")
	}

	// ---- Routes ----
	mux := setupRoutes(cfg, settings, authHandler, brandingHandler, oauthSvc, broker, authMW, loginRL, logger)

	// ---- Middleware chain ----
	handler := middleware.Secure(
		middleware.CSRFBypass(
			middleware.CSRFProtect(settings, trustedOrigins)(mux),
		),
		settings.Security2.HTTPS,
	)

	logger.Info("middleware chain ready",
		"chain", "Secure→CSRFBypass→CSRFProtect→mux",
		"jwt_ttl", jwtTTL,
		"rate_limit_login", maxAttempts,
		"trusted_origins", trustedOrigins,
		"https", settings.Security2.HTTPS,
		"http3", settings.HTTP3.Enabled,
	)

	// ---- Adresse d'écoute ----
	httpAddr := ":8800"
	if settings.HTTP3.Enabled {
		httpAddr = fmt.Sprintf(":%d", settings.HTTP3.Port)
	}

	httpSrv := &http.Server{
		Addr:              httpAddr,
		Handler:           handler,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      0,
		IdleTimeout:       120 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
	}

	srv := &Server{HTTP: httpSrv, addr: httpAddr}

	// ---- HTTP/3 (QUIC) — optionnel ----
	if settings.HTTP3.Enabled {
		if settings.HTTP3.CertFile == "" || settings.HTTP3.KeyFile == "" {
			logger.Error("http3 enabled but cert_file/key_file missing — falling back to HTTP")
		} else {
			cert, err := tls.LoadX509KeyPair(settings.HTTP3.CertFile, settings.HTTP3.KeyFile)
			if err != nil {
				logger.Error("http3 tls load failed", "err", err, "cert", settings.HTTP3.CertFile)
			} else {
				tlsCfg := &tls.Config{
					Certificates: []tls.Certificate{cert},
					MinVersion:   tls.VersionTLS13,
					NextProtos:   []string{"h3"},
				}
				httpSrv.TLSConfig = tlsCfg

				h3addr := fmt.Sprintf(":%d", settings.HTTP3.Port)
				srv.HTTP3 = &http3.Server{
					Addr:      h3addr,
					Handler:   handler,
					TLSConfig: tlsCfg,
				}

				// Alt-Svc : annonce HTTP/3 au navigateur
				altSvc := fmt.Sprintf(`h3=":%d"; ma=86400`, settings.HTTP3.Port)
				origHandler := httpSrv.Handler
				httpSrv.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Alt-Svc", altSvc)
					origHandler.ServeHTTP(w, r)
				})

				logger.Info("http3 configured", "addr", h3addr, "cert", settings.HTTP3.CertFile)
			}
		}
	} else {
		logger.Info("listening", "addr", httpAddr)
	}

	// ---- Redirection HTTP → HTTPS ----
	if settings.HTTP3.Enabled && settings.HTTP3.RedirectHTTP {
		go func() {
			httpsPort := settings.HTTP3.Port
			redirect := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				target := fmt.Sprintf("https://%s:%d%s", r.Host, httpsPort, r.RequestURI)
				http.Redirect(w, r, target, http.StatusMovedPermanently)
			})
			redirectSrv := &http.Server{
				Addr:              ":8800",
				Handler:           redirect,
				ReadTimeout:       5 * time.Second,
				ReadHeaderTimeout: 3 * time.Second,
			}
			logger.Info("http redirect listening", "addr", ":8800", "target_port", httpsPort)
			if err := redirectSrv.ListenAndServe(); err != nil {
				logger.Error("http redirect error", "err", err)
			}
		}()
	}

	return srv
}
