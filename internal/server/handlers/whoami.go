package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// Set at build time via ldflags.
var (
	Version   = "dev"
	GitHash   = "unknown"
	BuildTime = "unknown"
)

// WhoAmI returns application identity information.
func (h *Handler) WhoAmI(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{
		"name":       "sampo",
		"version":    Version,
		"hash":       GitHash,
		"build_time": BuildTime,
	}); err != nil {
		slog.Error("encoding whoami response", "error", err)
	}
}

// Health returns a simple health check response.
func (h *Handler) Health(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{"status": "ok"}); err != nil {
		slog.Error("encoding health response", "error", err)
	}
}
