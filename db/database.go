// db/database.go
package db

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/fuba/iepg-server/models"
)

// InitDB は、programsテーブルと除外チャンネルテーブルを作成する。
func InitDB(dataSourceName string) (*sql.DB, error) {
	models.Log.Debug("InitDB: Connecting to database: %s", dataSourceName)
	db, err := sql.Open("sqlite3", dataSourceName)
	if err != nil {
		models.Log.Error("InitDB: Failed to open database: %v", err)
		return nil, err
	}

	// Enable foreign key constraints
	_, err = db.Exec("PRAGMA foreign_keys = ON")
	if err != nil {
		models.Log.Error("InitDB: Failed to enable foreign keys: %v", err)
		db.Close()
		return nil, err
	}

	models.Log.Debug("InitDB: Creating tables")
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
			descForSearch TEXT,
			seriesId      INTEGER,
			seriesEpisode INTEGER,
			seriesLastEpisode INTEGER,
			seriesName    TEXT,
			seriesRepeat  INTEGER,
			seriesPattern INTEGER,
			seriesExpiresAt INTEGER
		);
	`)
	if err != nil {
		models.Log.Error("InitDB: Failed to create programs table: %v", err)
		db.Close()
		return nil, err
	}

	// 除外チャンネルテーブルの作成
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS excluded_services (
			serviceId     INTEGER PRIMARY KEY,
			name          TEXT,
			createdAt     INTEGER
		);
	`)
	if err != nil {
		models.Log.Error("InitDB: Failed to create excluded_services table: %v", err)
		db.Close()
		return nil, err
	}

	// 予約テーブルの作成
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS reservations (
			id                TEXT PRIMARY KEY,
			programId         INTEGER NOT NULL,
			serviceId         INTEGER NOT NULL,
			name              TEXT NOT NULL,
			startAt           INTEGER NOT NULL,
			duration          INTEGER NOT NULL,
			recorderUrl       TEXT NOT NULL,
			recorderProgramId TEXT NOT NULL,
			status            TEXT NOT NULL,
			createdAt         INTEGER NOT NULL,
			updatedAt         INTEGER NOT NULL,
			error             TEXT
		);
	`)
	if err != nil {
		models.Log.Error("InitDB: Failed to create reservations table: %v", err)
		db.Close()
		return nil, err
	}

	// 自動予約ルールテーブルの作成
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS auto_reservation_rules (
			id          TEXT PRIMARY KEY,
			type        TEXT NOT NULL,
			name        TEXT NOT NULL,
			enabled     INTEGER NOT NULL DEFAULT 1,
			priority    INTEGER NOT NULL DEFAULT 0,
			recorderUrl TEXT NOT NULL,
			createdAt   INTEGER NOT NULL,
			updatedAt   INTEGER NOT NULL
		);
	`)
	if err != nil {
		models.Log.Error("InitDB: Failed to create auto_reservation_rules table: %v", err)
		db.Close()
		return nil, err
	}

	// キーワードルールテーブルの作成
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS keyword_rules (
			ruleId       TEXT NOT NULL,
			keywords     TEXT NOT NULL,
			genres       TEXT,
			serviceIds   TEXT,
			excludeWords TEXT,
			FOREIGN KEY (ruleId) REFERENCES auto_reservation_rules(id) ON DELETE CASCADE
		);
	`)
	if err != nil {
		models.Log.Error("InitDB: Failed to create keyword_rules table: %v", err)
		db.Close()
		return nil, err
	}

	// シリーズルールテーブルの作成
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS series_rules (
			ruleId      TEXT NOT NULL,
			seriesId    TEXT NOT NULL,
			programName TEXT,
			serviceId   INTEGER,
			FOREIGN KEY (ruleId) REFERENCES auto_reservation_rules(id) ON DELETE CASCADE
		);
	`)
	if err != nil {
		models.Log.Error("InitDB: Failed to create series_rules table: %v", err)
		db.Close()
		return nil, err
	}

	// 自動予約ログテーブルの作成
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS auto_reservation_logs (
			id            TEXT PRIMARY KEY,
			ruleId        TEXT NOT NULL,
			programId     INTEGER NOT NULL,
			reservationId TEXT,
			status        TEXT NOT NULL,
			reason        TEXT,
			createdAt     INTEGER NOT NULL,
			FOREIGN KEY (ruleId) REFERENCES auto_reservation_rules(id) ON DELETE CASCADE
		);
	`)
	if err != nil {
		models.Log.Error("InitDB: Failed to create auto_reservation_logs table: %v", err)
		db.Close()
		return nil, err
	}

	// インデックスの作成
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_reservations_programId ON reservations(programId);`)
	if err != nil {
		models.Log.Error("InitDB: Failed to create index on programId: %v", err)
	}
	
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_reservations_status ON reservations(status);`)
	if err != nil {
		models.Log.Error("InitDB: Failed to create index on status: %v", err)
	}

	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_programs_seriesId ON programs(seriesId);`)
	if err != nil {
		models.Log.Error("InitDB: Failed to create index on seriesId: %v", err)
	}

	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_auto_reservation_logs_ruleId ON auto_reservation_logs(ruleId);`)
	if err != nil {
		models.Log.Error("InitDB: Failed to create index on auto_reservation_logs.ruleId: %v", err)
	}

	models.Log.Debug("InitDB: Database initialization completed successfully")
	return db, nil
}

// ProgramEvent はMirakurunから受信するプログラムイベントの構造体
type ProgramEvent struct {
	Resource string         `json:"resource"`
	Type     string         `json:"type"`
	Data     models.Program `json:"data"`
	Time     int64          `json:"time"`
}

// StartStreamFetcher は Mirakurun の getProgramStream API を購読し、
// resourceがprogramのイベントを受信して DB に INSERT OR REPLACE する。
func StartStreamFetcher(ctx context.Context, db *sql.DB, apiURL string) {
	models.Log.Debug("StartStreamFetcher: Starting stream fetcher with URL: %s", apiURL)

	for {
		err := func() error {
			models.Log.Debug("StreamFetcher: Creating new request to Mirakurun")
			req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
			if err != nil {
				models.Log.Error("StreamFetcher: Failed to create request: %v", err)
				return err
			}

			models.Log.Debug("StreamFetcher: Sending request to Mirakurun")
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				models.Log.Error("StreamFetcher: Request failed: %v", err)
				return err
			}
			defer resp.Body.Close()

			models.Log.Info("StreamFetcher: Connected to Mirakurun stream, status: %s", resp.Status)

			// JSONストリームからデータを読み込むための設定
			reader := bufio.NewReader(resp.Body)

			// 最初の'['を読み飛ばす
			firstChar, err := reader.ReadByte()
			if err != nil {
				models.Log.Error("StreamFetcher: Failed to read first byte: %v", err)
				return err
			}

			if firstChar != '[' {
				models.Log.Debug("StreamFetcher: First character is not '[', it's: %c", firstChar)
				// '['でなければバッファに戻す
				reader.UnreadByte()
			} else {
				models.Log.Debug("StreamFetcher: Found opening bracket '[' at stream start")
			}

			eventCount := 0

			// JSONオブジェクトを行単位で処理
			for {
				// 行を読み取る
				line, err := reader.ReadString('\n')
				if err != nil {
					if err == io.EOF {
						models.Log.Info("StreamFetcher: End of stream reached")
						break
					}
					models.Log.Error("StreamFetcher: Error reading line: %v", err)
					return err
				}

				// トリミングして先頭のカンマを除去
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}

				// カンマで始まっていれば除去
				if strings.HasPrefix(line, ",") {
					line = strings.TrimPrefix(line, ",")
				}

				// 終了ブラケットをスキップ
				if line == "]" {
					models.Log.Debug("StreamFetcher: Found closing bracket ']'")
					continue
				}

				// 空であれば除去
				if line == "" {
					models.Log.Debug("StreamFetcher: Empty line, skipping")
					continue
				}

				models.Log.Debug("StreamFetcher: Received JSON line: %s", line)

				// JSONをパース
				var event ProgramEvent
				if err := json.Unmarshal([]byte(line), &event); err != nil {
					models.Log.Error("StreamFetcher: JSON unmarshal error: %v, line: %s", err, line)
					continue
				}

				// プログラムイベントの処理
				if event.Resource == "program" {
					eventCount++
					p := event.Data

					models.Log.Debug("StreamFetcher: Processing program event: ID=%d, Name=%s, Type=%s",
						p.ID, p.Name, event.Type)

					// event.Type が 'remove' の場合は削除する
					if event.Type == "remove" {
						// プログラムをDBから削除
						_, err = db.Exec(`DELETE FROM programs WHERE id = ?`, p.ID)
						if err != nil {
							models.Log.Error("StreamFetcher: DB delete error: %v", err)
						} else {
							models.Log.Debug("StreamFetcher: Program deleted from database: ID=%d", p.ID)
						}
					} else {
						// 検索用に名前と説明を正規化
						p.NameForSearch = models.NormalizeForSearch(p.Name)
						p.DescForSearch = models.NormalizeForSearch(p.Description)
						
						// programs テーブルへ INSERT OR REPLACE
						var seriesId, seriesEpisode, seriesLastEpisode, seriesRepeat, seriesPattern interface{}
						var seriesName interface{}
						var seriesExpiresAt interface{}
						
						if p.Series != nil {
							seriesId = p.Series.ID
							seriesEpisode = p.Series.Episode
							seriesLastEpisode = p.Series.LastEpisode
							seriesName = p.Series.Name
							seriesRepeat = p.Series.Repeat
							seriesPattern = p.Series.Pattern
							seriesExpiresAt = p.Series.ExpiresAt
						}
						
						_, err = db.Exec(`
							INSERT OR REPLACE INTO programs
								(id, serviceId, startAt, duration, name, description, nameForSearch, descForSearch,
								 seriesId, seriesEpisode, seriesLastEpisode, seriesName, seriesRepeat, seriesPattern, seriesExpiresAt)
							VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);
						`, p.ID, p.ServiceID, p.StartAt, p.Duration, p.Name, p.Description, p.NameForSearch, p.DescForSearch,
						   seriesId, seriesEpisode, seriesLastEpisode, seriesName, seriesRepeat, seriesPattern, seriesExpiresAt)

						if err != nil {
							models.Log.Error("StreamFetcher: DB insert error: %v", err)
						} else {
							models.Log.Debug("StreamFetcher: Program inserted into database: ID=%d", p.ID)
						}
					}

					if eventCount%100 == 0 {
						models.Log.Info("StreamFetcher: Processed %d program events", eventCount)
					}
				} else {
					models.Log.Debug("StreamFetcher: Skipping non-program event: %s", event.Resource)
				}
			}

			models.Log.Info("StreamFetcher: Stream connection closed, processed %d events", eventCount)
			return nil
		}()

		if err != nil {
			models.Log.Error("StreamFetcher: Connection error: %v", err)
		}

		// 接続が切れたら一定間隔後に再接続（無限リトライ）
		retryDelay := 5 * time.Second
		models.Log.Info("StreamFetcher: Retrying connection in %v", retryDelay)
		time.Sleep(retryDelay)
	}
}

// StartCleanupRoutine は定期的に放送終了した番組をDBから削除する
func StartCleanupRoutine(db *sql.DB) {
	models.Log.Debug("StartCleanupRoutine: Starting cleanup routine")

	for {
		now := time.Now().UnixMilli()
		models.Log.Debug("CleanupRoutine: Running cleanup for programs ended before %d", now)

		result, err := db.Exec("DELETE FROM programs WHERE startAt + duration < ?", now)
		if err != nil {
			models.Log.Error("CleanupRoutine: Cleanup error: %v", err)
		} else {
			if rowsAffected, err := result.RowsAffected(); err == nil {
				models.Log.Info("CleanupRoutine: Deleted %d expired programs", rowsAffected)
			}
		}

		sleepDuration := 30 * time.Minute
		models.Log.Debug("CleanupRoutine: Next cleanup in %v", sleepDuration)
		time.Sleep(sleepDuration)
	}
}

// SearchPrograms は検索条件に一致する番組を取得する共通関数
func SearchPrograms(db *sql.DB, q string, serviceId, startFrom, startTo int64, channelType int) ([]models.Program, error) {
	models.Log.Debug("SearchPrograms: Query=%s, ServiceId=%d, StartFrom=%d, StartTo=%d, ChannelType=%d",
		q, serviceId, startFrom, startTo, channelType)

	var args []interface{}
	var query string
	var conditions []string
	// 放送種別でフィルタリングするためのサービスID一覧
	var serviceIDs []int64

	if q != "" {
		// クエリを解析して正負の検索条件に分ける
		positiveTermsMap := make(map[string]bool) // 正の検索語の重複を防ぐためのマップ
		positiveTerms := []string{}
		negativeTerms := []string{}
		phraseTerms := []string{}

		// ダブルクォーテーションで囲まれたフレーズを抽出する正規表現
		var currentPhrase strings.Builder
		var inPhrase bool
		var escapeNext bool

		// 文字ごとに処理してフレーズを抽出
		for i := 0; i < len(q); i++ {
			c := q[i]

			// エスケープ文字の処理
			if c == '\\' && !escapeNext {
				escapeNext = true
				continue
			}

			// ダブルクォートの処理
			if c == '"' && !escapeNext {
				if inPhrase {
					// フレーズの終了
					phraseStr := currentPhrase.String()
					if phraseStr != "" {
						phraseTerms = append(phraseTerms, models.NormalizeForSearch(phraseStr))
					}
					currentPhrase.Reset()
					inPhrase = false
				} else {
					// フレーズの開始
					inPhrase = true
				}
				continue
			}

			if inPhrase {
				// フレーズ内の文字を追加
				currentPhrase.WriteByte(c)
			}

			escapeNext = false
		}

		// 閉じていないフレーズがある場合は通常の単語として扱う
		if inPhrase && currentPhrase.Len() > 0 {
			// フレーズが閉じられていない場合、単語ごとに分割して通常の検索語として処理
			for _, word := range strings.Fields(currentPhrase.String()) {
				if word != "" {
					normalizedWord := models.NormalizeForSearch(word)
					positiveTermsMap[normalizedWord] = true
				}
			}
			currentPhrase.Reset()
			inPhrase = false  // フレーズ処理を終了してフラグをリセット
		}

		// ダブルクォーテーションの外にある通常の検索語を処理
		remainingQuery := q
		for {
			// フレーズを検出して除去
			startIdx := strings.Index(remainingQuery, "\"")
			if startIdx == -1 {
				break
			}

			// startIdxの前の部分を処理
			if startIdx > 0 {
				beforePhrase := remainingQuery[:startIdx]
				// 単語を分離
				for _, word := range strings.Fields(beforePhrase) {
					if strings.HasPrefix(word, "-") && len(word) > 1 {
						// NOT検索条件
						negTerm := word[1:] // "-"を除去
						negativeTerms = append(negativeTerms, models.NormalizeForSearch(negTerm))
					} else if word != "" {
						// 通常の検索条件
						normalizedWord := models.NormalizeForSearch(word)
						positiveTermsMap[normalizedWord] = true
					}
				}
			}

			// フレーズの終わりを検索
			endIdx := -1
			for i := startIdx + 1; i < len(remainingQuery); i++ {
				if remainingQuery[i] == '"' && (i == 0 || remainingQuery[i-1] != '\\') {
					endIdx = i
					break
				}
			}

			if endIdx == -1 {
				// 閉じクォートがない場合は残りを通常の検索として処理
				remainingPart := remainingQuery[startIdx+1:]
				for _, word := range strings.Fields(remainingPart) {
					if strings.HasPrefix(word, "-") && len(word) > 1 {
						negativeTerms = append(negativeTerms, models.NormalizeForSearch(word[1:]))
					} else if word != "" {
						normalizedWord := models.NormalizeForSearch(word)
						positiveTermsMap[normalizedWord] = true
					}
				}
				break
			}

			// 次の検索を残りの部分で行う
			remainingQuery = remainingQuery[endIdx+1:]
		}

		// 残りのクエリから単語を抽出
		if !strings.Contains(remainingQuery, "\"") {
			for _, word := range strings.Fields(remainingQuery) {
				if strings.HasPrefix(word, "-") && len(word) > 1 {
					// NOT検索条件
					negTerm := word[1:] // "-"を除去
					negativeTerms = append(negativeTerms, models.NormalizeForSearch(negTerm))
				} else if word != "" {
					// 通常の検索条件
					normalizedWord := models.NormalizeForSearch(word)
					positiveTermsMap[normalizedWord] = true
				}
			}
		}

		// クエリの基本部分を構築
		query = `
			SELECT id, serviceId, startAt, duration, name, description,
				   seriesId, seriesEpisode, seriesLastEpisode, seriesName, seriesRepeat, seriesPattern, seriesExpiresAt
			FROM programs
			WHERE 1=1
		`

		// フレーズ検索条件（厳密な照合）
		if len(phraseTerms) > 0 {
			phraseConditions := []string{}
			for _, phrase := range phraseTerms {
				// フレーズは完全一致ではなく、その語順で含まれるものを検索
				phraseConditions = append(phraseConditions, "(nameForSearch LIKE ? OR descForSearch LIKE ?)")
				args = append(args, "%"+phrase+"%", "%"+phrase+"%")
			}
			// 複数フレーズの場合はAND条件で連結（すべてのフレーズを含む必要がある）
			query += " AND (" + strings.Join(phraseConditions, " AND ") + ")"
			models.Log.Debug("SearchPrograms: Added phrase search conditions: %v", phraseTerms)
		}

		// マップからユニークな検索語のスライスを作成
		positiveTerms = []string{} // 一度クリアして再作成
		for term := range positiveTermsMap {
			positiveTerms = append(positiveTerms, term)
		}

		// 肯定的な検索条件（AND結合）
		if len(positiveTerms) > 0 {
			posConditions := []string{}
			for _, term := range positiveTerms {
				likePattern := "%" + term + "%"
				posConditions = append(posConditions, "(nameForSearch LIKE ? OR descForSearch LIKE ?)")
				args = append(args, likePattern, likePattern)
			}

			// 各単語の検索条件をAND結合する
			if len(phraseTerms) == 0 {
				// フレーズ検索がない場合は、検索条件全体を括弧でグループ化する
				// 例: AND ((condition1) AND (condition2))
				query += " AND (" + strings.Join(posConditions, " AND ") + ")"
			} else {
				// フレーズ検索がある場合は、既に外側に括弧があるため、内側の括弧だけ追加
				// 例: AND (phraseConditions) AND (condition1) AND (condition2)
				query += " AND " + strings.Join(posConditions, " AND ")
			}
			models.Log.Debug("SearchPrograms: Added positive search conditions: %v", positiveTerms)
		}

		// 否定的な検索条件（NOT LIKE）
		for _, term := range negativeTerms {
			likePattern := "%" + term + "%"
			query += " AND (nameForSearch NOT LIKE ? AND descForSearch NOT LIKE ?)"
			args = append(args, likePattern, likePattern)
		}
		models.Log.Debug("SearchPrograms: Added negative search conditions: %v", negativeTerms)
		models.Log.Debug("SearchPrograms: Query after all search conditions: %s", query)
	} else {
		query = `SELECT id, serviceId, startAt, duration, name, description,
				 seriesId, seriesEpisode, seriesLastEpisode, seriesName, seriesRepeat, seriesPattern, seriesExpiresAt
				 FROM programs`
		models.Log.Debug("SearchPrograms: Using regular query without search terms")
	}

	// 除外チャンネルのリストを取得
	excludedServiceIds := make(map[int64]bool)
	rows, err := db.Query("SELECT serviceId FROM excluded_services")
	if err != nil {
		models.Log.Error("SearchPrograms: Failed to get excluded services: %v", err)
	} else {
		defer rows.Close()
		for rows.Next() {
			var serviceId int64
			if err := rows.Scan(&serviceId); err != nil {
				models.Log.Error("SearchPrograms: Failed to scan excluded service: %v", err)
				continue
			}
			excludedServiceIds[serviceId] = true
		}
	}
	models.Log.Debug("SearchPrograms: Loaded %d excluded services", len(excludedServiceIds))

	// 放送種別でフィルタリング
	if channelType > 0 && channelType <= 3 {
		// 指定された放送種別に該当するサービスIDのリストを取得
		services := models.ServiceMapInstance.GetAll()
		models.Log.Debug("SearchPrograms: Filtering by channelType %d, total services: %d", channelType, len(services))
		
		// サービスリストの内容をデバッグ出力
		for _, service := range services {
			models.Log.Debug("ServiceMap contains: ServiceID=%d, Name=%s, Type=%d", 
				service.ServiceID, service.Name, service.Type)
		}
		
		for _, service := range services {
			// 除外リストにあるサービスIDはスキップ
			if excludedServiceIds[service.ServiceID] {
				models.Log.Debug("SearchPrograms: Skipping excluded service: %d (%s)", 
					service.ServiceID, service.Name)
				continue
			}
			
			if service.Type == channelType {
				serviceIDs = append(serviceIDs, service.ServiceID)
				models.Log.Debug("SearchPrograms: Added service to filter: ServiceID=%d, Name=%s, Type=%d", 
					service.ServiceID, service.Name, service.Type)
			}
		}
		
		if len(serviceIDs) > 0 {
			placeholders := make([]string, len(serviceIDs))
			for i := range serviceIDs {
				placeholders[i] = "?"
				args = append(args, serviceIDs[i])
			}
			conditions = append(conditions, "serviceId IN ("+strings.Join(placeholders, ",")+")") 
			models.Log.Debug("SearchPrograms: Adding channelType condition for type %d with %d services", 
				channelType, len(serviceIDs))
		} else {
			models.Log.Debug("SearchPrograms: No services found for channel type: %d, services may not be loaded yet", channelType)
		}
	} else if serviceId == 0 {
		// 特定のサービスIDが指定されていない場合は、除外チャンネルを反映
		if len(excludedServiceIds) > 0 {
			excludedIds := make([]string, 0, len(excludedServiceIds))
			for id := range excludedServiceIds {
				excludedIds = append(excludedIds, "?")
				args = append(args, id)
			}
			conditions = append(conditions, fmt.Sprintf("serviceId NOT IN (%s)", strings.Join(excludedIds, ",")))
			models.Log.Debug("SearchPrograms: Adding exclusion condition for %d services", len(excludedServiceIds))
		}
	}
	
	if serviceId != 0 {
		// 特定のサービスIDが指定されていれば、放送種別より優先
		conditions = append(conditions, "serviceId = ?")
		args = append(args, serviceId)
		models.Log.Debug("SearchPrograms: Adding serviceId condition: %d", serviceId)
	}
	if startFrom != 0 {
		conditions = append(conditions, "startAt >= ?")
		args = append(args, startFrom)
		models.Log.Debug("SearchPrograms: Adding startFrom condition: %d", startFrom)
	}
	if startTo != 0 {
		conditions = append(conditions, "startAt <= ?")
		args = append(args, startTo)
		models.Log.Debug("SearchPrograms: Adding startTo condition: %d", startTo)
	}

	if len(conditions) > 0 {
		if !strings.Contains(query, "WHERE") {
			query += " WHERE " + strings.Join(conditions, " AND ")
		} else {
			query += " AND " + strings.Join(conditions, " AND ")
		}
	}
	query += " ORDER BY startAt"

	models.Log.Debug("SearchPrograms: Final query: %s, Args: %v", query, args)

	rows, err = db.Query(query, args...)
	if err != nil {
		models.Log.Error("SearchPrograms: Query error: %v", err)
		return nil, err
	}
	defer rows.Close()

	var programs []models.Program
	count := 0

	for rows.Next() {
		var p models.Program
		var seriesId, seriesEpisode, seriesLastEpisode, seriesRepeat, seriesPattern sql.NullInt64
		var seriesName sql.NullString
		var seriesExpiresAt sql.NullInt64
		
		if err := rows.Scan(&p.ID, &p.ServiceID, &p.StartAt, &p.Duration, &p.Name, &p.Description,
			&seriesId, &seriesEpisode, &seriesLastEpisode, &seriesName, &seriesRepeat, &seriesPattern, &seriesExpiresAt); err != nil {
			models.Log.Error("SearchPrograms: Scan error: %v", err)
			return nil, err
		}
		
		// Build series information if available
		if seriesId.Valid {
			p.Series = &models.Series{
				ID:          int(seriesId.Int64),
				Episode:     int(seriesEpisode.Int64),
				LastEpisode: int(seriesLastEpisode.Int64),
				Name:        seriesName.String,
				Repeat:      int(seriesRepeat.Int64),
				Pattern:     int(seriesPattern.Int64),
				ExpiresAt:   seriesExpiresAt.Int64,
			}
		}
		
		programs = append(programs, p)
		count++

		models.Log.Debug("SearchPrograms: Found program: ID=%d, Name=%s", p.ID, p.Name)
	}

	models.Log.Info("SearchPrograms: Query returned %d programs", count)
	return programs, nil
}

// GetProgramByID は指定されたIDの番組を取得する
func GetProgramByID(db *sql.DB, id int64) (*models.Program, error) {
	models.Log.Debug("GetProgramByID: Looking up program with ID: %d", id)

	var p models.Program
	var seriesId, seriesEpisode, seriesLastEpisode, seriesRepeat, seriesPattern sql.NullInt64
	var seriesName sql.NullString
	var seriesExpiresAt sql.NullInt64
	
	err := db.QueryRow(`SELECT id, serviceId, startAt, duration, name, description,
						seriesId, seriesEpisode, seriesLastEpisode, seriesName, seriesRepeat, seriesPattern, seriesExpiresAt 
						FROM programs WHERE id = ?`, id).
		Scan(&p.ID, &p.ServiceID, &p.StartAt, &p.Duration, &p.Name, &p.Description,
			 &seriesId, &seriesEpisode, &seriesLastEpisode, &seriesName, &seriesRepeat, &seriesPattern, &seriesExpiresAt)

	if err != nil {
		if err == sql.ErrNoRows {
			models.Log.Info("GetProgramByID: Program not found with ID: %d", id)
		} else {
			models.Log.Error("GetProgramByID: Query error: %v", err)
		}
		return nil, err
	}

	// Build series information if available
	if seriesId.Valid {
		p.Series = &models.Series{
			ID:          int(seriesId.Int64),
			Episode:     int(seriesEpisode.Int64),
			LastEpisode: int(seriesLastEpisode.Int64),
			Name:        seriesName.String,
			Repeat:      int(seriesRepeat.Int64),
			Pattern:     int(seriesPattern.Int64),
			ExpiresAt:   seriesExpiresAt.Int64,
		}
	}

	models.Log.Debug("GetProgramByID: Found program: ID=%d, Name=%s, StartAt=%d",
		p.ID, p.Name, p.StartAt)
	return &p, nil
}

// GetAllServices はServiceMapに保存されているすべてのサービス情報を取得する
func GetAllServices() []*models.Service {
	models.Log.Debug("GetAllServices: Retrieving all services from ServiceMap")
	services := models.ServiceMapInstance.GetAll()
	models.Log.Info("GetAllServices: Retrieved %d services", len(services))
	return services
}

// GetFilteredServices は指定されたタイプのサービスを取得する
// allowedTypes が空の場合は、タイプ1,2,3のすべてのサービスを返す
// excludedTypes に指定されたタイプのサービスは除外される
// また、除外チャンネルテーブルに登録されているサービスも除外される
func GetFilteredServices(db *sql.DB, allowedTypes []int, excludedTypes []int) []*models.Service {
	models.Log.Debug("GetFilteredServices: Retrieving services with types: %v, excluding: %v", allowedTypes, excludedTypes)
	allServices := models.ServiceMapInstance.GetAll()
	
	// 除外タイプのマップを作成
	excludeMap := make(map[int]bool)
	for _, t := range excludedTypes {
		excludeMap[t] = true
	}
	
	// 除外チャンネルのマップを作成
	excludedServiceIds := make(map[int64]bool)
	if db != nil {
		rows, err := db.Query("SELECT serviceId FROM excluded_services")
		if err != nil {
			models.Log.Error("GetFilteredServices: Failed to get excluded services: %v", err)
		} else {
			defer rows.Close()
			for rows.Next() {
				var serviceId int64
				if err := rows.Scan(&serviceId); err != nil {
					models.Log.Error("GetFilteredServices: Failed to scan excluded service: %v", err)
					continue
				}
				excludedServiceIds[serviceId] = true
			}
		}
	}
	models.Log.Debug("GetFilteredServices: Loaded %d excluded services", len(excludedServiceIds))
	
	// allowedTypesが空の場合、デフォルトで1,2,3を許可
	var allowMap map[int]bool
	if len(allowedTypes) > 0 {
		allowMap = make(map[int]bool)
		for _, t := range allowedTypes {
			allowMap[t] = true
		}
	}
	
	var filteredServices []*models.Service
	for _, service := range allServices {
		// 除外リストにあるタイプはスキップ
		if excludeMap[service.Type] {
			continue
		}
		
		// 除外サービスIDリストにあるサービスはスキップ
		if excludedServiceIds[service.ServiceID] {
			models.Log.Debug("GetFilteredServices: Skipping excluded service: %d (%s)", service.ServiceID, service.Name)
			continue
		}
		
		// allowMapが設定されている場合は、許可されたタイプのみを含める
		if allowMap != nil {
			if allowMap[service.Type] {
				filteredServices = append(filteredServices, service)
			}
		} else {
			// allowMapが設定されていない場合は、デフォルトでタイプ1,2,3を含める
			if service.Type == 1 || service.Type == 2 || service.Type == 3 {
				filteredServices = append(filteredServices, service)
			}
		}
	}
	
	models.Log.Info("GetFilteredServices: Retrieved %d services after filtering out of %d total services", 
		len(filteredServices), len(allServices))
	return filteredServices
}

// GetExcludedServices は除外チャンネルの一覧を取得する
func GetExcludedServices(db *sql.DB) ([]models.ExcludedService, error) {
	models.Log.Debug("GetExcludedServices: Retrieving excluded services")
	
	rows, err := db.Query("SELECT serviceId, name, createdAt FROM excluded_services")
	if err != nil {
		models.Log.Error("GetExcludedServices: Failed to query: %v", err)
		return nil, err
	}
	defer rows.Close()
	
	var services []models.ExcludedService
	for rows.Next() {
		var s models.ExcludedService
		if err := rows.Scan(&s.ServiceID, &s.Name, &s.CreatedAt); err != nil {
			models.Log.Error("GetExcludedServices: Failed to scan: %v", err)
			return nil, err
		}
		
		// サービスマップから追加情報を取得
		if service, ok := models.ServiceMapInstance.Get(s.ServiceID); ok {
			s.Type = service.Type
			s.NetworkID = service.NetworkID
			s.RemoteControlKeyID = service.RemoteControlKeyID
			s.ChannelType = service.ChannelType
			s.ChannelNumber = service.ChannelNumber
			
			// もしDBから取得した名前が空または単にサービスIDだけなら、正確な名前を使用
			if s.Name == "" || s.Name == fmt.Sprintf("Service %d", s.ServiceID) {
				s.Name = service.Name
			}
			
			models.Log.Debug("GetExcludedServices: Enhanced service info for %d: Type=%d, RCKey=%d, ChannelType=%s", 
				s.ServiceID, s.Type, s.RemoteControlKeyID, s.ChannelType)
		} else {
			models.Log.Debug("GetExcludedServices: No service info found in ServiceMap for ID %d", s.ServiceID)
		}
		
		services = append(services, s)
	}
	
	if err := rows.Err(); err != nil {
		models.Log.Error("GetExcludedServices: Rows error: %v", err)
		return nil, err
	}
	
	// サービスIDでソート
	sort.Slice(services, func(i, j int) bool {
		return services[i].ServiceID < services[j].ServiceID
	})
	
	models.Log.Info("GetExcludedServices: Retrieved and enhanced %d excluded services", len(services))
	return services, nil
}

// AddExcludedService は除外チャンネルを追加する
func AddExcludedService(db *sql.DB, serviceId int64, name string) error {
	models.Log.Debug("AddExcludedService: Adding service %d (%s) to excluded list", serviceId, name)
	
	_, err := db.Exec(
		"INSERT OR REPLACE INTO excluded_services (serviceId, name, createdAt) VALUES (?, ?, ?)",
		serviceId, name, time.Now().UnixMilli(),
	)
	if err != nil {
		models.Log.Error("AddExcludedService: Failed to insert: %v", err)
		return err
	}
	
	models.Log.Info("AddExcludedService: Service %d (%s) added to excluded list", serviceId, name)
	return nil
}

// RemoveExcludedService は除外チャンネルを削除する
func RemoveExcludedService(db *sql.DB, serviceId int64) error {
	models.Log.Debug("RemoveExcludedService: Removing service %d from excluded list", serviceId)
	
	res, err := db.Exec("DELETE FROM excluded_services WHERE serviceId = ?", serviceId)
	if err != nil {
		models.Log.Error("RemoveExcludedService: Failed to delete: %v", err)
		return err
	}
	
	affected, _ := res.RowsAffected()
	models.Log.Info("RemoveExcludedService: %d service(s) removed from excluded list", affected)
	return nil
}

// InitProgramsFromAPI は、Mirakurunの/api/programsエンドポイントから
// すべての番組情報を取得し、データベースを初期化する
func InitProgramsFromAPI(ctx context.Context, db *sql.DB, mirakurunBaseURL string) error {
	apiURL := mirakurunBaseURL
	if !strings.HasPrefix(apiURL, "http") {
		apiURL = "http://" + apiURL
	}
	
	// URLが/apiで終わっていたら/programsを追加
	if strings.HasSuffix(apiURL, "/api") {
		apiURL += "/programs"
	} else if !strings.HasSuffix(apiURL, "/programs") {
		// URLが/api/programsで終わっていなければ調整
		if !strings.HasSuffix(apiURL, "/") {
			apiURL += "/"
		}
		if !strings.Contains(apiURL, "/api/") {
			apiURL += "api/programs"
		} else if !strings.Contains(apiURL, "/programs") {
			apiURL += "programs"
		}
	}
	
	models.Log.Debug("InitProgramsFromAPI: Constructed URL: %s from base URL: %s", apiURL, mirakurunBaseURL)

	models.Log.Info("InitProgramsFromAPI: Fetching all programs from: %s", apiURL)

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		models.Log.Error("InitProgramsFromAPI: Failed to create request: %v", err)
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		models.Log.Error("InitProgramsFromAPI: Request failed: %v", err)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		models.Log.Error("InitProgramsFromAPI: API returned non-OK status: %s", resp.Status)
		return err
	}

	models.Log.Info("InitProgramsFromAPI: Connected to Mirakurun API, status: %s", resp.Status)

	// レスポンス全体をメモリに読み込む
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		models.Log.Error("InitProgramsFromAPI: Failed to read response body: %v", err)
		return err
	}

	// JSONをパース
	var programs []models.Program
	if err := json.Unmarshal(bodyBytes, &programs); err != nil {
		models.Log.Error("InitProgramsFromAPI: JSON unmarshal error: %v", err)
		return err
	}

	models.Log.Info("InitProgramsFromAPI: Successfully parsed %d programs", len(programs))

	// トランザクションを開始
	tx, err := db.Begin()
	if err != nil {
		models.Log.Error("InitProgramsFromAPI: Failed to begin transaction: %v", err)
		return err
	}

	// 既存のテーブルを空にする
	_, err = tx.Exec("DELETE FROM programs")
	if err != nil {
		tx.Rollback()
		models.Log.Error("InitProgramsFromAPI: Failed to clear programs table: %v", err)
		return err
	}

	models.Log.Debug("InitProgramsFromAPI: Cleared existing tables")

	// バッチインサートのためのステートメント準備
	stmtPrograms, err := tx.Prepare(`
		INSERT INTO programs (id, serviceId, startAt, duration, name, description, nameForSearch, descForSearch,
							  seriesId, seriesEpisode, seriesLastEpisode, seriesName, seriesRepeat, seriesPattern, seriesExpiresAt)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);
	`)
	if err != nil {
		tx.Rollback()
		models.Log.Error("InitProgramsFromAPI: Failed to prepare programs insert statement: %v", err)
		return err
	}
	defer stmtPrograms.Close()

	// すべてのプログラムを挿入
	for i, p := range programs {
		// 検索用に名前と説明を正規化
		p.NameForSearch = models.NormalizeForSearch(p.Name)
		p.DescForSearch = models.NormalizeForSearch(p.Description)
		
		// Series information
		var seriesId, seriesEpisode, seriesLastEpisode, seriesRepeat, seriesPattern interface{}
		var seriesName interface{}
		var seriesExpiresAt interface{}
		
		if p.Series != nil {
			seriesId = p.Series.ID
			seriesEpisode = p.Series.Episode
			seriesLastEpisode = p.Series.LastEpisode
			seriesName = p.Series.Name
			seriesRepeat = p.Series.Repeat
			seriesPattern = p.Series.Pattern
			seriesExpiresAt = p.Series.ExpiresAt
		}
		
		_, err = stmtPrograms.Exec(p.ID, p.ServiceID, p.StartAt, p.Duration, p.Name, p.Description, p.NameForSearch, p.DescForSearch,
								  seriesId, seriesEpisode, seriesLastEpisode, seriesName, seriesRepeat, seriesPattern, seriesExpiresAt)
		if err != nil {
			tx.Rollback()
			models.Log.Error("InitProgramsFromAPI: Failed to insert program %d: %v", p.ID, err)
			return err
		}

		if (i+1)%500 == 0 || i+1 == len(programs) {
			models.Log.Info("InitProgramsFromAPI: Inserted %d/%d programs", i+1, len(programs))
		}
	}

	// トランザクションをコミット
	if err := tx.Commit(); err != nil {
		models.Log.Error("InitProgramsFromAPI: Failed to commit transaction: %v", err)
		return err
	}

	models.Log.Info("InitProgramsFromAPI: Successfully initialized database with %d programs", len(programs))
	return nil
}