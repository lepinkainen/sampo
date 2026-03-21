package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/lepinkainen/filemanager/internal/analysis"
)

type analysisSettingsResponse struct {
	AutoBrowseEnabled bool            `json:"autoBrowseEnabled"`
	BrowseStatus      analysis.Status `json:"browseStatus"`
}

type analysisSettingsRequest struct {
	AutoBrowseEnabled bool `json:"autoBrowseEnabled"`
}

func (h *Handler) analysisSettingsResponse() analysisSettingsResponse {
	var status analysis.Status
	if h.browseCoordinator != nil {
		status = h.browseCoordinator.Status()
	}
	return analysisSettingsResponse{
		AutoBrowseEnabled: h.AutoBrowseEnabled(),
		BrowseStatus:      status,
	}
}

// GetAnalysisSettings returns runtime browse analysis settings.
func (h *Handler) GetAnalysisSettings(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(h.analysisSettingsResponse())
}

// UpdateAnalysisSettings updates runtime browse analysis settings.
func (h *Handler) UpdateAnalysisSettings(w http.ResponseWriter, r *http.Request) {
	var req analysisSettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	h.SetAutoBrowseEnabled(req.AutoBrowseEnabled)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(h.analysisSettingsResponse())
}
