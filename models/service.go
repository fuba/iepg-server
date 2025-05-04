// models/service.go
package models

import (
	"sync"
)

// Service はMirakurunから取得するサービス情報（テレビ局情報）の構造体
type Service struct {
	ID                 int64  `json:"id"`
	ServiceID          int64  `json:"serviceId"`
	NetworkID          int64  `json:"networkId"`
	Name               string `json:"name"`
	Type               int    `json:"type"`
	LogoID             int    `json:"logoId,omitempty"`
	HasLogoData        bool   `json:"hasLogoData,omitempty"`
	RemoteControlKeyID int    `json:"remoteControlKeyId,omitempty"`

	// Channel情報
	ChannelType    string `json:"channelType,omitempty"`
	ChannelNumber  string `json:"channelNumber,omitempty"`
	ChannelName    string `json:"channelName,omitempty"`
	ChannelTSMFRel int    `json:"channelTsmfRelTs,omitempty"`
	
	// 除外フラグ（UI表示用）
	IsExcluded     bool   `json:"isExcluded,omitempty"`
}

// ChannelInfo はMirakurunから取得するChannel情報の構造体
type ChannelInfo struct {
	Type    string `json:"type"`
	Channel string `json:"channel"`
	Name    string `json:"name,omitempty"`
	TSMFRel int    `json:"tsmfRelTs,omitempty"`
}

// ServiceMap はサービスIDをキーとしてServiceの参照を保持するマップ
type ServiceMap struct {
	mu       sync.RWMutex
	services map[int64]*Service // サービスIDをキーにしたマップ
}

// NewServiceMap は新しいServiceMapを作成する
func NewServiceMap() *ServiceMap {
	return &ServiceMap{
		services: make(map[int64]*Service),
	}
}

// Add はサービス情報をマップに追加する
func (sm *ServiceMap) Add(service *Service) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.services[service.ServiceID] = service
}

// Update はサービス情報を更新する
func (sm *ServiceMap) Update(service *Service) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.services[service.ServiceID] = service
}

// Remove はサービス情報を削除する
func (sm *ServiceMap) Remove(serviceID int64) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	delete(sm.services, serviceID)
}

// Get はサービスIDからサービス情報を取得する
func (sm *ServiceMap) Get(serviceID int64) (*Service, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	service, ok := sm.services[serviceID]
	return service, ok
}

// GetAll はすべてのサービス情報を取得する
func (sm *ServiceMap) GetAll() []*Service {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	
	var result []*Service
	for _, service := range sm.services {
		result = append(result, service)
	}
	return result
}

// ExcludedService は除外チャンネルの情報を表す構造体
type ExcludedService struct {
	ServiceID          int64  `json:"serviceId"`
	Name               string `json:"name"`
	CreatedAt          int64  `json:"createdAt"`
	Type               int    `json:"type"`               // 1=地上波、2=BS、3=CS
	NetworkID          int64  `json:"networkId"`          // ネットワークID
	RemoteControlKeyID int    `json:"remoteControlKeyId"` // リモコンキーID
	ChannelType        string `json:"channelType"`        // "GR", "BS", "CS"など
	ChannelNumber      string `json:"channelNumber"`      // チャンネル番号
}

// SearchableService は検索対象となるチャンネルの簡易情報を表す構造体
type SearchableService struct {
	ServiceID    int64  `json:"serviceId"`
	Name         string `json:"name"`        // 表示用の名前（リモコンキー含む）
	Type         int    `json:"type"`        // 1=地上波、2=BS、3=CS
	TypeName     string `json:"typeName"`    // "地上波"、"BS"、"CS"など
	ChannelType  string `json:"channelType"` // "GR", "BS", "CS"など
}

// ServiceMapInstance はグローバルに使用するServiceMapのインスタンス
var ServiceMapInstance = NewServiceMap()