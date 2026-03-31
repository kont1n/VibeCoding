// Package main is the entrypoint for the face-grouper application.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/kont1n/face-grouper/internal/app"
	"github.com/kont1n/face-grouper/internal/config"
	"github.com/kont1n/face-grouper/platform/pkg/closer"
	"github.com/kont1n/face-grouper/platform/pkg/logger"
)

const configPath = ".env"

const (
	shutdownTimeout   = 30 * time.Second
	loggerSyncTimeout = 5 * time.Second
)

func main() {
	// CLI флаги для переопределения .env.
	viewOnly := flag.Bool("view", false, "only start web UI without processing")
	serve := flag.Bool("serve", false, "start web UI after processing")
	process := flag.Bool("process", false, "run processing pipeline from INPUT_DIR (without this flag, only web UI starts)")
	port := flag.Int("port", 8080, "web UI port")
	flag.Parse()

	// Загрузка конфигурации.
	if err := config.Load(configPath); err != nil {
		panic(fmt.Errorf("failed to load config: %w", err))
	}

	// Переопределение из CLI.
	if *serve {
		config.AppConfig.Web.Serve = true
	}
	if *viewOnly {
		config.AppConfig.Web.ViewOnly = true
	}
	if *port != 8080 {
		config.AppConfig.Web.Port = *port
	}

	// По умолчанию (без флагов) запускаем только веб-UI.
	// Обработка запускается только через --process или --serve.
	if !*process && !*serve && !*viewOnly {
		config.AppConfig.Web.ViewOnly = true
	}

	// Контекст с сигналом завершения.
	appCtx, appCancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)

	// Настройка closer.
	closer.Configure(syscall.SIGINT, syscall.SIGTERM)

	// Создание приложения.
	a, err := app.New(appCtx)
	if err != nil {
		logger.Error(appCtx, "failed to create app", zap.Error(err))
		appCancel()
		return
	}

	// Запуск.
	err = a.Run(appCtx, *viewOnly)
	if err != nil {
		logger.Error(appCtx, "app error", zap.Error(err))
		appCancel()
		return
	}

	// Graceful shutdown.
	gracefulShutdown(appCancel)
}

// gracefulShutdown выполняет корректное завершение работы приложения.
func gracefulShutdown(appCancel context.CancelFunc) {
	logger.Info(context.Background(), "starting graceful shutdown")
	defer appCancel()

	// Создаём контекст с таймаутом для shutdown.
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer shutdownCancel()

	// Закрываем все зарегистрированные ресурсы.
	if err := closer.CloseAll(shutdownCtx); err != nil {
		logger.Error(shutdownCtx, "error during shutdown", zap.Error(err))
	}

	// Синхронизируем логгер с отдельным таймаутом.
	syncCtx, syncCancel := context.WithTimeout(context.Background(), loggerSyncTimeout)
	defer syncCancel()

	// Создаём канал для завершения синхронизации.
	done := make(chan struct{})
	go func() {
		defer close(done)
		if err := logger.Sync(); err != nil {
			fmt.Fprintf(os.Stderr, "error syncing logger: %v\n", err)
		}
	}()

	// Ждём завершения синхронизации или таймаута.
	select {
	case <-done:
		logger.Info(shutdownCtx, "graceful shutdown completed")
	case <-syncCtx.Done():
		fmt.Fprintf(os.Stderr, "logger sync timeout after %v\n", loggerSyncTimeout)
	case <-shutdownCtx.Done():
		fmt.Fprintf(os.Stderr, "shutdown timeout after %v\n", shutdownTimeout)
	}
}
