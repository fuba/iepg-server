package handlers

import (
	"testing"

	"github.com/fuba/iepg-server/db"
	"github.com/fuba/iepg-server/models"
)

func TestDebugServiceMap(t *testing.T) {
	models.InitLogger("debug")

	// テスト用のインメモリデータベースを初期化
	dbConn, err := db.InitDB(":memory:")
	if err != nil {
		t.Fatalf("failed to initialize database: %v", err)
	}
	defer dbConn.Close()

	// ServiceMapの状態をクリア
	serviceMap := models.NewServiceMap()
	models.ServiceMapInstance = serviceMap

	// テスト用のサービスをServiceMapに追加
	testService := models.Service{
		ServiceID:          12345,
		Name:              "テストチャンネル",
		Type:              1,
		NetworkID:         32737,
		RemoteControlKeyID: 1,
		ChannelType:       "GR",
		ChannelNumber:     "27",
		IsExcluded:        false, // 初期状態では除外されていない
	}
	models.ServiceMapInstance.Add(&testService)

	t.Log("=== Initial ServiceMap state ===")
	allServices := db.GetFilteredServices(nil, nil, []int{192})
	t.Logf("GetFilteredServices returned %d services", len(allServices))
	for _, svc := range allServices {
		t.Logf("Service: %d (%s) IsExcluded=%t", svc.ServiceID, svc.Name, svc.IsExcluded)
	}

	// サービスを除外
	t.Log("=== Adding to excluded list ===")
	if err := db.AddExcludedService(dbConn, testService.ServiceID, testService.Name); err != nil {
		t.Fatalf("Failed to add excluded service: %v", err)
	}

	// ServiceMapの状態を再確認
	t.Log("=== ServiceMap state after exclude ===")
	allServices = db.GetFilteredServices(nil, nil, []int{192})
	t.Logf("GetFilteredServices returned %d services", len(allServices))
	for _, svc := range allServices {
		t.Logf("Service: %d (%s) IsExcluded=%t", svc.ServiceID, svc.Name, svc.IsExcluded)
	}

	// ServiceMapから直接取得してみる
	if svc, ok := models.ServiceMapInstance.Get(testService.ServiceID); ok {
		t.Logf("Direct from ServiceMap: %d (%s) IsExcluded=%t", svc.ServiceID, svc.Name, svc.IsExcluded)
	}

	// 除外解除
	t.Log("=== Removing from excluded list ===")
	if err := db.RemoveExcludedService(dbConn, testService.ServiceID); err != nil {
		t.Fatalf("Failed to remove excluded service: %v", err)
	}

	// ServiceMapの状態を再確認
	t.Log("=== ServiceMap state after unexclude ===")
	allServices = db.GetFilteredServices(nil, nil, []int{192})
	t.Logf("GetFilteredServices returned %d services", len(allServices))
	for _, svc := range allServices {
		t.Logf("Service: %d (%s) IsExcluded=%t", svc.ServiceID, svc.Name, svc.IsExcluded)
	}

	// ServiceMapから直接取得してみる
	if svc, ok := models.ServiceMapInstance.Get(testService.ServiceID); ok {
		t.Logf("Direct from ServiceMap after unexclude: %d (%s) IsExcluded=%t", svc.ServiceID, svc.Name, svc.IsExcluded)
	}
}