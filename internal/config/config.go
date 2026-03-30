package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"

	"github.com/kont1n/face-grouper/internal/config/env"
)

var AppConfig *Config

// Config хранит конфигурацию приложения.
type Config struct {
	App       env.AppConfig
	Models    env.ModelsConfig
	Extract   env.ExtractConfig
	Cluster   env.ClusterConfig
	Organizer env.OrganizerConfig
	Web       env.WebConfig
	Logger    env.LoggerConfig
	Database  env.DatabaseConfig
	Redis     env.RedisConfig
}

// Load загружает конфигурацию из .env файла и переменных окружения.
func Load(path string) error {
	// Overload перезаписывает существующие переменные окружения.
	err := godotenv.Overload(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("load .env file: %w", err)
	}

	AppConfig = &Config{
		App:       env.NewAppConfig(),
		Models:    env.NewModelsConfig(),
		Extract:   env.NewExtractConfig(),
		Cluster:   env.NewClusterConfig(),
		Organizer: env.NewOrganizerConfig(),
		Web:       env.NewWebConfig(),
		Logger:    env.NewLoggerConfig(),
		Database:  env.NewDatabaseConfig(),
		Redis:     env.NewRedisConfig(),
	}

	// Валидация БД — опциональна, предупреждаем но не падаем.
	if err := AppConfig.Database.Validate(); err != nil {
		// Database is optional — continue without it.
		_ = err
	}

	return nil
}
