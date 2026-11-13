package api

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"
)

// --- CORS ---

func (s *Server) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// --- logging ---

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// Hijack delegates to the underlying writer so protocol upgrades (WebSocket)
// keep working through the logging middleware.
func (r *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if h, ok := r.ResponseWriter.(http.Hijacker); ok {
		return h.Hijack()
	}
	return nil, nil, fmt.Errorf("response writer does not support hijacking")
}

// Flush delegates to the underlying writer when supported.
func (r *statusRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (s *Server) logRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		s.logger.Printf("%s %s -> %d (%s)", r.Method, r.URL.Path, rec.status, time.Since(start).Round(time.Microsecond))
	})
}

// --- recovery ---

func (s *Server) recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				s.logger.Printf("api: panic serving %s %s: %v\n%s", r.Method, r.URL.Path, rec, debug.Stack())
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				w.WriteHeader(http.StatusInternalServerError)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "internal server error"})
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// --- rate limiting (bonus feature) ---

// rateLimiter implements a simple fixed-window per-IP limiter for the
// location update endpoint.
type rateLimiter struct {
	mu      sync.Mutex
	windows map[string]*limitWindow
	perMin  int // 0 disables limiting
}

type limitWindow struct {
	count   int
	started time.Time
}

func newRateLimiter(perMin int) *rateLimiter {
	return &rateLimiter{windows: map[string]*limitWindow{}, perMin: perMin}
}

// rateLimitPerMinute reads RATE_LIMIT_PER_MIN from the environment.
// Values <= 0 disable the limiter; the default is 300.
func rateLimitPerMinute() int {
	raw := strings.TrimSpace(os.Getenv("RATE_LIMIT_PER_MIN"))
	if raw == "" {
		return 300
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return 0
	}
	return n
}

// limit wraps a handler with the per-IP rate limiter (no-op when disabled).
func (s *Server) limit(next http.HandlerFunc) http.HandlerFunc {
	if s.limiter == nil || s.limiter.perMin <= 0 {
		return next
	}
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.limiter.allow(clientIP(r)) {
			start := time.Now()
			s.fail(w, start, http.StatusTooManyRequests, "rate limit exceeded, try again later")
			return
		}
		next(w, r)
	}
}

func (l *rateLimiter) allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	w, ok := l.windows[ip]
	if !ok || now.Sub(w.started) >= time.Minute {
		l.windows[ip] = &limitWindow{count: 1, started: now}
		// Opportunistically prune stale windows so the map stays bounded.
		if len(l.windows) > 10_000 {
			for k, v := range l.windows {
				if now.Sub(v.started) >= 2*time.Minute {
					delete(l.windows, k)
				}
			}
		}
		return true
	}
	w.count++
	return w.count <= l.perMin
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if i := strings.IndexByte(xff, ','); i >= 0 {
			return strings.TrimSpace(xff[:i])
		}
		return strings.TrimSpace(xff)
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
