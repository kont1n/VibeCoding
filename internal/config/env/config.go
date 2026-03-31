// Package env provides environment-based configuration loading.
package env

import (
	"fmt"
	"os"
	"strconv"
	"time"
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
	Workers          int
	DetInputSize     int
	MinFaceAreaRatio float64
	MinQualityScore  float64
	GPU              bool
	GPUDeviceID      int
	ForceCPU         bool
	ProviderPriority string
	GPUDetSessions   int
	GPURecSessions   int
	EmbedBatchSize   int
	EmbedFlushMS     int
	MaxDim           int
	DetThresh        float64
}

// ClusterConfig хранит настройки кластеризации.
type ClusterConfig struct {
	Threshold              float64
	RefineFactor           float64
	EnableTwoStage         bool
	PreclusterThreshold    float64
	CentroidMergeThreshold float64
	MutualK                int
	EnableAmbiguityGate    bool
	AmbiguityTopK          int
	AmbiguityMeanMin       float64
	AmbiguityMeanMax       float64
	AmbiguityCentroidMax   float64
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

// DatabaseConfig хранит настройки базы данных.
type DatabaseConfig struct {
	Host              string
	Port              int
	Database          string
	User              string
	Password          string
	SSLMode           string
	MaxConns          int
	MinConns          int
	MaxConnLifetime   int
	MaxConnIdleTime   int
	HealthCheckPeriod int
	RunMigrations     bool
}

// DSN возвращает строку подключения к базе данных.
func (c DatabaseConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.Database, c.SSLMode,
	)
}

// RedisConfig хранит настройки Redis.
type RedisConfig struct {
	Host     string
	Port     int
	Password string
	DB       int
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
		Workers:          getInt("EXTRACT_WORKERS", 4),
		DetInputSize:     getInt("DET_INPUT_SIZE", 640),
		MinFaceAreaRatio: getFloat("MIN_FACE_AREA_RATIO", 0.0),
		MinQualityScore:  getFloat("MIN_QUALITY_SCORE", 0.0),
		GPU:              getBool("GPU_ENABLED", false),
		GPUDeviceID:      getInt("GPU_DEVICE_ID", 0),
		ForceCPU:         getBool("FORCE_CPU", false),
		ProviderPriority: getEnv("PROVIDER_PRIORITY", "auto"),
		GPUDetSessions:   getInt("GPU_DET_SESSIONS", 2),
		GPURecSessions:   getInt("GPU_REC_SESSIONS", 2),
		EmbedBatchSize:   getInt("EMBED_BATCH_SIZE", 64),
		EmbedFlushMS:     getInt("EMBED_FLUSH_MS", 10),
		MaxDim:           getInt("MAX_DIM", 1920),
		DetThresh:        getFloat("DET_THRESH", 0.5),
	}
}

// NewClusterConfig создаёт конфигурацию кластеризации из ENV.
func NewClusterConfig() ClusterConfig {
	return ClusterConfig{
		Threshold:              getFloat("CLUSTER_THRESHOLD", 0.35),
		RefineFactor:           getFloat("CLUSTER_REFINE_FACTOR", 1.0),
		EnableTwoStage:         getBool("CLUSTER_ENABLE_TWO_STAGE", false),
		PreclusterThreshold:    getFloat("CLUSTER_PRECLUSTER_THRESHOLD", 0.0),
		CentroidMergeThreshold: getFloat("CLUSTER_CENTROID_MERGE_THRESHOLD", 0.0),
		MutualK:                getInt("CLUSTER_MUTUAL_K", 1),
		EnableAmbiguityGate:    getBool("CLUSTER_ENABLE_AMBIGUITY_GATE", false),
		AmbiguityTopK:          getInt("CLUSTER_AMBIGUITY_TOPK", 12),
		AmbiguityMeanMin:       getFloat("CLUSTER_AMBIGUITY_MEAN_MIN", 0.0),
		AmbiguityMeanMax:       getFloat("CLUSTER_AMBIGUITY_MEAN_MAX", 0.0),
		AmbiguityCentroidMax:   getFloat("CLUSTER_AMBIGUITY_CENTROID_MAX", 0.0),
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

// NewDatabaseConfig создаёт конфигурацию базы данных из ENV.
func NewDatabaseConfig() DatabaseConfig {
	return DatabaseConfig{
		Host:              getEnv("DB_HOST", ""),
		Port:              getInt("DB_PORT", 5432),
		Database:          getEnv("DB_NAME", ""),
		User:              getEnv("DB_USER", ""),
		Password:          getEnv("DB_PASSWORD", ""),
		SSLMode:           getEnv("DB_SSLMODE", "require"),
		MaxConns:          getInt("DB_MAX_CONNS", 25),
		MinConns:          getInt("DB_MIN_CONNS", 5),
		MaxConnLifetime:   getInt("DB_MAX_CONN_LIFETIME", 3600),
		MaxConnIdleTime:   getInt("DB_MAX_CONN_IDLE_TIME", 1800),
		HealthCheckPeriod: getInt("DB_HEALTH_CHECK_PERIOD", 60),
		RunMigrations:     getBool("DB_RUN_MIGRATIONS", true),
	}
}

// NewRedisConfig создаёт конфигурацию Redis из ENV.
func NewRedisConfig() RedisConfig {
	return RedisConfig{
		Host:     getEnv("REDIS_HOST", "localhost"),
		Port:     getInt("REDIS_PORT", 6379),
		Password: getEnv("REDIS_PASSWORD", ""),
		DB:       getInt("REDIS_DB", 0),
	}
}

// Validate проверяет критические настройки конфигурации.
func (c *DatabaseConfig) Validate() error {
	if c.Host == "" {
		return fmt.Errorf("DB_HOST is required")
	}
	if c.Database == "" {
		return fmt.Errorf("DB_NAME is required")
	}
	if c.User == "" {
		return fmt.Errorf("DB_USER is required")
	}
	if c.Password == "" {
		return fmt.Errorf("DB_PASSWORD is required")
	}
	if c.Port < 1 || c.Port > 65535 {
		return fmt.Errorf("DB_PORT must be between 1 and 65535")
	}
	return nil
}

// ConnLifetime возвращает MaxConnLifetime как time.Duration.
func (c *DatabaseConfig) ConnLifetime() time.Duration {
	return time.Duration(c.MaxConnLifetime) * time.Second
}

// ConnIdleTime возвращает MaxConnIdleTime как time.Duration.
func (c *DatabaseConfig) ConnIdleTime() time.Duration {
	return time.Duration(c.MaxConnIdleTime) * time.Second
}

// HealthCheckPeriodDuration возвращает HealthCheckPeriod как time.Duration.
func (c *DatabaseConfig) HealthCheckPeriodDuration() time.Duration {
	return time.Duration(c.HealthCheckPeriod) * time.Second
}

// requireEnv получает обязательную переменную окружения.
func requireEnv(key string) string {
	val := os.Getenv(key)
	if val == "" {
		panic(fmt.Sprintf("required environment variable %s is not set", key))
	}
	return val
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
