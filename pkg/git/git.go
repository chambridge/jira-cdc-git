package git

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/chambrid/jira-cdc-git/pkg/client"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// Repository defines the interface for Git operations
// This enables dependency injection and testing with mock implementations
type Repository interface {
	// Initialize creates a new Git repository if one doesn't exist
	Initialize(repoPath string) error

	// IsRepository checks if the given path is a Git repository
	IsRepository(repoPath string) bool

	// ValidateWorkingTree ensures the repository has no uncommitted changes
	ValidateWorkingTree(repoPath string) error

	// GetCurrentBranch returns the current branch name
	GetCurrentBranch(repoPath string) (string, error)

	// CommitIssueFile adds and commits a YAML issue file with conventional commit message
	CommitIssueFile(repoPath, filePath string, issue *client.Issue) error

	// GetRepositoryStatus returns the current status of the repository
	GetRepositoryStatus(repoPath string) (*RepositoryStatus, error)
}

// GitRepository implements Repository using go-git library
type GitRepository struct {
	// Author information for commits
	AuthorName  string
	AuthorEmail string
}

// RepositoryStatus represents the current status of a Git repository
type RepositoryStatus struct {
	IsClean        bool     // true if working tree has no uncommitted changes
	CurrentBranch  string   // name of the current branch
	UntrackedFiles []string // list of untracked files
	ModifiedFiles  []string // list of modified files
	StagedFiles    []string // list of staged files
}

// NewGitRepository creates a new Git repository manager
func NewGitRepository(authorName, authorEmail string) Repository {
	return &GitRepository{
		AuthorName:  authorName,
		AuthorEmail: authorEmail,
	}
}

// Initialize creates a new Git repository if one doesn't exist
func (g *GitRepository) Initialize(repoPath string) error {
	if repoPath == "" {
		return &GitError{
			Type:    "invalid_input",
			Message: "repository path cannot be empty",
		}
	}

	// Check if already a repository
	if g.IsRepository(repoPath) {
		return nil // Already initialized, nothing to do
	}

	// Ensure directory exists
	if err := os.MkdirAll(repoPath, 0755); err != nil {
		return &GitError{
			Type:    "filesystem_error",
			Message: fmt.Sprintf("failed to create directory: %s", repoPath),
			Err:     err,
		}
	}

	// Initialize Git repository
	_, err := git.PlainInit(repoPath, false)
	if err != nil {
		return &GitError{
			Type:    "git_operation_error",
			Message: "failed to initialize Git repository",
			Err:     err,
			Context: repoPath,
		}
	}

	return nil
}

// IsRepository checks if the given path is a Git repository
func (g *GitRepository) IsRepository(repoPath string) bool {
	_, err := git.PlainOpen(repoPath)
	return err == nil
}

// ValidateWorkingTree ensures the repository has no uncommitted changes
func (g *GitRepository) ValidateWorkingTree(repoPath string) error {
	status, err := g.GetRepositoryStatus(repoPath)
	if err != nil {
		return fmt.Errorf("failed to get repository status: %w", err)
	}

	if !status.IsClean {
		return &GitError{
			Type:    "dirty_working_tree",
			Message: "repository has uncommitted changes - please commit or stash changes before proceeding",
			Context: repoPath,
		}
	}

	return nil
}

// GetCurrentBranch returns the current branch name
func (g *GitRepository) GetCurrentBranch(repoPath string) (string, error) {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return "", &GitError{
			Type:    "repository_not_found",
			Message: "failed to open Git repository",
			Err:     err,
			Context: repoPath,
		}
	}

	ref, err := repo.Head()
	if err != nil {
		return "", &GitError{
			Type:    "git_operation_error",
			Message: "failed to get current branch",
			Err:     err,
			Context: repoPath,
		}
	}

	// Extract branch name from reference
	branchName := ref.Name().Short()
	return branchName, nil
}

// GetRepositoryStatus returns the current status of the repository
func (g *GitRepository) GetRepositoryStatus(repoPath string) (*RepositoryStatus, error) {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return nil, &GitError{
			Type:    "repository_not_found",
			Message: "failed to open Git repository",
			Err:     err,
			Context: repoPath,
		}
	}

	worktree, err := repo.Worktree()
	if err != nil {
		return nil, &GitError{
			Type:    "git_operation_error",
			Message: "failed to get working tree",
			Err:     err,
			Context: repoPath,
		}
	}

	status, err := worktree.Status()
	if err != nil {
		return nil, &GitError{
			Type:    "git_operation_error",
			Message: "failed to get repository status",
			Err:     err,
			Context: repoPath,
		}
	}

	// Get current branch
	currentBranch, err := g.GetCurrentBranch(repoPath)
	if err != nil {
		currentBranch = "unknown"
	}

	// Parse status
	repoStatus := &RepositoryStatus{
		IsClean:        status.IsClean(),
		CurrentBranch:  currentBranch,
		UntrackedFiles: make([]string, 0),
		ModifiedFiles:  make([]string, 0),
		StagedFiles:    make([]string, 0),
	}

	// Categorize files by status
	for file, fileStatus := range status {
		switch fileStatus.Staging {
		case git.Untracked:
			repoStatus.UntrackedFiles = append(repoStatus.UntrackedFiles, file)
		case git.Modified, git.Added:
			repoStatus.StagedFiles = append(repoStatus.StagedFiles, file)
		}

		switch fileStatus.Worktree {
		case git.Modified:
			repoStatus.ModifiedFiles = append(repoStatus.ModifiedFiles, file)
		}
	}

	return repoStatus, nil
}

// CommitIssueFile adds and commits a YAML issue file with conventional commit message
func (g *GitRepository) CommitIssueFile(repoPath, filePath string, issue *client.Issue) error {
	if issue == nil || issue.Key == "" {
		return &GitError{
			Type:    "invalid_input",
			Message: "issue cannot be nil and must have a key",
		}
	}

	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return &GitError{
			Type:    "repository_not_found",
			Message: "failed to open Git repository",
			Err:     err,
			Context: repoPath,
		}
	}

	worktree, err := repo.Worktree()
	if err != nil {
		return &GitError{
			Type:    "git_operation_error",
			Message: "failed to get working tree",
			Err:     err,
			Context: repoPath,
		}
	}

	// Convert absolute path to relative path for git operations
	relativeFilePath, err := filepath.Rel(repoPath, filePath)
	if err != nil {
		return &GitError{
			Type:    "filesystem_error",
			Message: "failed to convert file path to relative path",
			Err:     err,
			Context: filePath,
		}
	}

	// Add file to staging area
	_, err = worktree.Add(relativeFilePath)
	if err != nil {
		return &GitError{
			Type:    "git_operation_error",
			Message: fmt.Sprintf("failed to add file to staging area: %s", relativeFilePath),
			Err:     err,
			Context: repoPath,
		}
	}

	// Create conventional commit message
	commitMessage := g.formatConventionalCommitMessage(issue)

	// Create commit
	commit := &git.CommitOptions{
		Author: &object.Signature{
			Name:  g.AuthorName,
			Email: g.AuthorEmail,
			When:  time.Now(),
		},
	}

	_, err = worktree.Commit(commitMessage, commit)
	if err != nil {
		return &GitError{
			Type:    "git_operation_error",
			Message: "failed to create commit",
			Err:     err,
			Context: repoPath,
		}
	}

	return nil
}

// formatConventionalCommitMessage creates a conventional commit message for an issue
// Format: feat(PROJ): add issue PROJ-123 - Summary
//
// Body includes additional issue metadata
func (g *GitRepository) formatConventionalCommitMessage(issue *client.Issue) string {
	// Extract project key from issue key (e.g., "PROJ-123" -> "PROJ")
	projectKey := extractProjectKey(issue.Key)

	// Determine commit type based on issue type
	commitType := getCommitType(issue.IssueType)

	// Create commit subject line
	subject := fmt.Sprintf("%s(%s): add issue %s - %s",
		commitType, projectKey, issue.Key, issue.Summary)

	// Create commit body with issue metadata
	body := fmt.Sprintf(`
Issue Details:
- Type: %s
- Status: %s
- Priority: %s
- Assignee: %s
- Reporter: %s
- Created: %s
- Updated: %s

🤖 Generated with [Claude Code](https://claude.ai/code)

Co-Authored-By: Claude <noreply@anthropic.com>`,
		issue.IssueType,
		issue.Status.Name,
		issue.Priority,
		formatUserInfo(issue.Assignee),
		formatUserInfo(issue.Reporter),
		issue.Created,
		issue.Updated)

	return subject + body
}

// Helper functions

// extractProjectKey extracts the project key from an issue key
func extractProjectKey(issueKey string) string {
	// Find the first dash to separate project key from issue number
	for i, char := range issueKey {
		if char == '-' {
			return issueKey[:i]
		}
	}
	return issueKey // fallback if no dash found
}

// getCommitType maps JIRA issue types to conventional commit types
func getCommitType(issueType string) string {
	switch issueType {
	case "Bug":
		return "fix"
	case "Story", "Task", "Epic":
		return "feat"
	case "Improvement", "Enhancement":
		return "feat"
	case "Documentation":
		return "docs"
	case "Test":
		return "test"
	default:
		return "feat" // default to feature
	}
}

// formatUserInfo formats user information for commit messages
func formatUserInfo(user client.User) string {
	if user.Name == "" && user.Email == "" {
		return "Unassigned"
	}
	if user.Email == "" {
		return user.Name
	}
	if user.Name == "" {
		return user.Email
	}
	return fmt.Sprintf("%s <%s>", user.Name, user.Email)
}
