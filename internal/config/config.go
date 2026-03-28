package config

import (
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
}

// Load загружает конфигурацию из .env файла и переменных окружения.
func Load(path string) error {
	// Overload перезаписывает существующие переменные окружения
	err := godotenv.Overload(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	AppConfig = &Config{
		App:       env.NewAppConfig(),
		Models:    env.NewModelsConfig(),
		Extract:   env.NewExtractConfig(),
		Cluster:   env.NewClusterConfig(),
		Organizer: env.NewOrganizerConfig(),
		Web:       env.NewWebConfig(),
		Logger:    env.NewLoggerConfig(),
	}

	return nil
}
