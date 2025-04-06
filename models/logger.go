// models/logger.go
package models

import (
	"fmt"
	"log"
	"os"
)

// LogLevel はログレベルを表す型
type LogLevel int

const (
	// LogLevelError はエラーログのみを出力
	LogLevelError LogLevel = iota
	// LogLevelInfo は情報ログとエラーログを出力
	LogLevelInfo
	// LogLevelDebug はデバッグログ、情報ログ、エラーログを出力
	LogLevelDebug
)

// Logger はロギング機能を提供する構造体
type Logger struct {
	level  LogLevel
	logger *log.Logger
}

// NewLogger は新しいロガーインスタンスを作成
func NewLogger(level LogLevel) *Logger {
	return &Logger{
		level:  level,
		logger: log.New(os.Stdout, "", log.LstdFlags),
	}
}

// SetLevel はログレベルを設定
func (l *Logger) SetLevel(level LogLevel) {
	l.level = level
}

// Error はエラーレベルのログを出力
func (l *Logger) Error(format string, v ...interface{}) {
	l.logger.Printf("[ERROR] "+format, v...)
}

// Info は情報レベルのログを出力
func (l *Logger) Info(format string, v ...interface{}) {
	if l.level >= LogLevelInfo {
		l.logger.Printf("[INFO] "+format, v...)
	}
}

// Debug はデバッグレベルのログを出力
func (l *Logger) Debug(format string, v ...interface{}) {
	if l.level >= LogLevelDebug {
		l.logger.Printf("[DEBUG] "+format, v...)
	}
}

// Global logger instance
var Log *Logger

// InitLogger はグローバルロガーを初期化
func InitLogger(levelStr string) {
	level := LogLevelInfo // デフォルトはInfo

	switch levelStr {
	case "error":
		level = LogLevelError
	case "info":
		level = LogLevelInfo
	case "debug":
		level = LogLevelDebug
	}

	Log = NewLogger(level)
	Log.Debug("Logger initialized with level: %s", levelStr)
}

// GetLogLevelFromString は文字列からLogLevelを取得
func GetLogLevelFromString(levelStr string) LogLevel {
	switch levelStr {
	case "error":
		return LogLevelError
	case "info":
		return LogLevelInfo
	case "debug":
		return LogLevelDebug
	default:
		fmt.Printf("Unknown log level: %s, defaulting to info\n", levelStr)
		return LogLevelInfo
	}
}