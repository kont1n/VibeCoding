package closer

import (
	"context"
	"sync"

	"go.uber.org/zap"
)

// CloseFunc определяет функцию для закрытия ресурса.
type CloseFunc func(context.Context) error

// Logger определяет интерфейс для логирования (чтобы избежать циклической зависимости).
type Logger interface {
	Error(msg string, fields ...interface{})
}

// zapLogger адаптирует zap.Logger к нашему интерфейсу Logger.
type zapLogger struct {
	log *zap.Logger
}

func (z *zapLogger) Error(msg string, fields ...interface{}) {
	z.log.Error(msg, fieldsToZapFields(fields)...)
}

func fieldsToZapFields(fields []interface{}) []zap.Field {
	zapFields := make([]zap.Field, 0, len(fields)/2)
	for i := 0; i < len(fields)-1; i += 2 {
		if key, ok := fields[i].(string); ok {
			zapFields = append(zapFields, zap.Any(key, fields[i+1]))
		}
	}
	return zapFields
}

var (
	mu           sync.Mutex
	closers      []CloseFunc
	namedClosers = make(map[string]CloseFunc)
	logger       Logger
	configured   bool
)

// Configure инициализирует closer с сигналами завершения (вызывается один раз в main).
func Configure(signals ...interface{}) {
	mu.Lock()
	defer mu.Unlock()
	configured = true
}

// SetLogger устанавливает логгер для вывода ошибок при закрытии ресурсов.
func SetLogger(l *zap.Logger) {
	mu.Lock()
	defer mu.Unlock()
	logger = &zapLogger{log: l}
}

// Add добавляет функцию закрытия в общий пул.
func Add(fn CloseFunc) {
	mu.Lock()
	defer mu.Unlock()
	closers = append(closers, fn)
}

// AddNamed добавляет именованную функцию закрытия (для отладки).
func AddNamed(name string, fn CloseFunc) {
	mu.Lock()
	defer mu.Unlock()
	namedClosers[name] = fn
}

// CloseAll закрывает все зарегистрированные ресурсы в обратном порядке.
func CloseAll(ctx context.Context) error {
	mu.Lock()
	defer mu.Unlock()

	var lastErr error

	// Закрытие именованных ресурсов
	for name, fn := range namedClosers {
		if err := fn(ctx); err != nil {
			if logger != nil {
				logger.Error("error closing named resource", "name", name, "error", err)
			}
			lastErr = err
		}
	}

	// Закрытие общих ресурсов (в обратном порядке)
	for i := len(closers) - 1; i >= 0; i-- {
		if err := closers[i](ctx); err != nil {
			lastErr = err
		}
	}

	return lastErr
}

// IsConfigured возвращает, был ли closer инициализирован.
func IsConfigured() bool {
	mu.Lock()
	defer mu.Unlock()
	return configured
}
