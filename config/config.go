package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gamifykit/adapters/redis"
	"gamifykit/adapters/sqlx"
)

// Environment represents the deployment environment
type Environment string

const (
	EnvDevelopment Environment = "development"
	EnvTesting     Environment = "testing"
	EnvStaging     Environment = "staging"
	EnvProduction  Environment = "production"
)

// Config holds the complete application configuration
type Config struct {
	// Environment and profile settings
	Environment Environment `json:"environment" env:"GAMIFYKIT_ENV"`
	Profile     string      `json:"profile" env:"GAMIFYKIT_PROFILE"`

	// Server configuration
	Server ServerConfig `json:"server"`

	// Storage configuration
	Storage StorageConfig `json:"storage"`

	// Logging configuration
	Logging LoggingConfig `json:"logging"`

	// Metrics and monitoring
	Metrics MetricsConfig `json:"metrics"`

	// Security configuration
	Security SecurityConfig `json:"security"`
}

// ServerConfig holds HTTP server configuration
type ServerConfig struct {
	Address           string        `json:"address" env:"GAMIFYKIT_SERVER_ADDR"`
	PathPrefix        string        `json:"path_prefix" env:"GAMIFYKIT_SERVER_PATH_PREFIX"`
	CORSOrigin        string        `json:"cors_origin" env:"GAMIFYKIT_SERVER_CORS_ORIGIN"`
	ReadTimeout       time.Duration `json:"read_timeout" env:"GAMIFYKIT_SERVER_READ_TIMEOUT"`
	WriteTimeout      time.Duration `json:"write_timeout" env:"GAMIFYKIT_SERVER_WRITE_TIMEOUT"`
	IdleTimeout       time.Duration `json:"idle_timeout" env:"GAMIFYKIT_SERVER_IDLE_TIMEOUT"`
	ReadHeaderTimeout time.Duration `json:"read_header_timeout" env:"GAMIFYKIT_SERVER_READ_HEADER_TIMEOUT"`
	ShutdownTimeout   time.Duration `json:"shutdown_timeout" env:"GAMIFYKIT_SERVER_SHUTDOWN_TIMEOUT"`
}

// StorageConfig holds storage adapter configuration
type StorageConfig struct {
	Adapter string       `json:"adapter" env:"GAMIFYKIT_STORAGE_ADAPTER"`
	Redis   redis.Config `json:"redis,omitempty"`
	SQL     sqlx.Config  `json:"sql,omitempty"`
	File    FileConfig   `json:"file,omitempty"`
}

// FileConfig holds JSON file storage configuration
type FileConfig struct {
	Path string `json:"path" env:"GAMIFYKIT_STORAGE_FILE_PATH"`
}

// LoggingConfig holds logging configuration
type LoggingConfig struct {
	Level      string            `json:"level" env:"GAMIFYKIT_LOG_LEVEL"`
	Format     string            `json:"format" env:"GAMIFYKIT_LOG_FORMAT"`
	Output     string            `json:"output" env:"GAMIFYKIT_LOG_OUTPUT"`
	Attributes map[string]string `json:"attributes,omitempty"`
}

// MetricsConfig holds metrics and monitoring configuration
type MetricsConfig struct {
	Enabled       bool   `json:"enabled" env:"GAMIFYKIT_METRICS_ENABLED"`
	Address       string `json:"address" env:"GAMIFYKIT_METRICS_ADDR"`
	Path          string `json:"path" env:"GAMIFYKIT_METRICS_PATH"`
	CollectSystem bool   `json:"collect_system" env:"GAMIFYKIT_METRICS_COLLECT_SYSTEM"`
}

// SecurityConfig holds security-related configuration
type SecurityConfig struct {
	EnableRateLimit bool            `json:"enable_rate_limit" env:"GAMIFYKIT_SECURITY_RATE_LIMIT_ENABLED"`
	RateLimit       RateLimitConfig `json:"rate_limit,omitempty"`
	APIKeys         []string        `json:"api_keys,omitempty" env:"GAMIFYKIT_SECURITY_API_KEYS"`
}

// RateLimitConfig holds rate limiting configuration
type RateLimitConfig struct {
	RequestsPerMinute int           `json:"requests_per_minute" env:"GAMIFYKIT_SECURITY_RATE_LIMIT_RPM"`
	BurstSize         int           `json:"burst_size" env:"GAMIFYKIT_SECURITY_RATE_LIMIT_BURST"`
	CleanupInterval   time.Duration `json:"cleanup_interval" env:"GAMIFYKIT_SECURITY_RATE_LIMIT_CLEANUP"`
}

// Validate validates security settings.
func (s SecurityConfig) Validate() error {
	var errs []string
	if s.EnableRateLimit {
		if s.RateLimit.RequestsPerMinute <= 0 {
			errs = append(errs, "rate_limit.requests_per_minute must be > 0 when rate limiting is enabled")
		}
		if s.RateLimit.BurstSize <= 0 {
			errs = append(errs, "rate_limit.burst_size must be > 0 when rate limiting is enabled")
		}
	}
	for i, key := range s.APIKeys {
		if strings.TrimSpace(key) == "" {
			errs = append(errs, fmt.Sprintf("api_keys[%d] is empty", i))
		}
	}
	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

// Load loads configuration from environment variables and validates it
func Load() (*Config, error) {
	cfg := DefaultConfig()

	// Load from environment variables
	if err := loadFromEnv(cfg); err != nil {
		return nil, fmt.Errorf("failed to load config from environment: %w", err)
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return cfg, nil
}

// validateConfigPath validates that the config file path is safe
func validateConfigPath(path string) error {
	if path == "" {
		return errors.New("config file path cannot be empty")
	}

	cleanPath := filepath.Clean(path)

	if !strings.HasSuffix(strings.ToLower(cleanPath), ".json") {
		return errors.New("config file must have .json extension")
	}

	if _, err := os.Stat(cleanPath); err != nil {
		return fmt.Errorf("config file not accessible: %w", err)
	}

	return nil
}

// LoadFromFile loads configuration from a JSON file
func LoadFromFile(path string) (*Config, error) {
	// Validate the path for security
	if err := validateConfigPath(path); err != nil {
		return nil, fmt.Errorf("invalid config file path: %w", err)
	}

	// Open the file safely after validation
	file, err := os.Open(path) // #nosec G304 - Path validated above
	if err != nil {
		return nil, fmt.Errorf("failed to open config file %s: %w", path, err)
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", path, err)
	}

	cfg := DefaultConfig()
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file %s: %w", path, err)
	}

	// Environment variables override file values
	if err := loadFromEnv(cfg); err != nil {
		return nil, fmt.Errorf("failed to load config from environment: %w", err)
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return cfg, nil
}

// DefaultConfig returns a configuration with sensible defaults for development
func DefaultConfig() *Config {
	return &Config{
		Environment: EnvDevelopment,
		Profile:     "default",
		Server: ServerConfig{
			Address:           ":8080",
			PathPrefix:        "/api",
			CORSOrigin:        "*",
			ReadTimeout:       10 * time.Second,
			WriteTimeout:      10 * time.Second,
			IdleTimeout:       60 * time.Second,
			ReadHeaderTimeout: 5 * time.Second,
			ShutdownTimeout:   30 * time.Second,
		},
		Storage: StorageConfig{
			Adapter: "memory",
			Redis:   redis.DefaultConfig(),
			SQL:     sqlx.DefaultConfig(sqlx.DriverPostgres),
			File: FileConfig{
				Path: "./data/gamifykit.json",
			},
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
			Output: "stdout",
		},
		Metrics: MetricsConfig{
			Enabled:       false,
			Address:       ":9090",
			Path:          "/metrics",
			CollectSystem: true,
		},
		Security: SecurityConfig{
			EnableRateLimit: false,
			RateLimit: RateLimitConfig{
				RequestsPerMinute: 60,
				BurstSize:         10,
				CleanupInterval:   5 * time.Minute,
			},
			APIKeys: []string{},
		},
	}
}

// Validate validates the configuration and returns detailed error messages
func (c *Config) Validate() error {
	var errs []string

	// Validate environment
	if c.Environment == "" {
		errs = append(errs, "environment cannot be empty")
	}

	// Validate server config
	if err := c.Server.Validate(); err != nil {
		errs = append(errs, fmt.Sprintf("server config: %v", err))
	}

	// Validate storage config
	if err := c.Storage.Validate(); err != nil {
		errs = append(errs, fmt.Sprintf("storage config: %v", err))
	}

	// Validate logging config
	if err := c.Logging.Validate(); err != nil {
		errs = append(errs, fmt.Sprintf("logging config: %v", err))
	}

	// Validate metrics config
	if err := c.Metrics.Validate(); err != nil {
		errs = append(errs, fmt.Sprintf("metrics config: %v", err))
	}

	// Validate security config
	if err := c.Security.Validate(); err != nil {
		errs = append(errs, fmt.Sprintf("security config: %v", err))
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}

	return nil
}

// String returns a JSON representation of the config (with secrets redacted)
func (c *Config) String() string {
	// Create a copy for redaction
	cfg := *c

	// Redact sensitive information
	if cfg.Storage.SQL.DSN != "" {
		cfg.Storage.SQL.DSN = "[REDACTED]"
	}
	if cfg.Storage.Redis.Password != "" {
		cfg.Storage.Redis.Password = "[REDACTED]"
	}

	data, _ := json.MarshalIndent(cfg, "", "  ")
	return string(data)
}
