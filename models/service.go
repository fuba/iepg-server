// models/service.go
package models

import (
	"sort"
	"sync"
)

// MirakurunID composes the Mirakurun unique ID from networkID and serviceID.
// Matches Mirakurun's encoding: id = networkID * 100000 + serviceID.
func MirakurunID(networkID, serviceID int64) int64 {
	return networkID*100000 + serviceID
}

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

// ServiceMap は Mirakurun ID (service.ID = networkID*100000+serviceID) をキーとして
// Service の参照を保持するマップ。同じ serviceID でも networkID が異なれば別要素として扱う。
type ServiceMap struct {
	mu       sync.RWMutex
	services map[int64]*Service // Mirakurun ID (service.ID) をキーにしたマップ
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
	sm.services[service.ID] = service
}

// Update はサービス情報を更新する
func (sm *ServiceMap) Update(service *Service) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.services[service.ID] = service
}

// Remove は Mirakurun ID でサービス情報を削除する
func (sm *ServiceMap) Remove(mirakurunID int64) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	delete(sm.services, mirakurunID)
}

// Get は Mirakurun ID からサービス情報を取得する
func (sm *ServiceMap) Get(mirakurunID int64) (*Service, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	service, ok := sm.services[mirakurunID]
	return service, ok
}

// GetByServiceID は serviceID に一致するすべての Service を、
// Mirakurun ID (service.ID) 昇順で決定的に返す。
// networkID 情報が無い文脈（excluded_services 等）からの後方互換用。
func (sm *ServiceMap) GetByServiceID(serviceID int64) []*Service {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	var result []*Service
	for _, s := range sm.services {
		if s.ServiceID == serviceID {
			result = append(result, s)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

// RemoveByServiceID は serviceID に一致するすべての Service を削除する。
// Mirakurun ID が判らない remove イベントのフォールバック用。
func (sm *ServiceMap) RemoveByServiceID(serviceID int64) int {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	removed := 0
	for id, s := range sm.services {
		if s.ServiceID == serviceID {
			delete(sm.services, id)
			removed++
		}
	}
	return removed
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