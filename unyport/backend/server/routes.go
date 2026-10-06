package server

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"unyport/auth"
	"unyport/config"
	"unyport/middleware"
	"unyport/sse"
	"unyport/xenctl"
)

var publicVersionCache struct {
	sync.Mutex
	checked time.Time
	latest  string
}

// mimeTypes — table explicite pour les environnements sans /etc/mime.types
// (Alpine minimal, BusyBox). http.FileServerFS utilise mime.TypeByExtension
// qui dépend du système — on enregistre les types essentiels au démarrage.
var mimeTypes = map[string]string{
	".css":         "text/css; charset=utf-8",
	".js":          "text/javascript; charset=utf-8",
	".html":        "text/html; charset=utf-8",
	".json":        "application/json; charset=utf-8",
	".webmanifest": "application/manifest+json; charset=utf-8",
	".svg":         "image/svg+xml",
	".png":         "image/png",
	".jpg":         "image/jpeg",
	".jpeg":        "image/jpeg",
	".ico":         "image/x-icon",
	".woff":        "font/woff",
	".woff2":       "font/woff2",
	".ttf":         "font/ttf",
	".map":         "application/json",
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
		switch r.URL.Path {
		case "/sw.js", "/manifest.json":
			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		}
		if r.URL.Path == "/manifest.json" {
			w.Header().Set("Content-Type", "application/manifest+json; charset=utf-8")
			h.ServeHTTP(w, r)
			return
		}
		ext := strings.ToLower(filepath.Ext(r.URL.Path))
		if ct, ok := mimeTypes[ext]; ok {
			w.Header().Set("Content-Type", ct)
		}
		h.ServeHTTP(w, r)
	})
}

func publicVersionHandler(w http.ResponseWriter, r *http.Request) {
	latest := latestUnyPortRelease()
	resp := map[string]any{
		"version":    config.Version,
		"latest":     latest,
		"up_to_date": latest == "" || compareDotVersions(strings.TrimPrefix(config.Version, "v"), strings.TrimPrefix(latest, "v")) >= 0,
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(resp)
}

func latestUnyPortRelease() string {
	publicVersionCache.Lock()
	defer publicVersionCache.Unlock()
	if publicVersionCache.latest != "" && time.Since(publicVersionCache.checked) < 10*time.Minute {
		return publicVersionCache.latest
	}
	publicVersionCache.checked = time.Now()

	client := &http.Client{Timeout: 3 * time.Second}
	req, err := http.NewRequest(http.MethodGet, "https://api.github.com/repos/tony-bonnin/unyport/releases/latest", nil)
	if err != nil {
		return publicVersionCache.latest
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "unyport-version-check")
	res, err := client.Do(req)
	if err != nil {
		return publicVersionCache.latest
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return publicVersionCache.latest
	}
	var payload struct {
		TagName string `json:"tag_name"`
	}
	if json.NewDecoder(res.Body).Decode(&payload) != nil {
		return publicVersionCache.latest
	}
	if payload.TagName != "" {
		publicVersionCache.latest = strings.TrimPrefix(payload.TagName, "v")
	}
	return publicVersionCache.latest
}

func compareDotVersions(a, b string) int {
	as := strings.Split(a, ".")
	bs := strings.Split(b, ".")
	for i := 0; i < 3; i++ {
		av, bv := 0, 0
		if i < len(as) {
			_, _ = fmt.Sscanf(as[i], "%d", &av)
		}
		if i < len(bs) {
			_, _ = fmt.Sscanf(bs[i], "%d", &bv)
		}
		if av > bv {
			return 1
		}
		if av < bv {
			return -1
		}
	}
	return 0
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
	mux.HandleFunc("/api/public/version", publicVersionHandler)
	mux.Handle("/api/login", loginRL(http.HandlerFunc(authHandler.Login)))
	mux.HandleFunc("/api/logout", authHandler.Logout)
	mux.HandleFunc("/api/session", authHandler.Session)

	// Branding — GET public, PATCH/DELETE admin
	mux.HandleFunc("/api/branding", brandingHandler.GetBranding)
	mux.HandleFunc("/api/oauth/providers", oauthSvc.ProvidersHandler)
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
		mux.Handle("/sitemap.xml", mimeFixFS(http.FileServer(http.Dir(assetsDir))))
		mux.Handle("/manifest.json", mimeFixFS(http.FileServer(http.Dir(assetsDir))))
		mux.Handle("/sw.js", mimeFixFS(http.FileServer(http.Dir(assetsDir))))
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
		mux.Handle("/sitemap.xml", mimeFixFS(http.FileServerFS(pub)))
		mux.Handle("/manifest.json", mimeFixFS(http.FileServerFS(pub)))
		mux.Handle("/sw.js", mimeFixFS(http.FileServerFS(pub)))
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

	// ---- API Xen native xl — lecture tous rôles, actions operator/admin ----
	xenHandler := xenctl.NewHandler(xenctl.NewClient(), logger)
	mux.Handle("/api/xen/info", authMW(http.HandlerFunc(xenHandler.Info)))
	mux.Handle("/api/xen/domains", authMW(http.HandlerFunc(xenHandler.Domains)))
	mux.Handle("/api/xen/domains/create", adminMW(http.HandlerFunc(xenHandler.Create)))
	mux.Handle("/api/xen/domains/", writeMW(http.HandlerFunc(xenHandler.DomainAction)))

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

	return mux
}

// spaFallback — mode prod (embed FS)
func spaFallback(fsys fs.FS, index string) http.Handler {
	deny := []string{"/api/", "/sse/", "/css/", "/app/", "/media/", "/assets/", "/static/", "/vendor/"}

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
	deny := []string{"/api/", "/sse/", "/css/", "/app/", "/media/", "/assets/", "/static/", "/vendor/"}

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
