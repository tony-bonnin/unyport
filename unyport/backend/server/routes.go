package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"path"
	"strings"

	"unyport/auth"
	"unyport/config"
	"unyport/middleware"
	"unyport/proxy"
	"unyport/sse"
)

func setupRoutes(
	cfg config.Config,
	settings *config.Settings,
	authHandler *auth.Handler,
	oauthSvc *auth.OAuthService,
	broker *sse.Broker,
	authMW func(http.Handler) http.Handler,
	logger *slog.Logger,
) *http.ServeMux {
	mux := http.NewServeMux()

	// ---- Publiques ----
	mux.HandleFunc("/api/csrf", middleware.CSRFTokenHandler)
	mux.HandleFunc("/api/login", authHandler.Login)
	mux.HandleFunc("/api/logout", authHandler.Logout)
	mux.HandleFunc("/api/session", authHandler.Session)
	mux.HandleFunc("/api/oauth/login", oauthSvc.LoginHandler)
	mux.HandleFunc("/api/oauth/callback", oauthSvc.CallbackHandler)

	// ---- Assets statiques ----
	for _, dir := range []string{"css", "app", "media", "assets", "static", "vendor", "webfonts", "fonts"} {
		prefix := "/" + dir + "/"
		mux.Handle(prefix, http.StripPrefix(prefix, staticHandler("../frontend/public/"+dir)))
	}
	mux.Handle("/favicon.ico", staticHandler("../frontend/public"))
	mux.Handle("/robots.txt", staticHandler("../frontend/public"))
	mux.Handle("/manifest.json", staticHandler("../frontend/public"))

	// ---- SSE métriques (protégé) ----
	mux.Handle("/sse/system", authMW(http.HandlerFunc(broker.Handler)))

	// ---- /api/system : infos statiques HW/OS (protégé) ----
	mux.Handle("/api/system", authMW(http.HandlerFunc(broker.SystemInfoHandler)))

	// ---- /api/apps : liste des apps proxifiées (protégé) ----
	mux.Handle("/api/apps", authMW(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		type appInfo struct {
			Name   string `json:"name"`
			Host   string `json:"host"`
			Port   int    `json:"port"`
			Path   string `json:"path"`
			Target string `json:"target"`
			Type   string `json:"type"`
		}
		list := make([]appInfo, 0, len(cfg.Apps))
		for _, app := range cfg.Apps {
			name := strings.ToLower(app.Name)
			list = append(list, appInfo{
				Name:   name,
				Host:   strings.ToLower(app.Host),
				Port:   app.Port,
				Path:   "/proxy/" + name + "/",
				Target: app.TargetURL(),
				Type:   strings.ToLower(app.Type),
			})
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(list)
	})))

	// ---- API admin (protégée) ----
	mux.Handle("/api/admin/users", authMW(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// TODO: CRUD complet, pour l'instant count only
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
	})))

	// ---- Proxies ----
	for _, app := range cfg.Apps {
		lname := strings.ToLower(app.Name)
		prefix := path.Clean("/proxy/" + lname)
		handler := proxy.Make(app, prefix, logger)

		mux.Handle(prefix+"/", authMW(handler))
		func(p string) {
			mux.Handle(p, authMW(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, p+"/", http.StatusMovedPermanently)
			})))
		}(prefix)
	}

	// /proxy/ sans slug → 404
	mux.Handle("/proxy/", authMW(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})))

	// ---- Fallback HTMX ----
	// / est public — le template décide login vs dashboard
	mux.Handle("/", spaFallback("../frontend/public", "index.html"))

	return mux
}

// staticHandler sert les fichiers statiques depuis dir.
func staticHandler(dir string) http.Handler {
	return http.FileServer(http.Dir(dir))
}

// spaFallback sert index.html pour toute route inconnue (HTMX navigation).
func spaFallback(dir, index string) http.Handler {
	fs := http.FileServer(http.Dir(dir))
	deny := []string{"/api/", "/proxy/", "/sse/", "/css/", "/app/", "/media/", "/assets/", "/static/"}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.NotFound(w, r)
			return
		}
		for _, p := range deny {
			if strings.HasPrefix(r.URL.Path, p) {
				http.NotFound(w, r)
				return
			}
		}
		fs.ServeHTTP(w, r)
	})
}