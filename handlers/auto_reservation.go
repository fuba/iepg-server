// handlers/auto_reservation.go
package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/fuba/iepg-server/db"
	"github.com/fuba/iepg-server/models"
)

// CreateAutoReservationRuleRequest represents the request payload for creating an auto reservation rule
type CreateAutoReservationRuleRequest struct {
	Type        string                   `json:"type"`        // "keyword" or "series"
	Name        string                   `json:"name"`
	Enabled     bool                     `json:"enabled"`
	Priority    int                      `json:"priority"`
	RecorderURL string                   `json:"recorderUrl"`
	KeywordRule *models.KeywordRule      `json:"keywordRule,omitempty"`
	SeriesRule  *models.SeriesRule       `json:"seriesRule,omitempty"`
}

// HandleCreateAutoReservationRule handles POST /auto-reservations/rules
func HandleCreateAutoReservationRule(database *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		models.Log.Debug("HandleCreateAutoReservationRule: Processing request")

		var req CreateAutoReservationRuleRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			models.Log.Error("HandleCreateAutoReservationRule: Invalid JSON: %v", err)
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		// Validate request
		if req.Name == "" {
			http.Error(w, "Name is required", http.StatusBadRequest)
			return
		}
		if req.Type != "keyword" && req.Type != "series" {
			http.Error(w, "Type must be 'keyword' or 'series'", http.StatusBadRequest)
			return
		}
		if req.RecorderURL == "" {
			http.Error(w, "RecorderURL is required", http.StatusBadRequest)
			return
		}

		// Validate rule-specific data
		if req.Type == "keyword" {
			if req.KeywordRule == nil {
				http.Error(w, "KeywordRule is required for keyword type", http.StatusBadRequest)
				return
			}
			if len(req.KeywordRule.Keywords) == 0 {
				http.Error(w, "At least one keyword is required", http.StatusBadRequest)
				return
			}
		} else if req.Type == "series" {
			if req.SeriesRule == nil {
				http.Error(w, "SeriesRule is required for series type", http.StatusBadRequest)
				return
			}
			if req.SeriesRule.SeriesID == "" {
				http.Error(w, "SeriesID is required", http.StatusBadRequest)
				return
			}
		}

		// Create main rule
		rule := &models.AutoReservationRule{
			Type:        req.Type,
			Name:        req.Name,
			Enabled:     req.Enabled,
			Priority:    req.Priority,
			RecorderURL: req.RecorderURL,
		}

		if err := db.CreateAutoReservationRule(database, rule); err != nil {
			models.Log.Error("HandleCreateAutoReservationRule: Failed to create rule: %v", err)
			http.Error(w, "Failed to create rule", http.StatusInternalServerError)
			return
		}

		// Create rule-specific data
		if req.Type == "keyword" && req.KeywordRule != nil {
			req.KeywordRule.RuleID = rule.ID
			if err := db.CreateKeywordRule(database, req.KeywordRule); err != nil {
				models.Log.Error("HandleCreateAutoReservationRule: Failed to create keyword rule: %v", err)
				// Try to cleanup the main rule
				db.DeleteAutoReservationRule(database, rule.ID)
				http.Error(w, "Failed to create keyword rule", http.StatusInternalServerError)
				return
			}
		} else if req.Type == "series" && req.SeriesRule != nil {
			req.SeriesRule.RuleID = rule.ID
			if err := db.CreateSeriesRule(database, req.SeriesRule); err != nil {
				models.Log.Error("HandleCreateAutoReservationRule: Failed to create series rule: %v", err)
				// Try to cleanup the main rule
				db.DeleteAutoReservationRule(database, rule.ID)
				http.Error(w, "Failed to create series rule", http.StatusInternalServerError)
				return
			}
		}

		// Return created rule with details
		createdRule, err := db.GetAutoReservationRuleByID(database, rule.ID)
		if err != nil {
			models.Log.Error("HandleCreateAutoReservationRule: Failed to retrieve created rule: %v", err)
			http.Error(w, "Rule created but failed to retrieve", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(createdRule)

		models.Log.Info("HandleCreateAutoReservationRule: Created rule %s (%s)", rule.ID, rule.Name)
	}
}

// HandleGetAutoReservationRules handles GET /auto-reservations/rules
func HandleGetAutoReservationRules(database *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		models.Log.Debug("HandleGetAutoReservationRules: Processing request")

		rules, err := db.GetAutoReservationRules(database)
		if err != nil {
			models.Log.Error("HandleGetAutoReservationRules: Failed to get rules: %v", err)
			http.Error(w, "Failed to get rules", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(rules)

		models.Log.Debug("HandleGetAutoReservationRules: Returned %d rules", len(rules))
	}
}

// HandleGetAutoReservationRule handles GET /auto-reservations/rules/{id}
func HandleGetAutoReservationRule(database *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Extract ID from URL path
		pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(pathParts) < 3 {
			http.Error(w, "Invalid URL", http.StatusBadRequest)
			return
		}
		id := pathParts[2] // /auto-reservations/rules/{id}

		models.Log.Debug("HandleGetAutoReservationRule: Processing request for ID: %s", id)

		rule, err := db.GetAutoReservationRuleByID(database, id)
		if err != nil {
			if err == sql.ErrNoRows {
				http.Error(w, "Rule not found", http.StatusNotFound)
				return
			}
			models.Log.Error("HandleGetAutoReservationRule: Failed to get rule: %v", err)
			http.Error(w, "Failed to get rule", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(rule)
	}
}

// HandleUpdateAutoReservationRule handles PUT /auto-reservations/rules/{id}
func HandleUpdateAutoReservationRule(database *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Extract ID from URL path
		pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(pathParts) < 3 {
			http.Error(w, "Invalid URL", http.StatusBadRequest)
			return
		}
		id := pathParts[2] // /auto-reservations/rules/{id}

		models.Log.Debug("HandleUpdateAutoReservationRule: Processing request for ID: %s", id)

		var req CreateAutoReservationRuleRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			models.Log.Error("HandleUpdateAutoReservationRule: Invalid JSON: %v", err)
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		// Validate request
		if req.Name == "" {
			http.Error(w, "Name is required", http.StatusBadRequest)
			return
		}
		if req.Type != "keyword" && req.Type != "series" {
			http.Error(w, "Type must be 'keyword' or 'series'", http.StatusBadRequest)
			return
		}
		if req.RecorderURL == "" {
			http.Error(w, "RecorderURL is required", http.StatusBadRequest)
			return
		}

		// Check if rule exists
		existingRule, err := db.GetAutoReservationRuleByID(database, id)
		if err != nil {
			if err == sql.ErrNoRows {
				http.Error(w, "Rule not found", http.StatusNotFound)
				return
			}
			models.Log.Error("HandleUpdateAutoReservationRule: Failed to get existing rule: %v", err)
			http.Error(w, "Failed to get rule", http.StatusInternalServerError)
			return
		}

		// Update main rule
		rule := &models.AutoReservationRule{
			ID:          id,
			Type:        req.Type,
			Name:        req.Name,
			Enabled:     req.Enabled,
			Priority:    req.Priority,
			RecorderURL: req.RecorderURL,
			CreatedAt:   existingRule.CreatedAt, // Keep original creation time
		}

		if err := db.UpdateAutoReservationRule(database, rule); err != nil {
			models.Log.Error("HandleUpdateAutoReservationRule: Failed to update rule: %v", err)
			http.Error(w, "Failed to update rule", http.StatusInternalServerError)
			return
		}

		// Update rule-specific data
		if req.Type == "keyword" && req.KeywordRule != nil {
			req.KeywordRule.RuleID = id
			if err := db.CreateKeywordRule(database, req.KeywordRule); err != nil {
				models.Log.Error("HandleUpdateAutoReservationRule: Failed to update keyword rule: %v", err)
				http.Error(w, "Failed to update keyword rule", http.StatusInternalServerError)
				return
			}
		} else if req.Type == "series" && req.SeriesRule != nil {
			req.SeriesRule.RuleID = id
			if err := db.CreateSeriesRule(database, req.SeriesRule); err != nil {
				models.Log.Error("HandleUpdateAutoReservationRule: Failed to update series rule: %v", err)
				http.Error(w, "Failed to update series rule", http.StatusInternalServerError)
				return
			}
		}

		// Return updated rule with details
		updatedRule, err := db.GetAutoReservationRuleByID(database, id)
		if err != nil {
			models.Log.Error("HandleUpdateAutoReservationRule: Failed to retrieve updated rule: %v", err)
			http.Error(w, "Rule updated but failed to retrieve", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(updatedRule)

		models.Log.Info("HandleUpdateAutoReservationRule: Updated rule %s", id)
	}
}

// HandleDeleteAutoReservationRule handles DELETE /auto-reservations/rules/{id}
func HandleDeleteAutoReservationRule(database *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Extract ID from URL path
		pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(pathParts) < 3 {
			http.Error(w, "Invalid URL", http.StatusBadRequest)
			return
		}
		id := pathParts[2] // /auto-reservations/rules/{id}

		models.Log.Debug("HandleDeleteAutoReservationRule: Processing request for ID: %s", id)

		if err := db.DeleteAutoReservationRule(database, id); err != nil {
			if strings.Contains(err.Error(), "not found") {
				http.Error(w, "Rule not found", http.StatusNotFound)
				return
			}
			models.Log.Error("HandleDeleteAutoReservationRule: Failed to delete rule: %v", err)
			http.Error(w, "Failed to delete rule", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
		models.Log.Info("HandleDeleteAutoReservationRule: Deleted rule %s", id)
	}
}

// HandleGetAutoReservationLogs handles GET /auto-reservations/logs
func HandleGetAutoReservationLogs(database *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		models.Log.Debug("HandleGetAutoReservationLogs: Processing request")

		// Parse query parameters
		ruleID := r.URL.Query().Get("ruleId")
		limitStr := r.URL.Query().Get("limit")
		
		limit := 100 // Default limit
		if limitStr != "" {
			if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
				limit = parsedLimit
			}
		}

		logs, err := db.GetAutoReservationLogs(database, ruleID, limit)
		if err != nil {
			models.Log.Error("HandleGetAutoReservationLogs: Failed to get logs: %v", err)
			http.Error(w, "Failed to get logs", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(logs)

		models.Log.Debug("HandleGetAutoReservationLogs: Returned %d logs", len(logs))
	}
}