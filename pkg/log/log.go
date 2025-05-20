// Package log 提供日志功能
package log

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/rs/zerolog"
)

var (
	// DefaultLogger 是全局的日志实例.
	//nolint:gochecknoglobals // 使用全局日志实例是必要的，因为需要在程序的不同部分记录日志
	DefaultLogger *Logger
)

// Logger 提供日志功能.
type Logger struct {
	logger zerolog.Logger
}

// New 创建一个新的日志实例.
func New(logPath string) (*Logger, error) {
	// 创建日志目录
	if err := os.MkdirAll(logPath, 0750); err != nil {
		return nil, fmt.Errorf("创建日志目录失败: %w", err)
	}

	// 创建日志文件
	logFile, err := os.OpenFile(
		filepath.Join(logPath, time.Now().Format("2006-01-02")+".log"),
		os.O_CREATE|os.O_WRONLY|os.O_APPEND,
		0600,
	)
	if err != nil {
		return nil, fmt.Errorf("创建日志文件失败: %w", err)
	}

	// 配置日志输出
	logger := zerolog.New(logFile).With().Timestamp().Logger()
	DefaultLogger = &Logger{logger: logger}
	return DefaultLogger, nil
}

// Error 记录错误日志.
func (l *Logger) Error() *zerolog.Event {
	return l.logger.Error()
}

// Info 记录信息日志.
func (l *Logger) Info() *zerolog.Event {
	return l.logger.Info()
}

// Warn 记录警告日志.
func (l *Logger) Warn() *zerolog.Event {
	return l.logger.Warn()
}

// Debug 记录调试日志.
func (l *Logger) Debug() *zerolog.Event {
	return l.logger.Debug()
}
