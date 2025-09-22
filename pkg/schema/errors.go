package schema

import "fmt"

// SchemaError represents errors that occur during schema operations
type SchemaError struct {
	Type    string // Type of error (invalid_input, serialization_error, file_error, etc.)
	Message string // Human-readable error message
	Err     error  // Underlying error
	Context string // Additional context (file path, issue key, etc.)
}

func (e *SchemaError) Error() string {
	if e.Context != "" {
		return fmt.Sprintf("schema error (%s) for %s: %s", e.Type, e.Context, e.Message)
	}
	return fmt.Sprintf("schema error (%s): %s", e.Type, e.Message)
}

func (e *SchemaError) Unwrap() error {
	return e.Err
}

// IsSerializationError checks if the error is related to YAML serialization
func IsSerializationError(err error) bool {
	if schemaErr, ok := err.(*SchemaError); ok {
		return schemaErr.Type == "serialization_error" || schemaErr.Type == "deserialization_error"
	}
	return false
}

// IsFileError checks if the error is related to file operations
func IsFileError(err error) bool {
	if schemaErr, ok := err.(*SchemaError); ok {
		return schemaErr.Type == "file_error"
	}
	return false
}

// IsInvalidInputError checks if the error is related to invalid input
func IsInvalidInputError(err error) bool {
	if schemaErr, ok := err.(*SchemaError); ok {
		return schemaErr.Type == "invalid_input"
	}
	return false
}
