package config

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad(t *testing.T) {
	// Test loading default config
	cfg, err := Load()
	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Verify defaults
	assert.Equal(t, EnvDevelopment, cfg.Environment)
	assert.Equal(t, ":8080", cfg.Server.Address)
	assert.Equal(t, "memory", cfg.Storage.Adapter)
	assert.Equal(t, "info", cfg.Logging.Level)
	assert.Equal(t, "json", cfg.Logging.Format)
}

func TestLoadFromFile(t *testing.T) {
	// Create a temporary config file
	configContent := `{
		"environment": "testing",
		"server": {
			"address": ":9090"
		},
		"storage": {
			"adapter": "memory"
		}
	}`

	tmpFile, err := os.CreateTemp("", "config_test_*.json")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(configContent)
	require.NoError(t, err)
	tmpFile.Close()

	// Load config from file
	cfg, err := LoadFromFile(tmpFile.Name())
	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Verify loaded values
	assert.Equal(t, EnvTesting, cfg.Environment)
	assert.Equal(t, ":9090", cfg.Server.Address)
	assert.Equal(t, "memory", cfg.Storage.Adapter)
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		expectError bool
	}{
		{
			name: "valid config",
			config: &Config{
				Environment: EnvDevelopment,
				Server: ServerConfig{
					Address:           ":8080",
					ReadTimeout:       time.Second,
					WriteTimeout:      time.Second,
					IdleTimeout:       time.Second,
					ReadHeaderTimeout: time.Second,
					ShutdownTimeout:   time.Second,
				},
				Storage: StorageConfig{
					Adapter: "memory",
				},
				Logging: LoggingConfig{
					Level:  "info",
					Format: "json",
					Output: "stdout",
				},
			},
			expectError: false,
		},
		{
			name: "invalid environment",
			config: &Config{
				Environment: "",
				Server: ServerConfig{
					Address:           ":8080",
					ReadTimeout:       time.Second,
					WriteTimeout:      time.Second,
					IdleTimeout:       time.Second,
					ReadHeaderTimeout: time.Second,
					ShutdownTimeout:   time.Second,
				},
				Storage: StorageConfig{
					Adapter: "memory",
				},
				Logging: LoggingConfig{
					Level:  "info",
					Format: "json",
					Output: "stdout",
				},
			},
			expectError: true,
		},
		{
			name: "invalid server timeout",
			config: &Config{
				Environment: EnvDevelopment,
				Server: ServerConfig{
					Address:           ":8080",
					ReadTimeout:       0,
					WriteTimeout:      time.Second,
					IdleTimeout:       time.Second,
					ReadHeaderTimeout: time.Second,
					ShutdownTimeout:   time.Second,
				},
				Storage: StorageConfig{
					Adapter: "memory",
				},
				Logging: LoggingConfig{
					Level:  "info",
					Format: "json",
					Output: "stdout",
				},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestProfiles(t *testing.T) {
	tests := []struct {
		name         string
		profileName  string
		expectConfig bool
		environment  Environment
	}{
		{"development", "development", true, EnvDevelopment},
		{"testing", "testing", true, EnvTesting},
		{"staging", "staging", true, EnvStaging},
		{"production", "production", true, EnvProduction},
		{"unknown", "unknown", false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := LoadProfile(tt.profileName)
			if tt.expectConfig {
				require.NoError(t, err)
				require.NotNil(t, cfg)
				assert.Equal(t, tt.environment, cfg.Environment)
			} else {
				assert.Error(t, err)
				assert.Nil(t, cfg)
			}
		})
	}
}

func TestSecrets(t *testing.T) {
	// Test environment secret store
	store := NewEnvironmentSecretStore()

	// Set test environment variable
	testKey := "TEST_SECRET_KEY"
	testValue := "test_secret_value"
	os.Setenv(testKey, testValue)
	defer os.Unsetenv(testKey)

	ctx := context.Background()

	// Test Get
	value, err := store.Get(ctx, testKey)
	assert.NoError(t, err)
	assert.Equal(t, testValue, value)

	// Test GetWithDefault
	defaultValue := "default"
	value = store.GetWithDefault(ctx, "NONEXISTENT_KEY", defaultValue)
	assert.Equal(t, defaultValue, value)

	value = store.GetWithDefault(ctx, testKey, defaultValue)
	assert.Equal(t, testValue, value)
}

func TestValidateConfigPath(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		expectError bool
		setup       func() string // returns path to cleanup
	}{
		{
			name:        "valid json file",
			path:        "config_test.json",
			expectError: false,
			setup: func() string {
				tmpFile, _ := os.CreateTemp("", "config_test_*.json")
				tmpFile.WriteString("{}")
				tmpFile.Close()
				return tmpFile.Name()
			},
		},
		{
			name:        "empty path",
			path:        "",
			expectError: true,
			setup:       func() string { return "" },
		},
		{
			name:        "path traversal",
			path:        "../../../etc/passwd",
			expectError: true,
			setup:       func() string { return "" },
		},
		{
			name:        "non-json file",
			path:        "config.txt",
			expectError: true,
			setup: func() string {
				tmpFile, _ := os.CreateTemp("", "config_test_*.txt")
				tmpFile.WriteString("{}")
				tmpFile.Close()
				return tmpFile.Name()
			},
		},
		{
			name:        "nonexistent file",
			path:        "nonexistent.json",
			expectError: true,
			setup:       func() string { return "" },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanupPath := tt.setup()
			if cleanupPath != "" {
				defer os.Remove(cleanupPath)
				if tt.path == "config_test.json" || tt.path == "config.txt" {
					tt.path = cleanupPath
				}
			}

			err := validateConfigPath(tt.path)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
