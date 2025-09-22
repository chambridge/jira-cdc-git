package cli

import (
	"bytes"
	"os"
	"testing"

	"github.com/spf13/cobra"
)

func TestValidateIssueKey(t *testing.T) {
	tests := []struct {
		name      string
		issueKey  string
		expectErr bool
	}{
		{"valid simple key", "PROJ-123", false},
		{"valid complex key", "MY-PROJECT-456", false},
		{"valid multi-part key", "ABC-DEF-789", false},
		{"valid single char project", "A-123", false},
		{"empty key", "", true},
		{"invalid format - no number", "PROJ-", true},
		{"invalid format - no dash", "PROJ123", true},
		{"invalid format - lowercase", "proj-123", true},
		{"invalid format - starts with number", "123-PROJ", true},
		{"invalid format - special chars", "PROJ@-123", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateIssueKey(tt.issueKey)
			if tt.expectErr && err == nil {
				t.Errorf("Expected error for issue key '%s', but got none", tt.issueKey)
			}
			if !tt.expectErr && err != nil {
				t.Errorf("Expected no error for issue key '%s', but got: %v", tt.issueKey, err)
			}
		})
	}
}

func TestParseIssueList(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		expected  []string
		expectErr bool
	}{
		{"single issue", "PROJ-123", []string{"PROJ-123"}, false},
		{"multiple issues", "PROJ-1,PROJ-2,PROJ-3", []string{"PROJ-1", "PROJ-2", "PROJ-3"}, false},
		{"issues with spaces", "PROJ-1, PROJ-2 , PROJ-3", []string{"PROJ-1", "PROJ-2", "PROJ-3"}, false},
		{"issues with extra commas", "PROJ-1,,PROJ-2,", []string{"PROJ-1", "PROJ-2"}, false},
		{"empty input", "", nil, true},
		{"only commas", ",,", nil, true},
		{"only spaces", "   ", nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseIssueList(tt.input)

			if tt.expectErr {
				if err == nil {
					t.Errorf("Expected error for input '%s', but got none", tt.input)
				}
				return
			}

			if err != nil {
				t.Errorf("Expected no error for input '%s', but got: %v", tt.input, err)
				return
			}

			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d issues, got %d", len(tt.expected), len(result))
				return
			}

			for i, expected := range tt.expected {
				if result[i] != expected {
					t.Errorf("Expected issue[%d] = '%s', got '%s'", i, expected, result[i])
				}
			}
		})
	}
}

func TestValidateIssueList(t *testing.T) {
	tests := []struct {
		name      string
		input     []string
		expected  []string
		expectErr bool
	}{
		{"valid single issue", []string{"PROJ-123"}, []string{"PROJ-123"}, false},
		{"valid multiple issues", []string{"PROJ-1", "PROJ-2", "PROJ-3"}, []string{"PROJ-1", "PROJ-2", "PROJ-3"}, false},
		{"issues with duplicates", []string{"PROJ-1", "PROJ-2", "PROJ-1"}, []string{"PROJ-1", "PROJ-2"}, false},
		{"mixed valid and invalid", []string{"PROJ-1", "invalid", "PROJ-2"}, nil, true},
		{"all invalid", []string{"invalid1", "invalid2"}, nil, true},
		{"empty list", []string{}, nil, true},
		{"valid complex keys", []string{"MY-PROJECT-1", "ABC-DEF-2"}, []string{"MY-PROJECT-1", "ABC-DEF-2"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := validateIssueList(tt.input)

			if tt.expectErr {
				if err == nil {
					t.Errorf("Expected error for input %v, but got none", tt.input)
				}
				return
			}

			if err != nil {
				t.Errorf("Expected no error for input %v, but got: %v", tt.input, err)
				return
			}

			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d issues, got %d", len(tt.expected), len(result))
				return
			}

			for i, expected := range tt.expected {
				if result[i] != expected {
					t.Errorf("Expected issue[%d] = '%s', got '%s'", i, expected, result[i])
				}
			}
		})
	}
}

func TestValidateRepoPath(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "cli-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	tests := []struct {
		name      string
		repoPath  string
		expectErr bool
	}{
		{"valid existing directory", tempDir, false},
		{"valid relative path", ".", false},
		{"empty path", "", true},
		{"non-existent parent", "/non/existent/parent/repo", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateRepoPath(tt.repoPath)
			if tt.expectErr && err == nil {
				t.Errorf("Expected error for repo path '%s', but got none", tt.repoPath)
			}
			if !tt.expectErr && err != nil {
				t.Errorf("Expected no error for repo path '%s', but got: %v", tt.repoPath, err)
			}
		})
	}
}

func TestSyncCommand_Flags(t *testing.T) {
	// Test that the sync command has the required flags
	cmd := syncCmd

	// Check that issues flag exists
	issuesFlag := cmd.Flags().Lookup("issues")
	if issuesFlag == nil {
		t.Error("Expected --issues flag to exist")
		return
	}

	// Check that jql flag exists
	jqlFlag := cmd.Flags().Lookup("jql")
	if jqlFlag == nil {
		t.Error("Expected --jql flag to exist")
		return
	}

	// Check that repo flag exists and is required
	repoFlag := cmd.Flags().Lookup("repo")
	if repoFlag == nil {
		t.Error("Expected --repo flag to exist")
		return
	}

	// Check that concurrency flag exists
	concurrencyFlag := cmd.Flags().Lookup("concurrency")
	if concurrencyFlag == nil {
		t.Error("Expected --concurrency flag to exist")
		return
	}

	// Test flag shorthand
	if issuesFlag.Shorthand != "i" {
		t.Errorf("Expected issues flag shorthand to be 'i', got '%s'", issuesFlag.Shorthand)
	}

	if jqlFlag.Shorthand != "j" {
		t.Errorf("Expected jql flag shorthand to be 'j', got '%s'", jqlFlag.Shorthand)
	}

	if repoFlag.Shorthand != "r" {
		t.Errorf("Expected repo flag shorthand to be 'r', got '%s'", repoFlag.Shorthand)
	}

	if concurrencyFlag.Shorthand != "c" {
		t.Errorf("Expected concurrency flag shorthand to be 'c', got '%s'", concurrencyFlag.Shorthand)
	}
}

func TestSyncCommand_MissingFlags(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"missing issues flag", []string{"--repo=/tmp"}},
		{"missing repo flag", []string{"--issues=PROJ-123"}},
		{"missing both flags", []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a new command instance for isolated testing
			cmd := &cobra.Command{
				Use:  "sync",
				RunE: runSync,
			}
			cmd.Flags().StringP("issues", "i", "", "JIRA issue key(s) - single issue or comma-separated list")
			cmd.Flags().StringP("jql", "j", "", "JQL query to find issues to sync")
			cmd.Flags().StringP("repo", "r", "", "Target Git repository path (required)")
			cmd.Flags().IntP("concurrency", "c", 5, "Number of parallel workers")
			_ = cmd.MarkFlagRequired("repo")

			// Capture output
			var buf bytes.Buffer
			cmd.SetOut(&buf)
			cmd.SetErr(&buf)

			// Set arguments and execute
			cmd.SetArgs(tt.args)
			err := cmd.Execute()

			// Should return error for missing required flags
			if err == nil {
				t.Errorf("Expected error for missing flags, but command succeeded")
			}
		})
	}
}

func TestSyncCommand_ValidationErrors(t *testing.T) {
	tests := []struct {
		name     string
		issues   string
		jql      string
		repo     string
		errorMsg string
	}{
		{
			name:     "missing both issues and jql flags",
			issues:   "",
			jql:      "",
			repo:     "/tmp",
			errorMsg: "must specify either --issues or --jql flag",
		},
		{
			name:     "both issues and jql flags provided",
			issues:   "PROJ-123",
			jql:      "project = PROJ",
			repo:     "/tmp",
			errorMsg: "cannot specify both --issues and --jql flags",
		},
		{
			name:     "invalid repo path",
			issues:   "PROJ-123",
			jql:      "",
			repo:     "/non/existent/parent/repo",
			errorMsg: "invalid repository path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a new command instance for isolated testing
			cmd := &cobra.Command{
				Use:  "sync",
				RunE: runSync,
			}
			cmd.Flags().StringP("issues", "i", "", "JIRA issue key(s) - single issue or comma-separated list")
			cmd.Flags().StringP("jql", "j", "", "JQL query to find issues to sync")
			cmd.Flags().StringP("repo", "r", "", "Target Git repository path (required)")
			cmd.Flags().IntP("concurrency", "c", 5, "Number of parallel workers")

			// Set flags
			if tt.issues != "" {
				_ = cmd.Flags().Set("issues", tt.issues)
			}
			if tt.jql != "" {
				_ = cmd.Flags().Set("jql", tt.jql)
			}
			_ = cmd.Flags().Set("repo", tt.repo)

			// Execute command
			err := cmd.Execute()

			// Should return validation error
			if err == nil {
				t.Errorf("Expected validation error, but command succeeded")
			}

			if err != nil && tt.errorMsg != "" {
				if !contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error to contain '%s', but got: %v", tt.errorMsg, err)
				}
			}
		})
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
