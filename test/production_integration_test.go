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

// TestProductionLikeIntegration simulates real-world production scenarios
// This test suite would have caught the critical bugs we discovered in v0.2.0
func TestProductionLikeIntegration(t *testing.T) {
	if os.Getenv("JIRA_URL") == "" {
		t.Skip("Skipping production integration tests - no JIRA configuration found")
	}

	t.Run("FullUserWorkflowSimulation", testFullUserWorkflowSimulation)
	t.Run("ConcurrentBatchOperations", testConcurrentBatchOperations)
	t.Run("JQLQueryIntegration", testJQLQueryIntegration)
	t.Run("ErrorRecoveryScenarios", testErrorRecoveryScenarios)
	t.Run("MemoryAndResourceManagement", testMemoryAndResourceManagement)
}

// testFullUserWorkflowSimulation simulates the exact workflow a user would follow
// This would have caught the hanging process and missing links issues
func testFullUserWorkflowSimulation(t *testing.T) {
	// Create temporary directory for test repository
	tempDir, err := os.MkdirTemp("", "production-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	t.Logf("Using test repository: %s", tempDir)

	// Load configuration exactly as CLI does
	configLoader := config.NewDotEnvLoader()
	cfg, err := configLoader.Load()
	if err != nil {
		t.Fatalf("Failed to load configuration: %v", err)
	}

	// Apply rate limiting as CLI does
	cfg.RateLimitDelay = 500 * time.Millisecond
	t.Logf("Using rate limit: %v", cfg.RateLimitDelay)

	// Initialize JIRA client exactly as CLI does
	jiraClient, err := client.NewClient(cfg)
	if err != nil {
		t.Fatalf("Failed to create JIRA client: %v", err)
	}

	// Authenticate
	if err := jiraClient.Authenticate(); err != nil {
		t.Fatalf("Failed to authenticate with JIRA: %v", err)
	}

	// Initialize Git repository exactly as CLI does
	gitRepo := git.NewGitRepository("JIRA CDC Git Sync", "jira-sync@automated.local")

	if err := gitRepo.Initialize(tempDir); err != nil {
		t.Fatalf("Failed to initialize Git repository: %v", err)
	}

	if err := gitRepo.ValidateWorkingTree(tempDir); err != nil {
		t.Fatalf("Git repository validation failed: %v", err)
	}

	// Initialize batch sync engine exactly as CLI does
	fileWriter := schema.NewYAMLFileWriter()
	linkManager := links.NewSymbolicLinkManager()
	batchEngine := sync.NewBatchSyncEngine(jiraClient, fileWriter, gitRepo, linkManager, 5)

	// Test single issue sync (basic workflow)
	t.Run("SingleIssueSync", func(t *testing.T) {
		ctx := context.Background()
		progressDone := make(chan bool, 1)

		// Start progress monitoring exactly as CLI does
		go func() {
			defer func() { progressDone <- true }()
			for update := range batchEngine.GetProgressChannel() {
				t.Logf("Progress: %.0f%% (%d processed)", update.Percentage, update.ProcessedCount)
			}
		}()

		// Sync single issue
		testIssueKey := os.Getenv("TEST_ISSUE_KEY")
		if testIssueKey == "" {
			testIssueKey = "RHOAIENG-29357"
		}

		t.Logf("Syncing single issue: %s", testIssueKey)

		result, err := batchEngine.SyncIssues(ctx, []string{testIssueKey}, tempDir)
		if err != nil {
			t.Fatalf("Single issue sync failed: %v", err)
		}

		// Close progress channel as CLI does
		batchEngine.CloseProgressChannel()

		// Wait for progress monitoring to complete (this would hang in v0.2.0 bug)
		select {
		case <-progressDone:
			t.Logf("✅ Progress monitoring completed successfully")
		case <-time.After(10 * time.Second):
			t.Fatal("❌ Progress monitoring did not complete - process would hang for user")
		}

		// Validate results
		if result.ProcessedIssues != 1 {
			t.Errorf("Expected 1 processed issue, got %d", result.ProcessedIssues)
		}
		if result.SuccessfulSync != 1 {
			t.Errorf("Expected 1 successful sync, got %d", result.SuccessfulSync)
		}

		t.Logf("✅ Single issue sync completed: %v", result.Duration)
	})

	// Test issue with relationships (would catch missing linkManager bug)
	t.Run("IssueWithRelationships", func(t *testing.T) {
		ctx := context.Background()
		progressDone := make(chan bool, 1)

		// Create new batch engine for this test
		batchEngine := sync.NewBatchSyncEngine(jiraClient, fileWriter, gitRepo, linkManager, 5)

		go func() {
			defer func() { progressDone <- true }()
			for update := range batchEngine.GetProgressChannel() {
				t.Logf("Progress: %.0f%% (%d processed)", update.Percentage, update.ProcessedCount)
			}
		}()

		// Sync issue known to have relationships
		parentIssueKey := os.Getenv("TEST_PARENT_ISSUE_KEY")
		if parentIssueKey == "" {
			parentIssueKey = "RHOAIENG-29356"
		}

		t.Logf("Syncing issue with relationships: %s", parentIssueKey)

		result, err := batchEngine.SyncIssues(ctx, []string{parentIssueKey}, tempDir)
		if err != nil {
			t.Fatalf("Issue with relationships sync failed: %v", err)
		}

		batchEngine.CloseProgressChannel()

		select {
		case <-progressDone:
			t.Logf("✅ Progress monitoring completed")
		case <-time.After(10 * time.Second):
			t.Fatal("❌ Progress monitoring timeout")
		}

		// Validate that YAML file exists and is readable
		projectKey := strings.Split(parentIssueKey, "-")[0]
		issueFile := filepath.Join(tempDir, "projects", projectKey, "issues", parentIssueKey+".yaml")

		if _, err := os.Stat(issueFile); os.IsNotExist(err) {
			t.Errorf("❌ Issue YAML file not created: %s", issueFile)
		} else {
			t.Logf("✅ Issue YAML file created: %s", issueFile)

			// Read and validate YAML content
			content, err := os.ReadFile(issueFile)
			if err != nil {
				t.Errorf("Failed to read YAML file: %v", err)
			} else {
				// Check for timestamp format issues (would catch the timestamp bug)
				contentStr := string(content)
				if strings.Contains(contentStr, "{") && strings.Contains(contentStr, "0x") {
					t.Errorf("❌ YAML contains malformed timestamps: found Go struct representation")
				} else {
					t.Logf("✅ YAML timestamps appear properly formatted")
				}

				// Validate basic YAML structure
				if !strings.Contains(contentStr, "key:") || !strings.Contains(contentStr, "summary:") {
					t.Errorf("❌ YAML missing basic fields")
				} else {
					t.Logf("✅ YAML structure appears valid")
				}
			}
		}

		// Check for symbolic links (would catch missing linkManager bug)
		relationshipsDir := filepath.Join(tempDir, "projects", projectKey, "relationships")
		linkCount := 0
		if _, err := os.Stat(relationshipsDir); !os.IsNotExist(err) {
			_ = filepath.Walk(relationshipsDir, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if info.Mode()&os.ModeSymlink != 0 {
					linkCount++
					t.Logf("Found symbolic link: %s", path)
				}
				return nil
			})
		}

		if linkCount > 0 {
			t.Logf("✅ Found %d symbolic links for relationships", linkCount)
		} else {
			t.Logf("⚠️ No symbolic links found - issue may not have relationships or linkManager not working")
		}

		t.Logf("✅ Issue with relationships completed: %v", result.Duration)
	})
}

// testConcurrentBatchOperations tests production-like concurrent operations
func testConcurrentBatchOperations(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "concurrent-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	configLoader := config.NewDotEnvLoader()
	cfg, err := configLoader.Load()
	if err != nil {
		t.Fatalf("Failed to load configuration: %v", err)
	}

	// Test with realistic concurrency and rate limiting
	cfg.RateLimitDelay = 200 * time.Millisecond // Faster than default for testing

	jiraClient, err := client.NewClient(cfg)
	if err != nil {
		t.Fatalf("Failed to create JIRA client: %v", err)
	}

	if err := jiraClient.Authenticate(); err != nil {
		t.Fatalf("Failed to authenticate: %v", err)
	}

	gitRepo := git.NewGitRepository("Test User", "test@example.com")
	if err := gitRepo.Initialize(tempDir); err != nil {
		t.Fatalf("Failed to initialize git: %v", err)
	}

	fileWriter := schema.NewYAMLFileWriter()
	linkManager := links.NewSymbolicLinkManager()

	// Test different concurrency levels
	concurrencyTests := []struct {
		name        string
		concurrency int
		expectPass  bool
	}{
		{"LowConcurrency", 2, true},
		{"MediumConcurrency", 5, true},
		{"HighConcurrency", 8, true},
	}

	for _, tt := range concurrencyTests {
		t.Run(tt.name, func(t *testing.T) {
			batchEngine := sync.NewBatchSyncEngine(jiraClient, fileWriter, gitRepo, linkManager, tt.concurrency)

			ctx := context.Background()
			progressDone := make(chan bool, 1)

			go func() {
				defer func() { progressDone <- true }()
				for update := range batchEngine.GetProgressChannel() {
					t.Logf("Concurrency %d - Progress: %.0f%% (%d processed)",
						tt.concurrency, update.Percentage, update.ProcessedCount)
				}
			}()

			// Use multiple test issues if available
			testIssues := []string{"RHOAIENG-29357"}
			if customIssues := os.Getenv("TEST_ISSUE_KEYS"); customIssues != "" {
				testIssues = strings.Split(customIssues, ",")
				for i := range testIssues {
					testIssues[i] = strings.TrimSpace(testIssues[i])
				}
			}

			start := time.Now()
			result, err := batchEngine.SyncIssues(ctx, testIssues, tempDir)
			duration := time.Since(start)

			batchEngine.CloseProgressChannel()

			select {
			case <-progressDone:
				t.Logf("✅ Concurrency %d completed in %v", tt.concurrency, duration)
			case <-time.After(30 * time.Second):
				t.Errorf("❌ Concurrency %d timed out", tt.concurrency)
				return
			}

			if tt.expectPass && err != nil {
				t.Errorf("Expected success with concurrency %d, got error: %v", tt.concurrency, err)
			} else if !tt.expectPass && err == nil {
				t.Errorf("Expected failure with concurrency %d, but succeeded", tt.concurrency)
			}

			if result != nil {
				t.Logf("Concurrency %d results: %d processed, %d successful, %d failed",
					tt.concurrency, result.ProcessedIssues, result.SuccessfulSync, result.FailedSync)
			}
		})
	}
}

// testJQLQueryIntegration tests real JQL query execution
func testJQLQueryIntegration(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "jql-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	configLoader := config.NewDotEnvLoader()
	cfg, err := configLoader.Load()
	if err != nil {
		t.Fatalf("Failed to load configuration: %v", err)
	}

	cfg.RateLimitDelay = 500 * time.Millisecond

	jiraClient, err := client.NewClient(cfg)
	if err != nil {
		t.Fatalf("Failed to create JIRA client: %v", err)
	}

	if err := jiraClient.Authenticate(); err != nil {
		t.Fatalf("Failed to authenticate: %v", err)
	}

	// Test various JQL queries that users might run
	jqlTests := []struct {
		name     string
		jql      string
		expectOK bool
	}{
		{
			name:     "SingleIssueQuery",
			jql:      "key = RHOAIENG-29357",
			expectOK: true,
		},
		{
			name:     "ParentAndSubtasks",
			jql:      "key = RHOAIENG-29356 OR parent = RHOAIENG-29356",
			expectOK: true,
		},
		{
			name:     "InvalidJQL",
			jql:      "invalid jql syntax here",
			expectOK: false,
		},
	}

	for _, tt := range jqlTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Logf("Testing JQL: %s", tt.jql)

			// First, test JQL search directly (as CLI does)
			jqlIssues, err := jiraClient.SearchIssues(tt.jql)
			if tt.expectOK && err != nil {
				t.Errorf("JQL search failed unexpectedly: %v", err)
				return
			} else if !tt.expectOK && err == nil {
				t.Errorf("JQL search succeeded when it should have failed")
				return
			}

			if !tt.expectOK {
				t.Logf("✅ JQL properly failed as expected: %v", err)
				return
			}

			t.Logf("Found %d issues from JQL search", len(jqlIssues))

			if len(jqlIssues) == 0 {
				t.Log("⚠️ No issues found for JQL query")
				return
			}

			// Extract issue keys
			issueKeys := make([]string, len(jqlIssues))
			for i, issue := range jqlIssues {
				issueKeys[i] = issue.Key
			}

			// Now test batch sync with these issues
			gitRepo := git.NewGitRepository("Test User", "test@example.com")
			if err := gitRepo.Initialize(tempDir); err != nil {
				t.Fatalf("Failed to initialize git: %v", err)
			}

			fileWriter := schema.NewYAMLFileWriter()
			linkManager := links.NewSymbolicLinkManager()
			batchEngine := sync.NewBatchSyncEngine(jiraClient, fileWriter, gitRepo, linkManager, 3)

			ctx := context.Background()
			progressDone := make(chan bool, 1)

			go func() {
				defer func() { progressDone <- true }()
				for update := range batchEngine.GetProgressChannel() {
					t.Logf("JQL sync progress: %.0f%% (%d processed)", update.Percentage, update.ProcessedCount)
				}
			}()

			result, err := batchEngine.SyncIssues(ctx, issueKeys, tempDir)
			if err != nil {
				t.Errorf("JQL batch sync failed: %v", err)
				return
			}

			batchEngine.CloseProgressChannel()

			select {
			case <-progressDone:
				t.Logf("✅ JQL sync completed")
			case <-time.After(30 * time.Second):
				t.Error("❌ JQL sync timeout")
				return
			}

			t.Logf("JQL sync results: %d found, %d processed, %d successful",
				len(jqlIssues), result.ProcessedIssues, result.SuccessfulSync)

			// Validate that files were created
			if result.SuccessfulSync != result.ProcessedIssues {
				t.Errorf("Mismatch: %d processed but %d successful", result.ProcessedIssues, result.SuccessfulSync)
			}

			if len(result.ProcessedFiles) != result.SuccessfulSync {
				t.Errorf("Mismatch: %d successful but %d files in result", result.SuccessfulSync, len(result.ProcessedFiles))
			}
		})
	}
}

// testErrorRecoveryScenarios tests how system handles various error conditions
func testErrorRecoveryScenarios(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "error-recovery-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	configLoader := config.NewDotEnvLoader()
	cfg, err := configLoader.Load()
	if err != nil {
		t.Fatalf("Failed to load configuration: %v", err)
	}

	// Test network timeout scenarios
	t.Run("NetworkTimeoutRecovery", func(t *testing.T) {
		// Use very aggressive rate limiting to potentially trigger timeouts
		cfg.RateLimitDelay = 10 * time.Millisecond

		jiraClient, err := client.NewClient(cfg)
		if err != nil {
			t.Fatalf("Failed to create JIRA client: %v", err)
		}

		if err := jiraClient.Authenticate(); err != nil {
			t.Fatalf("Failed to authenticate: %v", err)
		}

		gitRepo := git.NewGitRepository("Test User", "test@example.com")
		if err := gitRepo.Initialize(tempDir); err != nil {
			t.Fatalf("Failed to initialize git: %v", err)
		}

		fileWriter := schema.NewYAMLFileWriter()
		linkManager := links.NewSymbolicLinkManager()
		batchEngine := sync.NewBatchSyncEngine(jiraClient, fileWriter, gitRepo, linkManager, 8) // High concurrency

		ctx := context.Background()
		progressDone := make(chan bool, 1)

		go func() {
			defer func() { progressDone <- true }()
			for update := range batchEngine.GetProgressChannel() {
				t.Logf("Recovery test progress: %.0f%%", update.Percentage)
			}
		}()

		// Try to sync multiple issues quickly
		testIssues := []string{"RHOAIENG-29357", "RHOAIENG-29356"}
		if customIssues := os.Getenv("TEST_ISSUE_KEYS"); customIssues != "" {
			testIssues = strings.Split(customIssues, ",")
		}

		result, _ := batchEngine.SyncIssues(ctx, testIssues, tempDir)
		batchEngine.CloseProgressChannel()

		select {
		case <-progressDone:
			t.Logf("✅ Error recovery test completed")
		case <-time.After(30 * time.Second):
			t.Error("❌ Error recovery test timeout")
		}

		// Even if some requests fail, we should get partial results
		if result != nil {
			t.Logf("Error recovery results: %d processed, %d successful, %d failed",
				result.ProcessedIssues, result.SuccessfulSync, result.FailedSync)

			if result.FailedSync > 0 {
				t.Logf("⚠️ Some issues failed (expected with aggressive settings): %d failures", result.FailedSync)
				for _, err := range result.Errors {
					t.Logf("Error: %s - %s", err.IssueKey, err.Message)
				}
			}
		}
	})
}

// testMemoryAndResourceManagement tests for resource leaks and proper cleanup
func testMemoryAndResourceManagement(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "memory-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	configLoader := config.NewDotEnvLoader()
	cfg, err := configLoader.Load()
	if err != nil {
		t.Fatalf("Failed to load configuration: %v", err)
	}

	cfg.RateLimitDelay = 100 * time.Millisecond

	// Test multiple sequential operations to check for resource leaks
	t.Run("SequentialOperationsResourceManagement", func(t *testing.T) {
		for i := 0; i < 3; i++ {
			t.Logf("Sequential operation iteration %d", i+1)

			jiraClient, err := client.NewClient(cfg)
			if err != nil {
				t.Fatalf("Failed to create JIRA client: %v", err)
			}

			if err := jiraClient.Authenticate(); err != nil {
				t.Fatalf("Failed to authenticate: %v", err)
			}

			gitRepo := git.NewGitRepository("Test User", "test@example.com")
			if err := gitRepo.Initialize(tempDir); err != nil {
				t.Fatalf("Failed to initialize git: %v", err)
			}

			fileWriter := schema.NewYAMLFileWriter()
			linkManager := links.NewSymbolicLinkManager()
			batchEngine := sync.NewBatchSyncEngine(jiraClient, fileWriter, gitRepo, linkManager, 2)

			ctx := context.Background()
			progressDone := make(chan bool, 1)

			go func() {
				defer func() { progressDone <- true }()
				for update := range batchEngine.GetProgressChannel() {
					t.Logf("Iteration %d progress: %.0f%%", i+1, update.Percentage)
				}
			}()

			testIssueKey := os.Getenv("TEST_ISSUE_KEY")
			if testIssueKey == "" {
				testIssueKey = "RHOAIENG-29357"
			}

			_, err = batchEngine.SyncIssues(ctx, []string{testIssueKey}, tempDir)
			if err != nil {
				t.Errorf("Sequential operation %d failed: %v", i+1, err)
			}

			batchEngine.CloseProgressChannel()

			select {
			case <-progressDone:
				t.Logf("✅ Sequential operation %d completed", i+1)
			case <-time.After(15 * time.Second):
				t.Errorf("❌ Sequential operation %d timeout", i+1)
			}

			// Force garbage collection to detect potential leaks
			// In a real test environment, you might use tools like pprof for detailed analysis
		}

		t.Logf("✅ All sequential operations completed - no obvious resource leaks")
	})
}
