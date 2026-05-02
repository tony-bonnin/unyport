package proxy

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"unyport/config"
)

// Make crée un reverse proxy durci monté sous /proxy/<slug>/.
func Make(app config.App, mount string, logger *slog.Logger) http.Handler {
	mount = normalizMount(app.Name, mount)

	resolved := resolveHost(app.Host, logger)
	target := fmt.Sprintf("http://%s:%d", resolved, app.Port)

	backend, err := url.Parse(target)
	if err != nil {
		logger.Error("invalid backend url", "url", target)
		panic(err)
	}

	rp := httputil.NewSingleHostReverseProxy(backend)
	rp.Transport = &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           (&net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
	rp.FlushInterval = 100 * time.Millisecond

	orig := rp.Director
	rp.Director = func(req *http.Request) {
		orig(req)
		req.Header.Del("X-Forwarded-Host")
		req.Header.Del("Forwarded")
		req.Header.Del("Origin")
		if ip, _, err := net.SplitHostPort(req.RemoteAddr); err == nil {
			req.Header.Set("X-Forwarded-For", ip)
		}
		req.Header.Set("X-Forwarded-Proto", "http")
		req.Header.Set("X-Forwarded-Prefix", mount)
		req.Host = backend.Host
	}

	rp.ModifyResponse = func(resp *http.Response) error {
		h := resp.Header
		h.Set("X-Frame-Options", "DENY")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
		h.Set("X-Accel-Buffering", "no")

		if strings.Contains(strings.ToLower(mount), "/proxy/ttyd/") {
			setTTYdCSP(resp)
		}

		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			if !strings.HasPrefix(strings.ToLower(resp.Header.Get("Content-Type")), "application/json") {
				resp.Header.Set("Location", "/")
				resp.StatusCode = http.StatusFound
				resp.Header.Del("Content-Length")
				resp.Body = io.NopCloser(bytes.NewReader(nil))
				return nil
			}
		}

		if loc := resp.Header.Get("Location"); loc != "" {
			if newLoc, ok := rewriteLocation(loc, mount); ok {
				resp.Header.Set("Location", newLoc)
			}
		}

		if cookies := resp.Header.Values("Set-Cookie"); len(cookies) > 0 {
			resp.Header.Del("Set-Cookie")
			for _, c := range cookies {
				resp.Header.Add("Set-Cookie", rewriteCookiePath(c, mount))
			}
		}

		return nil
	}

	rp.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		logger.Error("proxy error", "app", app.Name, "err", err)
		http.Redirect(w, r, "/", http.StatusFound)
	}

	logger.Info("proxy registered", "app", app.Name, "target", target, "mount", mount)
	return http.StripPrefix(mount, rp)
}

// ---- Helpers ----

func normalizMount(name, mount string) string {
	if mount == "" {
		mount = "/proxy/" + sanitize(name)
	}
	if !strings.HasPrefix(mount, "/") {
		mount = "/" + mount
	}
	if !strings.HasSuffix(mount, "/") {
		mount += "/"
	}
	return mount
}

func resolveHost(host string, logger *slog.Logger) string {
	if addrs, err := net.LookupHost(host); err == nil && len(addrs) > 0 {
		logger.Debug("dns resolved", "host", host, "ip", addrs[0])
		return addrs[0]
	}
	return host
}

func sanitize(s string) string {
	return strings.ReplaceAll(strings.ToLower(strings.TrimSpace(s)), " ", "-")
}

func setTTYdCSP(resp *http.Response) {
	resp.Header.Del("Content-Security-Policy")
	resp.Header.Del("Content-Security-Policy-Report-Only")
	resp.Header.Set("Content-Security-Policy",
		"default-src 'self'; "+
			"script-src 'self' https://cdn.jsdelivr.net 'unsafe-inline'; "+
			"style-src 'self' https://cdn.jsdelivr.net 'unsafe-inline'; "+
			"connect-src 'self' ws: wss:; "+
			"img-src 'self' data:; font-src 'self' data:; "+
			"worker-src 'self' blob:; "+
			"frame-ancestors 'self'; base-uri 'self';")
}

func rewriteLocation(raw, mount string) (string, bool) {
	if strings.HasPrefix(raw, "/") && !strings.HasPrefix(raw, mount) {
		return mount + strings.TrimPrefix(raw, "/"), true
	}
	u, err := url.Parse(raw)
	if err == nil && u.Path != "" && !strings.HasPrefix(u.Path, mount) {
		u.Path = mount + strings.TrimPrefix(u.Path, "/")
		return u.String(), true
	}
	return raw, false
}

func rewriteCookiePath(cookieLine, mount string) string {
	parts := strings.Split(cookieLine, ";")
	out := make([]string, 0, len(parts))
	hasPath := false
	for _, p := range parts {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(p)), "path=") {
			hasPath = true
			out = append(out, "Path="+mount)
		} else {
			out = append(out, strings.TrimSpace(p))
		}
	}
	if !hasPath {
		out = append(out, "Path="+mount)
	}
	return strings.Join(out, "; ")
}