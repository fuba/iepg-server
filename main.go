// main.go
package main

import (
	"context"
	"log"
	"net/http"
	"os"

	_ "github.com/mattn/go-sqlite3"

	"github.com/fuba/iepg-server/db"
	"github.com/fuba/iepg-server/handlers"
	"github.com/fuba/iepg-server/models"
)

func main() {
	// ログレベルの設定
	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = "info" // デフォルトはinfo
	}
	models.InitLogger(logLevel)
	models.Log.Debug("Starting iepg-server with log level: %s", logLevel)

	// DB_PATH環境変数、またはデフォルト値を使用
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "./data/programs.db"
	}
	models.Log.Debug("Using database path: %s", dbPath)

	dbConn, err := db.InitDB(dbPath)
	if err != nil {
		models.Log.Error("Failed to initialize database: %v", err)
		log.Fatal(err)
	}
	defer dbConn.Close()
	models.Log.Info("Database initialized successfully")

	// MirakurunのベースURL
	mirakurunURL := os.Getenv("MIRAKURUN_URL")
	if mirakurunURL == "" {
		mirakurunURL = "http://localhost:40772/api"
	}
	models.Log.Debug("Using Mirakurun URL: %s", mirakurunURL)

	// 起動時にMirakurunから全番組情報を取得してDBを再構築する
	ctx := context.Background()
	skipInitialLoad := os.Getenv("SKIP_INITIAL_LOAD")
	if skipInitialLoad != "1" && skipInitialLoad != "true" {
		models.Log.Info("Starting initial program data load from Mirakurun API...")
		if err := db.InitProgramsFromAPI(ctx, dbConn, mirakurunURL); err != nil {
			models.Log.Error("Failed to load initial program data: %v", err)
			// 初期ロードが失敗しても継続する
		} else {
			models.Log.Info("Initial program data loaded successfully")
		}
	} else {
		models.Log.Info("Skipping initial program data load (SKIP_INITIAL_LOAD=%s)", skipInitialLoad)
	}

	// ストリーム購読URL
	streamURL := mirakurunURL
	if !os.IsPathSeparator(streamURL[len(streamURL)-1]) {
		streamURL += "/"
	}
	streamURL += "events/stream?resource=program"
	models.Log.Debug("Using stream URL: %s", streamURL)

	// ストリーム購読開始（無限リトライ）
	models.Log.Info("Starting stream fetcher...")
	go db.StartStreamFetcher(ctx, dbConn, streamURL)
	
	// サービス情報の取得開始
	models.Log.Info("Starting service fetcher...")
	go db.StartServiceFetcher(ctx, mirakurunURL)
	
	// サービスイベントストリームの購読開始
	models.Log.Info("Starting service event stream...")
	go db.StartServiceEventStream(ctx, mirakurunURL)

	// 定期クリーンアップ処理開始
	cleanupEnabledStr := os.Getenv("ENABLE_CLEANUP")
	cleanupEnabled := true // デフォルトは有効
	if cleanupEnabledStr == "0" || cleanupEnabledStr == "false" {
		cleanupEnabled = false
	}

	if cleanupEnabled {
		models.Log.Info("Starting cleanup routine...")
		go db.StartCleanupRoutine(dbConn)
	} else {
		models.Log.Info("Cleanup routine disabled (ENABLE_CLEANUP=%s)", cleanupEnabledStr)
	}

	// HTTPエンドポイントの設定
	http.HandleFunc("/search", func(w http.ResponseWriter, r *http.Request) {
		models.Log.Debug("Handling search request: %s", r.URL.String())
		handlers.HandleSimpleSearch(w, r, dbConn)
	})
	http.HandleFunc("/services", func(w http.ResponseWriter, r *http.Request) {
		models.Log.Debug("Handling services request: %s", r.URL.String())
		handlers.HandleGetServices(w, r, dbConn)
	})
	http.HandleFunc("/services/all", func(w http.ResponseWriter, r *http.Request) {
		models.Log.Debug("Handling all services request: %s", r.URL.String())
		handlers.HandleGetAllServices(w, r, dbConn)
	})
	http.HandleFunc("/services/searchable", func(w http.ResponseWriter, r *http.Request) {
		models.Log.Debug("Handling searchable services request: %s", r.URL.String())
		handlers.HandleGetSearchableServices(w, r, dbConn)
	})
	http.HandleFunc("/services/excluded", func(w http.ResponseWriter, r *http.Request) {
		models.Log.Debug("Handling excluded services request: %s", r.URL.String())
		handlers.HandleGetExcludedServices(w, r, dbConn)
	})
	http.HandleFunc("/services/exclude", func(w http.ResponseWriter, r *http.Request) {
		models.Log.Debug("Handling add excluded service request: %s", r.URL.String())
		handlers.HandleAddExcludedService(w, r, dbConn)
	})
	http.HandleFunc("/services/unexclude", func(w http.ResponseWriter, r *http.Request) {
		models.Log.Debug("Handling remove excluded service request: %s", r.URL.String())
		handlers.HandleRemoveExcludedService(w, r, dbConn)
	})
	http.HandleFunc("/program/", func(w http.ResponseWriter, r *http.Request) {
		models.Log.Debug("Handling IEPG request: %s", r.URL.String())
		handlers.HandleIEPG(w, r, dbConn)
	})
	http.Handle("/rpc", handlers.NewRPCServer(dbConn))

	// 静的ファイルの提供
	fs := http.FileServer(http.Dir("./static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	// Web UI のルート
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			models.Log.Debug("Redirecting root path to search UI")
			http.Redirect(w, r, "/ui/search", http.StatusFound)
			return
		}
		http.NotFound(w, r)
	})

	// 検索UI
	http.HandleFunc("/ui/search", func(w http.ResponseWriter, r *http.Request) {
		models.Log.Debug("Serving search UI")
		http.ServeFile(w, r, "./static/search.html")
	})

	// チャンネル除外設定UI
	http.HandleFunc("/ui/exclude-channels", func(w http.ResponseWriter, r *http.Request) {
		models.Log.Debug("Serving exclude channels UI")
		http.ServeFile(w, r, "./static/exclude-channels.html")
	})

	models.Log.Debug("HTTP endpoints registered")

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	models.Log.Info("Listening on :%s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		models.Log.Error("Server error: %v", err)
		log.Fatal(err)
	}
}