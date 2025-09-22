package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDotEnvLoader_Load_FileNotExists(t *testing.T) {
	// Set required env vars directly for the test
	envVars := map[string]string{
		"JIRA_BASE_URL": "https://test.atlassian.net",
		"JIRA_EMAIL":    "test@example.com",
		"JIRA_PAT":      "test-pat-token-123",
	}

	// Create a custom loader with mock env and non-existent file
	dotEnvLoader := &DotEnvLoader{
		Loader:   &Loader{envLoader: NewMockEnvLoader(envVars)},
		envFiles: []string{"non-existent.env"},
	}

	config, err := dotEnvLoader.Load()

	if err != nil {
		t.Fatalf("Expected no error for missing .env file, got: %v", err)
	}

	if config.JIRABaseURL != "https://test.atlassian.net" {
		t.Errorf("Expected config to be loaded from environment variables")
	}
}

func TestDotEnvLoader_Load_ValidFile(t *testing.T) {
	// Clear any existing environment variables that might interfere
	for _, key := range []string{"JIRA_BASE_URL", "JIRA_EMAIL", "JIRA_PAT", "LOG_LEVEL", "LOG_FORMAT"} {
		_ = os.Unsetenv(key)
	}

	// Create a temporary .env file
	tmpDir := t.TempDir()
	envFile := filepath.Join(tmpDir, ".env")

	envContent := `JIRA_BASE_URL=https://test.atlassian.net
JIRA_EMAIL=test@example.com
JIRA_PAT=test-pat-token-123
LOG_LEVEL=debug
LOG_FORMAT=json
`

	err := os.WriteFile(envFile, []byte(envContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test .env file: %v", err)
	}

	// Change to temp directory to load the .env file
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer func() { _ = os.Chdir(oldDir) }()

	err = os.Chdir(tmpDir)
	if err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	// Load configuration
	loader := NewDotEnvLoader()
	config, err := loader.Load()

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify configuration loaded from .env file
	if config.JIRABaseURL != "https://test.atlassian.net" {
		t.Errorf("Expected JIRA_BASE_URL from .env file, got '%s'", config.JIRABaseURL)
	}
	if config.LogLevel != "debug" {
		t.Errorf("Expected LOG_LEVEL 'debug' from .env file, got '%s'", config.LogLevel)
	}
	if config.LogFormat != "json" {
		t.Errorf("Expected LOG_FORMAT 'json' from .env file, got '%s'", config.LogFormat)
	}
}

func TestDotEnvLoader_Load_InvalidFile(t *testing.T) {
	// Clear any existing environment variables that might interfere
	for _, key := range []string{"JIRA_BASE_URL", "JIRA_EMAIL", "JIRA_PAT", "LOG_LEVEL", "LOG_FORMAT"} {
		_ = os.Unsetenv(key)
	}

	// Create a temporary .env file with invalid content
	tmpDir := t.TempDir()
	envFile := filepath.Join(tmpDir, ".env")

	// Invalid .env content (missing quotes around value with spaces)
	envContent := `JIRA_BASE_URL=https://test.atlassian.net
INVALID_LINE_WITHOUT_EQUALS
JIRA_EMAIL=test@example.com
`

	err := os.WriteFile(envFile, []byte(envContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test .env file: %v", err)
	}

	// Change to temp directory
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer func() { _ = os.Chdir(oldDir) }()

	err = os.Chdir(tmpDir)
	if err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	// Attempt to load configuration
	loader := NewDotEnvLoader()
	_, err = loader.Load()

	if err == nil {
		t.Fatal("Expected error for invalid .env file, got nil")
	}

	// Should be an EnvFileError
	var envFileErr *EnvFileError
	if !strings.Contains(err.Error(), "failed to load .env file") {
		t.Errorf("Expected EnvFileError, got: %v", err)
	}

	if envFileErr != nil && !strings.Contains(envFileErr.FilePath, ".env") {
		t.Errorf("Expected file path to contain .env, got: %s", envFileErr.FilePath)
	}
}

func TestDotEnvLoader_MultipleFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create multiple .env files
	env1 := filepath.Join(tmpDir, ".env.local")
	env2 := filepath.Join(tmpDir, ".env.test")

	// First file has some vars
	content1 := `JIRA_BASE_URL=https://test.atlassian.net
LOG_LEVEL=debug
`

	// Second file has other vars and overrides
	content2 := `JIRA_EMAIL=test@example.com
JIRA_PAT=test-pat-token-123
LOG_LEVEL=info
`

	err := os.WriteFile(env1, []byte(content1), 0644)
	if err != nil {
		t.Fatalf("Failed to create first .env file: %v", err)
	}

	err = os.WriteFile(env2, []byte(content2), 0644)
	if err != nil {
		t.Fatalf("Failed to create second .env file: %v", err)
	}

	// Clear any existing environment variables that might interfere
	for _, key := range []string{"JIRA_BASE_URL", "JIRA_EMAIL", "JIRA_PAT", "LOG_LEVEL", "LOG_FORMAT"} {
		_ = os.Unsetenv(key)
	}

	// Load with absolute paths (no need to change directory)
	loader := NewDotEnvLoader(env1, env2)
	config, err := loader.Load()

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify configuration
	if config.JIRABaseURL != "https://test.atlassian.net" {
		t.Errorf("Expected JIRA_BASE_URL from first file")
	}
	if config.JIRAEmail != "test@example.com" {
		t.Errorf("Expected JIRA_EMAIL from second file")
	}
	// LOG_LEVEL should be from the last loaded file (env2)
	// Note: godotenv loads files in order, later files override earlier ones
	if config.LogLevel != "info" {
		t.Errorf("Expected LOG_LEVEL 'info' (from second file), got '%s'", config.LogLevel)
	}
}

func TestEnvFileError(t *testing.T) {
	originalErr := os.ErrNotExist
	envErr := NewEnvFileError("/path/to/.env", originalErr)

	if !strings.Contains(envErr.Error(), "failed to load .env file '/path/to/.env'") {
		t.Errorf("Expected error message to contain file path, got: %s", envErr.Error())
	}

	// Test Unwrap
	if envErr.Unwrap() != originalErr {
		t.Errorf("Expected Unwrap to return original error")
	}
}

func TestLoadFromCurrentDir(t *testing.T) {
	// Create .env in temp directory
	tmpDir := t.TempDir()
	envFile := filepath.Join(tmpDir, ".env")

	envContent := `JIRA_BASE_URL=https://currentdir.atlassian.net
JIRA_EMAIL=currentdir@example.com
JIRA_PAT=currentdir-pat-123
`

	err := os.WriteFile(envFile, []byte(envContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create .env file: %v", err)
	}

	// Clear any existing environment variables that might interfere
	for _, key := range []string{"JIRA_BASE_URL", "JIRA_EMAIL", "JIRA_PAT", "LOG_LEVEL", "LOG_FORMAT"} {
		_ = os.Unsetenv(key)
	}

	// Change to temp directory
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer func() { _ = os.Chdir(oldDir) }()

	err = os.Chdir(tmpDir)
	if err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	// Load from current directory
	config, err := LoadFromCurrentDir()

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if config.JIRABaseURL != "https://currentdir.atlassian.net" {
		t.Errorf("Expected JIRA_BASE_URL 'https://currentdir.atlassian.net', got '%s'", config.JIRABaseURL)
	}
}

func TestLoadWithEnvFile(t *testing.T) {
	tmpDir := t.TempDir()
	envFile := filepath.Join(tmpDir, "custom.env")

	envContent := `JIRA_BASE_URL=https://custom.atlassian.net
JIRA_EMAIL=custom@example.com
JIRA_PAT=custom-pat-token-123
`

	err := os.WriteFile(envFile, []byte(envContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create custom .env file: %v", err)
	}

	// Clear any existing environment variables that might interfere
	for _, key := range []string{"JIRA_BASE_URL", "JIRA_EMAIL", "JIRA_PAT", "LOG_LEVEL", "LOG_FORMAT"} {
		_ = os.Unsetenv(key)
	}

	// Load with specific file path
	config, err := LoadWithEnvFile(envFile)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if config.JIRABaseURL != "https://custom.atlassian.net" {
		t.Errorf("Expected JIRA_BASE_URL 'https://custom.atlassian.net', got '%s'", config.JIRABaseURL)
	}
}
