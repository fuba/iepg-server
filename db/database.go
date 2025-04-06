// db/database.go
package db

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/fuba/iepg-server/models"
)

// InitDB は、programsテーブルを作成する。
func InitDB(dataSourceName string) (*sql.DB, error) {
	models.Log.Debug("InitDB: Connecting to database: %s", dataSourceName)
	db, err := sql.Open("sqlite3", dataSourceName)
	if err != nil {
		models.Log.Error("InitDB: Failed to open database: %v", err)
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
			descForSearch TEXT
		);
	`)
	if err != nil {
		models.Log.Error("InitDB: Failed to create programs table: %v", err)
		db.Close()
		return nil, err
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
						_, err = db.Exec(`
							INSERT OR REPLACE INTO programs
								(id, serviceId, startAt, duration, name, description, nameForSearch, descForSearch)
							VALUES (?, ?, ?, ?, ?, ?, ?, ?);
						`, p.ID, p.ServiceID, p.StartAt, p.Duration, p.Name, p.Description, p.NameForSearch, p.DescForSearch)

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
func SearchPrograms(db *sql.DB, q string, serviceId, startFrom, startTo int64) ([]models.Program, error) {
	models.Log.Debug("SearchPrograms: Query=%s, ServiceId=%d, StartFrom=%d, StartTo=%d",
		q, serviceId, startFrom, startTo)

	var args []interface{}
	var query string
	var conditions []string

	if q != "" {
		// 検索キーワードを正規化
		normalizedQ := models.NormalizeForSearch(q)
		
		// 正規化されたカラムを使って検索
		likePattern := "%" + normalizedQ + "%"
		query = `
			SELECT id, serviceId, startAt, duration, name, description 
			FROM programs 
			WHERE (nameForSearch LIKE ? OR descForSearch LIKE ?)
		`
		args = append(args, likePattern, likePattern)
		models.Log.Debug("SearchPrograms: Using normalized LIKE search with pattern: %s", likePattern)
	} else {
		query = "SELECT id, serviceId, startAt, duration, name, description FROM programs"
		models.Log.Debug("SearchPrograms: Using regular query without search terms")
	}

	if serviceId != 0 {
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

	rows, err := db.Query(query, args...)
	if err != nil {
		models.Log.Error("SearchPrograms: Query error: %v", err)
		return nil, err
	}
	defer rows.Close()

	var programs []models.Program
	count := 0

	for rows.Next() {
		var p models.Program
		if err := rows.Scan(&p.ID, &p.ServiceID, &p.StartAt, &p.Duration, &p.Name, &p.Description); err != nil {
			models.Log.Error("SearchPrograms: Scan error: %v", err)
			return nil, err
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
	err := db.QueryRow("SELECT id, serviceId, startAt, duration, name, description FROM programs WHERE id = ?", id).
		Scan(&p.ID, &p.ServiceID, &p.StartAt, &p.Duration, &p.Name, &p.Description)

	if err != nil {
		if err == sql.ErrNoRows {
			models.Log.Info("GetProgramByID: Program not found with ID: %d", id)
		} else {
			models.Log.Error("GetProgramByID: Query error: %v", err)
		}
		return nil, err
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
func GetFilteredServices(allowedTypes []int, excludedTypes []int) []*models.Service {
	models.Log.Debug("GetFilteredServices: Retrieving services with types: %v, excluding: %v", allowedTypes, excludedTypes)
	allServices := models.ServiceMapInstance.GetAll()
	
	// 除外タイプのマップを作成
	excludeMap := make(map[int]bool)
	for _, t := range excludedTypes {
		excludeMap[t] = true
	}
	
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
		INSERT INTO programs (id, serviceId, startAt, duration, name, description, nameForSearch, descForSearch)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?);
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
		
		_, err = stmtPrograms.Exec(p.ID, p.ServiceID, p.StartAt, p.Duration, p.Name, p.Description, p.NameForSearch, p.DescForSearch)
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