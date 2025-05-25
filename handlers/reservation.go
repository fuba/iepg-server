// handlers/reservation.go
package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
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
}

// NewReservationHandler creates a new reservation handler
func NewReservationHandler(database *sql.DB, recorderURL string) *ReservationHandler {
	return &ReservationHandler{
		DB:          database,
		RecorderURL: recorderURL,
	}
}

// CreateReservation handles POST /reservations
func (h *ReservationHandler) CreateReservation(w http.ResponseWriter, r *http.Request) {
	models.Log.Debug("CreateReservation: Processing request")
	
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
	
	// Call recorder API immediately
	go h.callRecorderAPI(reservation)
	
	models.Log.Info("CreateReservation: Created reservation %s for program %d", reservation.ID, program.ID)
	respondWithJSON(w, http.StatusCreated, models.ReservationResponse{
		Success: true,
		Data:    reservation,
	})
}

// GetReservations handles GET /reservations
func (h *ReservationHandler) GetReservations(w http.ResponseWriter, r *http.Request) {
	models.Log.Debug("GetReservations: Processing request")
	
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
	
	models.Log.Debug("GetReservations: Found %d reservations", len(reservations))
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
	
	models.Log.Debug("DeleteReservation: Processing request for ID %s", id)
	
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
	models.Log.Debug("callRecorderAPI: Calling recorder API for reservation %s", reservation.ID)
	
	// Construct API URL
	apiURL := reservation.RecorderURL
	if !strings.HasPrefix(apiURL, "http") {
		apiURL = "http://" + apiURL
	}
	if !strings.HasSuffix(apiURL, "/") {
		apiURL += "/"
	}
	apiURL += fmt.Sprintf("api/record?program_id=%s", reservation.RecorderProgramID)
	
	models.Log.Info("callRecorderAPI: Calling %s", apiURL)
	
	// Make HTTP request
	resp, err := http.Get(apiURL)
	if err != nil {
		models.Log.Error("callRecorderAPI: Request failed: %v", err)
		h.updateReservationError(reservation.ID, fmt.Sprintf("Failed to call recorder API: %v", err))
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

// respondWithJSON sends a JSON response
func respondWithJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		models.Log.Error("respondWithJSON: Failed to encode response: %v", err)
	}
}