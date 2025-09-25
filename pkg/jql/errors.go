package jql

import "fmt"

// JQLError represents errors that occur during JQL operations
type JQLError struct {
	Type    string                 `json:"type" yaml:"type"`
	Message string                 `json:"message" yaml:"message"`
	Context map[string]interface{} `json:"context,omitempty" yaml:"context,omitempty"`
	Err     error                  `json:"-" yaml:"-"`
}

func (e *JQLError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("JQL %s: %s (%v)", e.Type, e.Message, e.Err)
	}
	return fmt.Sprintf("JQL %s: %s", e.Type, e.Message)
}

func (e *JQLError) Unwrap() error {
	return e.Err
}

// JQL error types
const (
	ErrorTypeValidation       = "validation_error"
	ErrorTypeTemplate         = "template_error"
	ErrorTypeQuery            = "query_error"
	ErrorTypePreview          = "preview_error"
	ErrorTypeSavedQuery       = "saved_query_error"
	ErrorTypeOptimization     = "optimization_error"
	ErrorTypeEpicAnalysis     = "epic_analysis_error"
	ErrorTypeParameterMissing = "parameter_missing_error"
	ErrorTypeTemplateNotFound = "template_not_found_error"
	ErrorTypeFilesystem       = "filesystem_error"
)

// NewValidationError creates a validation error
func NewValidationError(message string, jql string) *JQLError {
	return &JQLError{
		Type:    ErrorTypeValidation,
		Message: message,
		Context: map[string]interface{}{
			"jql": jql,
		},
	}
}

// NewTemplateError creates a template error
func NewTemplateError(message string, templateName string, err error) *JQLError {
	return &JQLError{
		Type:    ErrorTypeTemplate,
		Message: message,
		Context: map[string]interface{}{
			"template": templateName,
		},
		Err: err,
	}
}

// NewQueryError creates a query error
func NewQueryError(message string, jql string, err error) *JQLError {
	return &JQLError{
		Type:    ErrorTypeQuery,
		Message: message,
		Context: map[string]interface{}{
			"jql": jql,
		},
		Err: err,
	}
}

// NewPreviewError creates a preview error
func NewPreviewError(message string, jql string, err error) *JQLError {
	return &JQLError{
		Type:    ErrorTypePreview,
		Message: message,
		Context: map[string]interface{}{
			"jql": jql,
		},
		Err: err,
	}
}

// NewSavedQueryError creates a saved query error
func NewSavedQueryError(message string, queryName string, err error) *JQLError {
	return &JQLError{
		Type:    ErrorTypeSavedQuery,
		Message: message,
		Context: map[string]interface{}{
			"query_name": queryName,
		},
		Err: err,
	}
}

// NewOptimizationError creates an optimization error
func NewOptimizationError(message string, jql string, err error) *JQLError {
	return &JQLError{
		Type:    ErrorTypeOptimization,
		Message: message,
		Context: map[string]interface{}{
			"jql": jql,
		},
		Err: err,
	}
}

// NewEpicAnalysisError creates an epic analysis error
func NewEpicAnalysisError(message string, epicKey string, err error) *JQLError {
	return &JQLError{
		Type:    ErrorTypeEpicAnalysis,
		Message: message,
		Context: map[string]interface{}{
			"epic_key": epicKey,
		},
		Err: err,
	}
}

// NewParameterMissingError creates a parameter missing error
func NewParameterMissingError(parameterName string, templateName string) *JQLError {
	return &JQLError{
		Type:    ErrorTypeParameterMissing,
		Message: fmt.Sprintf("required parameter '%s' missing", parameterName),
		Context: map[string]interface{}{
			"parameter": parameterName,
			"template":  templateName,
		},
	}
}

// NewTemplateNotFoundError creates a template not found error
func NewTemplateNotFoundError(templateName string) *JQLError {
	return &JQLError{
		Type:    ErrorTypeTemplateNotFound,
		Message: fmt.Sprintf("template '%s' not found", templateName),
		Context: map[string]interface{}{
			"template": templateName,
		},
	}
}

// NewFilesystemError creates a filesystem error
func NewFilesystemError(message string, filePath string, err error) *JQLError {
	return &JQLError{
		Type:    ErrorTypeFilesystem,
		Message: message,
		Context: map[string]interface{}{
			"file_path": filePath,
		},
		Err: err,
	}
}

// IsValidationError checks if the error is a validation error
func IsValidationError(err error) bool {
	if jqlErr, ok := err.(*JQLError); ok {
		return jqlErr.Type == ErrorTypeValidation
	}
	return false
}

// IsTemplateError checks if the error is a template error
func IsTemplateError(err error) bool {
	if jqlErr, ok := err.(*JQLError); ok {
		return jqlErr.Type == ErrorTypeTemplate || jqlErr.Type == ErrorTypeTemplateNotFound
	}
	return false
}

// IsQueryError checks if the error is a query error
func IsQueryError(err error) bool {
	if jqlErr, ok := err.(*JQLError); ok {
		return jqlErr.Type == ErrorTypeQuery
	}
	return false
}

// IsPreviewError checks if the error is a preview error
func IsPreviewError(err error) bool {
	if jqlErr, ok := err.(*JQLError); ok {
		return jqlErr.Type == ErrorTypePreview
	}
	return false
}

// IsSavedQueryError checks if the error is a saved query error
func IsSavedQueryError(err error) bool {
	if jqlErr, ok := err.(*JQLError); ok {
		return jqlErr.Type == ErrorTypeSavedQuery
	}
	return false
}

// IsEpicAnalysisError checks if the error is an epic analysis error
func IsEpicAnalysisError(err error) bool {
	if jqlErr, ok := err.(*JQLError); ok {
		return jqlErr.Type == ErrorTypeEpicAnalysis
	}
	return false
}

// GetErrorContext extracts context information from a JQL error
func GetErrorContext(err error) map[string]interface{} {
	if jqlErr, ok := err.(*JQLError); ok {
		return jqlErr.Context
	}
	return nil
}
