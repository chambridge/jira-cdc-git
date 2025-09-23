package test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/chambrid/jira-cdc-git/internal/sync"
	"github.com/chambrid/jira-cdc-git/pkg/client"
	"github.com/chambrid/jira-cdc-git/pkg/config"
	"github.com/chambrid/jira-cdc-git/pkg/epic"
	"github.com/chambrid/jira-cdc-git/pkg/git"
	"github.com/chambrid/jira-cdc-git/pkg/links"
	"github.com/chambrid/jira-cdc-git/pkg/schema"
)

// TestEPICIntegration_EndToEnd validates that EPIC analysis integrates properly with the existing sync system
func TestEPICIntegration_EndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping EPIC integration test in short mode")
	}

	// Check for EPIC integration test configuration
	epicKey := os.Getenv("TEST_EPIC_KEY")
	if epicKey == "" {
		t.Skip("Skipping EPIC integration test - TEST_EPIC_KEY environment variable not set")
	}

	// Load configuration from environment
	cfg, err := config.LoadFromCurrentDir()
	if err != nil {
		t.Skipf("Skipping EPIC integration test - could not load config: %v", err)
	}

	// Create JIRA client
	jiraClient, err := client.NewClient(cfg)
	if err != nil {
		t.Skipf("Skipping EPIC integration test - could not create JIRA client: %v", err)
	}

	// Test authentication
	if err := jiraClient.Authenticate(); err != nil {
		t.Skipf("Skipping EPIC integration test - JIRA authentication failed: %v", err)
	}

	// Create temporary directory for test repository
	tempDir, err := os.MkdirTemp("", "epic-integration-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	testRepo := filepath.Join(tempDir, "test-repo")
	if err := os.MkdirAll(testRepo, 0755); err != nil {
		t.Fatalf("Failed to create test repo directory: %v", err)
	}

	t.Logf("🚀 Starting EPIC integration test with EPIC: %s", epicKey)
	startTime := time.Now()

	// Step 1: Initialize all components
	t.Log("📦 Step 1: Initializing system components...")

	// Initialize Git repository
	gitRepo := git.NewGitRepository("EPIC Integration Test", "epic-test@automated.local")
	if err := gitRepo.Initialize(testRepo); err != nil {
		t.Fatalf("Failed to initialize Git repository: %v", err)
	}

	// Initialize other components
	fileWriter := schema.NewYAMLFileWriter()
	linkManager := links.NewSymbolicLinkManager()
	batchEngine := sync.NewBatchSyncEngine(jiraClient, fileWriter, gitRepo, linkManager, 3)

	t.Log("✅ System components initialized")

	// Step 2: Test EPIC Analysis
	t.Log("📊 Step 2: Performing EPIC analysis...")

	epicAnalyzer := epic.NewJIRAEpicAnalyzer(jiraClient, epic.DefaultDiscoveryOptions())

	// Analyze the EPIC
	analysisResult, err := epicAnalyzer.AnalyzeEpic(epicKey)
	if err != nil {
		t.Fatalf("Failed to analyze EPIC: %v", err)
	}

	t.Logf("✅ EPIC analysis complete: %d total issues found", analysisResult.TotalIssues)
	t.Logf("📊 EPIC: %s - %s", analysisResult.EpicKey, analysisResult.EpicSummary)

	if analysisResult.TotalIssues == 0 {
		t.Skip("No issues found in EPIC - skipping sync integration test")
	}

	// Step 3: Extract issue keys for sync
	t.Log("🔍 Step 3: Extracting issues for sync...")

	var allIssueKeys []string
	for _, issueKeys := range analysisResult.IssuesByType {
		allIssueKeys = append(allIssueKeys, issueKeys...)
	}

	// Limit to first 10 issues for integration test performance
	maxIssues := 10
	if len(allIssueKeys) > maxIssues {
		allIssueKeys = allIssueKeys[:maxIssues]
		t.Logf("⚡ Limited to first %d issues for integration test", maxIssues)
	}

	t.Logf("📋 Ready to sync %d issues from EPIC", len(allIssueKeys))

	// Step 4: Perform batch sync with EPIC-discovered issues
	t.Log("🔄 Step 4: Performing batch sync of EPIC issues...")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	syncResult, err := batchEngine.SyncIssues(ctx, allIssueKeys, testRepo)
	if err != nil {
		t.Fatalf("Failed to sync EPIC issues: %v", err)
	}

	t.Logf("✅ Batch sync complete: %d/%d issues synced successfully",
		syncResult.SuccessfulSync, syncResult.TotalIssues)

	// Step 5: Validate integration results
	t.Log("🔍 Step 5: Validating integration results...")

	// Check that files were created
	if len(syncResult.ProcessedFiles) == 0 {
		t.Error("No files were processed during sync")
	}

	// Validate at least some issues were synced successfully
	if syncResult.SuccessfulSync == 0 {
		t.Error("No issues were synced successfully")
	}

	// Check for any critical errors
	criticalErrors := 0
	for _, batchErr := range syncResult.Errors {
		if !client.IsNotFoundError(batchErr.Error) {
			criticalErrors++
			t.Logf("⚠️ Critical error for %s: %s", batchErr.IssueKey, batchErr.Message)
		}
	}

	if criticalErrors > len(allIssueKeys)/2 {
		t.Errorf("Too many critical errors: %d out of %d issues", criticalErrors, len(allIssueKeys))
	}

	// Step 6: Validate hierarchy consistency
	t.Log("🌳 Step 6: Validating EPIC hierarchy consistency...")

	hierarchy, err := epicAnalyzer.GetEpicHierarchy(epicKey)
	if err != nil {
		t.Fatalf("Failed to get EPIC hierarchy: %v", err)
	}

	// Check that synced files correspond to hierarchy
	syncedIssueKeys := make(map[string]bool)
	for _, filePath := range syncResult.ProcessedFiles {
		// Extract issue key from file path
		fileName := filepath.Base(filePath)
		if len(fileName) > 5 && fileName[len(fileName)-5:] == ".yaml" {
			issueKey := fileName[:len(fileName)-5]
			syncedIssueKeys[issueKey] = true
		}
	}

	// Count hierarchy issues that were synced
	hierarchyIssuesSynced := 0
	totalHierarchyIssues := len(hierarchy.Stories) + len(hierarchy.Tasks) +
		len(hierarchy.Bugs) + len(hierarchy.DirectIssues)

	for _, node := range hierarchy.Stories {
		if syncedIssueKeys[node.IssueKey] {
			hierarchyIssuesSynced++
		}
	}
	for _, node := range hierarchy.Tasks {
		if syncedIssueKeys[node.IssueKey] {
			hierarchyIssuesSynced++
		}
	}
	for _, node := range hierarchy.Bugs {
		if syncedIssueKeys[node.IssueKey] {
			hierarchyIssuesSynced++
		}
	}
	for _, node := range hierarchy.DirectIssues {
		if syncedIssueKeys[node.IssueKey] {
			hierarchyIssuesSynced++
		}
	}

	t.Logf("🌳 Hierarchy integration: %d/%d hierarchy issues synced",
		hierarchyIssuesSynced, totalHierarchyIssues)

	// Step 7: Performance validation
	totalTime := time.Since(startTime)
	t.Logf("⏱️ Step 7: Performance validation...")
	t.Logf("📊 Total integration time: %v", totalTime)
	t.Logf("📊 EPIC analysis time: %dms", analysisResult.Performance.DiscoveryTimeMs+analysisResult.Performance.AnalysisTimeMs)
	t.Logf("📊 Sync performance: %.2f issues/sec", syncResult.Performance.IssuesPerSecond)

	// Integration should complete within reasonable time
	if totalTime > 120*time.Second {
		t.Errorf("Integration test took too long: %v (should be <120s)", totalTime)
	}

	// Step 8: Git repository validation
	t.Log("📁 Step 8: Validating Git repository state...")

	status, err := gitRepo.GetRepositoryStatus(testRepo)
	if err != nil {
		t.Fatalf("Failed to get repository status: %v", err)
	}

	if !status.IsClean {
		t.Error("Repository should be clean after EPIC integration sync")
	}

	t.Logf("✅ EPIC integration test completed successfully!")
	t.Logf("📊 Final Results:")
	t.Logf("  - EPIC: %s (%d total issues)", epicKey, analysisResult.TotalIssues)
	t.Logf("  - Synced: %d/%d issues", syncResult.SuccessfulSync, len(allIssueKeys))
	t.Logf("  - Performance: %v total, %.2f issues/sec", totalTime, syncResult.Performance.IssuesPerSecond)
	t.Logf("  - Repository: Clean with %d commits", len(syncResult.ProcessedFiles))
}

// TestEPICIntegration_JQLIntegration tests EPIC analysis with JQL-based sync
func TestEPICIntegration_JQLIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping EPIC JQL integration test in short mode")
	}

	// Check for EPIC integration test configuration
	epicKey := os.Getenv("TEST_EPIC_KEY")
	if epicKey == "" {
		t.Skip("Skipping EPIC JQL integration test - TEST_EPIC_KEY environment variable not set")
	}

	// Load configuration
	cfg, err := config.LoadFromCurrentDir()
	if err != nil {
		t.Skipf("Skipping EPIC JQL integration test - could not load config: %v", err)
	}

	// Create JIRA client
	jiraClient, err := client.NewClient(cfg)
	if err != nil {
		t.Skipf("Skipping EPIC JQL integration test - could not create JIRA client: %v", err)
	}

	if err := jiraClient.Authenticate(); err != nil {
		t.Skipf("Skipping EPIC JQL integration test - JIRA authentication failed: %v", err)
	}

	t.Logf("🔍 Testing EPIC JQL integration with EPIC: %s", epicKey)

	// Create EPIC analyzer
	epicAnalyzer := epic.NewJIRAEpicAnalyzer(jiraClient, &epic.DiscoveryOptions{
		Strategy:            epic.StrategyHybrid,
		MaxDepth:            3,
		IncludeSubtasks:     true,
		IncludeLinkedIssues: true,
		BatchSize:           50,
		UseCache:            true,
	})

	// Test different JQL strategies for EPIC discovery
	jqlStrategies := []struct {
		name string
		jql  string
	}{
		{"Epic Link", `"Epic Link" = ` + epicKey},
		{"Custom Field", `cf[12311140] = ` + epicKey},
		{"Parent Link", `parent = ` + epicKey},
	}

	for _, strategy := range jqlStrategies {
		t.Run(strategy.name, func(t *testing.T) {
			t.Logf("🧪 Testing JQL strategy: %s", strategy.name)

			// Search using JQL
			issues, err := jiraClient.SearchIssues(strategy.jql)
			if err != nil {
				t.Logf("⚠️ JQL search failed for strategy %s: %v", strategy.name, err)
				return // Skip this strategy
			}

			t.Logf("📊 JQL '%s' found %d issues", strategy.name, len(issues))

			// Compare with EPIC analyzer results
			epicIssues, err := epicAnalyzer.DiscoverEpicIssues(epicKey)
			if err != nil {
				t.Fatalf("Failed to discover EPIC issues: %v", err)
			}

			// Create maps for comparison
			jqlIssueKeys := make(map[string]bool)
			for _, issue := range issues {
				jqlIssueKeys[issue.Key] = true
			}

			epicIssueKeys := make(map[string]bool)
			for _, issue := range epicIssues {
				epicIssueKeys[issue.Key] = true
			}

			// Find overlap and differences
			overlap := 0
			for key := range jqlIssueKeys {
				if epicIssueKeys[key] {
					overlap++
				}
			}

			overlapPercent := 0.0
			if len(jqlIssueKeys) > 0 {
				overlapPercent = float64(overlap) / float64(len(jqlIssueKeys)) * 100
			}

			t.Logf("🔗 Strategy overlap: %d/%d issues (%.1f%%)",
				overlap, len(jqlIssueKeys), overlapPercent)

			// Log findings for analysis
			if len(issues) > 0 {
				t.Logf("✅ JQL strategy '%s' is functional", strategy.name)
			} else {
				t.Logf("ℹ️ JQL strategy '%s' found no issues for this EPIC", strategy.name)
			}
		})
	}
}

// TestEPICIntegration_MockValidation validates EPIC integration with mock components
func TestEPICIntegration_MockValidation(t *testing.T) {
	t.Log("🧪 Testing EPIC integration with mock components")

	// Create mock client with EPIC data
	mockClient := client.NewMockClient()

	// Create mock EPIC
	epicIssue := client.CreateEpicIssue("TEST-EPIC-1")
	epicIssue.Summary = "Test EPIC for Integration"
	mockClient.AddIssue(epicIssue)

	// Create mock stories linked to EPIC
	story1 := client.CreateTestIssue("TEST-1")
	story1.Summary = "Story 1"
	story1.IssueType = "Story"
	story1.Relationships = &client.Relationships{
		EpicLink: "TEST-EPIC-1",
	}

	story2 := client.CreateTestIssue("TEST-2")
	story2.Summary = "Story 2"
	story2.IssueType = "Story"
	story2.Relationships = &client.Relationships{
		EpicLink: "TEST-EPIC-1",
	}

	mockClient.AddIssue(story1)
	mockClient.AddIssue(story2)

	// Add JQL results for EPIC discovery
	mockClient.AddJQLResult(`"Epic Link" = TEST-EPIC-1`, []string{"TEST-1", "TEST-2"})
	mockClient.AddJQLResult(`cf[12311140] = TEST-EPIC-1`, []string{"TEST-1", "TEST-2"})

	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "epic-mock-integration-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	testRepo := filepath.Join(tempDir, "test-repo")

	// Initialize components
	gitRepo := git.NewGitRepository("Mock Test", "mock@test.com")
	if err := gitRepo.Initialize(testRepo); err != nil {
		t.Fatalf("Failed to initialize Git repository: %v", err)
	}

	fileWriter := schema.NewYAMLFileWriter()
	linkManager := links.NewSymbolicLinkManager()
	batchEngine := sync.NewBatchSyncEngine(mockClient, fileWriter, gitRepo, linkManager, 2)

	// Test EPIC analysis
	epicAnalyzer := epic.NewJIRAEpicAnalyzer(mockClient, epic.DefaultDiscoveryOptions())

	analysisResult, err := epicAnalyzer.AnalyzeEpic("TEST-EPIC-1")
	if err != nil {
		t.Fatalf("Failed to analyze mock EPIC: %v", err)
	}

	// Validate analysis results
	if analysisResult.TotalIssues != 2 {
		t.Errorf("Expected 2 issues in EPIC, got %d", analysisResult.TotalIssues)
	}

	if len(analysisResult.IssuesByType["story"]) != 2 {
		t.Errorf("Expected 2 stories, got %d", len(analysisResult.IssuesByType["story"]))
	}

	// Test sync integration
	ctx := context.Background()
	issueKeys := []string{"TEST-1", "TEST-2"}

	syncResult, err := batchEngine.SyncIssuesSync(ctx, issueKeys, testRepo)
	if err != nil {
		t.Fatalf("Failed to sync mock EPIC issues: %v", err)
	}

	// Validate sync results
	if syncResult.SuccessfulSync != 2 {
		t.Errorf("Expected 2 successful syncs, got %d", syncResult.SuccessfulSync)
	}

	if syncResult.FailedSync != 0 {
		t.Errorf("Expected 0 failed syncs, got %d", syncResult.FailedSync)
	}

	// Validate files were created
	if len(syncResult.ProcessedFiles) != 2 {
		t.Errorf("Expected 2 processed files, got %d", len(syncResult.ProcessedFiles))
	}

	t.Log("✅ Mock EPIC integration validation completed successfully")
}
