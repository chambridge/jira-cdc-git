package jobs

import (
	"fmt"
	"time"
)

// JobErrorType represents different types of job errors
type JobErrorType string

const (
	ErrorTypeValidation     JobErrorType = "validation"
	ErrorTypeAuthentication JobErrorType = "authentication"
	ErrorTypeConnection     JobErrorType = "connection"
	ErrorTypeKubernetes     JobErrorType = "kubernetes"
	ErrorTypeTimeout        JobErrorType = "timeout"
	ErrorTypeResource       JobErrorType = "resource"
	ErrorTypeTemplate       JobErrorType = "template"
	ErrorTypeExecution      JobErrorType = "execution"
	ErrorTypeInternal       JobErrorType = "internal"
)

// ValidationError represents job configuration validation errors
type ValidationError struct {
	JobID   string
	Field   string
	Value   interface{}
	Message string
	Time    time.Time
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error for job %s: field '%s' with value '%v': %s",
		e.JobID, e.Field, e.Value, e.Message)
}

func (e *ValidationError) Type() JobErrorType {
	return ErrorTypeValidation
}

// AuthenticationError represents JIRA authentication errors
type AuthenticationError struct {
	JobID    string
	Endpoint string
	Message  string
	Time     time.Time
}

func (e *AuthenticationError) Error() string {
	return fmt.Sprintf("authentication error for job %s at %s: %s",
		e.JobID, e.Endpoint, e.Message)
}

func (e *AuthenticationError) Type() JobErrorType {
	return ErrorTypeAuthentication
}

// KubernetesError represents Kubernetes API errors
type KubernetesError struct {
	JobID     string
	Operation string
	Resource  string
	Message   string
	Time      time.Time
}

func (e *KubernetesError) Error() string {
	return fmt.Sprintf("kubernetes error for job %s during %s of %s: %s",
		e.JobID, e.Operation, e.Resource, e.Message)
}

func (e *KubernetesError) Type() JobErrorType {
	return ErrorTypeKubernetes
}

// TimeoutError represents job timeout errors
type TimeoutError struct {
	JobID         string
	TimeoutPeriod time.Duration
	ElapsedTime   time.Duration
	Message       string
	Time          time.Time
}

func (e *TimeoutError) Error() string {
	return fmt.Sprintf("timeout error for job %s after %v (limit: %v): %s",
		e.JobID, e.ElapsedTime, e.TimeoutPeriod, e.Message)
}

func (e *TimeoutError) Type() JobErrorType {
	return ErrorTypeTimeout
}

// ResourceError represents resource-related errors (CPU, memory, storage)
type ResourceError struct {
	JobID        string
	ResourceType string
	Requested    string
	Available    string
	Message      string
	Time         time.Time
}

func (e *ResourceError) Error() string {
	return fmt.Sprintf("resource error for job %s: %s resource (requested: %s, available: %s): %s",
		e.JobID, e.ResourceType, e.Requested, e.Available, e.Message)
}

func (e *ResourceError) Type() JobErrorType {
	return ErrorTypeResource
}

// TemplateError represents job template errors
type TemplateError struct {
	JobID        string
	JobType      JobType
	TemplatePath string
	Message      string
	Time         time.Time
}

func (e *TemplateError) Error() string {
	return fmt.Sprintf("template error for job %s (type: %s, template: %s): %s",
		e.JobID, e.JobType, e.TemplatePath, e.Message)
}

func (e *TemplateError) Type() JobErrorType {
	return ErrorTypeTemplate
}

// ExecutionError represents job execution errors
type ExecutionError struct {
	JobID     string
	PodName   string
	Container string
	ExitCode  int32
	Message   string
	Logs      string
	Time      time.Time
}

func (e *ExecutionError) Error() string {
	return fmt.Sprintf("execution error for job %s (pod: %s, container: %s, exit: %d): %s",
		e.JobID, e.PodName, e.Container, e.ExitCode, e.Message)
}

func (e *ExecutionError) Type() JobErrorType {
	return ErrorTypeExecution
}

// ConnectionError represents connection-related errors
type ConnectionError struct {
	JobID    string
	Target   string
	Protocol string
	Message  string
	Time     time.Time
}

func (e *ConnectionError) Error() string {
	return fmt.Sprintf("connection error for job %s to %s (%s): %s",
		e.JobID, e.Target, e.Protocol, e.Message)
}

func (e *ConnectionError) Type() JobErrorType {
	return ErrorTypeConnection
}

// InternalError represents internal system errors
type InternalError struct {
	JobID     string
	Component string
	Operation string
	Message   string
	Time      time.Time
}

func (e *InternalError) Error() string {
	return fmt.Sprintf("internal error for job %s in %s during %s: %s",
		e.JobID, e.Component, e.Operation, e.Message)
}

func (e *InternalError) Type() JobErrorType {
	return ErrorTypeInternal
}

// JobErrorWithType defines interface for job errors with types
type JobErrorWithType interface {
	error
	Type() JobErrorType
}

// NewValidationError creates a new validation error
func NewValidationError(jobID, field string, value interface{}, message string) *ValidationError {
	return &ValidationError{
		JobID:   jobID,
		Field:   field,
		Value:   value,
		Message: message,
		Time:    time.Now(),
	}
}

// NewAuthenticationError creates a new authentication error
func NewAuthenticationError(jobID, endpoint, message string) *AuthenticationError {
	return &AuthenticationError{
		JobID:    jobID,
		Endpoint: endpoint,
		Message:  message,
		Time:     time.Now(),
	}
}

// NewKubernetesError creates a new Kubernetes error
func NewKubernetesError(jobID, operation, resource, message string) *KubernetesError {
	return &KubernetesError{
		JobID:     jobID,
		Operation: operation,
		Resource:  resource,
		Message:   message,
		Time:      time.Now(),
	}
}

// NewTimeoutError creates a new timeout error
func NewTimeoutError(jobID string, timeoutPeriod, elapsedTime time.Duration, message string) *TimeoutError {
	return &TimeoutError{
		JobID:         jobID,
		TimeoutPeriod: timeoutPeriod,
		ElapsedTime:   elapsedTime,
		Message:       message,
		Time:          time.Now(),
	}
}

// NewResourceError creates a new resource error
func NewResourceError(jobID, resourceType, requested, available, message string) *ResourceError {
	return &ResourceError{
		JobID:        jobID,
		ResourceType: resourceType,
		Requested:    requested,
		Available:    available,
		Message:      message,
		Time:         time.Now(),
	}
}

// NewTemplateError creates a new template error
func NewTemplateError(jobID string, jobType JobType, templatePath, message string) *TemplateError {
	return &TemplateError{
		JobID:        jobID,
		JobType:      jobType,
		TemplatePath: templatePath,
		Message:      message,
		Time:         time.Now(),
	}
}

// NewExecutionError creates a new execution error
func NewExecutionError(jobID, podName, container string, exitCode int32, message, logs string) *ExecutionError {
	return &ExecutionError{
		JobID:     jobID,
		PodName:   podName,
		Container: container,
		ExitCode:  exitCode,
		Message:   message,
		Logs:      logs,
		Time:      time.Now(),
	}
}

// NewConnectionError creates a new connection error
func NewConnectionError(jobID, target, protocol, message string) *ConnectionError {
	return &ConnectionError{
		JobID:    jobID,
		Target:   target,
		Protocol: protocol,
		Message:  message,
		Time:     time.Now(),
	}
}

// NewInternalError creates a new internal error
func NewInternalError(jobID, component, operation, message string) *InternalError {
	return &InternalError{
		JobID:     jobID,
		Component: component,
		Operation: operation,
		Message:   message,
		Time:      time.Now(),
	}
}

// IsRetryableError determines if an error is retryable
func IsRetryableError(err error) bool {
	if jobErr, ok := err.(JobErrorWithType); ok {
		switch jobErr.Type() {
		case ErrorTypeConnection, ErrorTypeTimeout, ErrorTypeResource:
			return true
		case ErrorTypeKubernetes:
			// Some Kubernetes errors are retryable (e.g., temporary API server issues)
			return true
		default:
			return false
		}
	}
	return false
}

// GetErrorSeverity returns the severity level of an error
func GetErrorSeverity(err error) string {
	if jobErr, ok := err.(JobErrorWithType); ok {
		switch jobErr.Type() {
		case ErrorTypeValidation, ErrorTypeAuthentication, ErrorTypeTemplate:
			return "critical" // These require intervention
		case ErrorTypeTimeout, ErrorTypeResource:
			return "high" // These may resolve with retry
		case ErrorTypeConnection, ErrorTypeKubernetes:
			return "medium" // These may be transient
		case ErrorTypeExecution:
			return "high" // Depends on exit code, but generally serious
		case ErrorTypeInternal:
			return "critical" // System issues
		default:
			return "unknown"
		}
	}
	return "unknown"
}

// ErrorSummary provides a summary of error details
type ErrorSummary struct {
	Type        JobErrorType `json:"type"`
	Severity    string       `json:"severity"`
	Message     string       `json:"message"`
	Retryable   bool         `json:"retryable"`
	Timestamp   time.Time    `json:"timestamp"`
	JobID       string       `json:"job_id"`
	Suggestions []string     `json:"suggestions,omitempty"`
}

// SummarizeError creates an error summary
func SummarizeError(err error) *ErrorSummary {
	if err == nil {
		return nil
	}

	summary := &ErrorSummary{
		Message:   err.Error(),
		Retryable: IsRetryableError(err),
		Timestamp: time.Now(),
	}

	if jobErr, ok := err.(JobErrorWithType); ok {
		summary.Type = jobErr.Type()
		summary.Severity = GetErrorSeverity(err)

		// Extract job ID if available
		switch e := jobErr.(type) {
		case *ValidationError:
			summary.JobID = e.JobID
		case *AuthenticationError:
			summary.JobID = e.JobID
		case *KubernetesError:
			summary.JobID = e.JobID
		case *TimeoutError:
			summary.JobID = e.JobID
		case *ResourceError:
			summary.JobID = e.JobID
		case *TemplateError:
			summary.JobID = e.JobID
		case *ExecutionError:
			summary.JobID = e.JobID
		case *ConnectionError:
			summary.JobID = e.JobID
		case *InternalError:
			summary.JobID = e.JobID
		}

		// Add suggestions based on error type
		summary.Suggestions = getSuggestions(jobErr.Type())
	} else {
		summary.Type = ErrorTypeInternal
		summary.Severity = "unknown"
	}

	return summary
}

// getSuggestions provides remediation suggestions for different error types
func getSuggestions(errorType JobErrorType) []string {
	switch errorType {
	case ErrorTypeValidation:
		return []string{
			"Check job configuration parameters",
			"Verify required fields are provided",
			"Ensure values are in correct format",
		}
	case ErrorTypeAuthentication:
		return []string{
			"Verify JIRA credentials are correct",
			"Check if token has expired",
			"Ensure proper permissions are granted",
		}
	case ErrorTypeConnection:
		return []string{
			"Check network connectivity",
			"Verify JIRA server is accessible",
			"Check firewall and proxy settings",
		}
	case ErrorTypeKubernetes:
		return []string{
			"Check Kubernetes cluster status",
			"Verify namespace and permissions",
			"Check resource quotas",
		}
	case ErrorTypeTimeout:
		return []string{
			"Consider increasing timeout value",
			"Check if operation is taking longer than expected",
			"Verify target system performance",
		}
	case ErrorTypeResource:
		return []string{
			"Check cluster resource availability",
			"Consider reducing resource requests",
			"Scale cluster if needed",
		}
	case ErrorTypeTemplate:
		return []string{
			"Verify job template is valid",
			"Check template file exists and is readable",
			"Validate template syntax",
		}
	case ErrorTypeExecution:
		return []string{
			"Check job logs for detailed error information",
			"Verify container image and command",
			"Check volume mounts and permissions",
		}
	default:
		return []string{
			"Contact system administrator",
			"Check system logs for more details",
		}
	}
}
