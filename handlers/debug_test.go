package handlers

import (
	"testing"

	"github.com/fuba/iepg-server/db"
	"github.com/fuba/iepg-server/models"
)

func TestDebugExcludedServices(t *testing.T) {
	models.InitLogger("debug")

	// テスト用のインメモリデータベースを初期化
	dbConn, err := db.InitDB(":memory:")
	if err != nil {
		t.Fatalf("failed to initialize database: %v", err)
	}
	defer dbConn.Close()

	// テスト用のサービスをServiceMapに追加
	testService := models.Service{
		ServiceID:          12345,
		Name:              "テストチャンネル",
		Type:              1,
		NetworkID:         32737,
		RemoteControlKeyID: 1,
		ChannelType:       "GR",
		ChannelNumber:     "27",
	}
	models.ServiceMapInstance.Add(&testService)

	// 1. サービスを除外
	t.Log("=== Adding service to excluded list ===")
	if err := db.AddExcludedService(dbConn, testService.ServiceID, testService.Name); err != nil {
		t.Fatalf("Failed to add excluded service: %v", err)
	}

	// 2. DB状態を確認
	t.Log("=== Checking DB state after exclude ===")
	rows, err := dbConn.Query("SELECT serviceId, name FROM excluded_services")
	if err != nil {
		t.Fatalf("Failed to query excluded services: %v", err)
	}
	defer rows.Close()

	excludedCount := 0
	for rows.Next() {
		var serviceId int64
		var name string
		if err := rows.Scan(&serviceId, &name); err != nil {
			t.Fatalf("Failed to scan excluded service: %v", err)
		}
		t.Logf("Found excluded service: %d (%s)", serviceId, name)
		excludedCount++
	}
	t.Logf("Total excluded services in DB: %d", excludedCount)

	// 3. サービスの除外を解除
	t.Log("=== Removing service from excluded list ===")
	if err := db.RemoveExcludedService(dbConn, testService.ServiceID); err != nil {
		t.Fatalf("Failed to remove excluded service: %v", err)
	}

	// 4. DB状態を再確認
	t.Log("=== Checking DB state after unexclude ===")
	rows, err = dbConn.Query("SELECT serviceId, name FROM excluded_services")
	if err != nil {
		t.Fatalf("Failed to query excluded services after removal: %v", err)
	}
	defer rows.Close()

	excludedCount = 0
	for rows.Next() {
		var serviceId int64
		var name string
		if err := rows.Scan(&serviceId, &name); err != nil {
			t.Fatalf("Failed to scan excluded service: %v", err)
		}
		t.Logf("Found excluded service after removal: %d (%s)", serviceId, name)
		excludedCount++
	}
	t.Logf("Total excluded services in DB after removal: %d", excludedCount)

	if excludedCount > 0 {
		t.Fatalf("Expected 0 excluded services after removal, found %d", excludedCount)
	}

	// 5. GetExcludedServicesでも確認
	t.Log("=== Checking via GetExcludedServices ===")
	excludedServices, err := db.GetExcludedServices(dbConn)
	if err != nil {
		t.Fatalf("GetExcludedServices failed: %v", err)
	}
	t.Logf("GetExcludedServices returned %d services", len(excludedServices))
	
	for _, svc := range excludedServices {
		t.Logf("GetExcludedServices returned: %d (%s)", svc.ServiceID, svc.Name)
	}

	if len(excludedServices) > 0 {
		t.Fatalf("Expected 0 excluded services from GetExcludedServices, found %d", len(excludedServices))
	}
}