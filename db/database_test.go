package db

import (
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/fuba/iepg-server/models"
)

func init() {
	// テスト用にロガーを初期化
	models.InitLogger("debug")
}

func TestSearchPrograms(t *testing.T) {
	// テスト用のインメモリデータベースを初期化
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}
	defer db.Close()

	// programsテーブルの作成
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS programs (
			id            INTEGER PRIMARY KEY,
			serviceId     INTEGER,
			startAt       INTEGER,
			duration      INTEGER,
			name          TEXT,
			description   TEXT,
			nameForSearch TEXT,
			descForSearch TEXT
		);
	`)
	if err != nil {
		t.Fatalf("Failed to create test table: %v", err)
	}

	// テストデータの挿入
	testPrograms := []struct {
		id          int64
		serviceId   int64
		startAt     int64
		duration    int64
		name        string
		description string
	}{
		{1, 101, 1000, 30, "ニュース番組", "今日の最新ニュースをお届けします。"},
		{2, 102, 1030, 60, "ドラマ", "人気のドラマシリーズ。スポーツの話題も。"},
		{3, 101, 1090, 45, "スポーツ", "サッカーの試合特集。"},
		{4, 103, 1135, 30, "アニメ", "人気アニメの新シリーズ。"},
		{5, 102, 1165, 60, "映画", "古典的な映画の放送。"},
	}

	// データの挿入
	for _, p := range testPrograms {
		nameForSearch := models.NormalizeForSearch(p.name)
		descForSearch := models.NormalizeForSearch(p.description)
		_, err = db.Exec(
			`INSERT INTO programs (id, serviceId, startAt, duration, name, description, nameForSearch, descForSearch)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			p.id, p.serviceId, p.startAt, p.duration, p.name, p.description, nameForSearch, descForSearch,
		)
		if err != nil {
			t.Fatalf("Failed to insert test data: %v", err)
		}
	}

	// テストケース
	tests := []struct {
		name         string
		query        string
		serviceId    int64
		startFrom    int64
		startTo      int64
		channelType  int
		expectedIDs  []int64
		errorExpected bool
	}{
		{
			name:        "基本検索 - 'スポーツ'",
			query:       "スポーツ",
			channelType: 0,
			expectedIDs: []int64{2, 3},
		},
		{
			name:        "否定検索 - 'ドラマ -スポーツ'",
			query:       "ドラマ -スポーツ",
			channelType: 0,
			expectedIDs: []int64{},
		},
		{
			name:        "否定検索 - '-スポーツ'",
			query:       "-スポーツ",
			channelType: 0,
			expectedIDs: []int64{1, 4, 5},
		},
		{
			name:        "複合否定検索 - '-スポーツ -ニュース'",
			query:       "-スポーツ -ニュース",
			channelType: 0,
			expectedIDs: []int64{4, 5},
		},
		{
			name:        "サービスIDによるフィルタリング",
			query:       "",
			serviceId:   101,
			channelType: 0,
			expectedIDs: []int64{1, 3},
		},
		{
			name:        "時間によるフィルタリング",
			query:       "",
			startFrom:   1100,
			channelType: 0,
			expectedIDs: []int64{4, 5},
		},
		{
			name:        "複合フィルタリング - '映画' with serviceId",
			query:       "映画",
			serviceId:   102,
			channelType: 0,
			expectedIDs: []int64{5},
		},
		{
			name:        "複合フィルタリング - '-ニュース -スポーツ' with serviceId",
			query:       "-ニュース -スポーツ",
			serviceId:   102,
			channelType: 0,
			expectedIDs: []int64{5},
		},
	}

	// テストの実行
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			programs, err := SearchPrograms(db, tc.query, tc.serviceId, tc.startFrom, tc.startTo, tc.channelType)
			
			if tc.errorExpected {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}
			
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			
			// 結果の確認
			if len(programs) != len(tc.expectedIDs) {
				t.Errorf("Expected %d programs, got %d", len(tc.expectedIDs), len(programs))
			}
			
			// IDの確認
			foundIDs := make(map[int64]bool)
			for _, p := range programs {
				foundIDs[p.ID] = true
			}
			
			for _, id := range tc.expectedIDs {
				if !foundIDs[id] {
					t.Errorf("Expected to find program with ID %d, but it was not in the results", id)
				}
			}
		})
	}
}