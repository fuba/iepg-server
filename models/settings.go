package models

import (
	"database/sql"
	"encoding/json"
)

// Settings represents user settings
type Settings struct {
	ID           int    `json:"id" db:"id"`
	RecorderURLs string `json:"recorder_urls" db:"recorder_urls"` // JSON array of server URLs
	CreatedAt    string `json:"created_at" db:"created_at"`
	UpdatedAt    string `json:"updated_at" db:"updated_at"`
}

// RecorderServer represents a recorder server configuration
type RecorderServer struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// GetRecorderServers parses the recorder URLs JSON
func (s *Settings) GetRecorderServers() ([]RecorderServer, error) {
	if s.RecorderURLs == "" {
		return []RecorderServer{}, nil
	}
	
	var servers []RecorderServer
	err := json.Unmarshal([]byte(s.RecorderURLs), &servers)
	return servers, err
}

// SetRecorderServers sets the recorder URLs as JSON
func (s *Settings) SetRecorderServers(servers []RecorderServer) error {
	data, err := json.Marshal(servers)
	if err != nil {
		return err
	}
	s.RecorderURLs = string(data)
	return nil
}

// GetSettings retrieves or creates user settings
func GetSettings(db *sql.DB) (*Settings, error) {
	settings := &Settings{}
	
	// Try to get existing settings
	query := `SELECT id, recorder_urls, created_at, updated_at FROM settings WHERE id = 1`
	err := db.QueryRow(query).Scan(&settings.ID, &settings.RecorderURLs, &settings.CreatedAt, &settings.UpdatedAt)
	
	if err == sql.ErrNoRows {
		// Create default settings
		return createDefaultSettings(db)
	} else if err != nil {
		return nil, err
	}
	
	return settings, nil
}

// SaveSettings saves the settings to database
func (s *Settings) Save(db *sql.DB) error {
	query := `UPDATE settings SET recorder_urls = ?, updated_at = datetime('now') WHERE id = 1`
	_, err := db.Exec(query, s.RecorderURLs)
	return err
}

// createDefaultSettings creates default settings record
func createDefaultSettings(db *sql.DB) (*Settings, error) {
	// Create default recorder servers
	defaultServers := []RecorderServer{
		{Name: "Local Server", URL: "http://localhost:37569"},
	}
	
	settings := &Settings{ID: 1}
	err := settings.SetRecorderServers(defaultServers)
	if err != nil {
		return nil, err
	}
	
	query := `INSERT INTO settings (id, recorder_urls, created_at, updated_at) 
			  VALUES (1, ?, datetime('now'), datetime('now'))`
	_, err = db.Exec(query, settings.RecorderURLs)
	if err != nil {
		return nil, err
	}
	
	return GetSettings(db)
}