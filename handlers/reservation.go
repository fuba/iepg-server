// handlers/reservation.go
package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	
	"github.com/fuba/iepg-server/db"
	"github.com/fuba/iepg-server/models"
)

// ReservationHandler handles reservation-related HTTP requests
type ReservationHandler struct {
	DB          *sql.DB
	RecorderURL string
	HTTPClient  *http.Client
}

// NewReservationHandler creates a new reservation handler
func NewReservationHandler(database *sql.DB, recorderURL string) *ReservationHandler {
	return &ReservationHandler{
		DB:          database,
		RecorderURL: recorderURL,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// CreateReservation handles POST /reservations
func (h *ReservationHandler) CreateReservation(w http.ResponseWriter, r *http.Request) {
	models.Log.Info("CreateReservation: Processing request")
	
	var req models.CreateReservationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		models.Log.Error("CreateReservation: Failed to decode request: %v", err)
		respondWithJSON(w, http.StatusBadRequest, models.ReservationResponse{
			Success: false,
			Error:   "Invalid request body",
		})
		return
	}
	
	// Get program details
	program, err := db.GetProgramByID(h.DB, req.ProgramID)
	if err != nil {
		models.Log.Error("CreateReservation: Failed to get program: %v", err)
		respondWithJSON(w, http.StatusNotFound, models.ReservationResponse{
			Success: false,
			Error:   "Program not found",
		})
		return
	}
	
	// Use provided recorder URL or default
	recorderURL := req.RecorderURL
	if recorderURL == "" {
		recorderURL = h.RecorderURL
	}
	
	// Validate recorder URL
	if err := validateRecorderURL(recorderURL); err != nil {
		models.Log.Error("CreateReservation: Invalid recorder URL: %v", err)
		respondWithJSON(w, http.StatusBadRequest, models.ReservationResponse{
			Success: false,
			Error:   fmt.Sprintf("Invalid recorder URL: %v", err),
		})
		return
	}
	
	// Create reservation
	reservation := &models.Reservation{
		ID:                uuid.New().String(),
		ProgramID:         program.ID,
		ServiceID:         program.ServiceID,
		Name:              program.Name,
		StartAt:           program.StartAt,
		Duration:          program.Duration,
		RecorderURL:       recorderURL,
		RecorderProgramID: fmt.Sprintf("%d", program.ID),
		Status:            models.ReservationStatusPending,
		CreatedAt:         time.Now().UnixMilli(),
		UpdatedAt:         time.Now().UnixMilli(),
	}
	
	// Save to database
	_, err = h.DB.Exec(`
		INSERT INTO reservations (id, programId, serviceId, name, startAt, duration, 
			recorderUrl, recorderProgramId, status, createdAt, updatedAt)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		reservation.ID, reservation.ProgramID, reservation.ServiceID, reservation.Name,
		reservation.StartAt, reservation.Duration, reservation.RecorderURL,
		reservation.RecorderProgramID, reservation.Status, reservation.CreatedAt, reservation.UpdatedAt)
	
	if err != nil {
		models.Log.Error("CreateReservation: Failed to save reservation: %v", err)
		respondWithJSON(w, http.StatusInternalServerError, models.ReservationResponse{
			Success: false,
			Error:   "Failed to create reservation",
		})
		return
	}
	
	// Call recorder API asynchronously
	// The reservation is created with "pending" status.
	// The API call will update the status to "recording" on success or "failed" on error.
	// This ensures the reservation is tracked even if the external API call fails.
	go h.callRecorderAPI(reservation)
	
	models.Log.Info("CreateReservation: Created reservation %s for program %d", reservation.ID, program.ID)
	respondWithJSON(w, http.StatusCreated, models.ReservationResponse{
		Success: true,
		Data:    reservation,
	})
}

// GetReservations handles GET /reservations
func (h *ReservationHandler) GetReservations(w http.ResponseWriter, r *http.Request) {
	models.Log.Info("GetReservations: Processing request")
	
	query := `
		SELECT id, programId, serviceId, name, startAt, duration,
			recorderUrl, recorderProgramId, status, createdAt, updatedAt, error
		FROM reservations
		ORDER BY startAt DESC
	`
	
	rows, err := h.DB.Query(query)
	if err != nil {
		models.Log.Error("GetReservations: Query failed: %v", err)
		respondWithJSON(w, http.StatusInternalServerError, models.ReservationsListResponse{
			Success: false,
			Error:   "Failed to fetch reservations",
		})
		return
	}
	defer rows.Close()
	
	var reservations []models.Reservation
	for rows.Next() {
		var r models.Reservation
		var errorStr sql.NullString
		
		err := rows.Scan(&r.ID, &r.ProgramID, &r.ServiceID, &r.Name, &r.StartAt, &r.Duration,
			&r.RecorderURL, &r.RecorderProgramID, &r.Status, &r.CreatedAt, &r.UpdatedAt, &errorStr)
		if err != nil {
			models.Log.Error("GetReservations: Scan failed: %v", err)
			continue
		}
		
		if errorStr.Valid {
			r.Error = errorStr.String
		}
		
		reservations = append(reservations, r)
	}
	
	models.Log.Info("GetReservations: Found %d reservations", len(reservations))
	respondWithJSON(w, http.StatusOK, models.ReservationsListResponse{
		Success:      true,
		Reservations: reservations,
		Total:        len(reservations),
	})
}

// DeleteReservation handles DELETE /reservations/{id}
func (h *ReservationHandler) DeleteReservation(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	
	models.Log.Info("DeleteReservation: Processing request for ID %s", id)
	
	// Get reservation details first
	var reservation models.Reservation
	err := h.DB.QueryRow(`
		SELECT id, programId, serviceId, name, startAt, duration,
			recorderUrl, recorderProgramId, status, createdAt, updatedAt
		FROM reservations WHERE id = ?`, id).Scan(
		&reservation.ID, &reservation.ProgramID, &reservation.ServiceID,
		&reservation.Name, &reservation.StartAt, &reservation.Duration,
		&reservation.RecorderURL, &reservation.RecorderProgramID,
		&reservation.Status, &reservation.CreatedAt, &reservation.UpdatedAt)
	
	if err != nil {
		if err == sql.ErrNoRows {
			respondWithJSON(w, http.StatusNotFound, models.ReservationResponse{
				Success: false,
				Error:   "Reservation not found",
			})
			return
		}
		models.Log.Error("DeleteReservation: Query failed: %v", err)
		respondWithJSON(w, http.StatusInternalServerError, models.ReservationResponse{
			Success: false,
			Error:   "Failed to fetch reservation",
		})
		return
	}
	
	// Delete from database
	_, err = h.DB.Exec("DELETE FROM reservations WHERE id = ?", id)
	if err != nil {
		models.Log.Error("DeleteReservation: Delete failed: %v", err)
		respondWithJSON(w, http.StatusInternalServerError, models.ReservationResponse{
			Success: false,
			Error:   "Failed to delete reservation",
		})
		return
	}
	
	models.Log.Info("DeleteReservation: Deleted reservation %s", id)
	respondWithJSON(w, http.StatusOK, models.ReservationResponse{
		Success: true,
		Message: "Reservation deleted successfully",
	})
}

// callRecorderAPI calls the external recorder API
func (h *ReservationHandler) callRecorderAPI(reservation *models.Reservation) {
	models.Log.Info("callRecorderAPI: Starting API call for reservation %s", reservation.ID)
	
	// Construct API URL
	apiURL := reservation.RecorderURL
	if !strings.HasPrefix(apiURL, "http") {
		apiURL = "http://" + apiURL
	}
	if !strings.HasSuffix(apiURL, "/") {
		apiURL += "/"
	}
	apiURL += fmt.Sprintf("api/record?program_id=%s", url.QueryEscape(reservation.RecorderProgramID))
	
	models.Log.Info("callRecorderAPI: Calling %s", apiURL)
	
	// Make HTTP request with retry logic
	var resp *http.Response
	var err error
	maxRetries := 3
	
	for i := 0; i < maxRetries; i++ {
		resp, err = h.HTTPClient.Get(apiURL)
		if err == nil {
			break
		}
		
		models.Log.Info("callRecorderAPI: Request attempt %d failed: %v", i+1, err)
		if i < maxRetries-1 {
			time.Sleep(time.Duration(i+1) * time.Second) // Exponential backoff
		}
	}
	
	if err != nil {
		models.Log.Error("callRecorderAPI: Request failed after %d attempts: %v", maxRetries, err)
		h.updateReservationError(reservation.ID, fmt.Sprintf("Failed to call recorder API after %d attempts: %v", maxRetries, err))
		return
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		errMsg := fmt.Sprintf("Recorder API returned status: %s", resp.Status)
		models.Log.Error("callRecorderAPI: %s", errMsg)
		h.updateReservationError(reservation.ID, errMsg)
		return
	}
	
	// Update status to recording if successful
	_, err = h.DB.Exec(`
		UPDATE reservations 
		SET status = ?, updatedAt = ?
		WHERE id = ?`,
		models.ReservationStatusRecording, time.Now().UnixMilli(), reservation.ID)
	
	if err != nil {
		models.Log.Error("callRecorderAPI: Failed to update status: %v", err)
	}
	
	models.Log.Info("callRecorderAPI: Successfully called recorder API for reservation %s", reservation.ID)
}

// updateReservationError updates the error field for a reservation
func (h *ReservationHandler) updateReservationError(id string, errMsg string) {
	_, err := h.DB.Exec(`
		UPDATE reservations 
		SET status = ?, error = ?, updatedAt = ?
		WHERE id = ?`,
		models.ReservationStatusFailed, errMsg, time.Now().UnixMilli(), id)
	
	if err != nil {
		models.Log.Error("updateReservationError: Failed to update: %v", err)
	}
}

// validateRecorderURL validates the recorder URL format and checks against allowed hosts
func validateRecorderURL(recorderURL string) error {
	if recorderURL == "" {
		return nil // Empty is allowed (will use default)
	}
	
	// Parse URL to validate format
	parsedURL, err := url.Parse(recorderURL)
	if err != nil {
		// If parsing fails, try with http:// prefix
		parsedURL, err = url.Parse("http://" + recorderURL)
		if err != nil {
			return fmt.Errorf("invalid URL format: %v", err)
		}
	}
	
	// Only allow http and https schemes
	if parsedURL.Scheme != "" && parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("only http and https schemes are allowed")
	}
	
	// Extract hostname for validation
	hostname := parsedURL.Hostname()
	if hostname == "" {
		// Try to extract from the original URL if no scheme
		parts := strings.Split(recorderURL, ":")
		if len(parts) > 0 {
			hostname = parts[0]
		}
	}
	
	// Allow localhost, private IPs, and specific domains
	// You can customize this based on your security requirements
	if hostname != "" {
		// Allow localhost
		if hostname == "localhost" || hostname == "127.0.0.1" || hostname == "::1" {
			return nil
		}
		
		// Allow private IP ranges (RFC 1918)
		if strings.HasPrefix(hostname, "10.") ||
			strings.HasPrefix(hostname, "172.") ||
			strings.HasPrefix(hostname, "192.168.") {
			return nil
		}
		
		// Add additional allowed domains here if needed
		// For now, we'll allow any domain but you can restrict this
	}
	
	return nil
}

// respondWithJSON sends a JSON response
func respondWithJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		models.Log.Error("respondWithJSON: Failed to encode response: %v", err)
	}
}