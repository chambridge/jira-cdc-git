package epic

import "fmt"

// EpicError represents errors specific to EPIC operations
type EpicError struct {
	Type    string
	Message string
	EpicKey string
	Err     error
}

func (e *EpicError) Error() string {
	if e.EpicKey != "" {
		return fmt.Sprintf("epic %s: %s", e.EpicKey, e.Message)
	}
	return e.Message
}

func (e *EpicError) Unwrap() error {
	return e.Err
}

// Error type constants
const (
	ErrorTypeNotFound           = "not_found"
	ErrorTypeInvalidType        = "invalid_type"
	ErrorTypeDiscoveryFailed    = "discovery_failed"
	ErrorTypeAnalysisFailed     = "analysis_failed"
	ErrorTypeHierarchyFailed    = "hierarchy_failed"
	ErrorTypeCompletenessCheck  = "completeness_check"
	ErrorTypePerformanceTimeout = "performance_timeout"
)

// NewEpicError creates a new EPIC-specific error
func NewEpicError(errorType, message, epicKey string, err error) *EpicError {
	return &EpicError{
		Type:    errorType,
		Message: message,
		EpicKey: epicKey,
		Err:     err,
	}
}

// IsNotFoundError checks if the error is a "not found" error
func IsNotFoundError(err error) bool {
	if epicErr, ok := err.(*EpicError); ok {
		return epicErr.Type == ErrorTypeNotFound
	}
	return false
}

// IsInvalidTypeError checks if the error is an "invalid type" error
func IsInvalidTypeError(err error) bool {
	if epicErr, ok := err.(*EpicError); ok {
		return epicErr.Type == ErrorTypeInvalidType
	}
	return false
}

// IsDiscoveryFailedError checks if the error is a "discovery failed" error
func IsDiscoveryFailedError(err error) bool {
	if epicErr, ok := err.(*EpicError); ok {
		return epicErr.Type == ErrorTypeDiscoveryFailed
	}
	return false
}
