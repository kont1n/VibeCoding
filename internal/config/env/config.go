package env

import (
	"os"
	"strconv"
)

// AppConfig хранит основные настройки приложения.
type AppConfig struct {
	InputDir  string
	OutputDir string
}

// ModelsConfig хранит настройки моделей.
type ModelsConfig struct {
	Dir string
}

// ExtractConfig хранит настройки извлечения эмбеддингов.
type ExtractConfig struct {
	Workers        int
	GPU            bool
	GPUDetSessions int
	GPURecSessions int
	EmbedBatchSize int
	EmbedFlushMS   int
	MaxDim         int
	DetThresh      float64
}

// ClusterConfig хранит настройки кластеризации.
type ClusterConfig struct {
	Threshold float64
}

// OrganizerConfig хранит настройки организации результатов.
type OrganizerConfig struct {
	AvatarUpdateThreshold float64
}

// WebConfig хранит настройки веб-сервера.
type WebConfig struct {
	Port     int
	Serve    bool
	ViewOnly bool
}

// LoggerConfig хранит настройки логирования.
type LoggerConfig struct {
	Level  string
	AsJSON bool
}

// NewAppConfig создаёт конфигурацию приложения из ENV.
func NewAppConfig() AppConfig {
	return AppConfig{
		InputDir:  getEnv("INPUT_DIR", "./dataset"),
		OutputDir: getEnv("OUTPUT_DIR", "./output"),
	}
}

// NewModelsConfig создаёт конфигурацию моделей из ENV.
func NewModelsConfig() ModelsConfig {
	return ModelsConfig{
		Dir: getEnv("MODELS_DIR", "./models"),
	}
}

// NewExtractConfig создаёт конфигурацию экстракции из ENV.
func NewExtractConfig() ExtractConfig {
	return ExtractConfig{
		Workers:        getInt("EXTRACT_WORKERS", 4),
		GPU:            getBool("GPU_ENABLED", false),
		GPUDetSessions: getInt("GPU_DET_SESSIONS", 2),
		GPURecSessions: getInt("GPU_REC_SESSIONS", 2),
		EmbedBatchSize: getInt("EMBED_BATCH_SIZE", 64),
		EmbedFlushMS:   getInt("EMBED_FLUSH_MS", 10),
		MaxDim:         getInt("MAX_DIM", 1920),
		DetThresh:      getFloat("DET_THRESH", 0.5),
	}
}

// NewClusterConfig создаёт конфигурацию кластеризации из ENV.
func NewClusterConfig() ClusterConfig {
	return ClusterConfig{
		Threshold: getFloat("CLUSTER_THRESHOLD", 0.5),
	}
}

// NewOrganizerConfig создаёт конфигурацию организатора из ENV.
func NewOrganizerConfig() OrganizerConfig {
	return OrganizerConfig{
		AvatarUpdateThreshold: getFloat("AVATAR_UPDATE_THRESHOLD", 0.10),
	}
}

// NewWebConfig создаёт конфигурацию веб-сервера из ENV.
func NewWebConfig() WebConfig {
	return WebConfig{
		Port:     getInt("WEB_PORT", 8080),
		Serve:    getBool("WEB_SERVE", false),
		ViewOnly: getBool("WEB_VIEW_ONLY", false),
	}
}

// NewLoggerConfig создаёт конфигурацию логгера из ENV.
func NewLoggerConfig() LoggerConfig {
	return LoggerConfig{
		Level:  getEnv("LOG_LEVEL", "info"),
		AsJSON: getBool("LOG_JSON", false),
	}
}

// getEnv получает строковую переменную окружения со значением по умолчанию.
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getInt получает целочисленную переменную окружения со значением по умолчанию.
func getInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// getFloat получает float переменную окружения со значением по умолчанию.
func getFloat(key string, defaultValue float64) float64 {
	if value := os.Getenv(key); value != "" {
		if floatValue, err := strconv.ParseFloat(value, 64); err == nil {
			return floatValue
		}
	}
	return defaultValue
}

// getBool получает булеву переменную окружения со значением по умолчанию.
func getBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}
