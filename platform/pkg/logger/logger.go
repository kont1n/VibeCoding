package logger

import (
	"context"
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	log         *zap.Logger
	initMu      sync.Mutex
	initialized bool
)

// Init инициализирует глобальный логгер с указанным уровнем и форматом.
func Init(levelStr string, asJSON bool) error {
	initMu.Lock()
	defer initMu.Unlock()

	if initialized {
		return nil
	}

	level, err := zapcore.ParseLevel(levelStr)
	if err != nil {
		level = zapcore.InfoLevel
	}

	var cfg zap.Config
	if asJSON {
		cfg = zap.NewProductionConfig()
		cfg.Encoding = "json"
	} else {
		cfg = zap.NewDevelopmentConfig()
		cfg.Encoding = "console"
	}
	cfg.Level = zap.NewAtomicLevelAt(level)

	log, err = cfg.Build()
	if err != nil {
		return err
	}

	initialized = true
	return nil
}

// Logger возвращает глобальный экземпляр логгера.
// Если логгер не инициализирован, возвращает nop-логгер.
func Logger() *zap.Logger {
	initMu.Lock()
	defer initMu.Unlock()

	if !initialized || log == nil {
		return zap.NewNop()
	}
	return log
}

// Debug логирует сообщение уровня DEBUG.
func Debug(ctx context.Context, msg string, fields ...any) {
	if !initialized || log == nil {
		return
	}
	log.Debug(msg, fieldsToZapFields(fields)...)
}

// Info логирует сообщение уровня INFO.
func Info(ctx context.Context, msg string, fields ...any) {
	if !initialized || log == nil {
		return
	}
	log.Info(msg, fieldsToZapFields(fields)...)
}

// Warn логирует сообщение уровня WARN.
func Warn(ctx context.Context, msg string, fields ...any) {
	if !initialized || log == nil {
		return
	}
	log.Warn(msg, fieldsToZapFields(fields)...)
}

// Error логирует сообщение уровня ERROR.
func Error(ctx context.Context, msg string, fields ...any) {
	if !initialized || log == nil {
		return
	}
	log.Error(msg, fieldsToZapFields(fields)...)
}

// fieldsToZapFields преобразует интерфейс keys/values в zap.Field.
func fieldsToZapFields(fields []any) []zap.Field {
	zapFields := make([]zap.Field, 0, len(fields)/2)
	for i := 0; i < len(fields)-1; i += 2 {
		if key, ok := fields[i].(string); ok {
			zapFields = append(zapFields, zap.Any(key, fields[i+1]))
		}
	}
	return zapFields
}

// Sync синхронизирует логгер (вызывается при shutdown).
func Sync() error {
	initMu.Lock()
	defer initMu.Unlock()

	if log != nil {
		return log.Sync()
	}
	return nil
}
