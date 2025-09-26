package profile

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// ProfileError represents a profile-related error
type ProfileError struct {
	Type    string
	Profile string
	Field   string
	Message string
	Cause   error
}

func (e *ProfileError) Error() string {
	if e.Profile != "" && e.Field != "" {
		return fmt.Sprintf("%s: profile '%s', field '%s': %s", e.Type, e.Profile, e.Field, e.Message)
	} else if e.Profile != "" {
		return fmt.Sprintf("%s: profile '%s': %s", e.Type, e.Profile, e.Message)
	}
	return fmt.Sprintf("%s: %s", e.Type, e.Message)
}

func (e *ProfileError) Unwrap() error {
	return e.Cause
}

// IsProfileError checks if an error is a ProfileError
func IsProfileError(err error) bool {
	_, ok := err.(*ProfileError)
	return ok
}

// Error type constants
const (
	ErrorTypeValidation    = "ValidationError"
	ErrorTypeNotFound      = "NotFoundError"
	ErrorTypeAlreadyExists = "AlreadyExistsError"
	ErrorTypeTemplate      = "TemplateError"
	ErrorTypeImport        = "ImportError"
	ErrorTypeExport        = "ExportError"
	ErrorTypeStorage       = "StorageError"
	ErrorTypePermission    = "PermissionError"
	ErrorTypeFormat        = "FormatError"
)

// ValidationError creates a new validation error
func NewValidationError(profile, field, message string) *ProfileError {
	return &ProfileError{
		Type:    ErrorTypeValidation,
		Profile: profile,
		Field:   field,
		Message: message,
	}
}

// NotFoundError creates a new not found error
func NewNotFoundError(profile, message string) *ProfileError {
	return &ProfileError{
		Type:    ErrorTypeNotFound,
		Profile: profile,
		Message: message,
	}
}

// AlreadyExistsError creates a new already exists error
func NewAlreadyExistsError(profile, message string) *ProfileError {
	return &ProfileError{
		Type:    ErrorTypeAlreadyExists,
		Profile: profile,
		Message: message,
	}
}

// TemplateError creates a new template error
func NewTemplateError(template, message string, cause error) *ProfileError {
	return &ProfileError{
		Type:    ErrorTypeTemplate,
		Profile: template,
		Message: message,
		Cause:   cause,
	}
}

// ImportError creates a new import error
func NewImportError(message string, cause error) *ProfileError {
	return &ProfileError{
		Type:    ErrorTypeImport,
		Message: message,
		Cause:   cause,
	}
}

// ExportError creates a new export error
func NewExportError(message string, cause error) *ProfileError {
	return &ProfileError{
		Type:    ErrorTypeExport,
		Message: message,
		Cause:   cause,
	}
}

// StorageError creates a new storage error
func NewStorageError(message string, cause error) *ProfileError {
	return &ProfileError{
		Type:    ErrorTypeStorage,
		Message: message,
		Cause:   cause,
	}
}

// FormatError creates a new format error
func NewFormatError(message string, cause error) *ProfileError {
	return &ProfileError{
		Type:    ErrorTypeFormat,
		Message: message,
		Cause:   cause,
	}
}

// ValidationErrors represents multiple validation errors
type ValidationErrors struct {
	Errors []error
}

func (e *ValidationErrors) Error() string {
	if len(e.Errors) == 1 {
		return e.Errors[0].Error()
	}
	return fmt.Sprintf("multiple validation errors (%d errors)", len(e.Errors))
}

func (e *ValidationErrors) Add(err error) {
	e.Errors = append(e.Errors, err)
}

func (e *ValidationErrors) HasErrors() bool {
	return len(e.Errors) > 0
}

func (e *ValidationErrors) IsEmpty() bool {
	return len(e.Errors) == 0
}

// NewValidationErrors creates a new ValidationErrors instance
func NewValidationErrors() *ValidationErrors {
	return &ValidationErrors{
		Errors: make([]error, 0),
	}
}

// ProfileValidationError represents detailed validation error information
type ProfileValidationError struct {
	ProfileName string
	Errors      []FieldError
	Warnings    []FieldWarning
}

type FieldError struct {
	Field   string
	Value   interface{}
	Message string
	Code    string
}

type FieldWarning struct {
	Field   string
	Value   interface{}
	Message string
	Code    string
}

func (e *ProfileValidationError) Error() string {
	if len(e.Errors) == 1 {
		return fmt.Sprintf("validation failed for profile '%s': %s", e.ProfileName, e.Errors[0].Message)
	}
	return fmt.Sprintf("validation failed for profile '%s': %d errors", e.ProfileName, len(e.Errors))
}

func (e *ProfileValidationError) HasErrors() bool {
	return len(e.Errors) > 0
}

func (e *ProfileValidationError) HasWarnings() bool {
	return len(e.Warnings) > 0
}

func (e *ProfileValidationError) AddError(field, message, code string, value interface{}) {
	e.Errors = append(e.Errors, FieldError{
		Field:   field,
		Value:   value,
		Message: message,
		Code:    code,
	})
}

func (e *ProfileValidationError) AddWarning(field, message, code string, value interface{}) {
	e.Warnings = append(e.Warnings, FieldWarning{
		Field:   field,
		Value:   value,
		Message: message,
		Code:    code,
	})
}

// NewProfileValidationError creates a new ProfileValidationError
func NewProfileValidationError(profileName string) *ProfileValidationError {
	return &ProfileValidationError{
		ProfileName: profileName,
		Errors:      make([]FieldError, 0),
		Warnings:    make([]FieldWarning, 0),
	}
}

// Error codes for different validation issues
const (
	ValidationCodeRequired          = "REQUIRED"
	ValidationCodeInvalidFormat     = "INVALID_FORMAT"
	ValidationCodeInvalidValue      = "INVALID_VALUE"
	ValidationCodeOutOfRange        = "OUT_OF_RANGE"
	ValidationCodeMutualExclusive   = "MUTUAL_EXCLUSIVE"
	ValidationCodeDependencyMissing = "DEPENDENCY_MISSING"
	ValidationCodeIncompatible      = "INCOMPATIBLE"
	ValidationCodeDeprecated        = "DEPRECATED"
	ValidationCodePerformance       = "PERFORMANCE_WARNING"
)

// Common validation functions

// ValidateProfileName validates profile names according to naming rules
func ValidateProfileName(name string) error {
	if name == "" {
		return NewValidationError("", "name", "profile name cannot be empty")
	}

	if len(name) > 50 {
		return NewValidationError(name, "name", "profile name cannot exceed 50 characters")
	}

	// Check for reserved names
	reservedNames := []string{"default", "template", "system", "admin", "root"}
	for _, reserved := range reservedNames {
		if name == reserved {
			return NewValidationError(name, "name", fmt.Sprintf("'%s' is a reserved profile name", name))
		}
	}

	return nil
}

// ValidateJQLQuery performs basic JQL query validation
func ValidateJQLQuery(jql string) error {
	if jql == "" {
		return NewValidationError("", "jql", "JQL query cannot be empty")
	}

	if len(jql) > 1000 {
		return NewValidationError("", "jql", "JQL query cannot exceed 1000 characters")
	}

	// Basic syntax checks
	if strings.Contains(jql, "';") || strings.Contains(jql, "--;") {
		return NewValidationError("", "jql", "JQL query contains potentially dangerous characters")
	}

	return nil
}

// ValidateRepositoryPath validates repository paths
func ValidateRepositoryPath(path string) error {
	if path == "" {
		return NewValidationError("", "repository", "repository path cannot be empty")
	}

	// Check for suspicious patterns
	if strings.Contains(path, "..") {
		return NewValidationError("", "repository", "repository path cannot contain '..' (directory traversal)")
	}

	return nil
}

// ValidateIssueKeys validates issue key format
func ValidateIssueKeys(keys []string) error {
	if len(keys) == 0 {
		return NewValidationError("", "issue_keys", "at least one issue key is required")
	}

	for i, key := range keys {
		if err := ValidateIssueKey(key); err != nil {
			return NewValidationError("", fmt.Sprintf("issue_keys[%d]", i), err.Error())
		}
	}

	return nil
}

// ValidateIssueKey validates a single issue key format
func ValidateIssueKey(key string) error {
	if key == "" {
		return fmt.Errorf("issue key cannot be empty")
	}

	// Basic JIRA issue key format: PROJECT-NUMBER
	if !regexp.MustCompile(`^[A-Z][A-Z0-9]*(-[A-Z0-9]+)*-\d+$`).MatchString(key) {
		return fmt.Errorf("invalid issue key format '%s': must match pattern PROJECT-123", key)
	}

	return nil
}

// ValidateProfileOptions validates profile options
func ValidateProfileOptions(options ProfileOptions) *ProfileValidationError {
	validation := NewProfileValidationError("")

	// Validate concurrency
	if options.Concurrency < 1 || options.Concurrency > 10 {
		validation.AddError("options.concurrency", "concurrency must be between 1 and 10",
			ValidationCodeOutOfRange, options.Concurrency)
	}

	// Validate rate limit
	if options.RateLimit != "" {
		if _, err := time.ParseDuration(options.RateLimit); err != nil {
			validation.AddError("options.rate_limit", "invalid rate limit duration format",
				ValidationCodeInvalidFormat, options.RateLimit)
		}
	}

	// Check mutually exclusive options
	if options.Incremental && options.Force {
		validation.AddError("options", "incremental and force options are mutually exclusive",
			ValidationCodeMutualExclusive, nil)
	}

	// Performance warnings
	if options.Concurrency > 8 && options.RateLimit != "" {
		if duration, err := time.ParseDuration(options.RateLimit); err == nil && duration < 200*time.Millisecond {
			validation.AddWarning("options", "high concurrency with low rate limit may overwhelm JIRA API",
				ValidationCodePerformance, nil)
		}
	}

	return validation
}
