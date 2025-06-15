package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fuba/iepg-server/db"
	"github.com/fuba/iepg-server/models"
)

func TestExcludeUnexcludeAPIIntegration(t *testing.T) {
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

	// 1. 初期状態で除外されていないことを確認
	t.Run("Initial state - service not excluded", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/services/all", nil)
		w := httptest.NewRecorder()
		HandleGetAllServices(w, req, dbConn)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}

		var services []models.Service
		if err := json.Unmarshal(w.Body.Bytes(), &services); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}

		// テストサービスを探す
		var testSvc *models.Service
		for _, svc := range services {
			if svc.ServiceID == testService.ServiceID {
				testSvc = &svc
				break
			}
		}

		if testSvc == nil {
			t.Fatalf("test service not found in all services")
		}

		if testSvc.IsExcluded {
			t.Fatalf("service should not be excluded initially, but IsExcluded=%t", testSvc.IsExcluded)
		}
		t.Logf("Initial state: service %d is not excluded (IsExcluded=%t)", testSvc.ServiceID, testSvc.IsExcluded)
	})

	// 2. サービスを除外
	t.Run("Exclude service", func(t *testing.T) {
		requestBody := map[string]interface{}{
			"serviceId": testService.ServiceID,
			"name":      testService.Name,
		}
		bodyBytes, _ := json.Marshal(requestBody)

		req := httptest.NewRequest("POST", "/services/exclude", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		HandleAddExcludedService(w, req, dbConn)

		if w.Code != http.StatusCreated {
			t.Fatalf("expected status 201, got %d. Response: %s", w.Code, w.Body.String())
		}

		var response map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}

		if !response["success"].(bool) {
			t.Fatalf("exclude operation should succeed")
		}
		t.Logf("Exclude operation response: %+v", response)
	})

	// 3. 除外後に /services/all で IsExcluded が true になることを確認
	t.Run("Check service excluded in all services", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/services/all", nil)
		w := httptest.NewRecorder()
		HandleGetAllServices(w, req, dbConn)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}

		var services []models.Service
		if err := json.Unmarshal(w.Body.Bytes(), &services); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}

		// テストサービスを探す
		var testSvc *models.Service
		for _, svc := range services {
			if svc.ServiceID == testService.ServiceID {
				testSvc = &svc
				break
			}
		}

		if testSvc == nil {
			t.Fatalf("test service not found in all services after exclusion")
		}

		if !testSvc.IsExcluded {
			t.Fatalf("service should be excluded after exclude operation, but IsExcluded=%t", testSvc.IsExcluded)
		}
		t.Logf("After exclude: service %d is excluded (IsExcluded=%t)", testSvc.ServiceID, testSvc.IsExcluded)
	})

	// 4. /services/excluded で除外リストに含まれることを確認
	t.Run("Check service in excluded list", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/services/excluded", nil)
		w := httptest.NewRecorder()
		HandleGetExcludedServices(w, req, dbConn)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}

		var excludedServices []models.Service
		if err := json.Unmarshal(w.Body.Bytes(), &excludedServices); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}

		// テストサービスが除外リストに含まれているか確認
		var found bool
		for _, svc := range excludedServices {
			if svc.ServiceID == testService.ServiceID {
				found = true
				break
			}
		}

		if !found {
			t.Fatalf("test service should be in excluded list")
		}
		t.Logf("Service %d found in excluded list", testService.ServiceID)
	})

	// 5. サービスの除外を解除
	t.Run("Unexclude service", func(t *testing.T) {
		requestBody := map[string]interface{}{
			"serviceId": testService.ServiceID,
		}
		bodyBytes, _ := json.Marshal(requestBody)

		req := httptest.NewRequest("POST", "/services/unexclude", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		HandleRemoveExcludedService(w, req, dbConn)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d. Response: %s", w.Code, w.Body.String())
		}

		var response map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}

		if !response["success"].(bool) {
			t.Fatalf("unexclude operation should succeed")
		}
		t.Logf("Unexclude operation response: %+v", response)
	})

	// 6. 除外解除後に /services/all で IsExcluded が false になることを確認
	t.Run("Check service unexcluded in all services", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/services/all", nil)
		w := httptest.NewRecorder()
		HandleGetAllServices(w, req, dbConn)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}

		var services []models.Service
		if err := json.Unmarshal(w.Body.Bytes(), &services); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}

		// テストサービスを探す
		var testSvc *models.Service
		for _, svc := range services {
			if svc.ServiceID == testService.ServiceID {
				testSvc = &svc
				break
			}
		}

		if testSvc == nil {
			t.Fatalf("test service not found in all services after unexclusion")
		}

		if testSvc.IsExcluded {
			t.Fatalf("service should not be excluded after unexclude operation, but IsExcluded=%t", testSvc.IsExcluded)
		}
		t.Logf("After unexclude: service %d is not excluded (IsExcluded=%t)", testSvc.ServiceID, testSvc.IsExcluded)
	})

	// 7. /services/excluded で除外リストから削除されることを確認
	t.Run("Check service not in excluded list", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/services/excluded", nil)
		w := httptest.NewRecorder()
		HandleGetExcludedServices(w, req, dbConn)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}

		var excludedServices []models.Service
		if err := json.Unmarshal(w.Body.Bytes(), &excludedServices); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}

		// テストサービスが除外リストに含まれていないか確認
		for _, svc := range excludedServices {
			if svc.ServiceID == testService.ServiceID {
				t.Fatalf("test service should not be in excluded list after unexclude")
			}
		}
		t.Logf("Service %d not found in excluded list (correct)", testService.ServiceID)
	})
}

func TestRealServiceExcludeUnexclude(t *testing.T) {
	models.InitLogger("debug")

	// 実際のデータベースを使用したテスト
	dbPath := "./test_programs.db"
	dbConn, err := db.InitDB(dbPath)
	if err != nil {
		t.Fatalf("failed to initialize database: %v", err)
	}
	defer dbConn.Close()
	defer func() {
		// テスト後にテストDBを削除
		// os.Remove(dbPath)
	}()

	// 実際のサーバーにHTTPリクエストを送信してテスト
	t.Run("Test with real API calls", func(t *testing.T) {
		baseURL := "http://localhost:40870"

		// 1. 除外されているサービス一覧を取得
		resp, err := http.Get(baseURL + "/services/excluded")
		if err != nil {
			t.Fatalf("failed to get excluded services: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected status 200, got %d", resp.StatusCode)
		}

		var excludedServices []models.Service
		if err := json.NewDecoder(resp.Body).Decode(&excludedServices); err != nil {
			t.Fatalf("failed to decode excluded services: %v", err)
		}

		t.Logf("Current excluded services count: %d", len(excludedServices))

		if len(excludedServices) == 0 {
			t.Log("No excluded services to test with")
			return
		}

		// 最初のサービスを使用してテスト
		testService := excludedServices[0]
		t.Logf("Testing with service: %d (%s)", testService.ServiceID, testService.Name)

		// 2. /services/all で IsExcluded が true であることを確認
		resp, err = http.Get(baseURL + "/services/all")
		if err != nil {
			t.Fatalf("failed to get all services: %v", err)
		}
		defer resp.Body.Close()

		var allServices []models.Service
		if err := json.NewDecoder(resp.Body).Decode(&allServices); err != nil {
			t.Fatalf("failed to decode all services: %v", err)
		}

		var foundService *models.Service
		for _, svc := range allServices {
			if svc.ServiceID == testService.ServiceID {
				foundService = &svc
				break
			}
		}

		if foundService == nil {
			t.Fatalf("test service %d not found in all services", testService.ServiceID)
		}

		if !foundService.IsExcluded {
			t.Fatalf("service %d should be excluded but IsExcluded=%t", testService.ServiceID, foundService.IsExcluded)
		}
		t.Logf("Before unexclude: service %d IsExcluded=%t ✓", testService.ServiceID, foundService.IsExcluded)

		// 3. 除外を解除
		unexcludeBody := map[string]interface{}{
			"serviceId": testService.ServiceID,
		}
		bodyBytes, _ := json.Marshal(unexcludeBody)

		resp, err = http.Post(baseURL+"/services/unexclude", "application/json", bytes.NewReader(bodyBytes))
		if err != nil {
			t.Fatalf("failed to unexclude service: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected status 200 for unexclude, got %d", resp.StatusCode)
		}
		t.Logf("Unexclude request successful")

		// 4. 除外解除後に /services/all で IsExcluded が false になることを確認
		resp, err = http.Get(baseURL + "/services/all")
		if err != nil {
			t.Fatalf("failed to get all services after unexclude: %v", err)
		}
		defer resp.Body.Close()

		if err := json.NewDecoder(resp.Body).Decode(&allServices); err != nil {
			t.Fatalf("failed to decode all services after unexclude: %v", err)
		}

		foundService = nil
		for _, svc := range allServices {
			if svc.ServiceID == testService.ServiceID {
				foundService = &svc
				break
			}
		}

		if foundService == nil {
			t.Fatalf("test service %d not found in all services after unexclude", testService.ServiceID)
		}

		if foundService.IsExcluded {
			t.Fatalf("service %d should not be excluded after unexclude but IsExcluded=%t", testService.ServiceID, foundService.IsExcluded)
		}
		t.Logf("After unexclude: service %d IsExcluded=%t ✓", testService.ServiceID, foundService.IsExcluded)

		// 5. 元の状態に戻す（再度除外）
		excludeBody := map[string]interface{}{
			"serviceId": testService.ServiceID,
			"name":      testService.Name,
		}
		bodyBytes, _ = json.Marshal(excludeBody)

		resp, err = http.Post(baseURL+"/services/exclude", "application/json", bytes.NewReader(bodyBytes))
		if err != nil {
			t.Fatalf("failed to re-exclude service: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("expected status 201 for exclude, got %d", resp.StatusCode)
		}
		t.Logf("Re-exclude request successful, restored original state")
	})
}