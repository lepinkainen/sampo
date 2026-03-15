package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

type rootResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// ListRoots returns all configured roots.
func (h *Handler) ListRoots(w http.ResponseWriter, _ *http.Request) {
	roots := h.roots.List()

	resp := make([]rootResponse, 0, len(roots))
	for _, r := range roots {
		resp = append(resp, rootResponse{
			ID:   r.ID,
			Name: r.Name,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		slog.Error("encoding roots response", "error", err)
	}
}
