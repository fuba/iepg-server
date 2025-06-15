// services/auto_reservation_engine_test.go
package services

import (
	"database/sql"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/fuba/iepg-server/db"
	"github.com/fuba/iepg-server/models"
)

func setupEngineTestDB(t *testing.T) *sql.DB {
	database, err := db.InitDB(":memory:")
	if err != nil {
		t.Fatalf("Failed to init test database: %v", err)
	}
	return database
}

func TestCheckKeywordMatch(t *testing.T) {
	database := setupEngineTestDB(t)
	defer database.Close()

	engine := NewAutoReservationEngine(database, "http://localhost:37569")

	tests := []struct {
		name        string
		keywordRule *models.KeywordRule
		program     models.Program
		expected    bool
	}{
		{
			name: "Simple keyword match",
			keywordRule: &models.KeywordRule{
				Keywords: []string{"anime"},
			},
			program: models.Program{
				Name:        "Great Anime Show",
				Description: "An exciting anime series",
			},
			expected: true,
		},
		{
			name: "Multiple keyword match (AND condition)",
			keywordRule: &models.KeywordRule{
				Keywords: []string{"anime", "action"},
			},
			program: models.Program{
				Name:        "Action Anime Show",
				Description: "An exciting action anime series",
			},
			expected: true,
		},
		{
			name: "Multiple keyword partial match",
			keywordRule: &models.KeywordRule{
				Keywords: []string{"anime", "romance"},
			},
			program: models.Program{
				Name:        "Great Anime Show",
				Description: "An exciting anime series", // missing "romance"
			},
			expected: false,
		},
		{
			name: "Exclude word present",
			keywordRule: &models.KeywordRule{
				Keywords:     []string{"anime"},
				ExcludeWords: []string{"rerun"},
			},
			program: models.Program{
				Name:        "Great Anime Show (Rerun)",
				Description: "An exciting anime series",
			},
			expected: false,
		},
		{
			name: "Service ID filter match",
			keywordRule: &models.KeywordRule{
				Keywords:   []string{"anime"},
				ServiceIDs: []int64{1032, 1034},
			},
			program: models.Program{
				ServiceID:   1032,
				Name:        "Great Anime Show",
				Description: "An exciting anime series",
			},
			expected: true,
		},
		{
			name: "Service ID filter no match",
			keywordRule: &models.KeywordRule{
				Keywords:   []string{"anime"},
				ServiceIDs: []int64{1032, 1034},
			},
			program: models.Program{
				ServiceID:   1040,
				Name:        "Great Anime Show",
				Description: "An exciting anime series",
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := engine.checkKeywordMatch(tt.keywordRule, tt.program)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v for test: %s", tt.expected, result, tt.name)
			}
		})
	}
}

func TestCheckSeriesMatch(t *testing.T) {
	database := setupEngineTestDB(t)
	defer database.Close()

	engine := NewAutoReservationEngine(database, "http://localhost:37569")

	tests := []struct {
		name       string
		seriesRule *models.SeriesRule
		program    models.Program
		expected   bool
	}{
		{
			name: "Series ID match",
			seriesRule: &models.SeriesRule{
				SeriesID: "12345",
			},
			program: models.Program{
				Name: "Test Series Episode 1",
				Series: &models.Series{
					ID:   12345,
					Name: "Test Series",
				},
			},
			expected: true,
		},
		{
			name: "Series ID no match",
			seriesRule: &models.SeriesRule{
				SeriesID: "12345",
			},
			program: models.Program{
				Name: "Test Series Episode 1",
				Series: &models.Series{
					ID:   54321,
					Name: "Different Series",
				},
			},
			expected: false,
		},
		{
			name: "No series information",
			seriesRule: &models.SeriesRule{
				SeriesID: "12345",
			},
			program: models.Program{
				Name:   "Test Program",
				Series: nil,
			},
			expected: false,
		},
		{
			name: "Service ID filter match",
			seriesRule: &models.SeriesRule{
				SeriesID:  "12345",
				ServiceID: 1032,
			},
			program: models.Program{
				ServiceID: 1032,
				Name:      "Test Series Episode 1",
				Series: &models.Series{
					ID:   12345,
					Name: "Test Series",
				},
			},
			expected: true,
		},
		{
			name: "Service ID filter no match",
			seriesRule: &models.SeriesRule{
				SeriesID:  "12345",
				ServiceID: 1032,
			},
			program: models.Program{
				ServiceID: 1040,
				Name:      "Test Series Episode 1",
				Series: &models.Series{
					ID:   12345,
					Name: "Test Series",
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := engine.checkSeriesMatch(tt.seriesRule, tt.program)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v for test: %s", tt.expected, result, tt.name)
			}
		})
	}
}

func TestHasExistingReservation(t *testing.T) {
	database := setupEngineTestDB(t)
	defer database.Close()

	engine := NewAutoReservationEngine(database, "http://localhost:37569")

	// Create a test reservation
	_, err := database.Exec(`
		INSERT INTO reservations (id, programId, serviceId, name, startAt, duration, recorderUrl, recorderProgramId, status, createdAt, updatedAt)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, "test-reservation", 12345, 1032, "Test Program", time.Now().UnixMilli(), 3600000,
		"http://localhost:37569", "rec-123", "pending", time.Now().UnixMilli(), time.Now().UnixMilli())
	
	if err != nil {
		t.Fatalf("Failed to create test reservation: %v", err)
	}

	// Test existing reservation
	if !engine.hasExistingReservation(12345) {
		t.Error("Expected to find existing reservation for program 12345")
	}

	// Test non-existing reservation
	if engine.hasExistingReservation(54321) {
		t.Error("Expected no reservation for program 54321")
	}
}

func TestHasExistingLog(t *testing.T) {
	database := setupEngineTestDB(t)
	defer database.Close()

	engine := NewAutoReservationEngine(database, "http://localhost:37569")

	// Create a test rule first
	rule := &models.AutoReservationRule{
		Type:        "keyword",
		Name:        "Test Rule",
		Enabled:     true,
		Priority:    10,
		RecorderURL: "http://localhost:37569",
	}
	err := db.CreateAutoReservationRule(database, rule)
	if err != nil {
		t.Fatalf("Failed to create test rule: %v", err)
	}

	// Create a test log
	log := &models.AutoReservationLog{
		RuleID:    rule.ID,
		ProgramID: 12345,
		Status:    "matched",
	}
	err = db.CreateAutoReservationLog(database, log)
	if err != nil {
		t.Fatalf("Failed to create test log: %v", err)
	}

	// Test existing log
	if !engine.hasExistingLog(rule.ID, 12345) {
		t.Error("Expected to find existing log for rule and program")
	}

	// Test non-existing log
	if engine.hasExistingLog(rule.ID, 54321) {
		t.Error("Expected no log for program 54321")
	}
}

func TestCheckRuleMatch(t *testing.T) {
	database := setupEngineTestDB(t)
	defer database.Close()

	engine := NewAutoReservationEngine(database, "http://localhost:37569")

	// Create a test rule
	rule := &models.AutoReservationRule{
		Type:        "keyword",
		Name:        "Test Rule",
		Enabled:     true,
		Priority:    10,
		RecorderURL: "http://localhost:37569",
	}
	err := db.CreateAutoReservationRule(database, rule)
	if err != nil {
		t.Fatalf("Failed to create test rule: %v", err)
	}

	keywordRule := &models.KeywordRule{
		RuleID:   rule.ID,
		Keywords: []string{"anime"},
	}
	err = db.CreateKeywordRule(database, keywordRule)
	if err != nil {
		t.Fatalf("Failed to create keyword rule: %v", err)
	}

	ruleWithDetails := models.AutoReservationRuleWithDetails{
		AutoReservationRule: *rule,
		KeywordRule:         keywordRule,
	}

	program := models.Program{
		ID:          12345,
		Name:        "Great Anime Show",
		Description: "An exciting anime series",
	}

	// Test matching program
	if !engine.checkRuleMatch(ruleWithDetails, program) {
		t.Error("Expected rule to match program")
	}

	// Create existing reservation to test exclusion
	_, err = database.Exec(`
		INSERT INTO reservations (id, programId, serviceId, name, startAt, duration, recorderUrl, recorderProgramId, status, createdAt, updatedAt)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, "test-reservation", 12345, 1032, "Test Program", time.Now().UnixMilli(), 3600000,
		"http://localhost:37569", "rec-123", "pending", time.Now().UnixMilli(), time.Now().UnixMilli())
	
	if err != nil {
		t.Fatalf("Failed to create test reservation: %v", err)
	}

	// Test excluded due to existing reservation
	if engine.checkRuleMatch(ruleWithDetails, program) {
		t.Error("Expected rule to not match due to existing reservation")
	}
}

func TestLogAutoReservation(t *testing.T) {
	database := setupEngineTestDB(t)
	defer database.Close()

	engine := NewAutoReservationEngine(database, "http://localhost:37569")

	// Create a test rule
	rule := &models.AutoReservationRule{
		Type:        "keyword",
		Name:        "Test Rule",
		Enabled:     true,
		Priority:    10,
		RecorderURL: "http://localhost:37569",
	}
	err := db.CreateAutoReservationRule(database, rule)
	if err != nil {
		t.Fatalf("Failed to create test rule: %v", err)
	}

	// Test logging
	engine.logAutoReservation(rule.ID, 12345, "res-123", "reserved", "")

	// Verify log was created
	logs, err := db.GetAutoReservationLogs(database, rule.ID, 0)
	if err != nil {
		t.Fatalf("Failed to get logs: %v", err)
	}

	if len(logs) != 1 {
		t.Errorf("Expected 1 log, got %d", len(logs))
	}

	log := logs[0]
	if log.RuleID != rule.ID {
		t.Errorf("Expected RuleID %s, got %s", rule.ID, log.RuleID)
	}

	if log.ProgramID != 12345 {
		t.Errorf("Expected ProgramID 12345, got %d", log.ProgramID)
	}

	if log.ReservationID != "res-123" {
		t.Errorf("Expected ReservationID 'res-123', got %s", log.ReservationID)
	}

	if log.Status != "reserved" {
		t.Errorf("Expected Status 'reserved', got %s", log.Status)
	}
}