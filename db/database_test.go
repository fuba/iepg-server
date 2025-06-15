package db

import (
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/fuba/iepg-server/models"
)

func init() {
	// テスト用にロガーを初期化
	models.InitLogger("debug")
}

func TestSearchPrograms(t *testing.T) {
	// テスト用のデータベースを初期化（InitDBを使用して完全なスキーマを作成）
	db, err := InitDB(":memory:")
	if err != nil {
		t.Fatalf("Failed to initialize test database: %v", err)
	}
	defer db.Close()

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
		{6, 104, 1200, 45, "特集ドキュメンタリー", "日本の伝統文化についての特集番組です。"},
		{7, 105, 1245, 30, "料理番組", "美味しい料理の作り方を紹介します。"},
		{8, 106, 1275, 60, "旅行特集", "日本全国の名所を巡る旅行番組。美味しい料理も紹介。"},
		{9, 107, 1335, 45, "音楽番組", "最新の音楽情報と人気アーティストの特集。"},
		{10, 108, 1380, 30, "報道特集", "今日の出来事を詳しく解説する特別報道番組。"},
	}

	// データの挿入
	for _, p := range testPrograms {
		nameForSearch := models.NormalizeForSearch(p.name)
		descForSearch := models.NormalizeForSearch(p.description)
		_, err = db.Exec(
			`INSERT INTO programs (id, serviceId, startAt, duration, name, description, nameForSearch, descForSearch,
			                       seriesId, seriesEpisode, seriesLastEpisode, seriesName, seriesRepeat, seriesPattern, seriesExpiresAt)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			p.id, p.serviceId, p.startAt, p.duration, p.name, p.description, nameForSearch, descForSearch,
			nil, nil, nil, nil, nil, nil, nil,
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
			expectedIDs: []int64{1, 4, 5, 6, 7, 8, 9, 10},
		},
		{
			name:        "複合否定検索 - '-スポーツ -ニュース'",
			query:       "-スポーツ -ニュース",
			channelType: 0,
			expectedIDs: []int64{4, 5, 6, 7, 8, 9, 10}, // 報道特集はニュースに近いが別単語
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
			expectedIDs: []int64{4, 5, 6, 7, 8, 9, 10},
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
		// フレーズ検索のテストケース
		{
			name:        "フレーズ検索 - '\"美味しい料理\"'",
			query:       "\"美味しい料理\"",
			channelType: 0,
			expectedIDs: []int64{7, 8},
		},
		{
			name:        "フレーズ検索 - '\"特集番組\"'",
			query:       "\"特集番組\"",
			channelType: 0,
			expectedIDs: []int64{6},
		},
		{
			name:        "フレーズ検索と通常検索の組み合わせ - '\"美味しい料理\" 旅行'",
			query:       "\"美味しい料理\" 旅行",
			channelType: 0,
			expectedIDs: []int64{8},
		},
		{
			name:        "スペース区切りAND検索 - 'ニュース 最新'",
			query:       "ニュース 最新",
			channelType: 0,
			expectedIDs: []int64{1}, // ID=1だけが両方の単語を含む
		},
		{
			name:        "スペース区切りAND検索 - '料理 日本'",
			query:       "料理 日本",
			channelType: 0,
			expectedIDs: []int64{8}, // ID=8だけが両方の単語を含む
		},
		{
			name:        "スペース区切りAND検索と否定検索の組み合わせ - '料理 日本 -旅行'",
			query:       "料理 日本 -旅行",
			channelType: 0,
			expectedIDs: []int64{}, // 料理と日本の両方を含むのはID=8だが、旅行も含むので除外される
		},
		{
			name:        "フレーズ検索と否定検索の組み合わせ - '\"美味しい料理\" -旅行'",
			query:       "\"美味しい料理\" -旅行",
			channelType: 0,
			expectedIDs: []int64{7},
		},
		{
			name:        "複数フレーズ検索 - '\"美味しい料理\" \"日本全国\"'",
			query:       "\"美味しい料理\" \"日本全国\"",
			channelType: 0,
			expectedIDs: []int64{8},
		},
		{
			name:        "フレーズ検索 - エスケープされていないフレーズ",
			query:       "\"最新の音楽\"",
			channelType: 0,
			expectedIDs: []int64{9},
		},
		{
			name:        "不完全なフレーズ検索 - 閉じクォートなし",
			query:       "\"特集",
			channelType: 0,
			expectedIDs: []int64{6, 8, 9, 10, 3}, // ID=3 も特集を含む説明文があるのでマッチする
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

				// デバッグ用に詳細情報を出力
				t.Logf("Test case: %s", tc.name)
				t.Logf("Query: %s", tc.query)
				t.Logf("Expected IDs: %v", tc.expectedIDs)

				// 見つかったIDのリスト
				foundIDList := make([]int64, 0, len(programs))
				for _, p := range programs {
					foundIDList = append(foundIDList, p.ID)
					t.Logf("Found program: ID=%d, Name=%s, Description=%s", p.ID, p.Name, p.Description)
				}
				t.Logf("Found IDs: %v", foundIDList)
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