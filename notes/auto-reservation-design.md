# 自動予約システム設計検討記録

## 作業日: 2025/1/15

### 目的
iepg-serverに自動予約機能を追加し、キーワードや番組シリーズベースで自動的に録画予約を行えるようにする。

## 現在の実装調査結果

### 既存の予約システム
- **予約データモデル** (`models/reservation.go`)
  - UUID形式のID、番組情報、録画サーバーURL、ステータス管理
  - ステータス: pending/recording/completed/failed/cancelled
  - 録画開始1分前から録画可能と判定

- **予約API** (`handlers/reservation.go`)
  - POST /reservations - 予約作成（録画サーバーAPIを非同期呼び出し）
  - GET /reservations - 予約一覧取得
  - DELETE /reservations/{id} - 予約削除
  - リトライ機能付き（3回まで）

- **データベース**
  - SQLiteベースでreservationsテーブルを使用
  - programIdとstatusにインデックス設定

- **現在の制限事項**
  - スケジューラー未実装（予約時刻になっても自動録画開始しない）
  - ステータスの自動更新なし
  - 重複予約チェックなし
  - 録画サーバーからのコールバック受信なし

### Mirakurunのシリーズ情報
調査の結果、Mirakurun APIから以下のシリーズ情報が取得可能であることが判明：
```json
{
  "series": {
    "id": 12345,           // シリーズID（番組グループ化のキー）
    "episode": 5,          // 現在のエピソード番号
    "lastEpisode": 12,     // 最終話番号
    "name": "シリーズ名",
    "repeat": 0,           // リピート回数
    "pattern": 1,          // パターン
    "expiresAt": 1234567890000  // 有効期限
  }
}
```

ただし、現在の実装ではこのシリーズ情報は破棄されており、データベースにも保存されていない。

## 自動予約システムの設計案

### 1. データモデル設計

#### 自動予約ルール
```go
// AutoReservationRule - 自動予約の基本ルール
type AutoReservationRule struct {
    ID          string    // UUID
    Type        string    // "keyword" or "series"
    Name        string    // ルール名
    Enabled     bool      // 有効/無効
    Priority    int       // 優先度
    RecorderURL string    // 録画サーバーURL
    CreatedAt   time.Time
    UpdatedAt   time.Time
}
```

#### キーワードベース自動予約
```go
// KeywordRule - キーワード検索による自動予約
type KeywordRule struct {
    RuleID       string   // AutoReservationRule.ID
    Keywords     []string // 検索キーワード（AND条件）
    Genres       []int    // ジャンルフィルタ
    ServiceIDs   []int    // チャンネルフィルタ
    ExcludeWords []string // 除外キーワード
}
```

#### シリーズベース自動予約
```go
// SeriesRule - シリーズIDによる自動予約
type SeriesRule struct {
    RuleID      string // AutoReservationRule.ID
    SeriesID    string // MirakurunのシリーズID
    ProgramName string // 番組名（参考表示用）
    ServiceID   int    // チャンネル（オプション）
}
```

#### 実行ログ
```go
// AutoReservationLog - 自動予約の実行履歴
type AutoReservationLog struct {
    ID            string
    RuleID        string
    ProgramID     int
    ReservationID string // 実際に作成された予約のID
    Status        string // "matched", "reserved", "skipped", "failed"
    Reason        string // スキップ/失敗理由
    CreatedAt     time.Time
}
```

### 2. 実装方針

#### Programモデルの拡張
まず、Mirakurunから取得したシリーズ情報を保存できるようにProgramモデルを拡張する必要がある：

```go
// models/program.go に追加
type Series struct {
    ID          int    `json:"id"`
    Episode     int    `json:"episode,omitempty"`
    LastEpisode int    `json:"lastEpisode,omitempty"`
    Name        string `json:"name,omitempty"`
}

// Program構造体にフィールド追加
Series *Series `json:"series,omitempty"`
```

#### 番組監視システム
- 5分ごとに新番組をチェック
- 各自動予約ルールとマッチング
- マッチしたら既存の予約APIを呼び出し
- 実行結果をログに記録

#### APIエンドポイント
```
# ルール管理
POST   /auto-reservations/rules
GET    /auto-reservations/rules
PUT    /auto-reservations/rules/{id}
DELETE /auto-reservations/rules/{id}

# ルール詳細設定
POST   /auto-reservations/rules/{id}/keywords
POST   /auto-reservations/rules/{id}/series

# ログ閲覧
GET    /auto-reservations/logs
GET    /auto-reservations/logs?ruleId={id}
```

### 3. 実装の利点

1. **シリーズIDの活用**: 番組名の表記揺れに左右されない確実な予約
2. **既存システムとの統合**: 現在の予約APIをそのまま活用
3. **拡張性**: キーワードとシリーズの両方に対応
4. **透明性**: ログで自動予約の動作を追跡可能
5. **優先度管理**: 複数ルールがマッチした場合の制御

## 次のステップ

1. Programモデルにシリーズ情報を追加
2. データベーススキーマの更新
3. 自動予約ルールのCRUD実装
4. 番組監視エンジンの実装
5. テスト実装

## 技術的な考慮事項

- 監視間隔は5分程度が妥当（番組情報の更新頻度を考慮）
- 重複予約を防ぐため、予約作成前にチェックが必要
- シリーズIDは番組によっては存在しない場合があるため、null許容にする
- 自動予約ログは定期的に古いものを削除する仕組みが必要