package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config represents the application configuration
type Config struct {
	// JIRA configuration (based on SPIKE-001 findings)
	JIRABaseURL string `env:"JIRA_BASE_URL" validate:"required,url"`
	JIRAEmail   string `env:"JIRA_EMAIL" validate:"required,email"`
	JIRAPAT     string `env:"JIRA_PAT" validate:"required,min=10"`

	// Rate limiting configuration (JCG-010)
	RateLimitDelay         time.Duration `env:"RATE_LIMIT_DELAY" default:"100ms"`
	MaxConcurrentRequests  int           `env:"MAX_CONCURRENT_REQUESTS" default:"5"`
	ExponentialBackoffBase time.Duration `env:"EXPONENTIAL_BACKOFF_BASE" default:"1s"`
	MaxBackoffDelay        time.Duration `env:"MAX_BACKOFF_DELAY" default:"30s"`

	// Application configuration
	LogLevel  string `env:"LOG_LEVEL" validate:"oneof=debug info warn error" default:"info"`
	LogFormat string `env:"LOG_FORMAT" validate:"oneof=text json" default:"text"`
}

// Provider defines the interface for configuration management
// This enables dependency injection and easy testing
type Provider interface {
	Load() (*Config, error)
	Validate(*Config) error
	LoadFromEnv() (*Config, error)
}

// Loader implements the Provider interface
type Loader struct {
	envLoader EnvLoader
}

// EnvLoader defines interface for environment variable loading
// This allows for testing with mock environment variables
type EnvLoader interface {
	Getenv(key string) string
	LookupEnv(key string) (string, bool)
}

// OSEnvLoader implements EnvLoader using os package
type OSEnvLoader struct{}

func (o *OSEnvLoader) Getenv(key string) string {
	return os.Getenv(key)
}

func (o *OSEnvLoader) LookupEnv(key string) (string, bool) {
	return os.LookupEnv(key)
}

// NewLoader creates a new configuration loader
func NewLoader() Provider {
	return &Loader{
		envLoader: &OSEnvLoader{},
	}
}

// NewLoaderWithEnv creates a loader with custom environment loader (for testing)
func NewLoaderWithEnv(envLoader EnvLoader) Provider {
	return &Loader{
		envLoader: envLoader,
	}
}

// Load loads configuration from environment variables
func (l *Loader) Load() (*Config, error) {
	return l.LoadFromEnv()
}

// LoadFromEnv loads configuration from environment variables
func (l *Loader) LoadFromEnv() (*Config, error) {
	config := &Config{}

	// Load JIRA configuration
	config.JIRABaseURL = l.envLoader.Getenv("JIRA_BASE_URL")
	config.JIRAEmail = l.envLoader.Getenv("JIRA_EMAIL")
	config.JIRAPAT = l.envLoader.Getenv("JIRA_PAT")

	// Load rate limiting configuration with defaults (JCG-010)
	config.RateLimitDelay = l.getDurationWithDefault("RATE_LIMIT_DELAY", 100*time.Millisecond)
	config.MaxConcurrentRequests = l.getIntWithDefault("MAX_CONCURRENT_REQUESTS", 5)
	config.ExponentialBackoffBase = l.getDurationWithDefault("EXPONENTIAL_BACKOFF_BASE", 1*time.Second)
	config.MaxBackoffDelay = l.getDurationWithDefault("MAX_BACKOFF_DELAY", 30*time.Second)

	// Load application configuration with defaults
	config.LogLevel = l.getEnvWithDefault("LOG_LEVEL", "info")
	config.LogFormat = l.getEnvWithDefault("LOG_FORMAT", "text")

	// Validate configuration
	if err := l.Validate(config); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	return config, nil
}

// Validate validates the configuration
func (l *Loader) Validate(config *Config) error {
	var errors []string

	// Validate required JIRA fields
	if config.JIRABaseURL == "" {
		errors = append(errors, "JIRA_BASE_URL is required")
	} else if err := l.validateURL(config.JIRABaseURL); err != nil {
		errors = append(errors, fmt.Sprintf("JIRA_BASE_URL is invalid: %v", err))
	}

	if config.JIRAEmail == "" {
		errors = append(errors, "JIRA_EMAIL is required")
	} else if err := l.validateEmail(config.JIRAEmail); err != nil {
		errors = append(errors, fmt.Sprintf("JIRA_EMAIL is invalid: %v", err))
	}

	if config.JIRAPAT == "" {
		errors = append(errors, "JIRA_PAT is required")
	} else if len(config.JIRAPAT) < 10 {
		errors = append(errors, "JIRA_PAT must be at least 10 characters long")
	}

	// Validate rate limiting configuration (JCG-010)
	if config.RateLimitDelay < 0 {
		errors = append(errors, "RATE_LIMIT_DELAY must be non-negative")
	}
	if config.MaxConcurrentRequests < 1 {
		errors = append(errors, "MAX_CONCURRENT_REQUESTS must be at least 1")
	}
	if config.ExponentialBackoffBase < 0 {
		errors = append(errors, "EXPONENTIAL_BACKOFF_BASE must be non-negative")
	}
	if config.MaxBackoffDelay < 0 {
		errors = append(errors, "MAX_BACKOFF_DELAY must be non-negative")
	}
	if config.MaxBackoffDelay < config.ExponentialBackoffBase {
		errors = append(errors, "MAX_BACKOFF_DELAY must be greater than or equal to EXPONENTIAL_BACKOFF_BASE")
	}

	// Validate application configuration
	if err := l.validateLogLevel(config.LogLevel); err != nil {
		errors = append(errors, fmt.Sprintf("LOG_LEVEL is invalid: %v", err))
	}

	if err := l.validateLogFormat(config.LogFormat); err != nil {
		errors = append(errors, fmt.Sprintf("LOG_FORMAT is invalid: %v", err))
	}

	if len(errors) > 0 {
		return &ValidationError{Errors: errors}
	}

	return nil
}

// ValidationError represents configuration validation errors
type ValidationError struct {
	Errors []string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("configuration validation failed:\n  - %s", strings.Join(e.Errors, "\n  - "))
}

// Helper methods

func (l *Loader) getEnvWithDefault(key, defaultValue string) string {
	if value := l.envLoader.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func (l *Loader) validateURL(urlStr string) error {
	u, err := url.Parse(urlStr)
	if err != nil {
		return fmt.Errorf("invalid URL format: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("URL must use http or https scheme")
	}
	if u.Host == "" {
		return fmt.Errorf("URL must have a host")
	}
	return nil
}

func (l *Loader) validateEmail(email string) error {
	if !strings.Contains(email, "@") {
		return fmt.Errorf("email must contain @ symbol")
	}
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return fmt.Errorf("email must have exactly one @ symbol")
	}
	if parts[0] == "" || parts[1] == "" {
		return fmt.Errorf("email must have both local and domain parts")
	}
	return nil
}

func (l *Loader) validateLogLevel(level string) error {
	validLevels := []string{"debug", "info", "warn", "error"}
	for _, valid := range validLevels {
		if level == valid {
			return nil
		}
	}
	return fmt.Errorf("must be one of: %s", strings.Join(validLevels, ", "))
}

func (l *Loader) validateLogFormat(format string) error {
	validFormats := []string{"text", "json"}
	for _, valid := range validFormats {
		if format == valid {
			return nil
		}
	}
	return fmt.Errorf("must be one of: %s", strings.Join(validFormats, ", "))
}

// getDurationWithDefault gets a duration from environment with fallback to default
func (l *Loader) getDurationWithDefault(key string, defaultValue time.Duration) time.Duration {
	valueStr := l.envLoader.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}

	if duration, err := time.ParseDuration(valueStr); err == nil {
		return duration
	}

	return defaultValue
}

// getIntWithDefault gets an integer from environment with fallback to default
func (l *Loader) getIntWithDefault(key string, defaultValue int) int {
	valueStr := l.envLoader.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}

	if value, err := strconv.Atoi(valueStr); err == nil {
		return value
	}

	return defaultValue
}
