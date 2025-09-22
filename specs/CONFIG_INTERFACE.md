# Configuration Interface Specification

## Overview

The configuration system provides environment-based configuration management for the jira-cdc-git application. This specification defines the configuration structure, validation rules, and loading mechanisms.

## Interface Definition

```go
type Provider interface {
    Load() (*Config, error)
    Validate(*Config) error
    LoadFromEnv() (*Config, error)
}

type EnvLoader interface {
    Getenv(key string) string
    LookupEnv(key string) (string, bool)
}
```

## Configuration Structure

```go
type Config struct {
    // JIRA configuration (based on SPIKE-001 findings)
    JIRABaseURL string `env:"JIRA_BASE_URL" validate:"required,url"`
    JIRAEmail   string `env:"JIRA_EMAIL" validate:"required,email"`
    JIRAPAT     string `env:"JIRA_PAT" validate:"required,min=10"`

    // Application configuration
    LogLevel  string `env:"LOG_LEVEL" validate:"oneof=debug info warn error" default:"info"`
    LogFormat string `env:"LOG_FORMAT" validate:"oneof=text json" default:"text"`
}
```

## Configuration Sources

### 1. Environment Variables
Direct environment variable reading using standard `os.Getenv()`

### 2. .env File Loading
Using `godotenv` library for .env file support:
```go
func (l *DotEnvLoader) Load() (*Config, error) {
    // Load .env file if it exists
    if err := godotenv.Load(); err != nil {
        if !os.IsNotExist(err) {
            return nil, &EnvFileError{
                Operation: "Load",
                FilePath:  ".env",
                Err:       err,
            }
        }
        // .env file doesn't exist, continue with env vars only
    }
    
    return l.LoadFromEnv()
}
```

## Validation Rules

### JIRA Configuration
```go
// JIRABaseURL validation
func validateJIRABaseURL(url string) error {
    if url == "" {
        return errors.New("JIRA_BASE_URL is required")
    }
    
    parsed, err := url.Parse(url)
    if err != nil {
        return fmt.Errorf("JIRA_BASE_URL must be a valid URL: %w", err)
    }
    
    if parsed.Scheme != "https" {
        return errors.New("JIRA_BASE_URL must use HTTPS")
    }
    
    return nil
}

// JIRAEmail validation
func validateJIRAEmail(email string) error {
    if email == "" {
        return errors.New("JIRA_EMAIL is required")
    }
    
    emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
    if !emailRegex.MatchString(email) {
        return errors.New("JIRA_EMAIL must be a valid email address")
    }
    
    return nil
}

// JIRAPAT validation
func validateJIRAPAT(pat string) error {
    if pat == "" {
        return errors.New("JIRA_PAT is required")
    }
    
    if len(pat) < 10 {
        return errors.New("JIRA_PAT must be at least 10 characters long")
    }
    
    return nil
}
```

### Application Configuration
```go
// LogLevel validation
func validateLogLevel(level string) error {
    validLevels := []string{"debug", "info", "warn", "error"}
    for _, valid := range validLevels {
        if level == valid {
            return nil
        }
    }
    return fmt.Errorf("LOG_LEVEL must be one of: %s", strings.Join(validLevels, ", "))
}

// LogFormat validation  
func validateLogFormat(format string) error {
    validFormats := []string{"text", "json"}
    for _, valid := range validFormats {
        if format == valid {
            return nil
        }
    }
    return fmt.Errorf("LOG_FORMAT must be one of: %s", strings.Join(validFormats, ", "))
}
```

## Implementation

### DotEnvLoader Implementation
```go
type DotEnvLoader struct {
    envLoader EnvLoader
}

func NewDotEnvLoader() Provider {
    return &DotEnvLoader{
        envLoader: &OSEnvLoader{},
    }
}

func (l *DotEnvLoader) Load() (*Config, error) {
    // Load .env file if it exists
    if err := godotenv.Load(); err != nil {
        if !os.IsNotExist(err) {
            return nil, &EnvFileError{
                Operation: "Load",
                FilePath:  ".env", 
                Err:       err,
            }
        }
    }
    
    return l.LoadFromEnv()
}

func (l *DotEnvLoader) LoadFromEnv() (*Config, error) {
    config := &Config{
        JIRABaseURL: l.envLoader.Getenv("JIRA_BASE_URL"),
        JIRAEmail:   l.envLoader.Getenv("JIRA_EMAIL"),
        JIRAPAT:     l.envLoader.Getenv("JIRA_PAT"),
        LogLevel:    getEnvWithDefault(l.envLoader, "LOG_LEVEL", "info"),
        LogFormat:   getEnvWithDefault(l.envLoader, "LOG_FORMAT", "text"),
    }
    
    if err := l.Validate(config); err != nil {
        return nil, err
    }
    
    return config, nil
}
```

### Environment Variable Helper
```go
func getEnvWithDefault(loader EnvLoader, key, defaultValue string) string {
    if value := loader.Getenv(key); value != "" {
        return value
    }
    return defaultValue
}
```

## Error Handling

### Error Types
```go
type ValidationError struct {
    Field   string
    Value   string
    Message string
}

type EnvFileError struct {
    Operation string
    FilePath  string
    Err       error
}

func (e *ValidationError) Error() string {
    return fmt.Sprintf("validation failed for %s: %s", e.Field, e.Message)
}

func (e *EnvFileError) Error() string {
    return fmt.Sprintf("env file error (%s) for %s: %v", e.Operation, e.FilePath, e.Err)
}
```

### Validation Error Collection
```go
func (l *DotEnvLoader) Validate(config *Config) error {
    var errors []error
    
    if err := validateJIRABaseURL(config.JIRABaseURL); err != nil {
        errors = append(errors, &ValidationError{
            Field:   "JIRA_BASE_URL",
            Value:   config.JIRABaseURL,
            Message: err.Error(),
        })
    }
    
    // ... other validations
    
    if len(errors) > 0 {
        return &MultiValidationError{Errors: errors}
    }
    
    return nil
}
```

## Testing Requirements

### Unit Tests
- Test configuration loading from environment variables
- Test .env file loading and parsing
- Test validation rules for all fields
- Test error handling scenarios
- Test default value application

### Mock Implementation
```go
type MockEnvLoader struct {
    Env map[string]string
}

func (m *MockEnvLoader) Getenv(key string) string {
    return m.Env[key]
}

func (m *MockEnvLoader) LookupEnv(key string) (string, bool) {
    value, exists := m.Env[key]
    return value, exists
}
```

### Integration Tests
- Test with real .env files
- Test with various environment configurations
- Test configuration precedence (.env vs environment variables)
- Test invalid configuration scenarios

## Usage Examples

### Basic Usage
```go
// Load configuration
configLoader := config.NewDotEnvLoader()
cfg, err := configLoader.Load()
if err != nil {
    return fmt.Errorf("failed to load configuration: %w", err)
}

// Use configuration
client, err := jira.NewClient(cfg)
```

### Environment-Only Loading
```go
// Load from environment variables only (skip .env file)
cfg, err := configLoader.LoadFromEnv()
if err != nil {
    return fmt.Errorf("failed to load from environment: %w", err)
}
```

### Custom Validation
```go
// Validate existing configuration
if err := configLoader.Validate(cfg); err != nil {
    return fmt.Errorf("configuration validation failed: %w", err)
}
```

## Configuration File Example

### .env File Format
```bash
# JIRA Configuration (Required)
JIRA_BASE_URL=https://your-company.atlassian.net
JIRA_EMAIL=your-email@company.com
JIRA_PAT=your-personal-access-token-here

# Application Configuration (Optional)
LOG_LEVEL=info
LOG_FORMAT=text
```

### Environment Variables
```bash
export JIRA_BASE_URL=https://issues.redhat.com
export JIRA_EMAIL=user@redhat.com
export JIRA_PAT=abc123def456ghi789
export LOG_LEVEL=debug
export LOG_FORMAT=json
```

## Security Requirements

1. **Credential Protection**: PAT tokens must be stored securely
2. **No Credential Logging**: Never log JIRA_PAT values
3. **HTTPS Enforcement**: JIRA_BASE_URL must use HTTPS
4. **Input Sanitization**: All configuration values must be validated
5. **File Permissions**: .env files should have restricted permissions (0600)

## Performance Requirements

- **Configuration Load**: < 10ms
- **Validation**: < 5ms
- **Memory Usage**: < 1KB per configuration instance
- **File Reading**: < 5ms for .env file parsing

## Validation Coverage Requirements

1. All required fields must be validated
2. URL format validation for JIRA_BASE_URL
3. Email format validation for JIRA_EMAIL
4. Token length validation for JIRA_PAT
5. Enum validation for LOG_LEVEL and LOG_FORMAT
6. HTTPS scheme enforcement for security