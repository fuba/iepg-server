// main.go
package main

import (
	"context"
	"log"
	"net/http"
	"os"

	_ "github.com/mattn/go-sqlite3"
	"github.com/gorilla/mux"

	"github.com/fuba/iepg-server/db"
	"github.com/fuba/iepg-server/handlers"
	"github.com/fuba/iepg-server/models"
	"github.com/fuba/iepg-server/services"
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

	// 録画サーバーのURLを環境変数から取得
	recorderURL := os.Getenv("RECORDER_URL")
	if recorderURL == "" {
		recorderURL = "http://localhost:37569" // デフォルト値
	}
	models.Log.Debug("Using recorder URL: %s", recorderURL)

	// 予約ハンドラーの初期化
	reservationHandler := handlers.NewReservationHandler(dbConn, recorderURL)

	// 自動予約エンジンの初期化と開始
	autoReservationEngine := services.NewAutoReservationEngine(dbConn, recorderURL)
	autoReservationEnabledStr := os.Getenv("ENABLE_AUTO_RESERVATION")
	autoReservationEnabled := true // デフォルトは有効
	if autoReservationEnabledStr == "0" || autoReservationEnabledStr == "false" {
		autoReservationEnabled = false
	}

	if autoReservationEnabled {
		models.Log.Info("Starting auto reservation engine...")
		go autoReservationEngine.Start(ctx)
	} else {
		models.Log.Info("Auto reservation engine disabled (ENABLE_AUTO_RESERVATION=%s)", autoReservationEnabledStr)
	}

	// ルーターの設定
	router := mux.NewRouter()

	// HTTPエンドポイントの設定
	router.HandleFunc("/search", func(w http.ResponseWriter, r *http.Request) {
		models.Log.Debug("Handling search request: %s", r.URL.String())
		handlers.HandleSimpleSearch(w, r, dbConn)
	})
	router.HandleFunc("/services", func(w http.ResponseWriter, r *http.Request) {
		models.Log.Debug("Handling services request: %s", r.URL.String())
		handlers.HandleGetServices(w, r, dbConn)
	})
	router.HandleFunc("/services/all", func(w http.ResponseWriter, r *http.Request) {
		models.Log.Debug("Handling all services request: %s", r.URL.String())
		handlers.HandleGetAllServices(w, r, dbConn)
	})
	router.HandleFunc("/services/searchable", func(w http.ResponseWriter, r *http.Request) {
		models.Log.Debug("Handling searchable services request: %s", r.URL.String())
		handlers.HandleGetSearchableServices(w, r, dbConn)
	})
	router.HandleFunc("/services/excluded", func(w http.ResponseWriter, r *http.Request) {
		models.Log.Debug("Handling excluded services request: %s", r.URL.String())
		handlers.HandleGetExcludedServices(w, r, dbConn)
	})
	router.HandleFunc("/services/exclude", func(w http.ResponseWriter, r *http.Request) {
		models.Log.Debug("Handling add excluded service request: %s", r.URL.String())
		handlers.HandleAddExcludedService(w, r, dbConn)
	})
	router.HandleFunc("/services/unexclude", func(w http.ResponseWriter, r *http.Request) {
		models.Log.Debug("Handling remove excluded service request: %s", r.URL.String())
		handlers.HandleRemoveExcludedService(w, r, dbConn)
	})
	router.PathPrefix("/program/").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		models.Log.Debug("Handling IEPG request: %s", r.URL.String())
		handlers.HandleIEPG(w, r, dbConn)
	})
	router.Handle("/rpc", handlers.NewRPCServer(dbConn))

	// 予約関連のエンドポイント
	router.HandleFunc("/reservations", reservationHandler.CreateReservation).Methods("POST")
	router.HandleFunc("/reservations", reservationHandler.GetReservations).Methods("GET")
	router.HandleFunc("/reservations/{id}", reservationHandler.DeleteReservation).Methods("DELETE")

	// 自動予約関連のエンドポイント
	router.HandleFunc("/auto-reservations/rules", handlers.HandleCreateAutoReservationRule(dbConn)).Methods("POST")
	router.HandleFunc("/auto-reservations/rules", handlers.HandleGetAutoReservationRules(dbConn)).Methods("GET")
	router.HandleFunc("/auto-reservations/rules/{id}", handlers.HandleGetAutoReservationRule(dbConn)).Methods("GET")
	router.HandleFunc("/auto-reservations/rules/{id}", handlers.HandleUpdateAutoReservationRule(dbConn)).Methods("PUT")
	router.HandleFunc("/auto-reservations/rules/{id}", handlers.HandleDeleteAutoReservationRule(dbConn)).Methods("DELETE")
	router.HandleFunc("/auto-reservations/logs", handlers.HandleGetAutoReservationLogs(dbConn)).Methods("GET")

	// 静的ファイルの提供
	fs := http.FileServer(http.Dir("./static"))
	router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", fs))

	// Web UI のルート
	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			models.Log.Debug("Redirecting root path to search UI")
			http.Redirect(w, r, "/ui/search", http.StatusFound)
			return
		}
		http.NotFound(w, r)
	})

	// 検索UI
	router.HandleFunc("/ui/search", func(w http.ResponseWriter, r *http.Request) {
		models.Log.Debug("Serving search UI")
		http.ServeFile(w, r, "./static/search.html")
	})

	// チャンネル除外設定UI
	router.HandleFunc("/ui/exclude-channels", func(w http.ResponseWriter, r *http.Request) {
		models.Log.Debug("Serving exclude channels UI")
		http.ServeFile(w, r, "./static/exclude-channels.html")
	})

	models.Log.Debug("HTTP endpoints registered")

	port := os.Getenv("PORT")
	if port == "" {
		port = "40870" // default port when PORT is unset
	}
	models.Log.Info("Listening on :%s", port)
	if err := http.ListenAndServe(":"+port, router); err != nil {
		models.Log.Error("Server error: %v", err)
		log.Fatal(err)
	}
}
