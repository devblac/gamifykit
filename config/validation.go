package config

import (
	"errors"
	"fmt"
	"strings"
)

// Validate validates server configuration
func (s *ServerConfig) Validate() error {
	var errs []string

	if s.Address == "" {
		errs = append(errs, "address cannot be empty")
	}

	if s.ReadTimeout <= 0 {
		errs = append(errs, "read_timeout must be positive")
	}

	if s.WriteTimeout <= 0 {
		errs = append(errs, "write_timeout must be positive")
	}

	if s.IdleTimeout <= 0 {
		errs = append(errs, "idle_timeout must be positive")
	}

	if s.ReadHeaderTimeout <= 0 {
		errs = append(errs, "read_header_timeout must be positive")
	}

	if s.ShutdownTimeout <= 0 {
		errs = append(errs, "shutdown_timeout must be positive")
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}

	return nil
}

// Validate validates storage configuration
func (s *StorageConfig) Validate() error {
	var errs []string

	validAdapters := []string{"memory", "redis", "sql", "file"}
	isValidAdapter := false
	for _, adapter := range validAdapters {
		if s.Adapter == adapter {
			isValidAdapter = true
			break
		}
	}

	if !isValidAdapter {
		errs = append(errs, fmt.Sprintf("adapter must be one of: %s", strings.Join(validAdapters, ", ")))
	}

	// Validate adapter-specific configs
	switch s.Adapter {
	case "file":
		if err := s.File.Validate(); err != nil {
			errs = append(errs, fmt.Sprintf("file config: %v", err))
		}
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}

	return nil
}

// Validate validates file storage configuration
func (f *FileConfig) Validate() error {
	if f.Path == "" {
		return errors.New("path cannot be empty")
	}
	return nil
}

// Validate validates logging configuration
func (l *LoggingConfig) Validate() error {
	var errs []string

	validLevels := []string{"debug", "info", "warn", "error"}
	isValidLevel := false
	for _, level := range validLevels {
		if l.Level == level {
			isValidLevel = true
			break
		}
	}

	if !isValidLevel {
		errs = append(errs, fmt.Sprintf("level must be one of: %s", strings.Join(validLevels, ", ")))
	}

	validFormats := []string{"json", "text"}
	isValidFormat := false
	for _, format := range validFormats {
		if l.Format == format {
			isValidFormat = true
			break
		}
	}

	if !isValidFormat {
		errs = append(errs, fmt.Sprintf("format must be one of: %s", strings.Join(validFormats, ", ")))
	}

	validOutputs := []string{"stdout", "stderr"}
	isValidOutput := false
	for _, output := range validOutputs {
		if l.Output == output {
			isValidOutput = true
			break
		}
	}

	if !isValidOutput {
		errs = append(errs, fmt.Sprintf("output must be one of: %s", strings.Join(validOutputs, ", ")))
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}

	return nil
}

// Validate validates metrics configuration
func (m *MetricsConfig) Validate() error {
	var errs []string

	if m.Enabled {
		if m.Address == "" {
			errs = append(errs, "address cannot be empty when metrics are enabled")
		}

		if m.Path == "" {
			errs = append(errs, "path cannot be empty when metrics are enabled")
		}
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}

	return nil
}
