// db/service_fetcher.go
package db

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/fuba/iepg-server/models"
)

// mirakurunResponse はMirakurunから取得したサービス情報のレスポンス構造体
type mirakurunServiceResponse struct {
	ID                int64        `json:"id"`
	ServiceID         int64        `json:"serviceId"`
	NetworkID         int64        `json:"networkId"`
	Name              string       `json:"name"`
	Type              int          `json:"type"`
	LogoID            int          `json:"logoId,omitempty"`
	HasLogoData       bool         `json:"hasLogoData,omitempty"`
	RemoteControlKeyID int         `json:"remoteControlKeyId,omitempty"`
	Channel           channelInfo  `json:"channel,omitempty"`
}

type channelInfo struct {
	Type    string `json:"type"`
	Channel string `json:"channel"`
	Name    string `json:"name,omitempty"`
	TSMFRel int    `json:"tsmfRelTs,omitempty"`
}

// StartServiceFetcher は定期的にMirakurunからサービス情報を取得してメモリに格納する
func StartServiceFetcher(ctx context.Context, mirakurunBaseURL string) {
	models.Log.Debug("StartServiceFetcher: Starting service fetcher with URL: %s", mirakurunBaseURL)

	// 初回のフェッチは即時実行
	fetchServices(ctx, mirakurunBaseURL)

	// 以降は15分ごとに実行
	ticker := time.NewTicker(15 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			models.Log.Info("ServiceFetcher: Context cancelled, stopping service fetcher")
			return
		case <-ticker.C:
			fetchServices(ctx, mirakurunBaseURL)
		}
	}
}

// StartServiceEventStream はMirakurunのサービスイベントストリームを購読する
func StartServiceEventStream(ctx context.Context, mirakurunBaseURL string) {
	// サービスイベントのストリームURLを構築
	// URLのパスが正しいことを確認
	apiURL := mirakurunBaseURL
	if !strings.HasSuffix(apiURL, "/api") && !strings.HasSuffix(apiURL, "/api/") {
		// URLが/apiで終わっていない場合は追加
		if !strings.HasSuffix(apiURL, "/") {
			apiURL += "/"
		}
		if !strings.Contains(apiURL, "/api/") {
			apiURL += "api/"
		}
	} else if !strings.HasSuffix(apiURL, "/") {
		// /apiで終わっていて/が無い場合は追加
		apiURL += "/"
	}
	// イベントストリームのURLを構築
	apiURL += "events/stream?resource=service"
	models.Log.Debug("StartServiceEventStream: Starting service event stream with URL: %s", apiURL)

	for {
		err := func() error {
			// リクエスト作成
			req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
			if err != nil {
				models.Log.Error("ServiceEventStream: Failed to create request: %v", err)
				return err
			}

			// リクエスト送信
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				models.Log.Error("ServiceEventStream: Request failed: %v", err)
				return err
			}
			defer resp.Body.Close()

			models.Log.Info("ServiceEventStream: Connected to Mirakurun stream, status: %s", resp.Status)

			// イベントストリームを解析して処理
			decoder := json.NewDecoder(resp.Body)
			for {
				var event struct {
					Resource string             `json:"resource"`
					Type     string             `json:"type"`
					Data     mirakurunServiceResponse `json:"data"`
					Time     int64              `json:"time"`
				}

				if err := decoder.Decode(&event); err != nil {
					if err == io.EOF {
						models.Log.Info("ServiceEventStream: Stream ended")
						break
					}
					models.Log.Error("ServiceEventStream: Failed to decode event: %v", err)
					return err
				}

				// サービスイベントを処理
				if event.Resource == "service" {
					switch event.Type {
					case "create", "update":
						service := convertToService(&event.Data)
						models.ServiceMapInstance.Update(service)
						models.Log.Debug("ServiceEventStream: Updated service: %d - %s", service.ServiceID, service.Name)
					case "remove":
						models.ServiceMapInstance.Remove(event.Data.ServiceID)
						models.Log.Debug("ServiceEventStream: Removed service: %d", event.Data.ServiceID)
					}
				}
			}
			return nil
		}()

		if err != nil {
			models.Log.Error("ServiceEventStream: Error: %v", err)
		}

		// 接続が切れたら5秒後に再接続
		time.Sleep(5 * time.Second)
	}
}

// fetchServices はMirakurunからサービス情報を取得する
func fetchServices(ctx context.Context, mirakurunBaseURL string) {
	// サービス一覧のAPIエンドポイントURL
	apiURL := mirakurunBaseURL
	if !strings.HasSuffix(apiURL, "/api") && !strings.HasSuffix(apiURL, "/api/") {
		// URLが/apiで終わっていない場合は追加
		if !strings.HasSuffix(apiURL, "/") {
			apiURL += "/"
		}
		if !strings.Contains(apiURL, "/api/") {
			apiURL += "api/"
		}
	} else if !strings.HasSuffix(apiURL, "/") {
		// /apiで終わっていて/が無い場合は追加
		apiURL += "/"
	}
	// サービス一覧のURLを構築
	apiURL += "services"
	models.Log.Debug("fetchServices: Fetching services from: %s (base URL: %s)", apiURL, mirakurunBaseURL)

	// リクエスト作成
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		models.Log.Error("fetchServices: Failed to create request: %v", err)
		return
	}

	// リクエスト送信
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		models.Log.Error("fetchServices: Request failed: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		models.Log.Error("fetchServices: API returned non-OK status: %s", resp.Status)
		return
	}

	// レスポンスをパース
	var services []mirakurunServiceResponse
	if err := json.NewDecoder(resp.Body).Decode(&services); err != nil {
		models.Log.Error("fetchServices: Failed to decode response: %v", err)
		return
	}

	// サービス情報をグローバルマップに保存
	count := 0
	for _, svc := range services {
		service := convertToService(&svc)
		models.ServiceMapInstance.Update(service)
		count++
	}

	models.Log.Info("fetchServices: Updated %d services", count)
}

// convertToService はMirakurunのレスポンスをServiceモデルに変換する
func convertToService(resp *mirakurunServiceResponse) *models.Service {
	service := &models.Service{
		ID:                resp.ID,
		ServiceID:         resp.ServiceID,
		NetworkID:         resp.NetworkID,
		Name:              resp.Name,
		Type:              resp.Type,
		LogoID:            resp.LogoID,
		HasLogoData:       resp.HasLogoData,
		RemoteControlKeyID: resp.RemoteControlKeyID,
	}

	// Channel情報がある場合は追加
	if resp.Channel.Type != "" {
		service.ChannelType = resp.Channel.Type
		service.ChannelNumber = resp.Channel.Channel
		service.ChannelName = resp.Channel.Name
		service.ChannelTSMFRel = resp.Channel.TSMFRel
		
		// ChannelTypeに基づいてTypeも設定（重複更新になるが整合性を確保）
		// GR=地上波=1, BS=2, CS=3
		switch resp.Channel.Type {
		case "GR":
			service.Type = 1
		case "BS":
			service.Type = 2
		case "CS":
			service.Type = 3
		default:
			// 既存のTypeをそのまま使用
		}
	}
	
	models.Log.Debug("convertToService: Converted service: ID=%d, Name=%s, Type=%d, ChannelType=%s", 
		service.ID, service.Name, service.Type, service.ChannelType)

	return service
}