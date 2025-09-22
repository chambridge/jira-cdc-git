package git

import "fmt"

// GitError represents errors that occur during Git operations
type GitError struct {
	Type    string // Type of error (invalid_input, repository_not_found, git_operation_error, etc.)
	Message string // Human-readable error message
	Err     error  // Underlying error
	Context string // Additional context (repository path, file path, etc.)
}

func (e *GitError) Error() string {
	if e.Context != "" {
		return fmt.Sprintf("git error (%s) for %s: %s", e.Type, e.Context, e.Message)
	}
	return fmt.Sprintf("git error (%s): %s", e.Type, e.Message)
}

func (e *GitError) Unwrap() error {
	return e.Err
}

// IsRepositoryNotFoundError checks if the error is related to repository not being found
func IsRepositoryNotFoundError(err error) bool {
	if gitErr, ok := err.(*GitError); ok {
		return gitErr.Type == "repository_not_found"
	}
	return false
}

// IsDirtyWorkingTreeError checks if the error is related to uncommitted changes
func IsDirtyWorkingTreeError(err error) bool {
	if gitErr, ok := err.(*GitError); ok {
		return gitErr.Type == "dirty_working_tree"
	}
	return false
}

// IsGitOperationError checks if the error is related to Git operations
func IsGitOperationError(err error) bool {
	if gitErr, ok := err.(*GitError); ok {
		return gitErr.Type == "git_operation_error"
	}
	return false
}

// IsFilesystemError checks if the error is related to filesystem operations
func IsFilesystemError(err error) bool {
	if gitErr, ok := err.(*GitError); ok {
		return gitErr.Type == "filesystem_error"
	}
	return false
}

// IsInvalidInputError checks if the error is related to invalid input
func IsInvalidInputError(err error) bool {
	if gitErr, ok := err.(*GitError); ok {
		return gitErr.Type == "invalid_input"
	}
	return false
}
