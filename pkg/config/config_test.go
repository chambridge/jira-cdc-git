package config

import (
	"strings"
	"testing"
)

// MockEnvLoader implements EnvLoader for testing
type MockEnvLoader struct {
	vars map[string]string
}

func NewMockEnvLoader(vars map[string]string) *MockEnvLoader {
	return &MockEnvLoader{vars: vars}
}

func (m *MockEnvLoader) Getenv(key string) string {
	return m.vars[key]
}

func (m *MockEnvLoader) LookupEnv(key string) (string, bool) {
	val, exists := m.vars[key]
	return val, exists
}

func TestConfig_LoadFromEnv_Success(t *testing.T) {
	envVars := map[string]string{
		"JIRA_BASE_URL": "https://test.atlassian.net",
		"JIRA_EMAIL":    "test@example.com",
		"JIRA_PAT":      "test-pat-token-123",
		"LOG_LEVEL":     "debug",
		"LOG_FORMAT":    "json",
	}

	loader := NewLoaderWithEnv(NewMockEnvLoader(envVars))
	config, err := loader.Load()

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify JIRA configuration
	if config.JIRABaseURL != "https://test.atlassian.net" {
		t.Errorf("Expected JIRA_BASE_URL 'https://test.atlassian.net', got '%s'", config.JIRABaseURL)
	}
	if config.JIRAEmail != "test@example.com" {
		t.Errorf("Expected JIRA_EMAIL 'test@example.com', got '%s'", config.JIRAEmail)
	}
	if config.JIRAPAT != "test-pat-token-123" {
		t.Errorf("Expected JIRA_PAT 'test-pat-token-123', got '%s'", config.JIRAPAT)
	}

	// Verify application configuration
	if config.LogLevel != "debug" {
		t.Errorf("Expected LOG_LEVEL 'debug', got '%s'", config.LogLevel)
	}
	if config.LogFormat != "json" {
		t.Errorf("Expected LOG_FORMAT 'json', got '%s'", config.LogFormat)
	}
}

func TestConfig_LoadFromEnv_WithDefaults(t *testing.T) {
	envVars := map[string]string{
		"JIRA_BASE_URL": "https://test.atlassian.net",
		"JIRA_EMAIL":    "test@example.com",
		"JIRA_PAT":      "test-pat-token-123",
		// LOG_LEVEL and LOG_FORMAT not set - should use defaults
	}

	loader := NewLoaderWithEnv(NewMockEnvLoader(envVars))
	config, err := loader.Load()

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify defaults are applied
	if config.LogLevel != "info" {
		t.Errorf("Expected default LOG_LEVEL 'info', got '%s'", config.LogLevel)
	}
	if config.LogFormat != "text" {
		t.Errorf("Expected default LOG_FORMAT 'text', got '%s'", config.LogFormat)
	}
}

func TestConfig_Validation_MissingRequired(t *testing.T) {
	tests := []struct {
		name     string
		envVars  map[string]string
		expected string
	}{
		{
			name:     "missing JIRA_BASE_URL",
			envVars:  map[string]string{"JIRA_EMAIL": "test@example.com", "JIRA_PAT": "test-pat-123"},
			expected: "JIRA_BASE_URL is required",
		},
		{
			name:     "missing JIRA_EMAIL",
			envVars:  map[string]string{"JIRA_BASE_URL": "https://test.atlassian.net", "JIRA_PAT": "test-pat-123"},
			expected: "JIRA_EMAIL is required",
		},
		{
			name:     "missing JIRA_PAT",
			envVars:  map[string]string{"JIRA_BASE_URL": "https://test.atlassian.net", "JIRA_EMAIL": "test@example.com"},
			expected: "JIRA_PAT is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loader := NewLoaderWithEnv(NewMockEnvLoader(tt.envVars))
			_, err := loader.Load()

			if err == nil {
				t.Fatal("Expected validation error, got nil")
			}

			if !strings.Contains(err.Error(), tt.expected) {
				t.Errorf("Expected error to contain '%s', got: %v", tt.expected, err)
			}
		})
	}
}

func TestConfig_Validation_InvalidValues(t *testing.T) {
	tests := []struct {
		name     string
		envVars  map[string]string
		expected string
	}{
		{
			name: "invalid URL",
			envVars: map[string]string{
				"JIRA_BASE_URL": "not-a-url",
				"JIRA_EMAIL":    "test@example.com",
				"JIRA_PAT":      "test-pat-123",
			},
			expected: "JIRA_BASE_URL is invalid",
		},
		{
			name: "invalid email",
			envVars: map[string]string{
				"JIRA_BASE_URL": "https://test.atlassian.net",
				"JIRA_EMAIL":    "not-an-email",
				"JIRA_PAT":      "test-pat-123",
			},
			expected: "JIRA_EMAIL is invalid",
		},
		{
			name: "short PAT",
			envVars: map[string]string{
				"JIRA_BASE_URL": "https://test.atlassian.net",
				"JIRA_EMAIL":    "test@example.com",
				"JIRA_PAT":      "short",
			},
			expected: "JIRA_PAT must be at least 10 characters long",
		},
		{
			name: "invalid log level",
			envVars: map[string]string{
				"JIRA_BASE_URL": "https://test.atlassian.net",
				"JIRA_EMAIL":    "test@example.com",
				"JIRA_PAT":      "test-pat-123",
				"LOG_LEVEL":     "invalid",
			},
			expected: "LOG_LEVEL is invalid",
		},
		{
			name: "invalid log format",
			envVars: map[string]string{
				"JIRA_BASE_URL": "https://test.atlassian.net",
				"JIRA_EMAIL":    "test@example.com",
				"JIRA_PAT":      "test-pat-123",
				"LOG_FORMAT":    "invalid",
			},
			expected: "LOG_FORMAT is invalid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loader := NewLoaderWithEnv(NewMockEnvLoader(tt.envVars))
			_, err := loader.Load()

			if err == nil {
				t.Fatal("Expected validation error, got nil")
			}

			if !strings.Contains(err.Error(), tt.expected) {
				t.Errorf("Expected error to contain '%s', got: %v", tt.expected, err)
			}
		})
	}
}

func TestConfig_Validation_MultipleErrors(t *testing.T) {
	envVars := map[string]string{
		// Missing all required fields
	}

	loader := NewLoaderWithEnv(NewMockEnvLoader(envVars))
	_, err := loader.Load()

	if err == nil {
		t.Fatal("Expected validation error, got nil")
	}

	errorMsg := err.Error()

	// Should contain all required field errors
	expectedErrors := []string{
		"JIRA_BASE_URL is required",
		"JIRA_EMAIL is required",
		"JIRA_PAT is required",
	}

	for _, expected := range expectedErrors {
		if !strings.Contains(errorMsg, expected) {
			t.Errorf("Expected error to contain '%s', got: %v", expected, err)
		}
	}
}

func TestValidationError_Error(t *testing.T) {
	errors := []string{
		"JIRA_BASE_URL is required",
		"JIRA_EMAIL is invalid",
	}

	err := &ValidationError{Errors: errors}
	errorMsg := err.Error()

	expected := "configuration validation failed:\n  - JIRA_BASE_URL is required\n  - JIRA_EMAIL is invalid"
	if errorMsg != expected {
		t.Errorf("Expected error message:\n%s\nGot:\n%s", expected, errorMsg)
	}
}

func TestURL_Validation(t *testing.T) {
	loader := &Loader{}

	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{"valid https", "https://test.atlassian.net", false},
		{"valid http", "http://test.atlassian.net", false},
		{"missing scheme", "test.atlassian.net", true},
		{"invalid scheme", "ftp://test.atlassian.net", true},
		{"missing host", "https://", true},
		{"invalid format", "not-a-url", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := loader.validateURL(tt.url)
			hasErr := err != nil

			if hasErr != tt.wantErr {
				t.Errorf("validateURL() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestEmail_Validation(t *testing.T) {
	loader := &Loader{}

	tests := []struct {
		name    string
		email   string
		wantErr bool
	}{
		{"valid email", "test@example.com", false},
		{"valid email with subdomain", "user@mail.example.com", false},
		{"missing @", "testexample.com", true},
		{"multiple @", "test@@example.com", true},
		{"missing local part", "@example.com", true},
		{"missing domain", "test@", true},
		{"empty", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := loader.validateEmail(tt.email)
			hasErr := err != nil

			if hasErr != tt.wantErr {
				t.Errorf("validateEmail() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestLogLevel_Validation(t *testing.T) {
	loader := &Loader{}

	validLevels := []string{"debug", "info", "warn", "error"}
	for _, level := range validLevels {
		t.Run("valid_"+level, func(t *testing.T) {
			err := loader.validateLogLevel(level)
			if err != nil {
				t.Errorf("validateLogLevel(%s) should be valid, got error: %v", level, err)
			}
		})
	}

	invalidLevels := []string{"trace", "fatal", "panic", "invalid"}
	for _, level := range invalidLevels {
		t.Run("invalid_"+level, func(t *testing.T) {
			err := loader.validateLogLevel(level)
			if err == nil {
				t.Errorf("validateLogLevel(%s) should be invalid", level)
			}
		})
	}
}

func TestLogFormat_Validation(t *testing.T) {
	loader := &Loader{}

	validFormats := []string{"text", "json"}
	for _, format := range validFormats {
		t.Run("valid_"+format, func(t *testing.T) {
			err := loader.validateLogFormat(format)
			if err != nil {
				t.Errorf("validateLogFormat(%s) should be valid, got error: %v", format, err)
			}
		})
	}

	invalidFormats := []string{"xml", "yaml", "invalid"}
	for _, format := range invalidFormats {
		t.Run("invalid_"+format, func(t *testing.T) {
			err := loader.validateLogFormat(format)
			if err == nil {
				t.Errorf("validateLogFormat(%s) should be invalid", format)
			}
		})
	}
}
