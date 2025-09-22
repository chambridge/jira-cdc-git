package client

import "fmt"

// ClientError represents errors that occur during JIRA client operations
type ClientError struct {
	Type    string // Type of error (authentication_error, api_error, etc.)
	Message string // Human-readable error message
	Err     error  // Underlying error
	Context string // Additional context (issue key, operation, etc.)
}

func (e *ClientError) Error() string {
	if e.Context != "" {
		return fmt.Sprintf("JIRA client error (%s) for %s: %s", e.Type, e.Context, e.Message)
	}
	return fmt.Sprintf("JIRA client error (%s): %s", e.Type, e.Message)
}

func (e *ClientError) Unwrap() error {
	return e.Err
}

// IsAuthenticationError checks if the error is related to authentication
func IsAuthenticationError(err error) bool {
	if clientErr, ok := err.(*ClientError); ok {
		return clientErr.Type == "authentication_error"
	}
	return false
}

// IsNotFoundError checks if the error is related to a resource not being found
func IsNotFoundError(err error) bool {
	if clientErr, ok := err.(*ClientError); ok {
		return clientErr.Type == "not_found"
	}
	return false
}

// IsAuthorizationError checks if the error is related to insufficient permissions
func IsAuthorizationError(err error) bool {
	if clientErr, ok := err.(*ClientError); ok {
		return clientErr.Type == "authorization_error"
	}
	return false
}
