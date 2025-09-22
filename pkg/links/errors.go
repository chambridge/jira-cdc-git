package links

import "fmt"

// LinkError represents an error that occurred during symbolic link operations
type LinkError struct {
	Type    string // Error type for categorization
	Message string // Human-readable error message
	Err     error  // Underlying error, if any
}

func (e *LinkError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

func (e *LinkError) Unwrap() error {
	return e.Err
}

// Common error types for symbolic link operations
const (
	ErrorTypeInvalidInput      = "invalid_input"
	ErrorTypeDirectoryCreation = "directory_creation_error"
	ErrorTypeLinkCreation      = "link_creation_error"
	ErrorTypeLinkRemoval       = "link_removal_error"
	ErrorTypeLinkNotFound      = "link_not_found"
	ErrorTypeLinkAccess        = "link_access_error"
	ErrorTypeNotSymbolicLink   = "not_symbolic_link"
	ErrorTypeBrokenLink        = "broken_link"
	ErrorTypeTargetAccess      = "target_access_error"
	ErrorTypeCleanup           = "cleanup_error"
)

// Helper functions for creating common errors

func NewInvalidInputError(message string) *LinkError {
	return &LinkError{
		Type:    ErrorTypeInvalidInput,
		Message: message,
	}
}

func NewDirectoryCreationError(path string, err error) *LinkError {
	return &LinkError{
		Type:    ErrorTypeDirectoryCreation,
		Message: fmt.Sprintf("failed to create directory: %s", path),
		Err:     err,
	}
}

func NewLinkCreationError(linkPath, targetPath string, err error) *LinkError {
	return &LinkError{
		Type:    ErrorTypeLinkCreation,
		Message: fmt.Sprintf("failed to create symbolic link: %s -> %s", linkPath, targetPath),
		Err:     err,
	}
}

func NewBrokenLinkError(linkPath string, err error) *LinkError {
	return &LinkError{
		Type:    ErrorTypeBrokenLink,
		Message: fmt.Sprintf("symbolic link target does not exist: %s", linkPath),
		Err:     err,
	}
}
