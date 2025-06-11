package db

import (
	"testing"

	"github.com/fuba/iepg-server/models"
)

func TestAddAndRemoveExcludedService(t *testing.T) {
	models.InitLogger("debug")

	db, err := InitDB(":memory:")
	if err != nil {
		t.Fatalf("failed to initialize database: %v", err)
	}
	defer db.Close()

	// Add a service to the excluded list
	serviceID := int64(12345)
	name := "Test Service"
	if err := AddExcludedService(db, serviceID, name); err != nil {
		t.Fatalf("AddExcludedService returned error: %v", err)
	}

	services, err := GetExcludedServices(db)
	if err != nil {
		t.Fatalf("GetExcludedServices returned error: %v", err)
	}
	if len(services) != 1 {
		t.Fatalf("expected 1 excluded service, got %d", len(services))
	}
	if services[0].ServiceID != serviceID || services[0].Name != name {
		t.Errorf("returned service does not match: %+v", services[0])
	}

	// Remove the service
	if err := RemoveExcludedService(db, serviceID); err != nil {
		t.Fatalf("RemoveExcludedService returned error: %v", err)
	}

	services, err = GetExcludedServices(db)
	if err != nil {
		t.Fatalf("GetExcludedServices after remove returned error: %v", err)
	}
	if len(services) != 0 {
		t.Fatalf("expected 0 excluded services after removal, got %d", len(services))
	}
}
