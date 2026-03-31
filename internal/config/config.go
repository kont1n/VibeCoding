// Package config provides application configuration loading and management.
package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"

	"github.com/kont1n/face-grouper/internal/config/env"
)

// AppConfig is the global application configuration instance.
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

// Load loads configuration from .env file and environment variables.
func Load(path string) error {
	// godotenv.Load() only sets missing environment variables.
	// This preserves variables set via Docker/Compose.
	err := godotenv.Load(path)
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

	// Database configuration validation.
	if err := AppConfig.Database.Validate(); err != nil {
		return fmt.Errorf("validate database config: %w", err)
	}

	return nil
}
