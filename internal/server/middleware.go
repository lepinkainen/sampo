package server

import (
	"log/slog"
	"net/http"
	"time"
)

// statusWriter wraps http.ResponseWriter to capture the status code and bytes
// written for request logging.
type statusWriter struct {
	http.ResponseWriter
	status int
	bytes  int
}

// WriteHeader records the status code and forwards to the underlying writer.
func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

// Write forwards bytes to the underlying writer and accumulates the count.
func (w *statusWriter) Write(p []byte) (int, error) {
	n, err := w.ResponseWriter.Write(p)
	w.bytes += n
	return n, err
}

// noisyPaths are endpoints the frontend polls on a timer. Logging them floods
// the debug log with no diagnostic value, so they're skipped by requestLogger.
var noisyPaths = map[string]bool{
	"/health":                  true,
	"/api/analysis/settings":   true,
}

// requestLogger returns middleware that logs each HTTP request at Debug level,
// except for endpoints in noisyPaths (high-frequency polls).
func requestLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if noisyPaths[r.URL.Path] {
				next.ServeHTTP(w, r)
				return
			}
			start := time.Now()
			sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(sw, r)
			logger.Debug("http request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", sw.status,
				"bytes", sw.bytes,
				"duration_ms", time.Since(start).Milliseconds(),
			)
		})
	}
}
