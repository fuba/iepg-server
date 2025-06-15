// services/auto_reservation_engine.go
package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/fuba/iepg-server/db"
	"github.com/fuba/iepg-server/models"
)

// AutoReservationEngine manages automatic reservation processing
type AutoReservationEngine struct {
	database    *sql.DB
	recorderURL string
	interval    time.Duration
}

// NewAutoReservationEngine creates a new auto reservation engine
func NewAutoReservationEngine(database *sql.DB, recorderURL string) *AutoReservationEngine {
	return &AutoReservationEngine{
		database:    database,
		recorderURL: recorderURL,
		interval:    5 * time.Minute, // Check every 5 minutes
	}
}

// Start begins the auto reservation monitoring process
func (e *AutoReservationEngine) Start(ctx context.Context) {
	models.Log.Info("AutoReservationEngine: Starting auto reservation monitoring")
	
	ticker := time.NewTicker(e.interval)
	defer ticker.Stop()

	// Run initial check
	e.processAutoReservations()

	for {
		select {
		case <-ctx.Done():
			models.Log.Info("AutoReservationEngine: Stopping auto reservation monitoring")
			return
		case <-ticker.C:
			e.processAutoReservations()
		}
	}
}

// processAutoReservations checks all enabled rules against new programs
func (e *AutoReservationEngine) processAutoReservations() {
	models.Log.Debug("AutoReservationEngine: Starting auto reservation processing")

	// Get all enabled rules
	rules, err := db.GetEnabledAutoReservationRules(e.database)
	if err != nil {
		models.Log.Error("AutoReservationEngine: Failed to get enabled rules: %v", err)
		return
	}

	if len(rules) == 0 {
		models.Log.Debug("AutoReservationEngine: No enabled rules found")
		return
	}

	models.Log.Debug("AutoReservationEngine: Processing %d enabled rules", len(rules))

	// Get programs that might be candidates for auto reservation
	// Look for programs starting in the next 24 hours that don't have reservations yet
	now := time.Now()
	startFrom := now.UnixMilli()
	startTo := now.Add(24 * time.Hour).UnixMilli()

	programs, err := db.SearchPrograms(e.database, "", 0, startFrom, startTo, 0)
	if err != nil {
		models.Log.Error("AutoReservationEngine: Failed to get programs: %v", err)
		return
	}

	models.Log.Debug("AutoReservationEngine: Found %d programs to check", len(programs))

	// Check each rule against matching programs
	for _, rule := range rules {
		e.processRule(rule, programs)
	}

	models.Log.Debug("AutoReservationEngine: Completed auto reservation processing")
}

// processRule processes a single auto reservation rule against programs
func (e *AutoReservationEngine) processRule(rule models.AutoReservationRuleWithDetails, programs []models.Program) {
	models.Log.Debug("AutoReservationEngine: Processing rule %s (%s)", rule.ID, rule.Name)

	matchCount := 0
	for _, program := range programs {
		if e.checkRuleMatch(rule, program) {
			matchCount++
			e.createReservationForProgram(rule, program)
		}
	}

	models.Log.Debug("AutoReservationEngine: Rule %s matched %d programs", rule.ID, matchCount)
}

// checkRuleMatch checks if a program matches the given rule
func (e *AutoReservationEngine) checkRuleMatch(rule models.AutoReservationRuleWithDetails, program models.Program) bool {
	// Check if we already have a reservation for this program
	if e.hasExistingReservation(program.ID) {
		return false
	}

	// Check if we already processed this program for this rule
	if e.hasExistingLog(rule.ID, program.ID) {
		return false
	}

	switch rule.Type {
	case "keyword":
		return e.checkKeywordMatch(rule.KeywordRule, program)
	case "series":
		return e.checkSeriesMatch(rule.SeriesRule, program)
	default:
		models.Log.Error("AutoReservationEngine: Unknown rule type: %s", rule.Type)
		return false
	}
}

// checkKeywordMatch checks if a program matches keyword rule criteria
func (e *AutoReservationEngine) checkKeywordMatch(keywordRule *models.KeywordRule, program models.Program) bool {
	if keywordRule == nil {
		return false
	}

	// Check service ID filter
	if len(keywordRule.ServiceIDs) > 0 {
		serviceMatched := false
		for _, serviceID := range keywordRule.ServiceIDs {
			if program.ServiceID == serviceID {
				serviceMatched = true
				break
			}
		}
		if !serviceMatched {
			return false
		}
	}

	// Normalize program text for search
	programText := strings.ToLower(program.Name + " " + program.Description)
	
	// Check keywords (all must match - AND condition)
	for _, keyword := range keywordRule.Keywords {
		if !strings.Contains(programText, strings.ToLower(keyword)) {
			return false
		}
	}

	// Check exclude words (none must match)
	for _, excludeWord := range keywordRule.ExcludeWords {
		if strings.Contains(programText, strings.ToLower(excludeWord)) {
			return false
		}
	}

	return true
}

// checkSeriesMatch checks if a program matches series rule criteria
func (e *AutoReservationEngine) checkSeriesMatch(seriesRule *models.SeriesRule, program models.Program) bool {
	if seriesRule == nil {
		return false
	}

	// Check if program has series information
	if program.Series == nil {
		return false
	}

	// Check series ID match
	if fmt.Sprintf("%d", program.Series.ID) != seriesRule.SeriesID {
		return false
	}

	// Check service ID filter if specified
	if seriesRule.ServiceID != 0 && program.ServiceID != seriesRule.ServiceID {
		return false
	}

	return true
}

// hasExistingReservation checks if a reservation already exists for the program
func (e *AutoReservationEngine) hasExistingReservation(programID int64) bool {
	var count int
	err := e.database.QueryRow("SELECT COUNT(*) FROM reservations WHERE programId = ?", programID).Scan(&count)
	if err != nil {
		models.Log.Error("AutoReservationEngine: Failed to check existing reservation: %v", err)
		return false
	}
	return count > 0
}

// hasExistingLog checks if we already processed this program for this rule
func (e *AutoReservationEngine) hasExistingLog(ruleID string, programID int64) bool {
	var count int
	err := e.database.QueryRow("SELECT COUNT(*) FROM auto_reservation_logs WHERE ruleId = ? AND programId = ?", ruleID, programID).Scan(&count)
	if err != nil {
		models.Log.Error("AutoReservationEngine: Failed to check existing log: %v", err)
		return false
	}
	return count > 0
}

// createReservationForProgram attempts to create a reservation for the matched program
func (e *AutoReservationEngine) createReservationForProgram(rule models.AutoReservationRuleWithDetails, program models.Program) {
	models.Log.Info("AutoReservationEngine: Creating reservation for program %d (%s) using rule %s", 
		program.ID, program.Name, rule.Name)

	// Create reservation request
	reservationData := map[string]interface{}{
		"programId":   program.ID,
		"serviceId":   program.ServiceID,
		"name":        program.Name,
		"startAt":     program.StartAt,
		"duration":    program.Duration,
		"recorderUrl": rule.RecorderURL,
	}

	jsonData, err := json.Marshal(reservationData)
	if err != nil {
		e.logAutoReservation(rule.ID, program.ID, "", "failed", fmt.Sprintf("JSON marshal error: %v", err))
		return
	}

	// Make HTTP request to create reservation
	resp, err := http.Post("http://localhost:40870/reservations", "application/json", strings.NewReader(string(jsonData)))
	if err != nil {
		e.logAutoReservation(rule.ID, program.ID, "", "failed", fmt.Sprintf("HTTP request error: %v", err))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		e.logAutoReservation(rule.ID, program.ID, "", "failed", fmt.Sprintf("HTTP %d: %s", resp.StatusCode, resp.Status))
		return
	}

	// Parse response to get reservation ID
	var reservationResponse map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&reservationResponse); err != nil {
		e.logAutoReservation(rule.ID, program.ID, "", "reserved", "Reservation created but failed to parse response")
		return
	}

	reservationID := ""
	if id, ok := reservationResponse["id"].(string); ok {
		reservationID = id
	}

	e.logAutoReservation(rule.ID, program.ID, reservationID, "reserved", "")
	models.Log.Info("AutoReservationEngine: Successfully created reservation %s for program %d", reservationID, program.ID)
}

// logAutoReservation creates a log entry for auto reservation processing
func (e *AutoReservationEngine) logAutoReservation(ruleID string, programID int64, reservationID, status, reason string) {
	log := &models.AutoReservationLog{
		RuleID:        ruleID,
		ProgramID:     programID,
		ReservationID: reservationID,
		Status:        status,
		Reason:        reason,
	}

	if err := db.CreateAutoReservationLog(e.database, log); err != nil {
		models.Log.Error("AutoReservationEngine: Failed to create log: %v", err)
	}
}