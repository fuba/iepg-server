// handlers/reservation_test.go
package handlers

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/fuba/iepg-server/db"
	"github.com/fuba/iepg-server/models"
)

func setupTestDB(t *testing.T) *sql.DB {
	// Initialize logger for tests
	models.InitLogger("error")
	
	// In-memory SQLite database for testing
	database, err := db.InitDB(":memory:")
	if err != nil {
		t.Fatalf("Failed to initialize test database: %v", err)
	}

	// Insert test program data
	_, err = database.Exec(`
		INSERT INTO programs (id, serviceId, startAt, duration, name, description, nameForSearch, descForSearch)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		12345, 1234, time.Now().Add(1*time.Hour).UnixMilli(), 3600000, "Test Program", "Test Description", "test program", "test description")
	if err != nil {
		t.Fatalf("Failed to insert test program: %v", err)
	}

	return database
}

func TestCreateReservation(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	// Create test server for mock recorder API
	mockRecorder := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the request
		if r.Method != "GET" {
			t.Errorf("Expected GET request, got %s", r.Method)
		}
		if r.URL.Query().Get("program_id") != "12345" {
			t.Errorf("Expected program_id=12345, got %s", r.URL.Query().Get("program_id"))
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer mockRecorder.Close()

	handler := NewReservationHandler(database, mockRecorder.URL)

	// Test case 1: Create reservation with default recorder URL
	reqBody := models.CreateReservationRequest{
		ProgramID: 12345,
	}
	body, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", "/reservations", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.CreateReservation(rr, req)

	if status := rr.Code; status != http.StatusCreated {
		t.Errorf("Handler returned wrong status code: got %v want %v", status, http.StatusCreated)
	}

	var response models.ReservationResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if !response.Success {
		t.Errorf("Expected success=true, got false. Error: %s", response.Error)
	}

	if response.Data == nil {
		t.Fatal("Expected reservation data, got nil")
	}

	if response.Data.ProgramID != 12345 {
		t.Errorf("Expected ProgramID=12345, got %d", response.Data.ProgramID)
	}

	// Verify reservation was saved to database
	var count int
	err := database.QueryRow("SELECT COUNT(*) FROM reservations WHERE programId = ?", 12345).Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query reservations: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 reservation, found %d", count)
	}

	// Test case 2: Create reservation with custom recorder URL
	reqBody2 := models.CreateReservationRequest{
		ProgramID:   12345,
		RecorderURL: "http://custom-recorder:8080",
	}
	body2, _ := json.Marshal(reqBody2)
	req2, _ := http.NewRequest("POST", "/reservations", bytes.NewBuffer(body2))
	req2.Header.Set("Content-Type", "application/json")
	rr2 := httptest.NewRecorder()

	handler.CreateReservation(rr2, req2)

	if status := rr2.Code; status != http.StatusCreated {
		t.Errorf("Handler returned wrong status code: got %v want %v", status, http.StatusCreated)
	}

	var response2 models.ReservationResponse
	if err := json.Unmarshal(rr2.Body.Bytes(), &response2); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if response2.Data.RecorderURL != "http://custom-recorder:8080" {
		t.Errorf("Expected custom recorder URL, got %s", response2.Data.RecorderURL)
	}
}

func TestCreateReservationInvalidProgram(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	handler := NewReservationHandler(database, "http://recorder:8080")

	// Test with non-existent program
	reqBody := models.CreateReservationRequest{
		ProgramID: 99999,
	}
	body, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", "/reservations", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.CreateReservation(rr, req)

	if status := rr.Code; status != http.StatusNotFound {
		t.Errorf("Handler returned wrong status code: got %v want %v", status, http.StatusNotFound)
	}

	var response models.ReservationResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if response.Success {
		t.Error("Expected success=false, got true")
	}

	if response.Error != "Program not found" {
		t.Errorf("Expected 'Program not found' error, got %s", response.Error)
	}
}

func TestGetReservations(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	handler := NewReservationHandler(database, "http://recorder:8080")

	// Insert test reservations
	_, err := database.Exec(`
		INSERT INTO reservations (id, programId, serviceId, name, startAt, duration, 
			recorderUrl, recorderProgramId, status, createdAt, updatedAt)
		VALUES 
			('test-id-1', 12345, 1234, 'Test Program 1', ?, 3600000, 'http://recorder:8080', '12345', 'pending', ?, ?),
			('test-id-2', 67890, 5678, 'Test Program 2', ?, 7200000, 'http://recorder:8080', '67890', 'recording', ?, ?)`,
		time.Now().Add(1*time.Hour).UnixMilli(), time.Now().UnixMilli(), time.Now().UnixMilli(),
		time.Now().Add(2*time.Hour).UnixMilli(), time.Now().UnixMilli(), time.Now().UnixMilli())
	if err != nil {
		t.Fatalf("Failed to insert test reservations: %v", err)
	}

	req, _ := http.NewRequest("GET", "/reservations", nil)
	rr := httptest.NewRecorder()

	handler.GetReservations(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	var response models.ReservationsListResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if !response.Success {
		t.Error("Expected success=true, got false")
	}

	if len(response.Reservations) != 2 {
		t.Errorf("Expected 2 reservations, got %d", len(response.Reservations))
	}

	if response.Total != 2 {
		t.Errorf("Expected total=2, got %d", response.Total)
	}
}

func TestDeleteReservation(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	handler := NewReservationHandler(database, "http://recorder:8080")

	// Insert test reservation
	_, err := database.Exec(`
		INSERT INTO reservations (id, programId, serviceId, name, startAt, duration, 
			recorderUrl, recorderProgramId, status, createdAt, updatedAt)
		VALUES ('test-delete-id', 12345, 1234, 'Test Program', ?, 3600000, 'http://recorder:8080', '12345', 'pending', ?, ?)`,
		time.Now().Add(1*time.Hour).UnixMilli(), time.Now().UnixMilli(), time.Now().UnixMilli())
	if err != nil {
		t.Fatalf("Failed to insert test reservation: %v", err)
	}

	// Create router for path variables
	router := mux.NewRouter()
	router.HandleFunc("/reservations/{id}", handler.DeleteReservation).Methods("DELETE")

	req, _ := http.NewRequest("DELETE", "/reservations/test-delete-id", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	var response models.ReservationResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if !response.Success {
		t.Error("Expected success=true, got false")
	}

	// Verify reservation was deleted
	var count int
	err = database.QueryRow("SELECT COUNT(*) FROM reservations WHERE id = ?", "test-delete-id").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query reservations: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected 0 reservations, found %d", count)
	}
}

func TestDeleteReservationNotFound(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	handler := NewReservationHandler(database, "http://recorder:8080")

	// Create router for path variables
	router := mux.NewRouter()
	router.HandleFunc("/reservations/{id}", handler.DeleteReservation).Methods("DELETE")

	req, _ := http.NewRequest("DELETE", "/reservations/non-existent-id", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusNotFound {
		t.Errorf("Handler returned wrong status code: got %v want %v", status, http.StatusNotFound)
	}

	var response models.ReservationResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if response.Success {
		t.Error("Expected success=false, got true")
	}

	if response.Error != "Reservation not found" {
		t.Errorf("Expected 'Reservation not found' error, got %s", response.Error)
	}
}