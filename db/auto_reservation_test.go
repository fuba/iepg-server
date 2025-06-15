// db/auto_reservation_test.go
package db

import (
	"database/sql"
	"os"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/fuba/iepg-server/models"
)

func init() {
	// テスト用にロガーを初期化
	models.InitLogger("debug")
}

func setupAutoReservationTestDB(t *testing.T) *sql.DB {
	dbPath := "/tmp/test_auto_reservation_" + t.Name() + ".db"
	db, err := InitDB(dbPath)
	if err != nil {
		t.Fatalf("Failed to init test database: %v", err)
	}
	
	// Clean up function
	t.Cleanup(func() {
		db.Close()
		// Remove test database file
		if err := os.Remove(dbPath); err != nil && !os.IsNotExist(err) {
			t.Logf("Failed to remove test database: %v", err)
		}
	})
	
	return db
}

func TestCreateAutoReservationRule(t *testing.T) {
	db := setupAutoReservationTestDB(t)
	defer db.Close()

	rule := &models.AutoReservationRule{
		Type:        "keyword",
		Name:        "Test Rule",
		Enabled:     true,
		Priority:    10,
		RecorderURL: "http://localhost:37569",
	}

	err := CreateAutoReservationRule(db, rule)
	if err != nil {
		t.Fatalf("Failed to create auto reservation rule: %v", err)
	}

	if rule.ID == "" {
		t.Error("Expected rule ID to be generated")
	}

	if rule.CreatedAt.IsZero() {
		t.Error("Expected CreatedAt to be set")
	}

	if rule.UpdatedAt.IsZero() {
		t.Error("Expected UpdatedAt to be set")
	}
}

func TestCreateKeywordRule(t *testing.T) {
	db := setupAutoReservationTestDB(t)
	defer db.Close()

	// First create an auto reservation rule
	rule := &models.AutoReservationRule{
		Type:        "keyword",
		Name:        "Test Rule",
		Enabled:     true,
		Priority:    10,
		RecorderURL: "http://localhost:37569",
	}
	err := CreateAutoReservationRule(db, rule)
	if err != nil {
		t.Fatalf("Failed to create auto reservation rule: %v", err)
	}

	// Now create a keyword rule
	keywordRule := &models.KeywordRule{
		RuleID:       rule.ID,
		Keywords:     []string{"anime", "drama"},
		Genres:       []int{1, 2},
		ServiceIDs:   []int64{1032, 1034},
		ExcludeWords: []string{"rerun", "repeat"},
	}

	err = CreateKeywordRule(db, keywordRule)
	if err != nil {
		t.Fatalf("Failed to create keyword rule: %v", err)
	}

	// Verify the keyword rule was created
	retrievedRule, err := getKeywordRule(db, rule.ID)
	if err != nil {
		t.Fatalf("Failed to retrieve keyword rule: %v", err)
	}

	if len(retrievedRule.Keywords) != 2 {
		t.Errorf("Expected 2 keywords, got %d", len(retrievedRule.Keywords))
	}

	if retrievedRule.Keywords[0] != "anime" || retrievedRule.Keywords[1] != "drama" {
		t.Errorf("Keywords not stored correctly: %v", retrievedRule.Keywords)
	}
}

func TestCreateSeriesRule(t *testing.T) {
	db := setupAutoReservationTestDB(t)
	defer db.Close()

	// First create an auto reservation rule
	rule := &models.AutoReservationRule{
		Type:        "series",
		Name:        "Test Series Rule",
		Enabled:     true,
		Priority:    10,
		RecorderURL: "http://localhost:37569",
	}
	err := CreateAutoReservationRule(db, rule)
	if err != nil {
		t.Fatalf("Failed to create auto reservation rule: %v", err)
	}

	// Now create a series rule
	seriesRule := &models.SeriesRule{
		RuleID:      rule.ID,
		SeriesID:    "12345",
		ProgramName: "Test Series",
		ServiceID:   1032,
	}

	err = CreateSeriesRule(db, seriesRule)
	if err != nil {
		t.Fatalf("Failed to create series rule: %v", err)
	}

	// Verify the series rule was created
	retrievedRule, err := getSeriesRule(db, rule.ID)
	if err != nil {
		t.Fatalf("Failed to retrieve series rule: %v", err)
	}

	if retrievedRule.SeriesID != "12345" {
		t.Errorf("Expected SeriesID 12345, got %s", retrievedRule.SeriesID)
	}

	if retrievedRule.ProgramName != "Test Series" {
		t.Errorf("Expected ProgramName 'Test Series', got %s", retrievedRule.ProgramName)
	}
}

func TestGetAutoReservationRules(t *testing.T) {
	db := setupAutoReservationTestDB(t)
	defer db.Close()

	// Create multiple rules
	rule1 := &models.AutoReservationRule{
		Type:        "keyword",
		Name:        "Keyword Rule",
		Enabled:     true,
		Priority:    20,
		RecorderURL: "http://localhost:37569",
	}
	err := CreateAutoReservationRule(db, rule1)
	if err != nil {
		t.Fatalf("Failed to create rule1: %v", err)
	}

	rule2 := &models.AutoReservationRule{
		Type:        "series",
		Name:        "Series Rule",
		Enabled:     false,
		Priority:    10,
		RecorderURL: "http://localhost:37569",
	}
	err = CreateAutoReservationRule(db, rule2)
	if err != nil {
		t.Fatalf("Failed to create rule2: %v", err)
	}

	// Create keyword rule for rule1
	keywordRule := &models.KeywordRule{
		RuleID:   rule1.ID,
		Keywords: []string{"test"},
	}
	err = CreateKeywordRule(db, keywordRule)
	if err != nil {
		t.Fatalf("Failed to create keyword rule: %v", err)
	}

	// Create series rule for rule2
	seriesRule := &models.SeriesRule{
		RuleID:   rule2.ID,
		SeriesID: "54321",
	}
	err = CreateSeriesRule(db, seriesRule)
	if err != nil {
		t.Fatalf("Failed to create series rule: %v", err)
	}

	// Get all rules
	rules, err := GetAutoReservationRules(db)
	if err != nil {
		t.Fatalf("Failed to get auto reservation rules: %v", err)
	}

	if len(rules) != 2 {
		t.Errorf("Expected 2 rules, got %d", len(rules))
	}

	// Rules should be sorted by priority DESC, so rule1 (priority 20) should come first
	if rules[0].Priority != 20 {
		t.Errorf("Expected first rule to have priority 20, got %d", rules[0].Priority)
	}

	// Check that keyword rule details are loaded
	if rules[0].Type == "keyword" && rules[0].KeywordRule == nil {
		t.Error("Expected keyword rule details to be loaded")
	}

	// Check that series rule details are loaded
	if rules[1].Type == "series" && rules[1].SeriesRule == nil {
		t.Error("Expected series rule details to be loaded")
	}
}

func TestGetEnabledAutoReservationRules(t *testing.T) {
	db := setupAutoReservationTestDB(t)
	defer db.Close()

	// Create enabled rule
	enabledRule := &models.AutoReservationRule{
		Type:        "keyword",
		Name:        "Enabled Rule",
		Enabled:     true,
		Priority:    10,
		RecorderURL: "http://localhost:37569",
	}
	err := CreateAutoReservationRule(db, enabledRule)
	if err != nil {
		t.Fatalf("Failed to create enabled rule: %v", err)
	}

	// Create disabled rule
	disabledRule := &models.AutoReservationRule{
		Type:        "keyword",
		Name:        "Disabled Rule",
		Enabled:     false,
		Priority:    20,
		RecorderURL: "http://localhost:37569",
	}
	err = CreateAutoReservationRule(db, disabledRule)
	if err != nil {
		t.Fatalf("Failed to create disabled rule: %v", err)
	}

	// Create keyword rules
	keywordRule1 := &models.KeywordRule{
		RuleID:   enabledRule.ID,
		Keywords: []string{"enabled"},
	}
	err = CreateKeywordRule(db, keywordRule1)
	if err != nil {
		t.Fatalf("Failed to create keyword rule 1: %v", err)
	}

	keywordRule2 := &models.KeywordRule{
		RuleID:   disabledRule.ID,
		Keywords: []string{"disabled"},
	}
	err = CreateKeywordRule(db, keywordRule2)
	if err != nil {
		t.Fatalf("Failed to create keyword rule 2: %v", err)
	}

	// Get only enabled rules
	rules, err := GetEnabledAutoReservationRules(db)
	if err != nil {
		t.Fatalf("Failed to get enabled auto reservation rules: %v", err)
	}

	if len(rules) != 1 {
		t.Errorf("Expected 1 enabled rule, got %d", len(rules))
	}

	if !rules[0].Enabled {
		t.Error("Expected rule to be enabled")
	}

	if rules[0].Name != "Enabled Rule" {
		t.Errorf("Expected rule name 'Enabled Rule', got %s", rules[0].Name)
	}
}

func TestUpdateAutoReservationRule(t *testing.T) {
	db := setupAutoReservationTestDB(t)
	defer db.Close()

	// Create rule
	rule := &models.AutoReservationRule{
		Type:        "keyword",
		Name:        "Original Name",
		Enabled:     true,
		Priority:    10,
		RecorderURL: "http://localhost:37569",
	}
	err := CreateAutoReservationRule(db, rule)
	if err != nil {
		t.Fatalf("Failed to create rule: %v", err)
	}

	// Create keyword rule
	keywordRule := &models.KeywordRule{
		RuleID:   rule.ID,
		Keywords: []string{"test"},
	}
	err = CreateKeywordRule(db, keywordRule)
	if err != nil {
		t.Fatalf("Failed to create keyword rule: %v", err)
	}

	originalCreatedAt := rule.CreatedAt

	// Update rule
	rule.Name = "Updated Name"
	rule.Enabled = false
	rule.Priority = 20

	time.Sleep(10 * time.Millisecond) // Ensure UpdatedAt is different

	err = UpdateAutoReservationRule(db, rule)
	if err != nil {
		t.Fatalf("Failed to update rule: %v", err)
	}

	// Verify update
	updatedRule, err := GetAutoReservationRuleByID(db, rule.ID)
	if err != nil {
		t.Fatalf("Failed to get updated rule: %v", err)
	}

	if updatedRule.Name != "Updated Name" {
		t.Errorf("Expected name 'Updated Name', got %s", updatedRule.Name)
	}

	if updatedRule.Enabled {
		t.Error("Expected rule to be disabled")
	}

	if updatedRule.Priority != 20 {
		t.Errorf("Expected priority 20, got %d", updatedRule.Priority)
	}

	if updatedRule.CreatedAt.UnixMilli() != originalCreatedAt.UnixMilli() {
		t.Errorf("Expected CreatedAt to remain unchanged. Original: %v, Updated: %v", originalCreatedAt, updatedRule.CreatedAt)
	}

	if !updatedRule.UpdatedAt.After(originalCreatedAt) {
		t.Error("Expected UpdatedAt to be after CreatedAt")
	}
}

func TestDeleteAutoReservationRule(t *testing.T) {
	db := setupAutoReservationTestDB(t)
	defer db.Close()

	// Create rule
	rule := &models.AutoReservationRule{
		Type:        "keyword",
		Name:        "Test Rule",
		Enabled:     true,
		Priority:    10,
		RecorderURL: "http://localhost:37569",
	}
	err := CreateAutoReservationRule(db, rule)
	if err != nil {
		t.Fatalf("Failed to create rule: %v", err)
	}

	// Create keyword rule
	keywordRule := &models.KeywordRule{
		RuleID:   rule.ID,
		Keywords: []string{"test"},
	}
	err = CreateKeywordRule(db, keywordRule)
	if err != nil {
		t.Fatalf("Failed to create keyword rule: %v", err)
	}

	// Delete rule
	err = DeleteAutoReservationRule(db, rule.ID)
	if err != nil {
		t.Fatalf("Failed to delete rule: %v", err)
	}

	// Verify rule is deleted
	_, err = GetAutoReservationRuleByID(db, rule.ID)
	if err != sql.ErrNoRows {
		t.Error("Expected rule to be deleted")
	}

	// Verify related keyword rule is also deleted (due to foreign key cascade)
	_, err = getKeywordRule(db, rule.ID)
	if err != sql.ErrNoRows {
		t.Error("Expected keyword rule to be deleted due to cascade")
	}
}

func TestCreateAutoReservationLog(t *testing.T) {
	db := setupAutoReservationTestDB(t)
	defer db.Close()

	// Create rule first
	rule := &models.AutoReservationRule{
		Type:        "keyword",
		Name:        "Test Rule",
		Enabled:     true,
		Priority:    10,
		RecorderURL: "http://localhost:37569",
	}
	err := CreateAutoReservationRule(db, rule)
	if err != nil {
		t.Fatalf("Failed to create rule: %v", err)
	}

	// Create log
	log := &models.AutoReservationLog{
		RuleID:        rule.ID,
		ProgramID:     12345,
		ReservationID: "res-123",
		Status:        "reserved",
		Reason:        "",
	}

	err = CreateAutoReservationLog(db, log)
	if err != nil {
		t.Fatalf("Failed to create auto reservation log: %v", err)
	}

	if log.ID == "" {
		t.Error("Expected log ID to be generated")
	}

	if log.CreatedAt.IsZero() {
		t.Error("Expected CreatedAt to be set")
	}
}

func TestGetAutoReservationLogs(t *testing.T) {
	db := setupAutoReservationTestDB(t)
	defer db.Close()

	// Create rule
	rule := &models.AutoReservationRule{
		Type:        "keyword",
		Name:        "Test Rule",
		Enabled:     true,
		Priority:    10,
		RecorderURL: "http://localhost:37569",
	}
	err := CreateAutoReservationRule(db, rule)
	if err != nil {
		t.Fatalf("Failed to create rule: %v", err)
	}

	// Create multiple logs
	log1 := &models.AutoReservationLog{
		RuleID:    rule.ID,
		ProgramID: 11111,
		Status:    "reserved",
	}
	err = CreateAutoReservationLog(db, log1)
	if err != nil {
		t.Fatalf("Failed to create log1: %v", err)
	}

	log2 := &models.AutoReservationLog{
		RuleID:    rule.ID,
		ProgramID: 22222,
		Status:    "failed",
		Reason:    "Test failure",
	}
	err = CreateAutoReservationLog(db, log2)
	if err != nil {
		t.Fatalf("Failed to create log2: %v", err)
	}

	// Get all logs
	logs, err := GetAutoReservationLogs(db, "", 0)
	if err != nil {
		t.Fatalf("Failed to get logs: %v", err)
	}

	if len(logs) != 2 {
		t.Errorf("Expected 2 logs, got %d", len(logs))
	}

	// Get logs for specific rule
	ruleLogs, err := GetAutoReservationLogs(db, rule.ID, 0)
	if err != nil {
		t.Fatalf("Failed to get rule logs: %v", err)
	}

	if len(ruleLogs) != 2 {
		t.Errorf("Expected 2 rule logs, got %d", len(ruleLogs))
	}

	// Get logs with limit
	limitedLogs, err := GetAutoReservationLogs(db, "", 1)
	if err != nil {
		t.Fatalf("Failed to get limited logs: %v", err)
	}

	if len(limitedLogs) != 1 {
		t.Errorf("Expected 1 limited log, got %d", len(limitedLogs))
	}
}