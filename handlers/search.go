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
func HandleGetServices(w http.ResponseWriter, r *http.Request) {
	models.Log.Debug("HandleGetServices: Processing request from %s", r.RemoteAddr)
	
	// タイプ192は常に除外する
	excludedTypes := []int{192}
	
	// すべてのサービスを取得（タイプ192を除く）
	services := db.GetFilteredServices(nil, excludedTypes)
	
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