package git

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/chambrid/jira-cdc-git/pkg/client"
)

// Integration tests that work with real Git repositories

func TestGitRepository_Integration_Initialize(t *testing.T) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "git-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	repo := NewGitRepository("Test User", "test@example.com")

	// Test initialization
	err = repo.Initialize(tempDir)
	if err != nil {
		t.Fatalf("Failed to initialize repository: %v", err)
	}

	// Verify it's now a Git repository
	if !repo.IsRepository(tempDir) {
		t.Error("Expected directory to be a Git repository after initialization")
	}

	// Test double initialization (should not error)
	err = repo.Initialize(tempDir)
	if err != nil {
		t.Errorf("Second initialization should not error: %v", err)
	}
}

func TestGitRepository_Integration_RepositoryStatus(t *testing.T) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "git-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	repo := NewGitRepository("Test User", "test@example.com")

	// Initialize repository
	err = repo.Initialize(tempDir)
	if err != nil {
		t.Fatalf("Failed to initialize repository: %v", err)
	}

	// Get status of clean repository
	status, err := repo.GetRepositoryStatus(tempDir)
	if err != nil {
		t.Fatalf("Failed to get repository status: %v", err)
	}

	if !status.IsClean {
		t.Error("Expected clean repository to be clean")
	}

	// Note: Newly initialized repositories don't have a current branch until first commit
	// So we expect the branch to be "unknown" initially
	if status.CurrentBranch != "unknown" {
		t.Logf("Current branch: %s (expected 'unknown' for new repo)", status.CurrentBranch)
	}

	// Test getting current branch - should fail for new repository
	_, err = repo.GetCurrentBranch(tempDir)
	if err == nil {
		t.Error("Expected error getting current branch from new repository")
	}
}

func TestGitRepository_Integration_ValidateWorkingTree(t *testing.T) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "git-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	repo := NewGitRepository("Test User", "test@example.com")

	// Initialize repository
	err = repo.Initialize(tempDir)
	if err != nil {
		t.Fatalf("Failed to initialize repository: %v", err)
	}

	// Clean repository should pass validation
	err = repo.ValidateWorkingTree(tempDir)
	if err != nil {
		t.Errorf("Clean repository should pass validation: %v", err)
	}

	// Create a file to make repository dirty
	testFile := filepath.Join(tempDir, "test.txt")
	err = os.WriteFile(testFile, []byte("test content"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Repository with untracked files should fail validation
	// (go-git considers untracked files as making the working tree not clean)
	err = repo.ValidateWorkingTree(tempDir)
	if err == nil {
		t.Error("Repository with untracked files should fail validation")
	}

	// Verify it's the correct error type
	if !IsDirtyWorkingTreeError(err) {
		t.Errorf("Expected dirty working tree error, got: %v", err)
	}
}

func TestGitRepository_Integration_CommitIssueFile(t *testing.T) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "git-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	repo := NewGitRepository("Test User", "test@example.com")

	// Initialize repository
	err = repo.Initialize(tempDir)
	if err != nil {
		t.Fatalf("Failed to initialize repository: %v", err)
	}

	// Create test issue
	issue := &client.Issue{
		Key:       "TEST-123",
		Summary:   "Integration test issue",
		IssueType: "Story",
		Status:    client.Status{Name: "In Progress"},
		Priority:  "Medium",
		Assignee:  client.User{Name: "John Doe", Email: "john@example.com"},
		Reporter:  client.User{Name: "Jane Smith", Email: "jane@example.com"},
		Created:   time.Now().UTC().Format(time.RFC3339),
		Updated:   time.Now().UTC().Format(time.RFC3339),
	}

	// Create test file
	testFile := filepath.Join(tempDir, "test-issue.yaml")
	err = os.WriteFile(testFile, []byte("key: TEST-123\nsummary: Integration test issue"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Commit the file
	err = repo.CommitIssueFile(tempDir, testFile, issue)
	if err != nil {
		t.Fatalf("Failed to commit issue file: %v", err)
	}

	// Verify repository is still clean after commit
	status, err := repo.GetRepositoryStatus(tempDir)
	if err != nil {
		t.Fatalf("Failed to get repository status: %v", err)
	}

	if !status.IsClean {
		t.Error("Expected repository to be clean after commit")
	}

	// Validate working tree should pass
	err = repo.ValidateWorkingTree(tempDir)
	if err != nil {
		t.Errorf("Working tree validation should pass after commit: %v", err)
	}
}

func TestGitRepository_Integration_MultipleCommits(t *testing.T) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "git-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	repo := NewGitRepository("Test User", "test@example.com")

	// Initialize repository
	err = repo.Initialize(tempDir)
	if err != nil {
		t.Fatalf("Failed to initialize repository: %v", err)
	}

	// Create and commit multiple issues
	issues := []*client.Issue{
		{
			Key:       "TEST-100",
			Summary:   "First test issue",
			IssueType: "Bug",
			Status:    client.Status{Name: "To Do"},
			Priority:  "High",
		},
		{
			Key:       "TEST-200",
			Summary:   "Second test issue",
			IssueType: "Story",
			Status:    client.Status{Name: "In Progress"},
			Priority:  "Medium",
		},
		{
			Key:       "TEST-300",
			Summary:   "Third test issue",
			IssueType: "Task",
			Status:    client.Status{Name: "Done"},
			Priority:  "Low",
		},
	}

	for i, issue := range issues {
		// Create test file
		testFile := filepath.Join(tempDir, issue.Key+".yaml")
		content := "key: " + issue.Key + "\nsummary: " + issue.Summary
		err = os.WriteFile(testFile, []byte(content), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file %d: %v", i, err)
		}

		// Commit the file
		err = repo.CommitIssueFile(tempDir, testFile, issue)
		if err != nil {
			t.Fatalf("Failed to commit issue file %d: %v", i, err)
		}

		// Verify repository is clean after each commit
		status, err := repo.GetRepositoryStatus(tempDir)
		if err != nil {
			t.Fatalf("Failed to get repository status after commit %d: %v", i, err)
		}

		if !status.IsClean {
			t.Errorf("Expected repository to be clean after commit %d", i)
		}
	}
}

func TestGitRepository_Integration_ConventionalCommitFormat(t *testing.T) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "git-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	repo := NewGitRepository("Test User", "test@example.com")

	// Initialize repository
	err = repo.Initialize(tempDir)
	if err != nil {
		t.Fatalf("Failed to initialize repository: %v", err)
	}

	// Test different issue types generate correct commit types
	testCases := []struct {
		issueType      string
		summary        string
		expectedCommit string
	}{
		{"Bug", "Bug fix issue", "fix(TEST): add issue TEST-123 - Bug fix issue"},
		{"Story", "Story issue", "feat(TEST): add issue TEST-123 - Story issue"},
		{"Task", "Task issue", "feat(TEST): add issue TEST-123 - Task issue"},
		{"Documentation", "Documentation issue", "docs(TEST): add issue TEST-123 - Documentation issue"},
		{"Test", "Test issue", "test(TEST): add issue TEST-123 - Test issue"},
	}

	for i, tc := range testCases {
		issue := &client.Issue{
			Key:       "TEST-123",
			Summary:   tc.summary,
			IssueType: tc.issueType,
			Status:    client.Status{Name: "To Do"},
			Priority:  "Medium",
		}

		// Test the commit message format
		gitRepo := repo.(*GitRepository)
		message := gitRepo.formatConventionalCommitMessage(issue)

		// Check that the subject line matches expected format
		if !contains(message, tc.expectedCommit) {
			t.Errorf("Test case %d: Expected commit message to contain '%s', got: %s", i, tc.expectedCommit, message)
		}

		// Verify body contains issue details
		expectedContent := []string{
			"Issue Details:",
			"Type: " + tc.issueType,
			"Status: To Do",
			"Priority: Medium",
			"🤖 Generated with [Claude Code]",
			"Co-Authored-By: Claude",
		}

		for _, expected := range expectedContent {
			if !contains(message, expected) {
				t.Errorf("Test case %d: Expected commit message to contain '%s', got: %s", i, expected, message)
			}
		}
	}
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			containsAt(s, substr, 1)))
}

func containsAt(s, substr string, start int) bool {
	if start >= len(s) {
		return false
	}
	if start+len(substr) > len(s) {
		return containsAt(s, substr, start+1)
	}
	if s[start:start+len(substr)] == substr {
		return true
	}
	return containsAt(s, substr, start+1)
}
