// models/reservation.go
package models

import (
	"time"
)

// ReservationStatus represents the status of a reservation
type ReservationStatus string

const (
	ReservationStatusPending   ReservationStatus = "pending"
	ReservationStatusRecording ReservationStatus = "recording"
	ReservationStatusCompleted ReservationStatus = "completed"
	ReservationStatusFailed    ReservationStatus = "failed"
	ReservationStatusCancelled ReservationStatus = "cancelled"
)

// Reservation represents a recording reservation
type Reservation struct {
	ID                string            `json:"id"`
	ProgramID         int64             `json:"programId"`
	ServiceID         int64             `json:"serviceId"`
	Name              string            `json:"name"`
	StartAt           int64             `json:"startAt"`
	Duration          int64             `json:"duration"`
	RecorderURL       string            `json:"recorderUrl"`
	RecorderProgramID string            `json:"recorderProgramId"`
	Status            ReservationStatus `json:"status"`
	CreatedAt         int64             `json:"createdAt"`
	UpdatedAt         int64             `json:"updatedAt"`
	Error             string            `json:"error,omitempty"`
}

// CreateReservationRequest represents a request to create a reservation
type CreateReservationRequest struct {
	ProgramID   int64  `json:"programId"`
	RecorderURL string `json:"recorderUrl"`
}

// ReservationResponse represents the API response for a reservation
type ReservationResponse struct {
	Success bool         `json:"success"`
	Message string       `json:"message,omitempty"`
	Data    *Reservation `json:"data,omitempty"`
	Error   string       `json:"error,omitempty"`
}

// ReservationsListResponse represents the API response for multiple reservations
type ReservationsListResponse struct {
	Success      bool          `json:"success"`
	Reservations []Reservation `json:"reservations"`
	Total        int           `json:"total"`
	Error        string        `json:"error,omitempty"`
}

// IsExpired checks if the reservation has expired (program has ended)
func (r *Reservation) IsExpired() bool {
	endTime := r.StartAt + r.Duration
	return time.Now().UnixMilli() > endTime
}

// IsActive checks if the reservation is currently recording
func (r *Reservation) IsActive() bool {
	now := time.Now().UnixMilli()
	return r.Status == ReservationStatusRecording &&
		now >= r.StartAt &&
		now < (r.StartAt+r.Duration)
}

// ShouldStartRecording checks if the reservation should start recording
func (r *Reservation) ShouldStartRecording() bool {
	if r.Status != ReservationStatusPending {
		return false
	}
	// Start recording 1 minute before the scheduled time
	return time.Now().UnixMilli() >= (r.StartAt - 60000)
}