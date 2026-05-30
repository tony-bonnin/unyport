package middleware

import (
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// bucket suit les tentatives d'une IP sur une fenêtre glissante d'1 minute.
type bucket struct {
	count   int
	resetAt time.Time
}

// LoginRateLimiter limite les tentatives de connexion par IP.
// Fenêtre glissante de 1 minute, sans dépendance externe.
// Paradigme TRINITY : minimaliste, prévisible, pas de bibliothèque tierce inutile.
type LoginRateLimiter struct {
	mu          sync.Mutex
	buckets     map[string]*bucket
	maxAttempts int
	window      time.Duration
	logger      *slog.Logger
}

// NewLoginRateLimiter retourne un middleware HTTP qui bloque les IP ayant
// dépassé maxAttempts sur /api/login dans la dernière minute.
func NewLoginRateLimiter(maxAttempts int, logger *slog.Logger) func(http.Handler) http.Handler {
	rl := &LoginRateLimiter{
		buckets:     make(map[string]*bucket),
		maxAttempts: maxAttempts,
		window:      time.Minute,
		logger:      logger,
	}
	// Nettoyage périodique — RAM précieuse en Alpine DDM
	go rl.janitor()
	return rl.middleware
}

func (rl *LoginRateLimiter) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Uniquement sur POST — GET renverrait 405 de toute façon
		if r.Method != http.MethodPost {
			next.ServeHTTP(w, r)
			return
		}

		ip := extractIP(r)

		rl.mu.Lock()
		b, ok := rl.buckets[ip]
		now := time.Now()
		if !ok || now.After(b.resetAt) {
			b = &bucket{count: 0, resetAt: now.Add(rl.window)}
			rl.buckets[ip] = b
		}
		b.count++
		count := b.count
		rl.mu.Unlock()

		if count > rl.maxAttempts {
			rl.logger.Warn("rate limit login",
				"ip", ip,
				"attempts", count,
				"max", rl.maxAttempts,
			)
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.Header().Set("Retry-After", "60")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":"rate_limit","retry_after":60}`))
			return
		}

		next.ServeHTTP(w, r)
	})
}

// janitor purge les buckets expirés toutes les 2 minutes.
func (rl *LoginRateLimiter) janitor() {
	ticker := time.NewTicker(2 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		now := time.Now()
		rl.mu.Lock()
		for ip, b := range rl.buckets {
			if now.After(b.resetAt) {
				delete(rl.buckets, ip)
			}
		}
		rl.mu.Unlock()
	}
}

// extractIP extrait l'IP réelle du client.
// Respecte X-Forwarded-For uniquement si présent (derrière proxy).
func extractIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Premier segment = IP d'origine
		if i := strings.IndexByte(xff, ','); i >= 0 {
			xff = xff[:i]
		}
		if ip := net.ParseIP(strings.TrimSpace(xff)); ip != nil {
			return ip.String()
		}
	}
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}