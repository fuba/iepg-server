// handlers/auto_reservation_test.go
package handlers

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/fuba/iepg-server/db"
	"github.com/fuba/iepg-server/models"
)

func setupHandlerTestDB(t *testing.T) *sql.DB {
	database, err := db.InitDB(":memory:")
	if err != nil {
		t.Fatalf("Failed to init test database: %v", err)
	}
	return database
}

func TestHandleCreateAutoReservationRule_Keyword(t *testing.T) {
	database := setupHandlerTestDB(t)
	defer database.Close()

	request := CreateAutoReservationRuleRequest{
		Type:        "keyword",
		Name:        "Test Keyword Rule",
		Enabled:     true,
		Priority:    10,
		RecorderURL: "http://localhost:37569",
		KeywordRule: &models.KeywordRule{
			Keywords:     []string{"anime", "drama"},
			ExcludeWords: []string{"rerun"},
		},
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}

	req := httptest.NewRequest("POST", "/auto-reservations/rules", bytes.NewReader(jsonData))
	req.Header.Set("Content-Type", "application/json")
	
	recorder := httptest.NewRecorder()
	handler := HandleCreateAutoReservationRule(database)
	handler(recorder, req)

	if recorder.Code != http.StatusCreated {
		t.Errorf("Expected status %d, got %d. Body: %s", http.StatusCreated, recorder.Code, recorder.Body.String())
	}

	var response models.AutoReservationRuleWithDetails
	err = json.Unmarshal(recorder.Body.Bytes(), &response)
	if err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if response.Name != "Test Keyword Rule" {
		t.Errorf("Expected name 'Test Keyword Rule', got %s", response.Name)
	}

	if response.Type != "keyword" {
		t.Errorf("Expected type 'keyword', got %s", response.Type)
	}

	if response.KeywordRule == nil {
		t.Error("Expected keyword rule to be present")
	} else {
		if len(response.KeywordRule.Keywords) != 2 {
			t.Errorf("Expected 2 keywords, got %d", len(response.KeywordRule.Keywords))
		}
	}
}

func TestHandleCreateAutoReservationRule_Series(t *testing.T) {
	database := setupHandlerTestDB(t)
	defer database.Close()

	request := CreateAutoReservationRuleRequest{
		Type:        "series",
		Name:        "Test Series Rule",
		Enabled:     true,
		Priority:    20,
		RecorderURL: "http://localhost:37569",
		SeriesRule: &models.SeriesRule{
			SeriesID:    "12345",
			ProgramName: "Test Series",
			ServiceID:   1032,
		},
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}

	req := httptest.NewRequest("POST", "/auto-reservations/rules", bytes.NewReader(jsonData))
	req.Header.Set("Content-Type", "application/json")
	
	recorder := httptest.NewRecorder()
	handler := HandleCreateAutoReservationRule(database)
	handler(recorder, req)

	if recorder.Code != http.StatusCreated {
		t.Errorf("Expected status %d, got %d. Body: %s", http.StatusCreated, recorder.Code, recorder.Body.String())
	}

	var response models.AutoReservationRuleWithDetails
	err = json.Unmarshal(recorder.Body.Bytes(), &response)
	if err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if response.Name != "Test Series Rule" {
		t.Errorf("Expected name 'Test Series Rule', got %s", response.Name)
	}

	if response.Type != "series" {
		t.Errorf("Expected type 'series', got %s", response.Type)
	}

	if response.SeriesRule == nil {
		t.Error("Expected series rule to be present")
	} else {
		if response.SeriesRule.SeriesID != "12345" {
			t.Errorf("Expected SeriesID '12345', got %s", response.SeriesRule.SeriesID)
		}
	}
}

func TestHandleCreateAutoReservationRule_Validation(t *testing.T) {
	database := setupHandlerTestDB(t)
	defer database.Close()

	tests := []struct {
		name           string
		request        CreateAutoReservationRuleRequest
		expectedStatus int
	}{
		{
			name: "Missing name",
			request: CreateAutoReservationRuleRequest{
				Type:        "keyword",
				Enabled:     true,
				RecorderURL: "http://localhost:37569",
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "Invalid type",
			request: CreateAutoReservationRuleRequest{
				Type:        "invalid",
				Name:        "Test Rule",
				Enabled:     true,
				RecorderURL: "http://localhost:37569",
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "Missing recorder URL",
			request: CreateAutoReservationRuleRequest{
				Type:    "keyword",
				Name:    "Test Rule",
				Enabled: true,
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "Keyword type without keyword rule",
			request: CreateAutoReservationRuleRequest{
				Type:        "keyword",
				Name:        "Test Rule",
				Enabled:     true,
				RecorderURL: "http://localhost:37569",
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "Series type without series rule",
			request: CreateAutoReservationRuleRequest{
				Type:        "series",
				Name:        "Test Rule",
				Enabled:     true,
				RecorderURL: "http://localhost:37569",
			},
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jsonData, err := json.Marshal(tt.request)
			if err != nil {
				t.Fatalf("Failed to marshal request: %v", err)
			}

			req := httptest.NewRequest("POST", "/auto-reservations/rules", bytes.NewReader(jsonData))
			req.Header.Set("Content-Type", "application/json")
			
			recorder := httptest.NewRecorder()
			handler := HandleCreateAutoReservationRule(database)
			handler(recorder, req)

			if recorder.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d for test '%s'. Body: %s", 
					tt.expectedStatus, recorder.Code, tt.name, recorder.Body.String())
			}
		})
	}
}

func TestHandleGetAutoReservationRules(t *testing.T) {
	database := setupHandlerTestDB(t)
	defer database.Close()

	// Create test rules
	rule1 := &models.AutoReservationRule{
		Type:        "keyword",
		Name:        "Keyword Rule",
		Enabled:     true,
		Priority:    10,
		RecorderURL: "http://localhost:37569",
	}
	err := db.CreateAutoReservationRule(database, rule1)
	if err != nil {
		t.Fatalf("Failed to create rule1: %v", err)
	}

	keywordRule := &models.KeywordRule{
		RuleID:   rule1.ID,
		Keywords: []string{"test"},
	}
	err = db.CreateKeywordRule(database, keywordRule)
	if err != nil {
		t.Fatalf("Failed to create keyword rule: %v", err)
	}

	req := httptest.NewRequest("GET", "/auto-reservations/rules", nil)
	recorder := httptest.NewRecorder()
	handler := HandleGetAutoReservationRules(database)
	handler(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, recorder.Code)
	}

	var response []models.AutoReservationRuleWithDetails
	err = json.Unmarshal(recorder.Body.Bytes(), &response)
	if err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if len(response) != 1 {
		t.Errorf("Expected 1 rule, got %d", len(response))
	}

	if response[0].Name != "Keyword Rule" {
		t.Errorf("Expected name 'Keyword Rule', got %s", response[0].Name)
	}
}

func TestHandleDeleteAutoReservationRule(t *testing.T) {
	database := setupHandlerTestDB(t)
	defer database.Close()

	// Create test rule
	rule := &models.AutoReservationRule{
		Type:        "keyword",
		Name:        "Test Rule",
		Enabled:     true,
		Priority:    10,
		RecorderURL: "http://localhost:37569",
	}
	err := db.CreateAutoReservationRule(database, rule)
	if err != nil {
		t.Fatalf("Failed to create rule: %v", err)
	}

	// Test delete existing rule
	req := httptest.NewRequest("DELETE", "/auto-reservations/rules/"+rule.ID, nil)
	recorder := httptest.NewRecorder()
	handler := HandleDeleteAutoReservationRule(database)
	handler(recorder, req)

	if recorder.Code != http.StatusNoContent {
		t.Errorf("Expected status %d, got %d", http.StatusNoContent, recorder.Code)
	}

	// Verify rule is deleted
	_, err = db.GetAutoReservationRuleByID(database, rule.ID)
	if err != sql.ErrNoRows {
		t.Error("Expected rule to be deleted")
	}

	// Test delete non-existing rule
	req = httptest.NewRequest("DELETE", "/auto-reservations/rules/non-existing", nil)
	recorder = httptest.NewRecorder()
	handler(recorder, req)

	if recorder.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, recorder.Code)
	}
}

func TestHandleGetAutoReservationLogs(t *testing.T) {
	database := setupHandlerTestDB(t)
	defer database.Close()

	// Create test rule and log
	rule := &models.AutoReservationRule{
		Type:        "keyword",
		Name:        "Test Rule",
		Enabled:     true,
		Priority:    10,
		RecorderURL: "http://localhost:37569",
	}
	err := db.CreateAutoReservationRule(database, rule)
	if err != nil {
		t.Fatalf("Failed to create rule: %v", err)
	}

	log := &models.AutoReservationLog{
		RuleID:    rule.ID,
		ProgramID: 12345,
		Status:    "reserved",
	}
	err = db.CreateAutoReservationLog(database, log)
	if err != nil {
		t.Fatalf("Failed to create log: %v", err)
	}

	// Test get all logs
	req := httptest.NewRequest("GET", "/auto-reservations/logs", nil)
	recorder := httptest.NewRecorder()
	handler := HandleGetAutoReservationLogs(database)
	handler(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, recorder.Code)
	}

	var response []models.AutoReservationLog
	err = json.Unmarshal(recorder.Body.Bytes(), &response)
	if err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if len(response) != 1 {
		t.Errorf("Expected 1 log, got %d", len(response))
	}

	if response[0].RuleID != rule.ID {
		t.Errorf("Expected RuleID %s, got %s", rule.ID, response[0].RuleID)
	}

	// Test get logs with rule filter
	req = httptest.NewRequest("GET", "/auto-reservations/logs?ruleId="+rule.ID, nil)
	recorder = httptest.NewRecorder()
	handler(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, recorder.Code)
	}

	err = json.Unmarshal(recorder.Body.Bytes(), &response)
	if err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if len(response) != 1 {
		t.Errorf("Expected 1 filtered log, got %d", len(response))
	}
}