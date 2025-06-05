// models/reservation_test.go
package models

import (
	"testing"
	"time"
)

func TestReservationIsExpired(t *testing.T) {
	tests := []struct {
		name     string
		startAt  int64
		duration int64
		expected bool
	}{
		{
			name:     "Future program",
			startAt:  time.Now().Add(1 * time.Hour).UnixMilli(),
			duration: 3600000, // 1 hour in milliseconds
			expected: false,
		},
		{
			name:     "Past program",
			startAt:  time.Now().Add(-2 * time.Hour).UnixMilli(),
			duration: 3600000, // 1 hour in milliseconds
			expected: true,
		},
		{
			name:     "Currently airing program",
			startAt:  time.Now().Add(-30 * time.Minute).UnixMilli(),
			duration: 3600000, // 1 hour in milliseconds
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Reservation{
				StartAt:  tt.startAt,
				Duration: tt.duration,
			}
			if got := r.IsExpired(); got != tt.expected {
				t.Errorf("IsExpired() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestReservationIsActive(t *testing.T) {
	tests := []struct {
		name     string
		status   ReservationStatus
		startAt  int64
		duration int64
		expected bool
	}{
		{
			name:     "Recording status and currently airing",
			status:   ReservationStatusRecording,
			startAt:  time.Now().Add(-30 * time.Minute).UnixMilli(),
			duration: 3600000, // 1 hour
			expected: true,
		},
		{
			name:     "Recording status but future program",
			status:   ReservationStatusRecording,
			startAt:  time.Now().Add(1 * time.Hour).UnixMilli(),
			duration: 3600000,
			expected: false,
		},
		{
			name:     "Recording status but past program",
			status:   ReservationStatusRecording,
			startAt:  time.Now().Add(-2 * time.Hour).UnixMilli(),
			duration: 3600000,
			expected: false,
		},
		{
			name:     "Pending status and currently airing",
			status:   ReservationStatusPending,
			startAt:  time.Now().Add(-30 * time.Minute).UnixMilli(),
			duration: 3600000,
			expected: false,
		},
		{
			name:     "Completed status and currently airing",
			status:   ReservationStatusCompleted,
			startAt:  time.Now().Add(-30 * time.Minute).UnixMilli(),
			duration: 3600000,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Reservation{
				Status:   tt.status,
				StartAt:  tt.startAt,
				Duration: tt.duration,
			}
			if got := r.IsActive(); got != tt.expected {
				t.Errorf("IsActive() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestReservationShouldStartRecording(t *testing.T) {
	tests := []struct {
		name     string
		status   ReservationStatus
		startAt  int64
		expected bool
	}{
		{
			name:     "Pending status and starts within 1 minute",
			status:   ReservationStatusPending,
			startAt:  time.Now().Add(30 * time.Second).UnixMilli(),
			expected: true,
		},
		{
			name:     "Pending status and starts in 2 minutes",
			status:   ReservationStatusPending,
			startAt:  time.Now().Add(2 * time.Minute).UnixMilli(),
			expected: false,
		},
		{
			name:     "Pending status and already started",
			status:   ReservationStatusPending,
			startAt:  time.Now().Add(-5 * time.Minute).UnixMilli(),
			expected: true,
		},
		{
			name:     "Recording status and starts within 1 minute",
			status:   ReservationStatusRecording,
			startAt:  time.Now().Add(30 * time.Second).UnixMilli(),
			expected: false,
		},
		{
			name:     "Completed status and starts within 1 minute",
			status:   ReservationStatusCompleted,
			startAt:  time.Now().Add(30 * time.Second).UnixMilli(),
			expected: false,
		},
		{
			name:     "Failed status and starts within 1 minute",
			status:   ReservationStatusFailed,
			startAt:  time.Now().Add(30 * time.Second).UnixMilli(),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Reservation{
				Status:  tt.status,
				StartAt: tt.startAt,
			}
			if got := r.ShouldStartRecording(); got != tt.expected {
				t.Errorf("ShouldStartRecording() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestReservationStatus(t *testing.T) {
	// Test that all status constants are defined correctly
	statusTests := []struct {
		status   ReservationStatus
		expected string
	}{
		{ReservationStatusPending, "pending"},
		{ReservationStatusRecording, "recording"},
		{ReservationStatusCompleted, "completed"},
		{ReservationStatusFailed, "failed"},
		{ReservationStatusCancelled, "cancelled"},
	}

	for _, tt := range statusTests {
		if string(tt.status) != tt.expected {
			t.Errorf("ReservationStatus %s = %s, want %s", tt.status, string(tt.status), tt.expected)
		}
	}
}