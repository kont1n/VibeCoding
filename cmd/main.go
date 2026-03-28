package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kont1n/face-grouper/internal/app"
	"github.com/kont1n/face-grouper/internal/config"
	"github.com/kont1n/face-grouper/platform/pkg/closer"
	"github.com/kont1n/face-grouper/platform/pkg/logger"
	"go.uber.org/zap"
)

const configPath = ".env"

func main() {
	// CLI флаги для переопределения .env
	viewOnly := flag.Bool("view", false, "only start web UI without processing")
	serve := flag.Bool("serve", false, "start web UI after processing")
	port := flag.Int("port", 8080, "web UI port")
	flag.Parse()

	// Загрузка конфигурации
	if err := config.Load(configPath); err != nil {
		panic(fmt.Errorf("failed to load config: %w", err))
	}

	// Переопределение из CLI
	if *serve {
		config.AppConfig.Web.Serve = true
	}
	if *viewOnly {
		config.AppConfig.Web.ViewOnly = true
	}
	if *port != 8080 {
		config.AppConfig.Web.Port = *port
	}

	// Контекст с сигналом завершения
	appCtx, appCancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer appCancel()
	defer gracefulShutdown()

	// Настройка closer
	closer.Configure(syscall.SIGINT, syscall.SIGTERM)

	// Создание приложения
	a, err := app.New(appCtx)
	if err != nil {
		logger.Error(appCtx, "failed to create app", zap.Error(err))
		return
	}

	// Запуск
	err = a.Run(appCtx, *viewOnly)
	if err != nil {
		logger.Error(appCtx, "app error", zap.Error(err))
		return
	}
}

func gracefulShutdown() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := closer.CloseAll(ctx); err != nil {
		logger.Error(ctx, "error during shutdown", zap.Error(err))
	}

	if err := logger.Sync(); err != nil {
		fmt.Fprintf(os.Stderr, "error syncing logger: %v\n", err)
	}
}
