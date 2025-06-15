// models/auto_reservation.go
package models

import "time"

// AutoReservationRule は自動予約の基本ルールを保持する構造体
type AutoReservationRule struct {
	ID          string    `json:"id"`
	Type        string    `json:"type"`        // "keyword" or "series"
	Name        string    `json:"name"`
	Enabled     bool      `json:"enabled"`
	Priority    int       `json:"priority"`
	RecorderURL string    `json:"recorderUrl"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// KeywordRule はキーワード検索による自動予約ルールを保持する構造体
type KeywordRule struct {
	RuleID       string   `json:"ruleId"`
	Keywords     []string `json:"keywords"`     // 検索キーワード（AND条件）
	Genres       []int    `json:"genres,omitempty"`      // ジャンルフィルタ
	ServiceIDs   []int64  `json:"serviceIds,omitempty"`   // チャンネルフィルタ
	ExcludeWords []string `json:"excludeWords,omitempty"` // 除外キーワード
}

// SeriesRule はシリーズIDによる自動予約ルールを保持する構造体
type SeriesRule struct {
	RuleID      string `json:"ruleId"`
	SeriesID    string `json:"seriesId"`    // MirakurunのシリーズID
	ProgramName string `json:"programName"` // 番組名（参考表示用）
	ServiceID   int64  `json:"serviceId,omitempty"`   // チャンネル（オプション）
}

// AutoReservationLog は自動予約の実行履歴を保持する構造体
type AutoReservationLog struct {
	ID            string    `json:"id"`
	RuleID        string    `json:"ruleId"`
	ProgramID     int64     `json:"programId"`
	ReservationID string    `json:"reservationId,omitempty"` // 実際に作成された予約のID
	Status        string    `json:"status"`        // "matched", "reserved", "skipped", "failed"
	Reason        string    `json:"reason,omitempty"`        // スキップ/失敗理由
	CreatedAt     time.Time `json:"createdAt"`
}

// AutoReservationRuleWithDetails は詳細情報を含む自動予約ルール
type AutoReservationRuleWithDetails struct {
	AutoReservationRule
	KeywordRule *KeywordRule `json:"keywordRule,omitempty"`
	SeriesRule  *SeriesRule  `json:"seriesRule,omitempty"`
}