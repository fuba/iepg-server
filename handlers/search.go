// handlers/search.go
package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"

	"github.com/fuba/iepg-server/db"
	"github.com/fuba/iepg-server/models"
)

// HandleSimpleSearch は /search エンドポイントのハンドラー
func HandleSimpleSearch(w http.ResponseWriter, r *http.Request, dbConn *sql.DB) {
	models.Log.Debug("HandleSimpleSearch: Processing request from %s", r.RemoteAddr)
	
	q := r.URL.Query().Get("q")
	serviceIdStr := r.URL.Query().Get("serviceId")
	startFromStr := r.URL.Query().Get("startFrom")
	startToStr := r.URL.Query().Get("startTo")
	channelTypeStr := r.URL.Query().Get("channelType")
	
	models.Log.Debug("HandleSimpleSearch: Query params - q=%s, serviceId=%s, startFrom=%s, startTo=%s, channelType=%s", 
		q, serviceIdStr, startFromStr, startToStr, channelTypeStr)

	var serviceId int64
	var startFrom, startTo int64
	var channelType int
	var err error
	
	if serviceIdStr != "" {
		serviceId, err = strconv.ParseInt(serviceIdStr, 10, 64)
		if err != nil {
			models.Log.Error("HandleSimpleSearch: Invalid serviceId: %s, error: %v", serviceIdStr, err)
			http.Error(w, "invalid serviceId", http.StatusBadRequest)
			return
		}
	}
	
	if channelTypeStr != "" {
		var channelTypeInt int64
		channelTypeInt, err = strconv.ParseInt(channelTypeStr, 10, 64)
		if err != nil {
			models.Log.Error("HandleSimpleSearch: Invalid channelType: %s, error: %v", channelTypeStr, err) 
			http.Error(w, "invalid channelType", http.StatusBadRequest)
			return
		}
		// チャンネルタイプは1〜3の範囲のみ許可
		if channelTypeInt >= 1 && channelTypeInt <= 3 {
			channelType = int(channelTypeInt)
		} else {
			models.Log.Error("HandleSimpleSearch: ChannelType out of range: %d", channelTypeInt)
			http.Error(w, "channelType must be 1, 2, or 3", http.StatusBadRequest)
			return
		}
	}
	
	if startFromStr != "" {
		startFrom, err = strconv.ParseInt(startFromStr, 10, 64)
		if err != nil {
			models.Log.Error("HandleSimpleSearch: Invalid startFrom: %s, error: %v", startFromStr, err)
			http.Error(w, "invalid startFrom", http.StatusBadRequest)
			return
		}
	}
	
	if startToStr != "" {
		startTo, err = strconv.ParseInt(startToStr, 10, 64)
		if err != nil {
			models.Log.Error("HandleSimpleSearch: Invalid startTo: %s, error: %v", startToStr, err)
			http.Error(w, "invalid startTo", http.StatusBadRequest)
			return
		}
	}
	
	models.Log.Debug("HandleSimpleSearch: Parsed params - q=%s, serviceId=%d, startFrom=%d, startTo=%d, channelType=%d", 
		q, serviceId, startFrom, startTo, channelType)

	programs, err := db.SearchPrograms(dbConn, q, serviceId, startFrom, startTo, channelType)
	if err != nil {
		models.Log.Error("HandleSimpleSearch: Search failed: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	
	// 各プログラムに追加情報を付与
	for i := range programs {
		// 番組名とプログラム説明を特殊文字変換
		programs[i].Name = normalizeSpecialCharacters(programs[i].Name)
		programs[i].Description = normalizeSpecialCharacters(programs[i].Description)
		
		// サービス情報を付与
		if service, ok := models.ServiceMapInstance.Get(programs[i].ServiceID); ok {
			// テレビ局情報を付与
			programs[i].StationName = service.Name
			
			// リモコンキーIDあるいはサービスIDを取得
			programs[i].RemoteControlKey = service.RemoteControlKeyID
			if service.RemoteControlKeyID > 0 {
				programs[i].StationID = fmt.Sprintf("%04d", service.RemoteControlKeyID)
			} else {
				programs[i].StationID = fmt.Sprintf("%04d", service.ServiceID)
			}
			
			// チャンネル情報を付与
			programs[i].ChannelType = service.ChannelType
			programs[i].ChannelNumber = service.ChannelNumber
			
			models.Log.Debug("HandleSimpleSearch: Added service info for program: %d - %s (%s)",
				programs[i].ID, programs[i].Name, programs[i].StationName)
		}
	}
	
	models.Log.Info("HandleSimpleSearch: Search completed, found %d programs", len(programs))
	models.Log.Debug("HandleSimpleSearch: Special characters normalized for display")

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(programs); err != nil {
		models.Log.Error("HandleSimpleSearch: Failed to encode JSON response: %v", err)
	}
}

// HandleGetServices はすべてのサービス情報を返すハンドラー
func HandleGetServices(w http.ResponseWriter, r *http.Request, dbConn *sql.DB) {
	models.Log.Debug("HandleGetServices: Processing request from %s", r.RemoteAddr)
	
	// タイプ192は常に除外する
	excludedTypes := []int{192}
	
	// すべてのサービスを取得（タイプ192および除外チャンネルを除く）
	services := db.GetFilteredServices(dbConn, nil, excludedTypes)
	
	// デバッグ: サービスタイプ情報をログに出力
	for _, svc := range services {
		models.Log.Debug("Service: ID=%d, ServiceID=%d, Name=%s, Type=%d, ChannelType=%s", 
			svc.ID, svc.ServiceID, svc.Name, svc.Type, svc.ChannelType)
	}
	
	// サービスをリモコンキーID順、次にサービスID順でソート
	sort.Slice(services, func(i, j int) bool {
		// サービスタイプでまずソート
		if services[i].Type != services[j].Type {
			return services[i].Type < services[j].Type
		}
		
		// 同じサービスタイプ内ではリモコンキー順
		if services[i].RemoteControlKeyID > 0 && services[j].RemoteControlKeyID > 0 {
			return services[i].RemoteControlKeyID < services[j].RemoteControlKeyID
		}
		
		// リモコンキーがあるほうが前
		if services[i].RemoteControlKeyID > 0 {
			return true
		}
		if services[j].RemoteControlKeyID > 0 {
			return false
		}
		
		// どちらもリモコンキーがない場合はサービスID順
		return services[i].ServiceID < services[j].ServiceID
	})
	
	models.Log.Info("HandleGetServices: Returning %d services", len(services))
	
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(services); err != nil {
		models.Log.Error("HandleGetServices: Failed to encode JSON response: %v", err)
	}
}

// HandleGetSearchableServices は検索対象となるサービス情報を返すハンドラー
// 除外設定されたチャンネルを除き、検索時に使用されるチャンネルのみを返す
func HandleGetSearchableServices(w http.ResponseWriter, r *http.Request, dbConn *sql.DB) {
	models.Log.Debug("HandleGetSearchableServices: Processing request from %s", r.RemoteAddr)
	
	// クエリパラメータでフィルタリング指定を受け取る
	channelTypeStr := r.URL.Query().Get("channelType")
	var channelType int
	
	if channelTypeStr != "" {
		channelTypeInt, err := strconv.ParseInt(channelTypeStr, 10, 64)
		if err != nil {
			models.Log.Error("HandleGetSearchableServices: Invalid channelType: %s, error: %v", channelTypeStr, err)
			http.Error(w, "invalid channelType", http.StatusBadRequest)
			return
		}
		
		// チャンネルタイプは1〜3の範囲のみ許可
		if channelTypeInt >= 1 && channelTypeInt <= 3 {
			channelType = int(channelTypeInt)
		} else {
			models.Log.Error("HandleGetSearchableServices: ChannelType out of range: %d", channelTypeInt)
			http.Error(w, "channelType must be 1, 2, or 3", http.StatusBadRequest)
			return
		}
	}
	
	// タイプ192は常に除外する
	excludedTypes := []int{192}
	
	// チャンネルタイプが指定されている場合は、そのタイプのみを許可
	var allowedTypes []int
	if channelType > 0 {
		allowedTypes = []int{channelType}
	}
	
	// すべてのサービスを取得（タイプ192および除外チャンネルを除く）
	services := db.GetFilteredServices(dbConn, allowedTypes, excludedTypes)
	
	// 注: 常に詳細情報のみを返すように変更
	
	// デバッグ: サービスタイプ情報をログに出力
	for _, svc := range services {
		models.Log.Debug("SearchableService: ID=%d, ServiceID=%d, Name=%s, Type=%d, ChannelType=%s", 
			svc.ID, svc.ServiceID, svc.Name, svc.Type, svc.ChannelType)
	}
	
	// サービスをサービスID順でソート
	sort.Slice(services, func(i, j int) bool {
		return services[i].ServiceID < services[j].ServiceID
	})
	
	models.Log.Info("HandleGetSearchableServices: Returning %d detailed services", len(services))
	
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(services); err != nil {
		models.Log.Error("HandleGetSearchableServices: Failed to encode JSON response: %v", err)
	}
}

// チャンネルタイプ名を取得する
func getChannelTypeName(typeCode int) string {
	switch typeCode {
	case 1:
		return "地上波"
	case 2:
		return "BS"
	case 3:
		return "CS"
	default:
		return "その他"
	}
}

// HandleGetAllServices は除外設定に関係なくすべてのサービス情報を返すハンドラー
func HandleGetAllServices(w http.ResponseWriter, r *http.Request, dbConn *sql.DB) {
	models.Log.Debug("HandleGetAllServices: Processing request from %s", r.RemoteAddr)
	
	// タイプ192は常に除外する
	excludedTypes := []int{192}
	
	// すべてのサービスを取得（タイプ192を除く、除外チャンネルは含む）
	services := db.GetFilteredServices(nil, nil, excludedTypes)
	
	// 除外されているサービスIDのマップを作成
	excludedIds := make(map[int64]bool)
	if dbConn != nil {
		rows, err := dbConn.Query("SELECT serviceId FROM excluded_services")
		if err != nil {
			models.Log.Error("HandleGetAllServices: Failed to query excluded services: %v", err)
		} else {
			defer rows.Close()
			for rows.Next() {
				var serviceId int64
				if err := rows.Scan(&serviceId); err != nil {
					models.Log.Error("HandleGetAllServices: Failed to scan excluded service: %v", err)
					continue
				}
				excludedIds[serviceId] = true
			}
		}
	}
	models.Log.Debug("HandleGetAllServices: Loaded %d excluded services", len(excludedIds))
	
	// 各サービスに除外フラグを追加
	for i, service := range services {
		if excludedIds[service.ServiceID] {
			service.IsExcluded = true
			services[i] = service // ポインタでなくコピーなのでインデックスで更新
			models.Log.Debug("HandleGetAllServices: Marked service as excluded: %d (%s)", 
				service.ServiceID, service.Name)
		}
	}
	
	// サービスをサービスID順でソート
	sort.Slice(services, func(i, j int) bool {
		return services[i].ServiceID < services[j].ServiceID
	})
	
	models.Log.Info("HandleGetAllServices: Returning %d services", len(services))
	
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(services); err != nil {
		models.Log.Error("HandleGetAllServices: Failed to encode JSON response: %v", err)
	}
}

// HandleGetExcludedServices は除外チャンネルの一覧を取得するハンドラー
func HandleGetExcludedServices(w http.ResponseWriter, r *http.Request, dbConn *sql.DB) {
	models.Log.Debug("HandleGetExcludedServices: Processing request from %s", r.RemoteAddr)
	
	excludedServices, err := db.GetExcludedServices(dbConn)
	if err != nil {
		models.Log.Error("HandleGetExcludedServices: Failed to get excluded services: %v", err)
		http.Error(w, "Failed to get excluded services", http.StatusInternalServerError)
		return
	}
	
	models.Log.Info("HandleGetExcludedServices: Returning %d excluded services", len(excludedServices))
	
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(excludedServices); err != nil {
		models.Log.Error("HandleGetExcludedServices: Failed to encode JSON response: %v", err)
	}
}

// HandleAddExcludedService は除外チャンネルを追加するハンドラー
func HandleAddExcludedService(w http.ResponseWriter, r *http.Request, dbConn *sql.DB) {
	models.Log.Debug("HandleAddExcludedService: Processing request from %s", r.RemoteAddr)
	
	if r.Method != "POST" {
		models.Log.Error("HandleAddExcludedService: Method not allowed: %s", r.Method)
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	
	// リクエストボディをパース
	var service struct {
		ServiceID int64  `json:"serviceId"`
		Name      string `json:"name"`
	}
	
	if err := json.NewDecoder(r.Body).Decode(&service); err != nil {
		models.Log.Error("HandleAddExcludedService: Failed to parse request: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	
	models.Log.Debug("HandleAddExcludedService: Adding service - ID=%d, Name=%s", service.ServiceID, service.Name)
	
	// サービス名が空の場合、ServiceMapから名前を取得
	if service.Name == "" {
		if svc, ok := models.ServiceMapInstance.Get(service.ServiceID); ok {
			service.Name = svc.Name
			models.Log.Debug("HandleAddExcludedService: Found service name from ServiceMap: %s", service.Name)
		} else {
			service.Name = fmt.Sprintf("Service %d", service.ServiceID)
			models.Log.Debug("HandleAddExcludedService: Using generated service name: %s", service.Name)
		}
	}
	
	if err := db.AddExcludedService(dbConn, service.ServiceID, service.Name); err != nil {
		models.Log.Error("HandleAddExcludedService: Failed to add excluded service: %v", err)
		http.Error(w, "Failed to add excluded service", http.StatusInternalServerError)
		return
	}
	
	models.Log.Info("HandleAddExcludedService: Successfully added service %d (%s) to excluded list", 
		service.ServiceID, service.Name)
	
	// 成功レスポンス
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Service added to exclusion list",
	})
}

// HandleRemoveExcludedService は除外チャンネルを削除するハンドラー
func HandleRemoveExcludedService(w http.ResponseWriter, r *http.Request, dbConn *sql.DB) {
	models.Log.Debug("HandleRemoveExcludedService: Processing request from %s", r.RemoteAddr)
	
	if r.Method != "POST" && r.Method != "DELETE" {
		models.Log.Error("HandleRemoveExcludedService: Method not allowed: %s", r.Method)
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	
	// リクエストボディをパース
	var service struct {
		ServiceID int64 `json:"serviceId"`
	}
	
	if err := json.NewDecoder(r.Body).Decode(&service); err != nil {
		models.Log.Error("HandleRemoveExcludedService: Failed to parse request: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	
	models.Log.Debug("HandleRemoveExcludedService: Removing service - ID=%d", service.ServiceID)
	
	if err := db.RemoveExcludedService(dbConn, service.ServiceID); err != nil {
		models.Log.Error("HandleRemoveExcludedService: Failed to remove excluded service: %v", err)
		http.Error(w, "Failed to remove excluded service", http.StatusInternalServerError)
		return
	}
	
	models.Log.Info("HandleRemoveExcludedService: Successfully removed service %d from excluded list", service.ServiceID)
	
	// 成功レスポンス
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Service removed from exclusion list",
	})
}