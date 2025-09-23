package test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chambrid/jira-cdc-git/pkg/client"
	"github.com/chambrid/jira-cdc-git/pkg/config"
	"github.com/chambrid/jira-cdc-git/pkg/git"
	"github.com/chambrid/jira-cdc-git/pkg/schema"
)

// TestEndToEndWorkflow tests the complete JIRA → YAML → Git workflow
// This test requires a valid .env file with JIRA credentials
func TestEndToEndWorkflow(t *testing.T) {
	// Skip if no real JIRA configuration
	if os.Getenv("JIRA_URL") == "" {
		t.Skip("Skipping end-to-end test - no JIRA configuration found")
	}

	// Skip if running in CI or if no .env file exists
	if os.Getenv("CI") != "" {
		t.Skip("Skipping end-to-end test in CI environment")
	}

	// Look for .env file in project root
	projectRoot, err := filepath.Abs("..")
	if err != nil {
		t.Fatalf("Failed to get project root: %v", err)
	}

	envFile := filepath.Join(projectRoot, ".env")
	if _, err := os.Stat(envFile); os.IsNotExist(err) {
		t.Skip("Skipping end-to-end test: .env file not found")
	}

	// Change to project root for config loading
	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()
	_ = os.Chdir(projectRoot)

	// Create temporary directory for test repository
	tempDir, err := os.MkdirTemp("", "e2e-workflow-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	testRepo := filepath.Join(tempDir, "test-repo")
	if err := os.MkdirAll(testRepo, 0755); err != nil {
		t.Fatalf("Failed to create test repo directory: %v", err)
	}

	// Test issue - use from .env or default
	testIssue := os.Getenv("TEST_JIRA_ISSUE")
	if testIssue == "" {
		testIssue = "RHOAIENG-29356" // Default test issue
	}

	t.Logf("🚀 Starting end-to-end workflow test with issue: %s", testIssue)
	startTime := time.Now()

	// Step 1: Load configuration
	t.Log("📄 Step 1: Loading configuration...")
	configLoader := config.NewDotEnvLoader()
	cfg, err := configLoader.Load()
	if err != nil {
		t.Fatalf("Failed to load configuration: %v", err)
	}
	t.Logf("✅ Configuration loaded from %s", cfg.JIRABaseURL)

	// Step 2: Initialize and authenticate JIRA client
	t.Log("🔗 Step 2: Connecting to JIRA...")
	jiraClient, err := client.NewClient(cfg)
	if err != nil {
		t.Fatalf("Failed to create JIRA client: %v", err)
	}

	if err := jiraClient.Authenticate(); err != nil {
		t.Fatalf("Failed to authenticate with JIRA: %v", err)
	}
	t.Log("✅ JIRA authentication successful")

	// Step 3: Fetch JIRA issue
	t.Logf("📋 Step 3: Fetching JIRA issue %s...", testIssue)
	issue, err := jiraClient.GetIssue(testIssue)
	if err != nil {
		t.Fatalf("Failed to fetch JIRA issue: %v", err)
	}
	t.Logf("✅ Issue fetched: %s - %s", issue.Key, issue.Summary)

	// Validate issue data
	if issue.Key != testIssue {
		t.Errorf("Expected issue key %s, got %s", testIssue, issue.Key)
	}
	if issue.Summary == "" {
		t.Error("Issue summary should not be empty")
	}
	if issue.Status.Name == "" {
		t.Error("Issue status should not be empty")
	}

	// Step 4: Initialize Git repository
	t.Logf("📁 Step 4: Preparing Git repository at %s...", testRepo)
	gitRepo := git.NewGitRepository("E2E Test User", "e2e-test@automated.local")

	if err := gitRepo.Initialize(testRepo); err != nil {
		t.Fatalf("Failed to initialize Git repository: %v", err)
	}
	t.Log("✅ Git repository initialized")

	// Validate working tree is clean
	if err := gitRepo.ValidateWorkingTree(testRepo); err != nil {
		t.Fatalf("Git repository validation failed: %v", err)
	}

	// Step 5: Write YAML file
	t.Logf("📝 Step 5: Writing YAML file for issue %s...", testIssue)
	fileWriter := schema.NewYAMLFileWriter()
	yamlFilePath, err := fileWriter.WriteIssueToYAML(issue, testRepo)
	if err != nil {
		t.Fatalf("Failed to write YAML file: %v", err)
	}
	t.Logf("✅ YAML file written: %s", yamlFilePath)

	// Validate file structure
	expectedPath := filepath.Join(testRepo, "projects", extractProjectKey(testIssue), "issues", testIssue+".yaml")
	if yamlFilePath != expectedPath {
		t.Errorf("Expected YAML file at %s, got %s", expectedPath, yamlFilePath)
	}

	// Validate file exists and has content
	if _, err := os.Stat(yamlFilePath); os.IsNotExist(err) {
		t.Fatalf("YAML file was not created at %s", yamlFilePath)
	}

	content, err := os.ReadFile(yamlFilePath)
	if err != nil {
		t.Fatalf("Failed to read YAML file: %v", err)
	}

	contentStr := string(content)
	expectedFields := []string{
		"key: " + testIssue,
		"summary:",
		"status:",
		"priority:",
	}

	for _, field := range expectedFields {
		if !strings.Contains(contentStr, field) {
			t.Errorf("YAML file missing expected field: %s", field)
		}
	}

	// Step 6: Commit to Git
	t.Log("💾 Step 6: Committing to Git repository...")
	if err := gitRepo.CommitIssueFile(testRepo, yamlFilePath, issue); err != nil {
		t.Fatalf("Failed to commit to Git: %v", err)
	}
	t.Log("✅ Successfully committed to Git")

	// Validate Git commit
	status, err := gitRepo.GetRepositoryStatus(testRepo)
	if err != nil {
		t.Fatalf("Failed to get repository status: %v", err)
	}

	if !status.IsClean {
		t.Error("Repository should be clean after commit")
	}

	// Step 7: Performance and validation
	elapsed := time.Since(startTime)
	t.Logf("⏱️  Total workflow time: %v", elapsed)

	// Performance requirement: <30 seconds for single issue sync
	if elapsed > 30*time.Second {
		t.Errorf("Workflow took too long: %v (should be <30s)", elapsed)
	}

	t.Logf("🎯 End-to-end workflow completed successfully!")
	t.Logf("📊 Performance: %v", elapsed)
	t.Logf("📁 File created: %s", yamlFilePath)
	t.Logf("📋 Issue synced: %s - %s", issue.Key, issue.Summary)
}

// TestEndToEndWorkflowErrorHandling tests error scenarios in the complete workflow
func TestEndToEndWorkflowErrorHandling(t *testing.T) {
	// Test 1: Invalid issue key
	t.Run("invalid_issue_key", func(t *testing.T) {
		if os.Getenv("CI") != "" {
			t.Skip("Skipping error handling test in CI environment")
		}

		// Change to project root for config loading
		projectRoot, err := filepath.Abs("..")
		if err != nil {
			t.Fatalf("Failed to get project root: %v", err)
		}

		envFile := filepath.Join(projectRoot, ".env")
		if _, err := os.Stat(envFile); os.IsNotExist(err) {
			t.Skip("No .env file available for error handling test")
		}

		oldWd, _ := os.Getwd()
		defer func() { _ = os.Chdir(oldWd) }()
		_ = os.Chdir(projectRoot)

		configLoader := config.NewDotEnvLoader()
		cfg, err := configLoader.Load()
		if err != nil {
			t.Skip("No .env file available for error handling test")
		}

		jiraClient, err := client.NewClient(cfg)
		if err != nil {
			t.Fatalf("Failed to create JIRA client: %v", err)
		}

		// Try to fetch non-existent issue
		_, err = jiraClient.GetIssue("INVALID-99999")
		if err == nil {
			t.Error("Expected error for invalid issue key")
		}

		// Should be a not found error
		if !client.IsNotFoundError(err) {
			t.Logf("Expected not found error, got: %v", err)
		}
	})

	// Test 2: Dirty repository
	t.Run("dirty_repository", func(t *testing.T) {
		// Create temporary directory with a dirty Git repository
		tempDir, err := os.MkdirTemp("", "e2e-dirty-test-*")
		if err != nil {
			t.Fatalf("Failed to create temp directory: %v", err)
		}
		defer func() { _ = os.RemoveAll(tempDir) }()

		// Initialize repository and create a dirty state
		gitRepo := git.NewGitRepository("Test User", "test@example.com")
		if err := gitRepo.Initialize(tempDir); err != nil {
			t.Fatalf("Failed to initialize Git repository: %v", err)
		}

		// Create an uncommitted file
		dirtyFile := filepath.Join(tempDir, "uncommitted.txt")
		if err := os.WriteFile(dirtyFile, []byte("uncommitted changes"), 0644); err != nil {
			t.Fatalf("Failed to create dirty file: %v", err)
		}

		// Validation should fail
		err = gitRepo.ValidateWorkingTree(tempDir)
		if err == nil {
			t.Error("Expected error for dirty working tree")
		}

		if !git.IsDirtyWorkingTreeError(err) {
			t.Errorf("Expected dirty working tree error, got: %v", err)
		}
	})
}

// Helper function to extract project key from issue key
func extractProjectKey(issueKey string) string {
	parts := strings.Split(issueKey, "-")
	if len(parts) > 0 {
		return parts[0]
	}
	return issueKey
}
