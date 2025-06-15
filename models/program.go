// models/program.go
package models

// Series は Mirakurun から取得するシリーズ情報を保持する構造体
type Series struct {
	ID          int    `json:"id"`
	Episode     int    `json:"episode,omitempty"`
	LastEpisode int    `json:"lastEpisode,omitempty"`
	Name        string `json:"name,omitempty"`
	Repeat      int    `json:"repeat,omitempty"`
	Pattern     int    `json:"pattern,omitempty"`
	ExpiresAt   int64  `json:"expiresAt,omitempty"`
}

// Program は Mirakurun から取得する番組情報の主要フィールドを保持する構造体
type Program struct {
	ID                int64  `json:"id"`
	ServiceID         int64  `json:"serviceId"`
	StartAt           int64  `json:"startAt"`
	Duration          int64  `json:"duration"`
	Name              string `json:"name"`
	Description       string `json:"description"`
	NameForSearch     string `json:"-"` // 検索用に正規化された番組名（JSONには含めない）
	DescForSearch     string `json:"-"` // 検索用に正規化された説明（JSONには含めない）
	
	// 追加の局情報（JSONにも含める）
	StationID         string `json:"stationId,omitempty"`
	StationName       string `json:"stationName,omitempty"`
	ChannelType       string `json:"channelType,omitempty"`
	ChannelNumber     string `json:"channelNumber,omitempty"`
	RemoteControlKey  int    `json:"remoteControlKey,omitempty"`
	
	// Series information from Mirakurun
	Series            *Series `json:"series,omitempty"`
}