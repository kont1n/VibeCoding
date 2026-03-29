package env

import (
	"os"
	"testing"
	"time"
)

func TestDatabaseConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  DatabaseConfig
		wantErr bool
	}{
		{
			name: "valid config",
			config: DatabaseConfig{
				Host:     "localhost",
				Port:     5432,
				Database: "testdb",
				User:     "testuser",
				Password: "testpass",
				SSLMode:  "disable",
			},
			wantErr: false,
		},
		{
			name: "missing host",
			config: DatabaseConfig{
				Host:     "",
				Port:     5432,
				Database: "testdb",
				User:     "testuser",
				Password: "testpass",
			},
			wantErr: true,
		},
		{
			name: "missing database",
			config: DatabaseConfig{
				Host:     "localhost",
				Port:     5432,
				Database: "",
				User:     "testuser",
				Password: "testpass",
			},
			wantErr: true,
		},
		{
			name: "missing user",
			config: DatabaseConfig{
				Host:     "localhost",
				Port:     5432,
				Database: "testdb",
				User:     "",
				Password: "testpass",
			},
			wantErr: true,
		},
		{
			name: "missing password",
			config: DatabaseConfig{
				Host:     "localhost",
				Port:     5432,
				Database: "testdb",
				User:     "testuser",
				Password: "",
			},
			wantErr: true,
		},
		{
			name: "invalid port - zero",
			config: DatabaseConfig{
				Host:     "localhost",
				Port:     0,
				Database: "testdb",
				User:     "testuser",
				Password: "testpass",
			},
			wantErr: true,
		},
		{
			name: "invalid port - too high",
			config: DatabaseConfig{
				Host:     "localhost",
				Port:     65536,
				Database: "testdb",
				User:     "testuser",
				Password: "testpass",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDatabaseConfig_DSN(t *testing.T) {
	config := DatabaseConfig{
		Host:     "localhost",
		Port:     5432,
		Database: "testdb",
		User:     "testuser",
		Password: "testpass",
		SSLMode:  "require",
	}

	expected := "host=localhost port=5432 user=testuser password=testpass dbname=testdb sslmode=require"
	dsn := config.DSN()

	if dsn != expected {
		t.Errorf("DSN() = %q, want %q", dsn, expected)
	}
}

func TestDatabaseConfig_DurationHelpers(t *testing.T) {
	config := DatabaseConfig{
		MaxConnLifetime:   3600,
		MaxConnIdleTime:   1800,
		HealthCheckPeriod: 60,
	}

	tests := []struct {
		name     string
		getter   func() time.Duration
		expected time.Duration
	}{
		{
			name:     "ConnLifetime",
			getter:   config.ConnLifetime,
			expected: 3600 * time.Second,
		},
		{
			name:     "ConnIdleTime",
			getter:   config.ConnIdleTime,
			expected: 1800 * time.Second,
		},
		{
			name:     "HealthCheckPeriod",
			getter:   config.HealthCheckPeriodDuration,
			expected: 60 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.getter()
			if result != tt.expected {
				t.Errorf("%s() = %v, want %v", tt.name, result, tt.expected)
			}
		})
	}
}

func TestRequireEnv(t *testing.T) {
	// Save original env.
	original := os.Getenv("TEST_VAR")
	defer func() {
		if original != "" {
			os.Setenv("TEST_VAR", original)
		} else {
			os.Unsetenv("TEST_VAR")
		}
	}()

	// Test missing variable.
	os.Unsetenv("TEST_VAR")
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("requireEnv did not panic for missing variable")
		}
	}()
	requireEnv("TEST_VAR")
}

func TestGetEnv(t *testing.T) {
	// Save original env.
	original := os.Getenv("TEST_GET_VAR")
	defer func() {
		if original != "" {
			os.Setenv("TEST_GET_VAR", original)
		} else {
			os.Unsetenv("TEST_GET_VAR")
		}
	}()

	// Test with value.
	os.Setenv("TEST_GET_VAR", "test_value")
	result := getEnv("TEST_GET_VAR", "default")
	if result != "test_value" {
		t.Errorf("getEnv() = %q, want %q", result, "test_value")
	}

	// Test without value.
	os.Unsetenv("TEST_GET_VAR")
	result = getEnv("TEST_GET_VAR", "default")
	if result != "default" {
		t.Errorf("getEnv() = %q, want %q", result, "default")
	}
}

func TestGetInt(t *testing.T) {
	// Save original env.
	original := os.Getenv("TEST_INT_VAR")
	defer func() {
		if original != "" {
			os.Setenv("TEST_INT_VAR", original)
		} else {
			os.Unsetenv("TEST_INT_VAR")
		}
	}()

	// Test with valid value.
	os.Setenv("TEST_INT_VAR", "42")
	result := getInt("TEST_INT_VAR", 0)
	if result != 42 {
		t.Errorf("getInt() = %d, want %d", result, 42)
	}

	// Test with invalid value (should use default).
	os.Setenv("TEST_INT_VAR", "not_a_number")
	result = getInt("TEST_INT_VAR", 100)
	if result != 100 {
		t.Errorf("getInt() = %d, want %d", result, 100)
	}

	// Test without value.
	os.Unsetenv("TEST_INT_VAR")
	result = getInt("TEST_INT_VAR", 50)
	if result != 50 {
		t.Errorf("getInt() = %d, want %d", result, 50)
	}
}

func TestGetFloat(t *testing.T) {
	// Save original env.
	original := os.Getenv("TEST_FLOAT_VAR")
	defer func() {
		if original != "" {
			os.Setenv("TEST_FLOAT_VAR", original)
		} else {
			os.Unsetenv("TEST_FLOAT_VAR")
		}
	}()

	// Test with valid value.
	os.Setenv("TEST_FLOAT_VAR", "3.14")
	result := getFloat("TEST_FLOAT_VAR", 0.0)
	if result != 3.14 {
		t.Errorf("getFloat() = %f, want %f", result, 3.14)
	}

	// Test without value.
	os.Unsetenv("TEST_FLOAT_VAR")
	result = getFloat("TEST_FLOAT_VAR", 2.71)
	if result != 2.71 {
		t.Errorf("getFloat() = %f, want %f", result, 2.71)
	}
}

func TestGetBool(t *testing.T) {
	// Save original env.
	original := os.Getenv("TEST_BOOL_VAR")
	defer func() {
		if original != "" {
			os.Setenv("TEST_BOOL_VAR", original)
		} else {
			os.Unsetenv("TEST_BOOL_VAR")
		}
	}()

	// Test with true value.
	os.Setenv("TEST_BOOL_VAR", "true")
	result := getBool("TEST_BOOL_VAR", false)
	if !result {
		t.Errorf("getBool() = %v, want %v", result, true)
	}

	// Test with false value.
	os.Setenv("TEST_BOOL_VAR", "false")
	result = getBool("TEST_BOOL_VAR", true)
	if result {
		t.Errorf("getBool() = %v, want %v", result, false)
	}

	// Test without value.
	os.Unsetenv("TEST_BOOL_VAR")
	result = getBool("TEST_BOOL_VAR", true)
	if !result {
		t.Errorf("getBool() = %v, want %v", result, true)
	}
}
