package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/fuba/iepg-server/models"
)

// SettingsHandler handles settings-related requests
type SettingsHandler struct {
	db *sql.DB
}

// NewSettingsHandler creates a new settings handler
func NewSettingsHandler(db *sql.DB) *SettingsHandler {
	return &SettingsHandler{db: db}
}

// GetSettings handles GET /api/settings
func (h *SettingsHandler) GetSettings(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	settings, err := models.GetSettings(h.db)
	if err != nil {
		models.Log.Error("GetSettings: Failed to get settings: %v", err)
		http.Error(w, "Failed to get settings", http.StatusInternalServerError)
		return
	}

	servers, err := settings.GetRecorderServers()
	if err != nil {
		models.Log.Error("GetSettings: Failed to parse recorder servers: %v", err)
		http.Error(w, "Failed to parse recorder servers", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"recorderServers": servers,
	}

	json.NewEncoder(w).Encode(response)
}

// UpdateRecorderServers handles POST /api/settings/recorder-servers
func (h *SettingsHandler) UpdateRecorderServers(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var request struct {
		RecorderServers []models.RecorderServer `json:"recorderServers"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		models.Log.Error("UpdateRecorderServers: Failed to decode request: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Validate servers
	for i, server := range request.RecorderServers {
		if server.Name == "" {
			models.Log.Error("UpdateRecorderServers: Server name is required at index %d", i)
			http.Error(w, "Server name is required", http.StatusBadRequest)
			return
		}
		if server.URL == "" {
			models.Log.Error("UpdateRecorderServers: Server URL is required at index %d", i)
			http.Error(w, "Server URL is required", http.StatusBadRequest)
			return
		}
	}

	settings, err := models.GetSettings(h.db)
	if err != nil {
		models.Log.Error("UpdateRecorderServers: Failed to get settings: %v", err)
		http.Error(w, "Failed to get settings", http.StatusInternalServerError)
		return
	}

	err = settings.SetRecorderServers(request.RecorderServers)
	if err != nil {
		models.Log.Error("UpdateRecorderServers: Failed to set recorder servers: %v", err)
		http.Error(w, "Failed to set recorder servers", http.StatusInternalServerError)
		return
	}

	err = settings.Save(h.db)
	if err != nil {
		models.Log.Error("UpdateRecorderServers: Failed to save settings: %v", err)
		http.Error(w, "Failed to save settings", http.StatusInternalServerError)
		return
	}

	models.Log.Info("Updated recorder servers: %d servers", len(request.RecorderServers))

	response := map[string]interface{}{
		"success": true,
		"message": "Recorder servers updated successfully",
	}

	json.NewEncoder(w).Encode(response)
}