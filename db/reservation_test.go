// db/reservation_test.go
package db

import (
	"testing"
	"time"

	"github.com/fuba/iepg-server/models"
)

func TestReservationDatabase(t *testing.T) {
	// Initialize test database
	db, err := InitDB(":memory:")
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Test 1: Insert a reservation
	t.Run("InsertReservation", func(t *testing.T) {
		_, err := db.Exec(`
			INSERT INTO reservations (id, programId, serviceId, name, startAt, duration, 
				recorderUrl, recorderProgramId, status, createdAt, updatedAt)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			"test-id-1", 12345, 1234, "Test Program", 
			time.Now().Add(1*time.Hour).UnixMilli(), 3600000,
			"http://recorder:8080", "12345", "pending",
			time.Now().UnixMilli(), time.Now().UnixMilli())
		
		if err != nil {
			t.Errorf("Failed to insert reservation: %v", err)
		}

		// Verify insertion
		var count int
		err = db.QueryRow("SELECT COUNT(*) FROM reservations WHERE id = ?", "test-id-1").Scan(&count)
		if err != nil {
			t.Errorf("Failed to query reservation: %v", err)
		}
		if count != 1 {
			t.Errorf("Expected 1 reservation, got %d", count)
		}
	})

	// Test 2: Query reservations
	t.Run("QueryReservations", func(t *testing.T) {
		// Insert multiple reservations
		_, err := db.Exec(`
			INSERT INTO reservations (id, programId, serviceId, name, startAt, duration, 
				recorderUrl, recorderProgramId, status, createdAt, updatedAt)
			VALUES 
				(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?),
				(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			"test-id-2", 23456, 2345, "Test Program 2", 
			time.Now().Add(2*time.Hour).UnixMilli(), 3600000,
			"http://recorder:8080", "23456", "pending",
			time.Now().UnixMilli(), time.Now().UnixMilli(),
			"test-id-3", 34567, 3456, "Test Program 3", 
			time.Now().Add(3*time.Hour).UnixMilli(), 7200000,
			"http://recorder:8080", "34567", "recording",
			time.Now().UnixMilli(), time.Now().UnixMilli())
		
		if err != nil {
			t.Fatalf("Failed to insert test reservations: %v", err)
		}

		// Query all reservations
		rows, err := db.Query(`
			SELECT id, programId, serviceId, name, startAt, duration,
				recorderUrl, recorderProgramId, status, createdAt, updatedAt
			FROM reservations
			ORDER BY startAt`)
		if err != nil {
			t.Fatalf("Failed to query reservations: %v", err)
		}
		defer rows.Close()

		var reservations []models.Reservation
		for rows.Next() {
			var r models.Reservation
			err := rows.Scan(&r.ID, &r.ProgramID, &r.ServiceID, &r.Name, &r.StartAt, &r.Duration,
				&r.RecorderURL, &r.RecorderProgramID, &r.Status, &r.CreatedAt, &r.UpdatedAt)
			if err != nil {
				t.Errorf("Failed to scan reservation: %v", err)
			}
			reservations = append(reservations, r)
		}

		if len(reservations) != 3 {
			t.Errorf("Expected 3 reservations, got %d", len(reservations))
		}
	})

	// Test 3: Update reservation status
	t.Run("UpdateReservationStatus", func(t *testing.T) {
		_, err := db.Exec(`
			UPDATE reservations 
			SET status = ?, updatedAt = ?
			WHERE id = ?`,
			"recording", time.Now().UnixMilli(), "test-id-1")
		
		if err != nil {
			t.Errorf("Failed to update reservation: %v", err)
		}

		// Verify update
		var status string
		err = db.QueryRow("SELECT status FROM reservations WHERE id = ?", "test-id-1").Scan(&status)
		if err != nil {
			t.Errorf("Failed to query updated reservation: %v", err)
		}
		if status != "recording" {
			t.Errorf("Expected status 'recording', got '%s'", status)
		}
	})

	// Test 4: Delete reservation
	t.Run("DeleteReservation", func(t *testing.T) {
		result, err := db.Exec("DELETE FROM reservations WHERE id = ?", "test-id-1")
		if err != nil {
			t.Errorf("Failed to delete reservation: %v", err)
		}

		rowsAffected, err := result.RowsAffected()
		if err != nil {
			t.Errorf("Failed to get rows affected: %v", err)
		}
		if rowsAffected != 1 {
			t.Errorf("Expected 1 row affected, got %d", rowsAffected)
		}

		// Verify deletion
		var count int
		err = db.QueryRow("SELECT COUNT(*) FROM reservations WHERE id = ?", "test-id-1").Scan(&count)
		if err != nil {
			t.Errorf("Failed to query deleted reservation: %v", err)
		}
		if count != 0 {
			t.Errorf("Expected 0 reservations, got %d", count)
		}
	})

	// Test 5: Query reservations by status
	t.Run("QueryReservationsByStatus", func(t *testing.T) {
		rows, err := db.Query(`
			SELECT id, programId, status
			FROM reservations
			WHERE status = ?`, "pending")
		if err != nil {
			t.Fatalf("Failed to query reservations by status: %v", err)
		}
		defer rows.Close()

		var pendingCount int
		for rows.Next() {
			var id string
			var programId int64
			var status string
			err := rows.Scan(&id, &programId, &status)
			if err != nil {
				t.Errorf("Failed to scan reservation: %v", err)
			}
			if status != "pending" {
				t.Errorf("Expected status 'pending', got '%s'", status)
			}
			pendingCount++
		}

		if pendingCount != 1 {
			t.Errorf("Expected 1 pending reservation, got %d", pendingCount)
		}
	})

	// Test 6: Handle NULL error field
	t.Run("HandleNullErrorField", func(t *testing.T) {
		// Insert reservation with error
		_, err := db.Exec(`
			INSERT INTO reservations (id, programId, serviceId, name, startAt, duration, 
				recorderUrl, recorderProgramId, status, createdAt, updatedAt, error)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			"test-error-id", 45678, 4567, "Test Error Program", 
			time.Now().Add(4*time.Hour).UnixMilli(), 3600000,
			"http://recorder:8080", "45678", "failed",
			time.Now().UnixMilli(), time.Now().UnixMilli(), "Connection timeout")
		
		if err != nil {
			t.Fatalf("Failed to insert reservation with error: %v", err)
		}

		// Query both with and without error
		rows, err := db.Query(`
			SELECT id, error
			FROM reservations
			WHERE id IN ('test-id-2', 'test-error-id')
			ORDER BY id`)
		if err != nil {
			t.Fatalf("Failed to query reservations: %v", err)
		}
		defer rows.Close()

		for rows.Next() {
			var id string
			var errorStr *string
			err := rows.Scan(&id, &errorStr)
			if err != nil {
				t.Errorf("Failed to scan reservation: %v", err)
			}

			if id == "test-error-id" {
				if errorStr == nil || *errorStr != "Connection timeout" {
					t.Errorf("Expected error 'Connection timeout', got %v", errorStr)
				}
			} else if id == "test-id-2" {
				if errorStr != nil {
					t.Errorf("Expected no error for test-id-2, got %v", *errorStr)
				}
			}
		}
	})
}

func TestReservationIndexes(t *testing.T) {
	// Initialize test database
	db, err := InitDB(":memory:")
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Check if indexes exist
	t.Run("CheckIndexes", func(t *testing.T) {
		// Check programId index
		var programIdxCount int
		err := db.QueryRow(`
			SELECT COUNT(*) 
			FROM sqlite_master 
			WHERE type='index' AND name='idx_reservations_programId'`).Scan(&programIdxCount)
		if err != nil {
			t.Errorf("Failed to check programId index: %v", err)
		}
		if programIdxCount != 1 {
			t.Error("programId index not found")
		}

		// Check status index
		var statusIdxCount int
		err = db.QueryRow(`
			SELECT COUNT(*) 
			FROM sqlite_master 
			WHERE type='index' AND name='idx_reservations_status'`).Scan(&statusIdxCount)
		if err != nil {
			t.Errorf("Failed to check status index: %v", err)
		}
		if statusIdxCount != 1 {
			t.Error("status index not found")
		}
	})
}