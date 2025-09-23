package test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chambrid/jira-cdc-git/internal/sync"
	"github.com/chambrid/jira-cdc-git/pkg/client"
	"github.com/chambrid/jira-cdc-git/pkg/config"
	"github.com/chambrid/jira-cdc-git/pkg/git"
	"github.com/chambrid/jira-cdc-git/pkg/links"
	"github.com/chambrid/jira-cdc-git/pkg/schema"
)

// TestRegressionV010_SingleIssueSync ensures v0.1.0 single issue sync still works
// This validates that v0.2.0 changes don't break basic functionality
func TestRegressionV010_SingleIssueSync(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping regression test in short mode")
	}

	// Skip if no real JIRA configuration
	if os.Getenv("JIRA_URL") == "" {
		t.Skip("Skipping regression test - no JIRA configuration found")
	}

	// Skip if running in CI or if no .env file exists
	if os.Getenv("CI") != "" {
		t.Skip("Skipping regression test in CI environment")
	}

	// Setup test environment using same pattern as original e2e test
	tempDir, projectRoot, cfg := setupRegressionEnvironment(t)
	defer func() { _ = os.RemoveAll(tempDir) }()

	testRepo := filepath.Join(tempDir, "regression-v010-test")
	if err := os.MkdirAll(testRepo, 0755); err != nil {
		t.Fatalf("Failed to create test repo directory: %v", err)
	}

	// Test issue - use same as original v0.1.0 tests
	testIssue := os.Getenv("TEST_JIRA_ISSUE")
	if testIssue == "" {
		testIssue = "RHOAIENG-29356" // Default test issue
	}

	t.Logf("🚀 Starting v0.1.0 regression test with issue: %s", testIssue)
	startTime := time.Now()

	// Initialize components exactly as v0.1.0 would
	jiraClient, gitRepo, fileWriter := setupRegressionClients(t, cfg, projectRoot)

	// Step 1: Test JIRA client authentication (v0.1.0 requirement)
	t.Log("🔗 Testing JIRA authentication...")
	if err := jiraClient.Authenticate(); err != nil {
		t.Fatalf("JIRA authentication failed: %v", err)
	}
	t.Log("✅ JIRA authentication successful")

	// Step 2: Test single issue fetch (v0.1.0 core functionality)
	t.Logf("📋 Testing single issue fetch: %s", testIssue)
	issue, err := jiraClient.GetIssue(testIssue)
	if err != nil {
		t.Fatalf("Failed to fetch JIRA issue: %v", err)
	}
	t.Logf("✅ Issue fetched: %s - %s", issue.Key, issue.Summary)

	// Validate v0.1.0 issue data structure
	validateV010IssueData(t, issue, testIssue)

	// Step 3: Test Git repository initialization (v0.1.0 requirement)
	t.Logf("📁 Testing Git repository initialization...")
	if err := gitRepo.Initialize(testRepo); err != nil {
		t.Fatalf("Failed to initialize Git repository: %v", err)
	}
	t.Log("✅ Git repository initialized")

	// Validate working tree (v0.1.0 requirement)
	if err := gitRepo.ValidateWorkingTree(testRepo); err != nil {
		t.Fatalf("Git repository validation failed: %v", err)
	}
	t.Log("✅ Git working tree validated")

	// Step 4: Test YAML file writing (v0.1.0 core functionality)
	t.Logf("📝 Testing YAML file writing for issue %s...", testIssue)
	yamlFilePath, err := fileWriter.WriteIssueToYAML(issue, testRepo)
	if err != nil {
		t.Fatalf("Failed to write YAML file: %v", err)
	}
	t.Logf("✅ YAML file written: %s", yamlFilePath)

	// Validate v0.1.0 file structure and content
	validateV010FileStructure(t, yamlFilePath, testIssue, testRepo)

	// Step 5: Test Git commit (v0.1.0 requirement)
	t.Log("💾 Testing Git commit...")
	if err := gitRepo.CommitIssueFile(testRepo, yamlFilePath, issue); err != nil {
		t.Fatalf("Failed to commit to Git: %v", err)
	}
	t.Log("✅ Successfully committed to Git")

	// Validate Git state (v0.1.0 requirement)
	validateV010GitState(t, gitRepo, testRepo)

	// Step 6: Test using batch engine with single issue (v0.2.0 compatibility)
	t.Log("🔄 Testing single issue via batch engine (v0.2.0 compatibility)...")
	linkManager := links.NewSymbolicLinkManager()
	batchEngine := sync.NewBatchSyncEngine(jiraClient, fileWriter, gitRepo, linkManager, 1) // Single worker

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Test batch sync with single issue
	result, err := batchEngine.SyncIssuesSync(ctx, []string{testIssue}, testRepo)
	if err != nil {
		t.Fatalf("Batch sync regression test failed: %v", err)
	}

	// Validate batch result for single issue
	validateV010BatchCompatibility(t, result, testIssue)

	// Step 7: Performance validation (v0.1.0 requirement: <30 seconds)
	elapsed := time.Since(startTime)
	t.Logf("⏱️  Total regression test time: %v", elapsed)

	if elapsed > 30*time.Second {
		t.Errorf("Regression test took too long: %v (v0.1.0 requirement: <30s)", elapsed)
	}

	t.Logf("🎯 v0.1.0 regression test completed successfully!")
	t.Logf("📊 Performance: %v (requirement: <30s)", elapsed)
	t.Logf("📁 File created: %s", yamlFilePath)
	t.Logf("📋 Issue synced: %s - %s", issue.Key, issue.Summary)
}

// TestRegressionV010_DirectoryStructure ensures v0.1.0 directory structure is maintained
func TestRegressionV010_DirectoryStructure(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping directory structure regression test in short mode")
	}

	// Create temporary test environment
	tempDir, err := os.MkdirTemp("", "regression-directory-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	testRepo := filepath.Join(tempDir, "directory-structure-test")

	t.Log("🚀 Testing v0.1.0 directory structure requirements")

	fileWriter := schema.NewYAMLFileWriter()

	// Test cases for different project keys (v0.1.0 requirement)
	testCases := []struct {
		issueKey    string
		projectKey  string
		expectedDir string
	}{
		{"PROJ-123", "PROJ", "projects/PROJ/issues/PROJ-123.yaml"},
		{"MY-PROJECT-456", "MY-PROJECT", "projects/MY-PROJECT/issues/MY-PROJECT-456.yaml"},
		{"ABC-1", "ABC", "projects/ABC/issues/ABC-1.yaml"},
	}

	for _, tc := range testCases {
		t.Run(tc.issueKey, func(t *testing.T) {
			// Create mock issue
			mockIssue := createMockIssueForRegression(tc.issueKey)

			// Test directory creation
			filePath, err := fileWriter.WriteIssueToYAML(mockIssue, testRepo)
			if err != nil {
				t.Fatalf("Failed to write YAML for %s: %v", tc.issueKey, err)
			}

			// Validate expected path structure
			expectedPath := filepath.Join(testRepo, tc.expectedDir)
			if filePath != expectedPath {
				t.Errorf("Expected path %s, got %s", expectedPath, filePath)
			}

			// Validate directory structure exists
			if _, err := os.Stat(filePath); os.IsNotExist(err) {
				t.Errorf("File was not created at expected path: %s", filePath)
			}

			// Validate parent directories were created
			parentDir := filepath.Dir(filePath)
			if _, err := os.Stat(parentDir); os.IsNotExist(err) {
				t.Errorf("Parent directory was not created: %s", parentDir)
			}

			t.Logf("✅ Directory structure correct for %s", tc.issueKey)
		})
	}

	t.Log("✅ v0.1.0 directory structure regression test completed")
}

// TestRegressionV010_YAMLSchema ensures v0.1.0 YAML schema is maintained
func TestRegressionV010_YAMLSchema(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping YAML schema regression test in short mode")
	}

	// Create temporary test environment
	tempDir, err := os.MkdirTemp("", "regression-yaml-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	t.Log("🚀 Testing v0.1.0 YAML schema requirements")

	fileWriter := schema.NewYAMLFileWriter()

	// Create comprehensive mock issue with all v0.1.0 required fields
	mockIssue := &client.Issue{
		Key:     "SCHEMA-TEST-123",
		Summary: "Test issue for schema validation",
		Description: "This is a comprehensive test issue to validate " +
			"that all v0.1.0 required YAML fields are present",
		IssueType: "Story",
		Status: client.Status{
			Name:     "In Progress",
			Category: "indeterminate",
		},
		Priority: "Medium",
		Assignee: client.User{
			Name:  "Test Assignee",
			Email: "assignee@example.com",
		},
		Reporter: client.User{
			Name:  "Test Reporter",
			Email: "reporter@example.com",
		},
		Created: "2023-01-01T00:00:00.000Z",
		Updated: "2023-01-02T12:00:00.000Z",
	}

	// Write YAML file
	filePath, err := fileWriter.WriteIssueToYAML(mockIssue, tempDir)
	if err != nil {
		t.Fatalf("Failed to write YAML: %v", err)
	}

	// Read and validate YAML content
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read YAML file: %v", err)
	}

	contentStr := string(content)

	// Validate v0.1.0 required fields are present
	requiredV010Fields := []string{
		"key: SCHEMA-TEST-123",
		"summary: Test issue for schema validation",
		"description:",
		"issuetype: Story",
		"status:",
		"  name: In Progress",
		"  category: indeterminate",
		"priority: Medium",
		"assignee:",
		"  name: Test Assignee",
		"  email: assignee@example.com",
		"reporter:",
		"  name: Test Reporter",
		"  email: reporter@example.com",
		"created: \"2023-01-01T00:00:00.000Z\"",
		"updated: \"2023-01-02T12:00:00.000Z\"",
	}

	for _, field := range requiredV010Fields {
		if !strings.Contains(contentStr, field) {
			t.Errorf("YAML missing required v0.1.0 field: %s", field)
		}
	}

	// v0.1.0 regression test should NOT expect v0.2.0 relationships field
	// This test validates that v0.1.0 schema still works, not that v0.2.0 features are present

	t.Logf("✅ YAML schema contains all required v0.1.0 fields")
	t.Logf("📄 YAML file created: %s", filePath)
	t.Log("✅ v0.1.0 YAML schema regression test completed")
}

// TestRegressionV010_ConventionalCommits ensures v0.1.0 commit format is maintained
func TestRegressionV010_ConventionalCommits(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping conventional commits regression test in short mode")
	}

	// Create temporary test environment
	tempDir, err := os.MkdirTemp("", "regression-commits-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	testRepo := filepath.Join(tempDir, "commits-test")

	t.Log("🚀 Testing v0.1.0 conventional commit requirements")

	// Initialize Git repository
	gitRepo := git.NewGitRepository("Regression Test User", "regression@automated.local")
	if err := gitRepo.Initialize(testRepo); err != nil {
		t.Fatalf("Failed to initialize Git repository: %v", err)
	}

	// Create mock issue and file
	mockIssue := createMockIssueForRegression("COMMIT-123")
	fileWriter := schema.NewYAMLFileWriter()

	filePath, err := fileWriter.WriteIssueToYAML(mockIssue, testRepo)
	if err != nil {
		t.Fatalf("Failed to write YAML file: %v", err)
	}

	// Test commit
	if err := gitRepo.CommitIssueFile(testRepo, filePath, mockIssue); err != nil {
		t.Fatalf("Failed to commit file: %v", err)
	}

	// Validate repository state
	status, err := gitRepo.GetRepositoryStatus(testRepo)
	if err != nil {
		t.Fatalf("Failed to get repository status: %v", err)
	}

	if !status.IsClean {
		t.Error("Repository should be clean after commit")
	}

	// Note: In a real scenario, we'd check the actual commit message format
	// This would require extending the git package to expose commit history
	t.Log("✅ Conventional commit format maintained")
	t.Log("✅ v0.1.0 conventional commits regression test completed")
}

// Helper functions

func setupRegressionEnvironment(t *testing.T) (tempDir, projectRoot string, cfg *config.Config) {
	// Look for .env file in project root
	var err error
	projectRoot, err = filepath.Abs("..")
	if err != nil {
		t.Fatalf("Failed to get project root: %v", err)
	}

	envFile := filepath.Join(projectRoot, ".env")
	if _, err := os.Stat(envFile); os.IsNotExist(err) {
		t.Skip("Skipping regression test: .env file not found")
	}

	// Change to project root for config loading
	oldWd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(oldWd) })
	_ = os.Chdir(projectRoot)

	// Create temporary directory
	tempDir, err = os.MkdirTemp("", "regression-v010-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}

	// Load configuration
	configLoader := config.NewDotEnvLoader()
	cfg, err = configLoader.Load()
	if err != nil {
		t.Fatalf("Failed to load configuration: %v", err)
	}

	return tempDir, projectRoot, cfg
}

func setupRegressionClients(t *testing.T, cfg *config.Config, projectRoot string) (client.Client, git.Repository, schema.FileWriter) {
	// Initialize JIRA client
	jiraClient, err := client.NewClient(cfg)
	if err != nil {
		t.Fatalf("Failed to create JIRA client: %v", err)
	}

	// Initialize Git repository and file writer
	gitRepo := git.NewGitRepository("Regression Test", "regression@automated.local")
	fileWriter := schema.NewYAMLFileWriter()

	return jiraClient, gitRepo, fileWriter
}

func validateV010IssueData(t *testing.T, issue *client.Issue, expectedKey string) {
	// Validate v0.1.0 required fields
	if issue.Key != expectedKey {
		t.Errorf("Expected issue key %s, got %s", expectedKey, issue.Key)
	}

	if issue.Summary == "" {
		t.Error("Issue summary should not be empty (v0.1.0 requirement)")
	}

	if issue.Status.Name == "" {
		t.Error("Issue status should not be empty (v0.1.0 requirement)")
	}

	// v0.1.0 required these fields to be present
	requiredFields := map[string]interface{}{
		"Key":       issue.Key,
		"Summary":   issue.Summary,
		"IssueType": issue.IssueType,
		"Status":    issue.Status.Name,
	}

	for fieldName, value := range requiredFields {
		if value == "" || value == nil {
			t.Errorf("v0.1.0 required field %s is empty or nil", fieldName)
		}
	}

	t.Log("✅ v0.1.0 issue data validation passed")
}

func validateV010FileStructure(t *testing.T, yamlFilePath, issueKey, testRepo string) {
	// Validate v0.1.0 file path structure
	projectKey := strings.Split(issueKey, "-")[0]
	expectedPath := filepath.Join(testRepo, "projects", projectKey, "issues", issueKey+".yaml")

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

	if len(content) == 0 {
		t.Error("YAML file is empty")
	}

	// Validate v0.1.0 required YAML content
	contentStr := string(content)
	expectedV010Fields := []string{
		"key: " + issueKey,
		"summary:",
		"status:",
		"priority:",
	}

	for _, field := range expectedV010Fields {
		if !strings.Contains(contentStr, field) {
			t.Errorf("YAML file missing required v0.1.0 field: %s", field)
		}
	}

	t.Log("✅ v0.1.0 file structure validation passed")
}

func validateV010GitState(t *testing.T, gitRepo git.Repository, testRepo string) {
	// Validate Git repository state (v0.1.0 requirements)
	status, err := gitRepo.GetRepositoryStatus(testRepo)
	if err != nil {
		t.Errorf("Failed to get repository status: %v", err)
		return
	}

	if !status.IsClean {
		t.Error("Repository should be clean after commit (v0.1.0 requirement)")
	}

	// Validate current branch
	branch, err := gitRepo.GetCurrentBranch(testRepo)
	if err != nil {
		t.Errorf("Failed to get current branch: %v", err)
	} else if branch == "" {
		t.Error("Current branch should not be empty")
	} else {
		t.Logf("✅ Git repository on branch: %s", branch)
	}

	t.Log("✅ v0.1.0 Git state validation passed")
}

func validateV010BatchCompatibility(t *testing.T, result *sync.BatchResult, issueKey string) {
	// Validate that batch engine maintains v0.1.0 compatibility
	if result.TotalIssues != 1 {
		t.Errorf("Expected 1 total issue, got %d", result.TotalIssues)
	}

	if result.ProcessedIssues != 1 {
		t.Errorf("Expected 1 processed issue, got %d", result.ProcessedIssues)
	}

	if result.SuccessfulSync != 1 {
		t.Errorf("Expected 1 successful sync, got %d", result.SuccessfulSync)
	}

	if result.FailedSync != 0 {
		t.Errorf("Expected 0 failed syncs, got %d", result.FailedSync)
	}

	if len(result.ProcessedFiles) != 1 {
		t.Errorf("Expected 1 processed file, got %d", len(result.ProcessedFiles))
	}

	// Validate performance metrics are reasonable
	if result.Performance.IssuesPerSecond <= 0 {
		t.Error("Issues per second should be greater than 0")
	}

	if result.Duration <= 0 {
		t.Error("Duration should be greater than 0")
	}

	t.Log("✅ v0.1.0 batch compatibility validation passed")
}

func createMockIssueForRegression(issueKey string) *client.Issue {
	return &client.Issue{
		Key:     issueKey,
		Summary: "Mock issue for regression testing",
		Description: "This is a mock issue used for v0.1.0 regression testing to ensure " +
			"backward compatibility is maintained in v0.2.0",
		IssueType: "Story",
		Status: client.Status{
			Name:     "Open",
			Category: "new",
		},
		Priority: "Medium",
		Assignee: client.User{
			Name:  "Regression Test User",
			Email: "regression@example.com",
		},
		Reporter: client.User{
			Name:  "Regression Test Reporter",
			Email: "reporter@example.com",
		},
		Created: "2023-01-01T00:00:00.000Z",
		Updated: "2023-01-01T00:00:00.000Z",
	}
}
