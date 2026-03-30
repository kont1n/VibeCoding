// Package logger configures the global structured logger (zap).
package logger

import (
	"context"
	"sync"
	"sync/atomic"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	initOnce    sync.Once
	initialized atomic.Bool
	log         atomic.Pointer[zap.Logger]
)

// Init инициализирует глобальный логгер с указанным уровнем и форматом.
func Init(levelStr string, asJSON bool) error {
	var initErr error
	initOnce.Do(func() {
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

		l, err := cfg.Build()
		if err != nil {
			initErr = err
			return
		}

		log.Store(l)
		initialized.Store(true)
	})
	return initErr
}

// Logger возвращает глобальный экземпляр логгера.
// Если логгер не инициализирован, возвращает nop-логгер.
func Logger() *zap.Logger {
	if !initialized.Load() {
		return zap.NewNop()
	}
	if l := log.Load(); l != nil {
		return l
	}
	return zap.NewNop()
}

// Debug логирует сообщение уровня DEBUG.
func Debug(ctx context.Context, msg string, fields ...any) {
	if !initialized.Load() {
		return
	}
	if l := log.Load(); l != nil {
		l.Debug(msg, fieldsToZapFields(fields)...)
	}
}

// Info логирует сообщение уровня INFO.
func Info(ctx context.Context, msg string, fields ...any) {
	if !initialized.Load() {
		return
	}
	if l := log.Load(); l != nil {
		l.Info(msg, fieldsToZapFields(fields)...)
	}
}

// Warn логирует сообщение уровня WARN.
func Warn(ctx context.Context, msg string, fields ...any) {
	if !initialized.Load() {
		return
	}
	if l := log.Load(); l != nil {
		l.Warn(msg, fieldsToZapFields(fields)...)
	}
}

// Error логирует сообщение уровня ERROR.
func Error(ctx context.Context, msg string, fields ...any) {
	if !initialized.Load() {
		return
	}
	if l := log.Load(); l != nil {
		l.Error(msg, fieldsToZapFields(fields)...)
	}
}

// fieldsToZapFields преобразует интерфейс keys/values в zap.Field.
func fieldsToZapFields(fields []any) []zap.Field {
	zapFields := make([]zap.Field, 0, len(fields))

	// Поддерживаем оба формата:
	// 1) logger.Info(ctx, msg, "key", value)
	// 2) logger.Info(ctx, msg, zap.String("key", "value"), zap.Error(err), ...)
	for i := 0; i < len(fields); {
		switch v := fields[i].(type) {
		case zap.Field:
			zapFields = append(zapFields, v)
			i++
		case string:
			if i+1 < len(fields) {
				zapFields = append(zapFields, zap.Any(v, fields[i+1]))
			}
			i += 2
		default:
			// Ignore unknown types to keep logger calls safe.
			i++
		}
	}

	return zapFields
}

// Sync синхронизирует логгер (вызывается при shutdown).
func Sync() error {
	if l := log.Load(); l != nil {
		return l.Sync()
	}
	return nil
}
