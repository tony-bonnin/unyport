package server

import (
	"encoding/json"
	"io"
	"io/fs"
	"log/slog"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"unyport/auth"
	"unyport/config"
	"unyport/middleware"
	"unyport/proxy"
	"unyport/sse"
)

// mimeTypes — table explicite pour les environnements sans /etc/mime.types
// (Alpine minimal, BusyBox). http.FileServerFS utilise mime.TypeByExtension
// qui dépend du système — on enregistre les types essentiels au démarrage.
var mimeTypes = map[string]string{
	".css":   "text/css; charset=utf-8",
	".js":    "text/javascript; charset=utf-8",
	".html":  "text/html; charset=utf-8",
	".json":  "application/json; charset=utf-8",
	".svg":   "image/svg+xml",
	".png":   "image/png",
	".jpg":   "image/jpeg",
	".jpeg":  "image/jpeg",
	".ico":   "image/x-icon",
	".woff":  "font/woff",
	".woff2": "font/woff2",
	".ttf":   "font/ttf",
	".map":   "application/json",
}

func init() {
	for ext, ct := range mimeTypes {
		mime.AddExtensionType(ext, ct)
	}
}

// mimeFixFS force le Content-Type depuis la table explicite.
// Immunise contre les Alpine sans /etc/mime.types (embed prod).
func mimeFixFS(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ext := strings.ToLower(filepath.Ext(r.URL.Path))
		if ct, ok := mimeTypes[ext]; ok {
			w.Header().Set("Content-Type", ct)
		}
		h.ServeHTTP(w, r)
	})
}

func setupRoutes(
	cfg config.Config,
	settings *config.Settings,
	authHandler *auth.Handler,
	brandingHandler *auth.BrandingHandler,
	oauthSvc *auth.OAuthService,
	broker *sse.Broker,
	authMW func(http.Handler) http.Handler,
	loginRL func(http.Handler) http.Handler,
	logger *slog.Logger,
) *http.ServeMux {
	mux := http.NewServeMux()

	adminMW := func(h http.Handler) http.Handler {
		return authMW(middleware.RequireRole("admin")(h))
	}
	writeMW := func(h http.Handler) http.Handler {
		return authMW(middleware.RequireRole("admin", "operator")(h))
	}

	// ---- Publiques ----
	mux.HandleFunc("/api/csrf", middleware.CSRFTokenHandler)
	mux.Handle("/api/login", loginRL(http.HandlerFunc(authHandler.Login)))
	mux.HandleFunc("/api/logout", authHandler.Logout)
	mux.HandleFunc("/api/session", authHandler.Session)

	// Branding — GET public, PATCH/DELETE admin
	mux.HandleFunc("/api/branding", brandingHandler.GetBranding)
	mux.HandleFunc("/api/oauth/login", oauthSvc.LoginHandler)
	mux.HandleFunc("/api/oauth/callback", oauthSvc.CallbackHandler)

	// ---- Assets statiques ----
	// Dev  : UNYPORT_ASSETS défini → http.Dir (live, sans rebuild)
	// Prod : embed compilé dans le binaire → fs.Sub(staticFS, "assets")
	assetsDir := os.Getenv("UNYPORT_ASSETS")
	if assetsDir != "" {
		// Mode dev — servir chaque sous-répertoire depuis le disque
		for _, dir := range []string{"css", "app", "media", "assets", "static", "vendor", "webfonts", "fonts"} {
			prefix := "/" + dir + "/"
			dirPath := assetsDir + "/" + dir
			mux.Handle(prefix, mimeFixFS(http.StripPrefix(prefix, http.FileServer(http.Dir(dirPath)))))
		}
		mux.Handle("/favicon.ico", mimeFixFS(http.FileServer(http.Dir(assetsDir))))
		mux.Handle("/robots.txt", mimeFixFS(http.FileServer(http.Dir(assetsDir))))
		mux.Handle("/manifest.json", mimeFixFS(http.FileServer(http.Dir(assetsDir))))
		mux.Handle("/", spaFallbackDir(assetsDir))
	} else {
		// Mode prod — embed
		pub, err := fs.Sub(staticFS, "assets")
		if err != nil {
			// Ne devrait jamais arriver si le build est correct,
			// mais on évite un panic silencieux.
			panic("embed: assets subtree missing — rebuild with cp frontend/public server/assets")
		}
		for _, dir := range []string{"css", "app", "media", "assets", "static", "vendor", "webfonts", "fonts"} {
			prefix := "/" + dir + "/"
			sub, err := fs.Sub(pub, dir)
			if err != nil {
				// Sous-dossier absent de frontend/public — on l'ignore proprement.
				logger.Debug("embed: static subdir not found, skipping", "dir", dir)
				continue
			}
			mux.Handle(prefix, mimeFixFS(http.StripPrefix(prefix, http.FileServerFS(sub))))
		}
		mux.Handle("/favicon.ico", mimeFixFS(http.FileServerFS(pub)))
		mux.Handle("/robots.txt", mimeFixFS(http.FileServerFS(pub)))
		mux.Handle("/manifest.json", mimeFixFS(http.FileServerFS(pub)))
		mux.Handle("/", spaFallback(pub, "index.html"))
	}

	// ---- SSE métriques (protégé — tous rôles) ----
	mux.Handle("/sse/system", authMW(http.HandlerFunc(broker.Handler)))

	// ---- /api/system : infos statiques HW/OS (protégé — tous rôles) ----
	mux.Handle("/api/system", authMW(http.HandlerFunc(broker.SystemInfoHandler)))

	// ---- /api/versions : versions latest TRINITY (protégé — tous rôles) ----
	mux.Handle("/api/versions", authMW(http.HandlerFunc(broker.VersionsHandler)))
	mux.Handle("/api/reboots", authMW(http.HandlerFunc(broker.RebootsHandler)))

	// ---- API sysinfo étendue — portage ACF Lua (protégé — tous rôles) ----
	mux.Handle("/api/bios", authMW(http.HandlerFunc(broker.BIOSHandler)))
	mux.Handle("/api/modules", authMW(http.HandlerFunc(broker.ModulesHandler)))
	mux.Handle("/api/gpus", authMW(http.HandlerFunc(broker.GPUsHandler)))
	mux.Handle("/api/packages", authMW(http.HandlerFunc(broker.PackagesHandler)))
	mux.Handle("/api/services", authMW(http.HandlerFunc(broker.ServicesHandler)))
	mux.Handle("/api/security", authMW(http.HandlerFunc(broker.SecurityHandler)))
	mux.Handle("/api/logs", authMW(http.HandlerFunc(broker.LogsListHandler)))
	mux.Handle("/api/logs/tail", authMW(http.HandlerFunc(broker.LogsTailHandler)))

	// ---- /api/apps : liste des apps proxifiées (protégé — tous rôles) ----
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

	// ---- Profil utilisateur (protégé — tous rôles) ----
	mux.Handle("/api/profile", authMW(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			authHandler.Profile(w, r)
		case http.MethodPatch:
			if middleware.UserRoleFromCtx(r.Context()) == "viewer" {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			authHandler.UpdateProfile(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})))
	mux.Handle("/api/profile/password", writeMW(http.HandlerFunc(authHandler.UpdatePassword)))

	// Branding write (admin uniquement)
	mux.Handle("/api/branding/update", adminMW(http.HandlerFunc(brandingHandler.UpdateBranding)))
	mux.Handle("/api/branding/reset", adminMW(http.HandlerFunc(brandingHandler.ResetBranding)))

	// ---- API admin users (admin uniquement) ----
	mux.Handle("/api/admin/users", adminMW(http.HandlerFunc(authHandler.AdminUsers)))
	mux.Handle("/api/admin/users/", adminMW(http.HandlerFunc(authHandler.AdminUserByEmail)))

	// ---- Proxies (protégés — tous rôles) ----
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

	mux.Handle("/proxy/", authMW(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})))

	return mux
}

// spaFallback — mode prod (embed FS)
func spaFallback(fsys fs.FS, index string) http.Handler {
	deny := []string{"/api/", "/proxy/", "/sse/", "/css/", "/app/", "/media/", "/assets/", "/static/", "/vendor/"}

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
		candidate := strings.TrimLeft(r.URL.Path, "/")
		if candidate != "" {
			if _, err := fs.Stat(fsys, candidate); err == nil {
				http.FileServerFS(fsys).ServeHTTP(w, r)
				return
			}
		}
		f, err := fsys.Open(index)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer f.Close()
		fi, _ := f.Stat()
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		http.ServeContent(w, r, "index.html", fi.ModTime(), f.(io.ReadSeeker))
	})
}

// spaFallbackDir — mode dev (http.Dir)
func spaFallbackDir(assetsDir string) http.Handler {
	deny := []string{"/api/", "/proxy/", "/sse/", "/css/", "/app/", "/media/", "/assets/", "/static/", "/vendor/"}

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
		candidate := assetsDir + r.URL.Path
		if r.URL.Path != "/" {
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				mimeFixFS(http.FileServer(http.Dir(assetsDir))).ServeHTTP(w, r)
				return
			}
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		http.ServeFile(w, r, assetsDir+"/index.html")
	})
}
