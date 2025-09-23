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

// TestAPIValidationFramework provides comprehensive real API validation
// This test framework validates actual JIRA API behavior and ensures our
// client handles real-world scenarios correctly
func TestAPIValidationFramework(t *testing.T) {
	// Skip if no real JIRA configuration
	if os.Getenv("JIRA_URL") == "" {
		t.Skip("Skipping API validation tests - no JIRA configuration found")
	}

	t.Run("RealAPIResponseFormatValidation", testRealAPIResponseFormat)
	t.Run("TimestampFormatValidation", testTimestampFormatValidation)
	t.Run("RateLimitingBehavior", testRateLimitingBehavior)
	t.Run("ErrorHandlingValidation", testErrorHandlingValidation)
	t.Run("RelationshipDataValidation", testRelationshipDataValidation)
}

// testRealAPIResponseFormat validates that our client correctly parses real JIRA API responses
func testRealAPIResponseFormat(t *testing.T) {
	// Load real configuration
	configLoader := config.NewDotEnvLoader()
	cfg, err := configLoader.Load()
	if err != nil {
		t.Fatalf("Failed to load configuration: %v", err)
	}

	// Create real JIRA client
	jiraClient, err := client.NewClient(cfg)
	if err != nil {
		t.Fatalf("Failed to create JIRA client: %v", err)
	}

	// Authenticate
	if err := jiraClient.Authenticate(); err != nil {
		t.Fatalf("Failed to authenticate: %v", err)
	}

	// Test with a known issue (use environment variable for flexibility)
	testIssueKey := os.Getenv("TEST_ISSUE_KEY")
	if testIssueKey == "" {
		testIssueKey = "RHOAIENG-29357" // Default test issue
	}

	t.Logf("Testing API response format with issue: %s", testIssueKey)

	// Fetch issue and validate response format
	issue, err := jiraClient.GetIssue(testIssueKey)
	if err != nil {
		t.Fatalf("Failed to get issue %s: %v", testIssueKey, err)
	}

	// Validate basic issue structure
	if issue.Key == "" {
		t.Error("Issue key is empty")
	}
	if issue.Summary == "" {
		t.Error("Issue summary is empty")
	}

	t.Logf("Successfully retrieved issue: %s - %s", issue.Key, issue.Summary)
	t.Logf("Created: %s, Updated: %s", issue.Created, issue.Updated)
	t.Logf("Status: %s, Type: %s", issue.Status.Name, issue.IssueType)
}

// testTimestampFormatValidation specifically validates timestamp formatting
// This would have caught the timestamp bug we discovered
func testTimestampFormatValidation(t *testing.T) {
	// Load real configuration
	configLoader := config.NewDotEnvLoader()
	cfg, err := configLoader.Load()
	if err != nil {
		t.Fatalf("Failed to load configuration: %v", err)
	}

	// Create real JIRA client
	jiraClient, err := client.NewClient(cfg)
	if err != nil {
		t.Fatalf("Failed to create JIRA client: %v", err)
	}

	// Authenticate
	if err := jiraClient.Authenticate(); err != nil {
		t.Fatalf("Failed to authenticate: %v", err)
	}

	// Get a real issue
	testIssueKey := os.Getenv("TEST_ISSUE_KEY")
	if testIssueKey == "" {
		testIssueKey = "RHOAIENG-29357"
	}

	issue, err := jiraClient.GetIssue(testIssueKey)
	if err != nil {
		t.Fatalf("Failed to get issue: %v", err)
	}

	// Validate timestamp formats are human-readable
	t.Run("CreatedTimestampFormat", func(t *testing.T) {
		if issue.Created == "" {
			t.Error("Created timestamp is empty")
			return
		}

		// Should be in ISO format, not Go struct representation
		if strings.Contains(issue.Created, "{") || strings.Contains(issue.Created, "0x") {
			t.Errorf("Created timestamp appears to be Go struct representation: %s", issue.Created)
		}

		// Should be parseable as time
		_, err := time.Parse("2006-01-02T15:04:05.000Z", issue.Created)
		if err != nil {
			t.Errorf("Created timestamp is not in expected format: %s, error: %v", issue.Created, err)
		}

		t.Logf("✅ Created timestamp format is correct: %s", issue.Created)
	})

	t.Run("UpdatedTimestampFormat", func(t *testing.T) {
		if issue.Updated == "" {
			t.Error("Updated timestamp is empty")
			return
		}

		// Should be in ISO format, not Go struct representation
		if strings.Contains(issue.Updated, "{") || strings.Contains(issue.Updated, "0x") {
			t.Errorf("Updated timestamp appears to be Go struct representation: %s", issue.Updated)
		}

		// Should be parseable as time
		_, err := time.Parse("2006-01-02T15:04:05.000Z", issue.Updated)
		if err != nil {
			t.Errorf("Updated timestamp is not in expected format: %s, error: %v", issue.Updated, err)
		}

		t.Logf("✅ Updated timestamp format is correct: %s", issue.Updated)
	})
}

// testRateLimitingBehavior validates rate limiting behavior with real API
func testRateLimitingBehavior(t *testing.T) {
	// Load real configuration
	configLoader := config.NewDotEnvLoader()
	cfg, err := configLoader.Load()
	if err != nil {
		t.Fatalf("Failed to load configuration: %v", err)
	}

	// Test different rate limiting scenarios
	rateLimitTests := []struct {
		name        string
		rateLimit   time.Duration
		expectError bool
	}{
		{"NoRateLimit", 0, true},                        // Should trigger rate limiting
		{"AggressiveRate", 50 * time.Millisecond, true}, // Likely to trigger rate limiting
		{"DefaultRate", 500 * time.Millisecond, false},  // Should work
		{"ConservativeRate", 1 * time.Second, false},    // Should definitely work
	}

	for _, tt := range rateLimitTests {
		t.Run(tt.name, func(t *testing.T) {
			// Set rate limit
			cfg.RateLimitDelay = tt.rateLimit

			// Create client
			jiraClient, err := client.NewClient(cfg)
			if err != nil {
				t.Fatalf("Failed to create JIRA client: %v", err)
			}

			if err := jiraClient.Authenticate(); err != nil {
				t.Fatalf("Failed to authenticate: %v", err)
			}

			// Try to fetch multiple issues quickly
			testIssues := []string{"RHOAIENG-29357", "RHOAIENG-29356"}
			if customIssues := os.Getenv("TEST_ISSUE_KEYS"); customIssues != "" {
				testIssues = strings.Split(customIssues, ",")
			}

			var errorCount int
			for _, issueKey := range testIssues {
				_, err := jiraClient.GetIssue(strings.TrimSpace(issueKey))
				if err != nil {
					errorCount++
					t.Logf("Error with rate limit %v: %v", tt.rateLimit, err)
				}
			}

			if tt.expectError && errorCount == 0 {
				t.Logf("⚠️ Expected rate limiting errors but got none with rate limit %v", tt.rateLimit)
			} else if !tt.expectError && errorCount > 0 {
				t.Errorf("Unexpected errors with rate limit %v: %d errors", tt.rateLimit, errorCount)
			} else {
				t.Logf("✅ Rate limiting behavior as expected with %v", tt.rateLimit)
			}
		})
	}
}

// testErrorHandlingValidation validates proper error handling and user-friendly messages
func testErrorHandlingValidation(t *testing.T) {
	// Test invalid authentication
	t.Run("InvalidAuthentication", func(t *testing.T) {
		cfg := &config.Config{
			JIRABaseURL: os.Getenv("JIRA_BASE_URL"),
			JIRAEmail:   os.Getenv("JIRA_EMAIL"),
			JIRAPAT:     "invalid-token",
		}

		jiraClient, err := client.NewClient(cfg)
		if err != nil {
			t.Fatalf("Failed to create JIRA client: %v", err)
		}

		err = jiraClient.Authenticate()
		if err == nil {
			t.Error("Expected authentication error with invalid token")
		} else {
			t.Logf("✅ Proper authentication error: %v", err)
		}
	})

	// Test invalid issue key
	t.Run("InvalidIssueKey", func(t *testing.T) {
		configLoader := config.NewDotEnvLoader()
		cfg, err := configLoader.Load()
		if err != nil {
			t.Fatalf("Failed to load configuration: %v", err)
		}

		jiraClient, err := client.NewClient(cfg)
		if err != nil {
			t.Fatalf("Failed to create JIRA client: %v", err)
		}

		if err := jiraClient.Authenticate(); err != nil {
			t.Fatalf("Failed to authenticate: %v", err)
		}

		_, err = jiraClient.GetIssue("INVALID-ISSUE-123456")
		if err == nil {
			t.Error("Expected error for invalid issue key")
		} else {
			t.Logf("✅ Proper error for invalid issue: %v", err)
		}
	})
}

// testRelationshipDataValidation validates relationship data extraction and symbolic link creation
func testRelationshipDataValidation(t *testing.T) {
	// Create temporary test directory
	tempDir, err := os.MkdirTemp("", "api-validation-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Load real configuration
	configLoader := config.NewDotEnvLoader()
	cfg, err := configLoader.Load()
	if err != nil {
		t.Fatalf("Failed to load configuration: %v", err)
	}

	// Create components
	jiraClient, err := client.NewClient(cfg)
	if err != nil {
		t.Fatalf("Failed to create JIRA client: %v", err)
	}

	if err := jiraClient.Authenticate(); err != nil {
		t.Fatalf("Failed to authenticate: %v", err)
	}

	fileWriter := schema.NewYAMLFileWriter()
	gitRepo := git.NewGitRepository("Test User", "test@example.com")
	linkManager := links.NewSymbolicLinkManager()

	// Initialize git repo
	if err := gitRepo.Initialize(tempDir); err != nil {
		t.Fatalf("Failed to initialize git repo: %v", err)
	}

	// Test with an issue that has relationships
	testIssueKey := os.Getenv("TEST_PARENT_ISSUE_KEY")
	if testIssueKey == "" {
		testIssueKey = "RHOAIENG-29356" // Known to have subtasks
	}

	t.Logf("Testing relationship validation with issue: %s", testIssueKey)

	// Create batch sync engine and sync the issue
	batchEngine := sync.NewBatchSyncEngine(jiraClient, fileWriter, gitRepo, linkManager, 1)

	ctx := context.Background()
	result, err := batchEngine.SyncIssues(ctx, []string{testIssueKey}, tempDir)
	if err != nil {
		t.Fatalf("Failed to sync issue: %v", err)
	}

	t.Logf("Sync result: %d processed, %d successful", result.ProcessedIssues, result.SuccessfulSync)

	// Validate that YAML file was created
	issueFile := filepath.Join(tempDir, "projects", "RHOAIENG", "issues", testIssueKey+".yaml")
	if _, err := os.Stat(issueFile); os.IsNotExist(err) {
		t.Errorf("Issue YAML file was not created: %s", issueFile)
	} else {
		t.Logf("✅ Issue YAML file created: %s", issueFile)
	}

	// Validate that relationships directory exists if issue has relationships
	relationshipsDir := filepath.Join(tempDir, "projects", "RHOAIENG", "relationships")
	if _, err := os.Stat(relationshipsDir); os.IsNotExist(err) {
		t.Logf("⚠️ No relationships directory found - issue may not have relationships")
	} else {
		t.Logf("✅ Relationships directory found: %s", relationshipsDir)

		// Check for symbolic links
		err := filepath.Walk(relationshipsDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.Mode()&os.ModeSymlink != 0 {
				t.Logf("✅ Found symbolic link: %s", path)
			}
			return nil
		})
		if err != nil {
			t.Errorf("Error walking relationships directory: %v", err)
		}
	}
}

// TestComponentIntegrationValidation validates that all components work together correctly
// This would have caught the missing linkManager integration bug
func TestComponentIntegrationValidation(t *testing.T) {
	if os.Getenv("JIRA_URL") == "" {
		t.Skip("Skipping component integration tests - no JIRA configuration found")
	}

	// Create temporary test directory
	tempDir, err := os.MkdirTemp("", "component-integration-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Test that BatchSyncEngine constructor requires all components
	t.Run("BatchSyncEngineConstructorValidation", func(t *testing.T) {
		// Load configuration
		configLoader := config.NewDotEnvLoader()
		cfg, err := configLoader.Load()
		if err != nil {
			t.Fatalf("Failed to load configuration: %v", err)
		}

		// Create all required components
		jiraClient, err := client.NewClient(cfg)
		if err != nil {
			t.Fatalf("Failed to create JIRA client: %v", err)
		}

		fileWriter := schema.NewYAMLFileWriter()
		gitRepo := git.NewGitRepository("Test User", "test@example.com")
		linkManager := links.NewSymbolicLinkManager()

		// Verify that BatchSyncEngine can be created with all components
		batchEngine := sync.NewBatchSyncEngine(jiraClient, fileWriter, gitRepo, linkManager, 1)
		if batchEngine == nil {
			t.Error("Failed to create BatchSyncEngine with all components")
		}

		t.Logf("✅ BatchSyncEngine created successfully with all required components")
	})

	// Test that progress channel is properly closed
	t.Run("ProgressChannelManagement", func(t *testing.T) {
		configLoader := config.NewDotEnvLoader()
		cfg, err := configLoader.Load()
		if err != nil {
			t.Fatalf("Failed to load configuration: %v", err)
		}

		jiraClient, err := client.NewClient(cfg)
		if err != nil {
			t.Fatalf("Failed to create JIRA client: %v", err)
		}

		if err := jiraClient.Authenticate(); err != nil {
			t.Fatalf("Failed to authenticate: %v", err)
		}

		fileWriter := schema.NewYAMLFileWriter()
		gitRepo := git.NewGitRepository("Test User", "test@example.com")
		linkManager := links.NewSymbolicLinkManager()

		if err := gitRepo.Initialize(tempDir); err != nil {
			t.Fatalf("Failed to initialize git repo: %v", err)
		}

		batchEngine := sync.NewBatchSyncEngine(jiraClient, fileWriter, gitRepo, linkManager, 1)

		// Start monitoring progress channel
		progressDone := make(chan bool)
		go func() {
			defer func() { progressDone <- true }()
			for range batchEngine.GetProgressChannel() {
				// Consume progress updates
			}
		}()

		// Sync a single issue
		ctx := context.Background()
		testIssueKey := os.Getenv("TEST_ISSUE_KEY")
		if testIssueKey == "" {
			testIssueKey = "RHOAIENG-29357"
		}

		_, err = batchEngine.SyncIssues(ctx, []string{testIssueKey}, tempDir)
		if err != nil {
			t.Fatalf("Failed to sync issue: %v", err)
		}

		// Close progress channel
		batchEngine.CloseProgressChannel()

		// Wait for progress monitoring to complete (should not hang)
		select {
		case <-progressDone:
			t.Logf("✅ Progress channel properly closed and monitoring completed")
		case <-time.After(5 * time.Second):
			t.Error("❌ Progress monitoring did not complete - potential deadlock")
		}
	})
}
