// db/auto_reservation.go
package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/fuba/iepg-server/models"
	"github.com/google/uuid"
)

// CreateAutoReservationRule creates a new auto reservation rule
func CreateAutoReservationRule(db *sql.DB, rule *models.AutoReservationRule) error {
	if rule.ID == "" {
		rule.ID = uuid.New().String()
	}
	rule.CreatedAt = time.Now()
	rule.UpdatedAt = time.Now()

	_, err := db.Exec(`
		INSERT INTO auto_reservation_rules (id, type, name, enabled, priority, recorderUrl, createdAt, updatedAt)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, rule.ID, rule.Type, rule.Name, rule.Enabled, rule.Priority, rule.RecorderURL, 
		rule.CreatedAt.UnixMilli(), rule.UpdatedAt.UnixMilli())
	
	if err != nil {
		models.Log.Error("CreateAutoReservationRule: Failed to create rule: %v", err)
		return err
	}

	models.Log.Info("CreateAutoReservationRule: Created rule %s (%s)", rule.ID, rule.Name)
	return nil
}

// CreateKeywordRule creates a keyword rule for an auto reservation rule
func CreateKeywordRule(db *sql.DB, rule *models.KeywordRule) error {
	// Convert slices to JSON strings for storage
	keywordsJSON, _ := json.Marshal(rule.Keywords)
	genresJSON, _ := json.Marshal(rule.Genres)
	serviceIDsJSON, _ := json.Marshal(rule.ServiceIDs)
	excludeWordsJSON, _ := json.Marshal(rule.ExcludeWords)

	_, err := db.Exec(`
		INSERT OR REPLACE INTO keyword_rules (ruleId, keywords, genres, serviceIds, excludeWords)
		VALUES (?, ?, ?, ?, ?)
	`, rule.RuleID, string(keywordsJSON), string(genresJSON), string(serviceIDsJSON), string(excludeWordsJSON))
	
	if err != nil {
		models.Log.Error("CreateKeywordRule: Failed to create keyword rule: %v", err)
		return err
	}

	models.Log.Info("CreateKeywordRule: Created keyword rule for rule %s", rule.RuleID)
	return nil
}

// CreateSeriesRule creates a series rule for an auto reservation rule
func CreateSeriesRule(db *sql.DB, rule *models.SeriesRule) error {
	_, err := db.Exec(`
		INSERT OR REPLACE INTO series_rules (ruleId, seriesId, programName, serviceId)
		VALUES (?, ?, ?, ?)
	`, rule.RuleID, rule.SeriesID, rule.ProgramName, rule.ServiceID)
	
	if err != nil {
		models.Log.Error("CreateSeriesRule: Failed to create series rule: %v", err)
		return err
	}

	models.Log.Info("CreateSeriesRule: Created series rule for rule %s", rule.RuleID)
	return nil
}

// GetAutoReservationRules retrieves all auto reservation rules
func GetAutoReservationRules(db *sql.DB) ([]models.AutoReservationRuleWithDetails, error) {
	rows, err := db.Query(`
		SELECT id, type, name, enabled, priority, recorderUrl, createdAt, updatedAt
		FROM auto_reservation_rules
		ORDER BY priority DESC, createdAt DESC
	`)
	if err != nil {
		models.Log.Error("GetAutoReservationRules: Query failed: %v", err)
		return nil, err
	}
	defer rows.Close()

	var rules []models.AutoReservationRuleWithDetails
	for rows.Next() {
		var rule models.AutoReservationRuleWithDetails
		var createdAt, updatedAt int64
		var enabled int
		
		err := rows.Scan(&rule.ID, &rule.Type, &rule.Name, &enabled, &rule.Priority, 
			&rule.RecorderURL, &createdAt, &updatedAt)
		if err != nil {
			models.Log.Error("GetAutoReservationRules: Scan failed: %v", err)
			continue
		}
		
		rule.Enabled = enabled != 0
		rule.CreatedAt = time.UnixMilli(createdAt)
		rule.UpdatedAt = time.UnixMilli(updatedAt)

		// Load rule details based on type
		switch rule.Type {
		case "keyword":
			keywordRule, err := getKeywordRule(db, rule.ID)
			if err != nil {
				models.Log.Error("GetAutoReservationRules: Failed to load keyword rule for %s: %v", rule.ID, err)
			} else {
				rule.KeywordRule = keywordRule
			}
		case "series":
			seriesRule, err := getSeriesRule(db, rule.ID)
			if err != nil {
				models.Log.Error("GetAutoReservationRules: Failed to load series rule for %s: %v", rule.ID, err)
			} else {
				rule.SeriesRule = seriesRule
			}
		}

		rules = append(rules, rule)
	}

	models.Log.Info("GetAutoReservationRules: Retrieved %d rules", len(rules))
	return rules, nil
}

// GetAutoReservationRuleByID retrieves a specific auto reservation rule by ID
func GetAutoReservationRuleByID(db *sql.DB, id string) (*models.AutoReservationRuleWithDetails, error) {
	var rule models.AutoReservationRuleWithDetails
	var createdAt, updatedAt int64
	var enabled int
	
	err := db.QueryRow(`
		SELECT id, type, name, enabled, priority, recorderUrl, createdAt, updatedAt
		FROM auto_reservation_rules WHERE id = ?
	`, id).Scan(&rule.ID, &rule.Type, &rule.Name, &enabled, &rule.Priority,
		&rule.RecorderURL, &createdAt, &updatedAt)
	
	if err != nil {
		if err == sql.ErrNoRows {
			models.Log.Info("GetAutoReservationRuleByID: Rule not found: %s", id)
		} else {
			models.Log.Error("GetAutoReservationRuleByID: Query failed: %v", err)
		}
		return nil, err
	}
	
	rule.Enabled = enabled != 0
	rule.CreatedAt = time.UnixMilli(createdAt)
	rule.UpdatedAt = time.UnixMilli(updatedAt)

	// Load rule details based on type
	switch rule.Type {
	case "keyword":
		keywordRule, err := getKeywordRule(db, rule.ID)
		if err != nil {
			models.Log.Error("GetAutoReservationRuleByID: Failed to load keyword rule: %v", err)
		} else {
			rule.KeywordRule = keywordRule
		}
	case "series":
		seriesRule, err := getSeriesRule(db, rule.ID)
		if err != nil {
			models.Log.Error("GetAutoReservationRuleByID: Failed to load series rule: %v", err)
		} else {
			rule.SeriesRule = seriesRule
		}
	}

	return &rule, nil
}

// UpdateAutoReservationRule updates an existing auto reservation rule
func UpdateAutoReservationRule(db *sql.DB, rule *models.AutoReservationRule) error {
	rule.UpdatedAt = time.Now()
	
	result, err := db.Exec(`
		UPDATE auto_reservation_rules 
		SET type = ?, name = ?, enabled = ?, priority = ?, recorderUrl = ?, updatedAt = ?
		WHERE id = ?
	`, rule.Type, rule.Name, rule.Enabled, rule.Priority, rule.RecorderURL, 
		rule.UpdatedAt.UnixMilli(), rule.ID)
	
	if err != nil {
		models.Log.Error("UpdateAutoReservationRule: Update failed: %v", err)
		return err
	}
	
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("rule not found: %s", rule.ID)
	}

	models.Log.Info("UpdateAutoReservationRule: Updated rule %s", rule.ID)
	return nil
}

// DeleteAutoReservationRule deletes an auto reservation rule and its related data
func DeleteAutoReservationRule(db *sql.DB, id string) error {
	result, err := db.Exec("DELETE FROM auto_reservation_rules WHERE id = ?", id)
	if err != nil {
		models.Log.Error("DeleteAutoReservationRule: Delete failed: %v", err)
		return err
	}
	
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("rule not found: %s", id)
	}

	models.Log.Info("DeleteAutoReservationRule: Deleted rule %s", id)
	return nil
}

// GetEnabledAutoReservationRules retrieves only enabled auto reservation rules for processing
func GetEnabledAutoReservationRules(db *sql.DB) ([]models.AutoReservationRuleWithDetails, error) {
	rows, err := db.Query(`
		SELECT id, type, name, enabled, priority, recorderUrl, createdAt, updatedAt
		FROM auto_reservation_rules
		WHERE enabled = 1
		ORDER BY priority DESC
	`)
	if err != nil {
		models.Log.Error("GetEnabledAutoReservationRules: Query failed: %v", err)
		return nil, err
	}
	defer rows.Close()

	var rules []models.AutoReservationRuleWithDetails
	for rows.Next() {
		var rule models.AutoReservationRuleWithDetails
		var createdAt, updatedAt int64
		var enabled int
		
		err := rows.Scan(&rule.ID, &rule.Type, &rule.Name, &enabled, &rule.Priority,
			&rule.RecorderURL, &createdAt, &updatedAt)
		if err != nil {
			models.Log.Error("GetEnabledAutoReservationRules: Scan failed: %v", err)
			continue
		}
		
		rule.Enabled = enabled != 0
		rule.CreatedAt = time.UnixMilli(createdAt)
		rule.UpdatedAt = time.UnixMilli(updatedAt)

		// Load rule details based on type
		switch rule.Type {
		case "keyword":
			keywordRule, err := getKeywordRule(db, rule.ID)
			if err != nil {
				models.Log.Error("GetEnabledAutoReservationRules: Failed to load keyword rule for %s: %v", rule.ID, err)
			} else {
				rule.KeywordRule = keywordRule
			}
		case "series":
			seriesRule, err := getSeriesRule(db, rule.ID)
			if err != nil {
				models.Log.Error("GetEnabledAutoReservationRules: Failed to load series rule for %s: %v", rule.ID, err)
			} else {
				rule.SeriesRule = seriesRule
			}
		}

		rules = append(rules, rule)
	}

	models.Log.Debug("GetEnabledAutoReservationRules: Retrieved %d enabled rules", len(rules))
	return rules, nil
}

// getKeywordRule is a helper function to retrieve keyword rule details
func getKeywordRule(db *sql.DB, ruleID string) (*models.KeywordRule, error) {
	var keywordsJSON, genresJSON, serviceIDsJSON, excludeWordsJSON string
	
	err := db.QueryRow(`
		SELECT keywords, genres, serviceIds, excludeWords
		FROM keyword_rules WHERE ruleId = ?
	`, ruleID).Scan(&keywordsJSON, &genresJSON, &serviceIDsJSON, &excludeWordsJSON)
	
	if err != nil {
		return nil, err
	}
	
	rule := &models.KeywordRule{RuleID: ruleID}
	
	// Parse JSON strings back to slices
	if keywordsJSON != "" {
		json.Unmarshal([]byte(keywordsJSON), &rule.Keywords)
	}
	if genresJSON != "" {
		json.Unmarshal([]byte(genresJSON), &rule.Genres)
	}
	if serviceIDsJSON != "" {
		json.Unmarshal([]byte(serviceIDsJSON), &rule.ServiceIDs)
	}
	if excludeWordsJSON != "" {
		json.Unmarshal([]byte(excludeWordsJSON), &rule.ExcludeWords)
	}
	
	return rule, nil
}

// getSeriesRule is a helper function to retrieve series rule details
func getSeriesRule(db *sql.DB, ruleID string) (*models.SeriesRule, error) {
	rule := &models.SeriesRule{RuleID: ruleID}
	
	err := db.QueryRow(`
		SELECT seriesId, programName, serviceId
		FROM series_rules WHERE ruleId = ?
	`, ruleID).Scan(&rule.SeriesID, &rule.ProgramName, &rule.ServiceID)
	
	if err != nil {
		return nil, err
	}
	
	return rule, nil
}

// CreateAutoReservationLog creates a log entry for auto reservation processing
func CreateAutoReservationLog(db *sql.DB, log *models.AutoReservationLog) error {
	if log.ID == "" {
		log.ID = uuid.New().String()
	}
	log.CreatedAt = time.Now()

	_, err := db.Exec(`
		INSERT INTO auto_reservation_logs (id, ruleId, programId, reservationId, status, reason, createdAt)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, log.ID, log.RuleID, log.ProgramID, log.ReservationID, log.Status, log.Reason, log.CreatedAt.UnixMilli())
	
	if err != nil {
		models.Log.Error("CreateAutoReservationLog: Failed to create log: %v", err)
		return err
	}

	return nil
}

// GetAutoReservationLogs retrieves auto reservation logs with optional filtering
func GetAutoReservationLogs(db *sql.DB, ruleID string, limit int) ([]models.AutoReservationLog, error) {
	var query strings.Builder
	var args []interface{}

	query.WriteString(`
		SELECT id, ruleId, programId, reservationId, status, reason, createdAt
		FROM auto_reservation_logs
	`)

	if ruleID != "" {
		query.WriteString(" WHERE ruleId = ?")
		args = append(args, ruleID)
	}

	query.WriteString(" ORDER BY createdAt DESC")

	if limit > 0 {
		query.WriteString(" LIMIT ?")
		args = append(args, limit)
	}

	rows, err := db.Query(query.String(), args...)
	if err != nil {
		models.Log.Error("GetAutoReservationLogs: Query failed: %v", err)
		return nil, err
	}
	defer rows.Close()

	var logs []models.AutoReservationLog
	for rows.Next() {
		var log models.AutoReservationLog
		var createdAt int64
		var reservationID, reason sql.NullString
		
		err := rows.Scan(&log.ID, &log.RuleID, &log.ProgramID, &reservationID, 
			&log.Status, &reason, &createdAt)
		if err != nil {
			models.Log.Error("GetAutoReservationLogs: Scan failed: %v", err)
			continue
		}
		
		log.ReservationID = reservationID.String
		log.Reason = reason.String
		log.CreatedAt = time.UnixMilli(createdAt)
		
		logs = append(logs, log)
	}

	return logs, nil
}