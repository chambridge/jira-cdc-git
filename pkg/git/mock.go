package git

import (
	"github.com/chambrid/jira-cdc-git/pkg/client"
)

// MockRepository implements Repository for testing
type MockRepository struct {
	// Repositories tracks which paths are considered Git repositories
	Repositories map[string]bool

	// RepositoryStatus tracks the status of each repository
	RepositoryStatuses map[string]*RepositoryStatus

	// CommittedFiles tracks files that have been committed
	CommittedFiles map[string][]*CommitInfo

	// InitializeError simulates initialization failures when set
	InitializeError error

	// ValidateError simulates working tree validation failures when set
	ValidateError error

	// CommitError simulates commit failures when set
	CommitError error

	// CallCounts track method invocations
	InitializeCallCount       int
	IsRepositoryCallCount     int
	ValidateCallCount         int
	GetCurrentBranchCallCount int
	CommitCallCount           int

	// LastCommittedIssue tracks the last issue that was committed
	LastCommittedIssue *client.Issue
}

// CommitInfo represents information about a committed file
type CommitInfo struct {
	FilePath      string
	Issue         *client.Issue
	CommitMessage string
}

// NewMockRepository creates a new mock Git repository for testing
func NewMockRepository() *MockRepository {
	return &MockRepository{
		Repositories:       make(map[string]bool),
		RepositoryStatuses: make(map[string]*RepositoryStatus),
		CommittedFiles:     make(map[string][]*CommitInfo),
	}
}

// Initialize simulates Git repository initialization
func (m *MockRepository) Initialize(repoPath string) error {
	m.InitializeCallCount++

	// Simulate initialization error if configured
	if m.InitializeError != nil {
		return m.InitializeError
	}

	if repoPath == "" {
		return &GitError{
			Type:    "invalid_input",
			Message: "repository path cannot be empty",
		}
	}

	// Mark as initialized
	m.Repositories[repoPath] = true

	// Set default clean status
	if m.RepositoryStatuses[repoPath] == nil {
		m.RepositoryStatuses[repoPath] = &RepositoryStatus{
			IsClean:       true,
			CurrentBranch: "main",
		}
	}

	return nil
}

// IsRepository simulates checking if a path is a Git repository
func (m *MockRepository) IsRepository(repoPath string) bool {
	m.IsRepositoryCallCount++
	return m.Repositories[repoPath]
}

// ValidateWorkingTree simulates working tree validation
func (m *MockRepository) ValidateWorkingTree(repoPath string) error {
	m.ValidateCallCount++

	// Simulate validation error if configured
	if m.ValidateError != nil {
		return m.ValidateError
	}

	// Check if repository exists
	if !m.IsRepository(repoPath) {
		return &GitError{
			Type:    "repository_not_found",
			Message: "repository not found",
			Context: repoPath,
		}
	}

	// Check repository status
	status := m.RepositoryStatuses[repoPath]
	if status != nil && !status.IsClean {
		return &GitError{
			Type:    "dirty_working_tree",
			Message: "repository has uncommitted changes",
			Context: repoPath,
		}
	}

	return nil
}

// GetCurrentBranch simulates getting the current branch
func (m *MockRepository) GetCurrentBranch(repoPath string) (string, error) {
	m.GetCurrentBranchCallCount++

	// Check if repository exists
	if !m.IsRepository(repoPath) {
		return "", &GitError{
			Type:    "repository_not_found",
			Message: "repository not found",
			Context: repoPath,
		}
	}

	status := m.RepositoryStatuses[repoPath]
	if status != nil {
		return status.CurrentBranch, nil
	}

	return "main", nil // default branch
}

// GetRepositoryStatus simulates getting repository status
func (m *MockRepository) GetRepositoryStatus(repoPath string) (*RepositoryStatus, error) {
	// Check if repository exists
	if !m.IsRepository(repoPath) {
		return nil, &GitError{
			Type:    "repository_not_found",
			Message: "repository not found",
			Context: repoPath,
		}
	}

	// Return stored status or default clean status
	status := m.RepositoryStatuses[repoPath]
	if status == nil {
		status = &RepositoryStatus{
			IsClean:       true,
			CurrentBranch: "main",
		}
	}

	return status, nil
}

// CommitIssueFile simulates committing an issue file
func (m *MockRepository) CommitIssueFile(repoPath, filePath string, issue *client.Issue) error {
	m.CommitCallCount++
	m.LastCommittedIssue = issue

	// Simulate commit error if configured
	if m.CommitError != nil {
		return m.CommitError
	}

	if issue == nil || issue.Key == "" {
		return &GitError{
			Type:    "invalid_input",
			Message: "issue cannot be nil and must have a key",
		}
	}

	// Check if repository exists
	if !m.IsRepository(repoPath) {
		return &GitError{
			Type:    "repository_not_found",
			Message: "repository not found",
			Context: repoPath,
		}
	}

	// Simulate creating commit message
	commitMessage := m.formatConventionalCommitMessage(issue)

	// Track the committed file
	commitInfo := &CommitInfo{
		FilePath:      filePath,
		Issue:         issue,
		CommitMessage: commitMessage,
	}

	if m.CommittedFiles[repoPath] == nil {
		m.CommittedFiles[repoPath] = make([]*CommitInfo, 0)
	}
	m.CommittedFiles[repoPath] = append(m.CommittedFiles[repoPath], commitInfo)

	return nil
}

// Helper methods for testing

// SetRepositoryAsInitialized marks a path as a Git repository
func (m *MockRepository) SetRepositoryAsInitialized(repoPath string, clean bool) {
	m.Repositories[repoPath] = true
	m.RepositoryStatuses[repoPath] = &RepositoryStatus{
		IsClean:       clean,
		CurrentBranch: "main",
	}
}

// SetRepositoryStatus sets the status for a repository
func (m *MockRepository) SetRepositoryStatus(repoPath string, status *RepositoryStatus) {
	m.RepositoryStatuses[repoPath] = status
}

// SetInitializeError configures the mock to return an initialization error
func (m *MockRepository) SetInitializeError(err error) {
	m.InitializeError = err
}

// SetValidateError configures the mock to return a validation error
func (m *MockRepository) SetValidateError(err error) {
	m.ValidateError = err
}

// SetCommitError configures the mock to return a commit error
func (m *MockRepository) SetCommitError(err error) {
	m.CommitError = err
}

// GetCommittedFiles returns all files committed to a repository
func (m *MockRepository) GetCommittedFiles(repoPath string) []*CommitInfo {
	return m.CommittedFiles[repoPath]
}

// VerifyFileCommitted checks if a specific file was committed
func (m *MockRepository) VerifyFileCommitted(repoPath, filePath string) bool {
	commits := m.CommittedFiles[repoPath]
	for _, commit := range commits {
		if commit.FilePath == filePath {
			return true
		}
	}
	return false
}

// Reset clears all mock state for clean test setup
func (m *MockRepository) Reset() {
	m.Repositories = make(map[string]bool)
	m.RepositoryStatuses = make(map[string]*RepositoryStatus)
	m.CommittedFiles = make(map[string][]*CommitInfo)
	m.InitializeError = nil
	m.ValidateError = nil
	m.CommitError = nil
	m.InitializeCallCount = 0
	m.IsRepositoryCallCount = 0
	m.ValidateCallCount = 0
	m.GetCurrentBranchCallCount = 0
	m.CommitCallCount = 0
	m.LastCommittedIssue = nil
}

// formatConventionalCommitMessage simulates the commit message formatting
// This duplicates logic from git.go for testing consistency
func (m *MockRepository) formatConventionalCommitMessage(issue *client.Issue) string {
	projectKey := extractProjectKey(issue.Key)
	commitType := getCommitType(issue.IssueType)

	subject := commitType + "(" + projectKey + "): add issue " + issue.Key + " - " + issue.Summary

	body := "\n\nIssue Details:\n- Type: " + issue.IssueType +
		"\n- Status: " + issue.Status.Name +
		"\n- Priority: " + issue.Priority

	if issue.Assignee.Name != "" {
		body += "\n- Assignee: " + issue.Assignee.Name
	}
	if issue.Reporter.Name != "" {
		body += "\n- Reporter: " + issue.Reporter.Name
	}

	body += "\n\n🤖 Generated with [Claude Code](https://claude.ai/code)\n\nCo-Authored-By: Claude <noreply@anthropic.com>"

	return subject + body
}
