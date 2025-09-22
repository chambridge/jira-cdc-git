package git

import (
	"strings"
	"testing"

	"github.com/chambrid/jira-cdc-git/pkg/client"
)

func TestNewGitRepository(t *testing.T) {
	repo := NewGitRepository("Test User", "test@example.com")

	if repo == nil {
		t.Fatal("Expected repository to be created, got nil")
	}

	gitRepo, ok := repo.(*GitRepository)
	if !ok {
		t.Fatalf("Expected *GitRepository, got %T", repo)
	}

	if gitRepo.AuthorName != "Test User" {
		t.Errorf("Expected author name 'Test User', got '%s'", gitRepo.AuthorName)
	}

	if gitRepo.AuthorEmail != "test@example.com" {
		t.Errorf("Expected author email 'test@example.com', got '%s'", gitRepo.AuthorEmail)
	}
}

func TestExtractProjectKey(t *testing.T) {
	tests := []struct {
		name     string
		issueKey string
		expected string
	}{
		{"standard key", "PROJ-123", "PROJ"},
		{"complex key", "MY-PROJECT-456", "MY"},
		{"single part", "PROJ", "PROJ"},
		{"empty key", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractProjectKey(tt.issueKey)
			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestGetCommitType(t *testing.T) {
	tests := []struct {
		name      string
		issueType string
		expected  string
	}{
		{"bug", "Bug", "fix"},
		{"story", "Story", "feat"},
		{"task", "Task", "feat"},
		{"epic", "Epic", "feat"},
		{"improvement", "Improvement", "feat"},
		{"enhancement", "Enhancement", "feat"},
		{"documentation", "Documentation", "docs"},
		{"test", "Test", "test"},
		{"unknown", "Unknown", "feat"},
		{"empty", "", "feat"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getCommitType(tt.issueType)
			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestFormatUserInfo(t *testing.T) {
	tests := []struct {
		name     string
		user     client.User
		expected string
	}{
		{
			name:     "full user info",
			user:     client.User{Name: "John Doe", Email: "john@example.com"},
			expected: "John Doe <john@example.com>",
		},
		{
			name:     "name only",
			user:     client.User{Name: "John Doe", Email: ""},
			expected: "John Doe",
		},
		{
			name:     "email only",
			user:     client.User{Name: "", Email: "john@example.com"},
			expected: "john@example.com",
		},
		{
			name:     "empty user",
			user:     client.User{Name: "", Email: ""},
			expected: "Unassigned",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatUserInfo(tt.user)
			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestGitRepository_FormatConventionalCommitMessage(t *testing.T) {
	repo := &GitRepository{
		AuthorName:  "Test User",
		AuthorEmail: "test@example.com",
	}

	issue := &client.Issue{
		Key:       "PROJ-123",
		Summary:   "Test issue summary",
		IssueType: "Story",
		Status:    client.Status{Name: "In Progress"},
		Priority:  "High",
		Assignee:  client.User{Name: "John Doe", Email: "john@example.com"},
		Reporter:  client.User{Name: "Jane Smith", Email: "jane@example.com"},
		Created:   "2024-01-01T10:00:00Z",
		Updated:   "2024-01-02T15:30:00Z",
	}

	message := repo.formatConventionalCommitMessage(issue)

	// Verify subject line
	lines := strings.Split(message, "\n")
	subject := lines[0]
	expectedSubject := "feat(PROJ): add issue PROJ-123 - Test issue summary"
	if subject != expectedSubject {
		t.Errorf("Expected subject '%s', got '%s'", expectedSubject, subject)
	}

	// Verify body contains expected information
	expectedContent := []string{
		"Issue Details:",
		"Type: Story",
		"Status: In Progress",
		"Priority: High",
		"Assignee: John Doe <john@example.com>",
		"Reporter: Jane Smith <jane@example.com>",
		"Created: 2024-01-01T10:00:00Z",
		"Updated: 2024-01-02T15:30:00Z",
		"🤖 Generated with [Claude Code]",
		"Co-Authored-By: Claude",
	}

	for _, expected := range expectedContent {
		if !strings.Contains(message, expected) {
			t.Errorf("Expected commit message to contain '%s', but it didn't. Message:\n%s", expected, message)
		}
	}
}

func TestGitError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *GitError
		expected string
	}{
		{
			name: "error with context",
			err: &GitError{
				Type:    "repository_not_found",
				Message: "repository not found",
				Context: "/path/to/repo",
			},
			expected: "git error (repository_not_found) for /path/to/repo: repository not found",
		},
		{
			name: "error without context",
			err: &GitError{
				Type:    "invalid_input",
				Message: "input is invalid",
			},
			expected: "git error (invalid_input): input is invalid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.err.Error()
			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestErrorTypeCheckers(t *testing.T) {
	repoNotFoundErr := &GitError{Type: "repository_not_found"}
	dirtyTreeErr := &GitError{Type: "dirty_working_tree"}
	gitOpErr := &GitError{Type: "git_operation_error"}
	filesystemErr := &GitError{Type: "filesystem_error"}
	invalidInputErr := &GitError{Type: "invalid_input"}
	otherErr := &GitError{Type: "other"}

	// Test IsRepositoryNotFoundError
	if !IsRepositoryNotFoundError(repoNotFoundErr) {
		t.Error("Expected repository not found error to be detected")
	}
	if IsRepositoryNotFoundError(otherErr) {
		t.Error("Expected other error to not be detected as repository not found")
	}

	// Test IsDirtyWorkingTreeError
	if !IsDirtyWorkingTreeError(dirtyTreeErr) {
		t.Error("Expected dirty working tree error to be detected")
	}
	if IsDirtyWorkingTreeError(otherErr) {
		t.Error("Expected other error to not be detected as dirty working tree")
	}

	// Test IsGitOperationError
	if !IsGitOperationError(gitOpErr) {
		t.Error("Expected git operation error to be detected")
	}
	if IsGitOperationError(otherErr) {
		t.Error("Expected other error to not be detected as git operation")
	}

	// Test IsFilesystemError
	if !IsFilesystemError(filesystemErr) {
		t.Error("Expected filesystem error to be detected")
	}
	if IsFilesystemError(otherErr) {
		t.Error("Expected other error to not be detected as filesystem")
	}

	// Test IsInvalidInputError
	if !IsInvalidInputError(invalidInputErr) {
		t.Error("Expected invalid input error to be detected")
	}
	if IsInvalidInputError(otherErr) {
		t.Error("Expected other error to not be detected as invalid input")
	}
}

func TestGitRepository_Initialize(t *testing.T) {
	tests := []struct {
		name        string
		repoPath    string
		expectError bool
		errorType   string
		setupFunc   func(string)
		cleanupFunc func(string)
	}{
		{
			name:        "empty path",
			repoPath:    "",
			expectError: true,
			errorType:   "invalid_input",
		},
		{
			name:        "invalid path",
			repoPath:    "/invalid/\x00/path",
			expectError: true,
			errorType:   "filesystem_error",
		},
	}

	repo := NewGitRepository("Test User", "test@example.com")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setupFunc != nil {
				tt.setupFunc(tt.repoPath)
			}

			if tt.cleanupFunc != nil {
				defer tt.cleanupFunc(tt.repoPath)
			}

			err := repo.Initialize(tt.repoPath)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
					return
				}

				gitErr, ok := err.(*GitError)
				if !ok {
					t.Errorf("Expected GitError but got %T", err)
					return
				}

				if gitErr.Type != tt.errorType {
					t.Errorf("Expected error type '%s', got '%s'", tt.errorType, gitErr.Type)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}

func TestGitRepository_IsRepository(t *testing.T) {
	repo := NewGitRepository("Test User", "test@example.com")

	// Test non-existent path
	if repo.IsRepository("/non/existent/path") {
		t.Error("Expected non-existent path to not be a repository")
	}

	// Test current directory (should not be a Git repository)
	if repo.IsRepository(".") {
		t.Error("Expected current directory to not be a Git repository")
	}
}

func TestGitRepository_ValidateWorkingTree(t *testing.T) {
	repo := NewGitRepository("Test User", "test@example.com")

	// Test non-existent repository
	err := repo.ValidateWorkingTree("/non/existent/path")
	if err == nil {
		t.Error("Expected error for non-existent repository")
	}
}

func TestGitRepository_GetCurrentBranch(t *testing.T) {
	repo := NewGitRepository("Test User", "test@example.com")

	// Test non-existent repository
	_, err := repo.GetCurrentBranch("/non/existent/path")
	if err == nil {
		t.Error("Expected error for non-existent repository")
	}

	gitErr, ok := err.(*GitError)
	if !ok {
		t.Errorf("Expected GitError but got %T", err)
	} else if gitErr.Type != "repository_not_found" {
		t.Errorf("Expected error type 'repository_not_found', got '%s'", gitErr.Type)
	}
}

func TestGitRepository_GetRepositoryStatus(t *testing.T) {
	repo := NewGitRepository("Test User", "test@example.com")

	// Test non-existent repository
	_, err := repo.GetRepositoryStatus("/non/existent/path")
	if err == nil {
		t.Error("Expected error for non-existent repository")
	}

	gitErr, ok := err.(*GitError)
	if !ok {
		t.Errorf("Expected GitError but got %T", err)
	} else if gitErr.Type != "repository_not_found" {
		t.Errorf("Expected error type 'repository_not_found', got '%s'", gitErr.Type)
	}
}

func TestGitRepository_CommitIssueFile(t *testing.T) {
	repo := NewGitRepository("Test User", "test@example.com")

	// Test nil issue
	err := repo.CommitIssueFile("/some/path", "/some/file", nil)
	if err == nil {
		t.Error("Expected error for nil issue")
	}

	gitErr, ok := err.(*GitError)
	if !ok {
		t.Errorf("Expected GitError but got %T", err)
	} else if gitErr.Type != "invalid_input" {
		t.Errorf("Expected error type 'invalid_input', got '%s'", gitErr.Type)
	}

	// Test issue with empty key
	emptyKeyIssue := &client.Issue{Key: ""}
	err = repo.CommitIssueFile("/some/path", "/some/file", emptyKeyIssue)
	if err == nil {
		t.Error("Expected error for issue with empty key")
	}

	gitErr, ok = err.(*GitError)
	if !ok {
		t.Errorf("Expected GitError but got %T", err)
	} else if gitErr.Type != "invalid_input" {
		t.Errorf("Expected error type 'invalid_input', got '%s'", gitErr.Type)
	}

	// Test non-existent repository
	validIssue := &client.Issue{
		Key:       "PROJ-123",
		Summary:   "Test issue",
		IssueType: "Story",
	}
	err = repo.CommitIssueFile("/non/existent/path", "/some/file", validIssue)
	if err == nil {
		t.Error("Expected error for non-existent repository")
	}

	gitErr, ok = err.(*GitError)
	if !ok {
		t.Errorf("Expected GitError but got %T", err)
	} else if gitErr.Type != "repository_not_found" {
		t.Errorf("Expected error type 'repository_not_found', got '%s'", gitErr.Type)
	}
}
