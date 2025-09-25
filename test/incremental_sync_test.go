package test

import (
	"context"
	"os"
	"testing"

	"github.com/chambrid/jira-cdc-git/internal/sync"
	"github.com/chambrid/jira-cdc-git/pkg/client"
	"github.com/chambrid/jira-cdc-git/pkg/config"
	"github.com/chambrid/jira-cdc-git/pkg/git"
	"github.com/chambrid/jira-cdc-git/pkg/links"
	"github.com/chambrid/jira-cdc-git/pkg/schema"
	"github.com/chambrid/jira-cdc-git/pkg/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIncrementalSyncWorkflow tests the complete incremental sync workflow
func TestIncrementalSyncWorkflow(t *testing.T) {
	// Skip if no JIRA config available
	if !hasJIRAConfig() {
		t.Skip("Skipping incremental sync test - no JIRA configuration found")
	}

	// Load configuration
	configLoader := config.NewDotEnvLoader()
	cfg, err := configLoader.Load()
	require.NoError(t, err, "Failed to load configuration")

	// Create temporary repository
	tempRepo, err := os.MkdirTemp("", "incremental-sync-test-*")
	require.NoError(t, err, "Failed to create temp repository")
	defer func() { _ = os.RemoveAll(tempRepo) }()

	// Initialize Git repository
	gitRepo := git.NewGitRepository("JIRA CDC Git Sync Test", "test@automated.local")
	err = gitRepo.Initialize(tempRepo)
	require.NoError(t, err, "Failed to initialize Git repository")

	// Initialize components
	jiraClient, err := client.NewClient(cfg)
	require.NoError(t, err, "Failed to create JIRA client")

	err = jiraClient.Authenticate()
	require.NoError(t, err, "Failed to authenticate with JIRA")

	fileWriter := schema.NewYAMLFileWriter()
	linkManager := links.NewSymbolicLinkManager()
	stateManager := state.NewFileStateManager(state.FormatYAML)

	// Create incremental sync engine
	incrementalEngine := sync.NewIncrementalBatchSyncEngine(
		jiraClient, fileWriter, gitRepo, linkManager, stateManager, 2)

	t.Run("initial_sync", func(t *testing.T) {
		// Perform initial sync with force flag
		options := sync.IncrementalSyncOptions{
			Force:           true,
			DryRun:          false,
			IncludeNew:      true,
			IncludeModified: true,
		}

		result, err := incrementalEngine.SyncIssuesIncremental(
			context.Background(),
			[]string{"TEST-1"},
			tempRepo,
			options,
		)

		require.NoError(t, err, "Initial sync failed")
		assert.Equal(t, 1, result.TotalIssues)
		assert.Equal(t, 1, result.ProcessedIssues)
		assert.Equal(t, 1, result.SuccessfulSync)
		assert.Equal(t, 0, result.FailedSync)
		assert.NotEmpty(t, result.ProcessedFiles)

		// Verify state was created
		assert.NotNil(t, incrementalEngine.GetState())

		// Verify issue file exists
		issueFile := result.ProcessedFiles[0]
		assert.FileExists(t, issueFile)

		// Verify state tracking
		issueState, exists := stateManager.GetIssueState(incrementalEngine.GetState(), "TEST-1")
		assert.True(t, exists)
		assert.NotNil(t, issueState)
		assert.Equal(t, "TEST-1", issueState.Key)
		assert.NotZero(t, issueState.LastSynced)
	})

	t.Run("incremental_sync_no_changes", func(t *testing.T) {
		// Perform incremental sync immediately after initial sync
		options := sync.IncrementalSyncOptions{
			Force:           false,
			DryRun:          false,
			IncludeNew:      true,
			IncludeModified: true,
		}

		result, err := incrementalEngine.SyncIssuesIncremental(
			context.Background(),
			[]string{"TEST-1"},
			tempRepo,
			options,
		)

		require.NoError(t, err, "Incremental sync failed")
		assert.Equal(t, 1, result.TotalIssues)
		// Should skip since no changes detected
		assert.Equal(t, 0, result.ProcessedIssues)
		assert.Equal(t, 0, result.SuccessfulSync)
	})

	t.Run("force_sync_after_initial", func(t *testing.T) {
		// Force sync should process issue even if no changes
		options := sync.IncrementalSyncOptions{
			Force:           true,
			DryRun:          false,
			IncludeNew:      true,
			IncludeModified: true,
		}

		result, err := incrementalEngine.SyncIssuesIncremental(
			context.Background(),
			[]string{"TEST-1"},
			tempRepo,
			options,
		)

		require.NoError(t, err, "Force sync failed")
		assert.Equal(t, 1, result.TotalIssues)
		assert.Equal(t, 1, result.ProcessedIssues)
		assert.Equal(t, 1, result.SuccessfulSync)
		assert.Equal(t, 0, result.FailedSync)
	})

	t.Run("dry_run_sync", func(t *testing.T) {
		// Dry run should show what would be synced without making changes
		options := sync.IncrementalSyncOptions{
			Force:           true,
			DryRun:          true,
			IncludeNew:      true,
			IncludeModified: true,
		}

		result, err := incrementalEngine.SyncIssuesIncremental(
			context.Background(),
			[]string{"TEST-1"},
			tempRepo,
			options,
		)

		require.NoError(t, err, "Dry run sync failed")
		assert.Equal(t, 1, result.TotalIssues)
		assert.Equal(t, 1, result.ProcessedIssues)
		assert.Equal(t, 1, result.SuccessfulSync)
		assert.Equal(t, 0, result.FailedSync)
		// Dry run should show files that would be processed
		assert.NotEmpty(t, result.ProcessedFiles)
	})

	t.Run("sync_statistics", func(t *testing.T) {
		// Verify sync statistics are tracked correctly
		stats := incrementalEngine.GetSyncStatistics()
		assert.Greater(t, stats.TotalOperations, 0)
		assert.Greater(t, stats.SuccessfulOps, 0)
		assert.Equal(t, 0, stats.FailedOps)
		assert.Equal(t, 1, stats.UniqueIssues)
		assert.Contains(t, stats.ActiveProjects, "TEST")
	})

	t.Run("state_validation", func(t *testing.T) {
		// Validate repository state
		validationResult, err := incrementalEngine.ValidateRepositoryState(tempRepo)
		require.NoError(t, err, "State validation failed")
		assert.True(t, validationResult.Valid)
		assert.Empty(t, validationResult.Errors)
		assert.Empty(t, validationResult.MissingIssues)
	})
}

// TestIncrementalSyncBackwardCompatibility verifies that incremental sync doesn't break existing workflows
func TestIncrementalSyncBackwardCompatibility(t *testing.T) {
	// Skip if no JIRA config available
	if !hasJIRAConfig() {
		t.Skip("Skipping backward compatibility test - no JIRA configuration found")
	}

	// Load configuration
	configLoader := config.NewDotEnvLoader()
	cfg, err := configLoader.Load()
	require.NoError(t, err, "Failed to load configuration")

	// Create temporary repository
	tempRepo, err := os.MkdirTemp("", "backward-compat-test-*")
	require.NoError(t, err, "Failed to create temp repository")
	defer func() { _ = os.RemoveAll(tempRepo) }()

	// Initialize Git repository
	gitRepo := git.NewGitRepository("JIRA CDC Git Sync Test", "test@automated.local")
	err = gitRepo.Initialize(tempRepo)
	require.NoError(t, err, "Failed to initialize Git repository")

	// Initialize components
	jiraClient, err := client.NewClient(cfg)
	require.NoError(t, err, "Failed to create JIRA client")

	err = jiraClient.Authenticate()
	require.NoError(t, err, "Failed to authenticate with JIRA")

	fileWriter := schema.NewYAMLFileWriter()
	linkManager := links.NewSymbolicLinkManager()

	t.Run("regular_batch_engine_still_works", func(t *testing.T) {
		// Test that the original BatchSyncEngine still works
		batchEngine := sync.NewBatchSyncEngine(jiraClient, fileWriter, gitRepo, linkManager, 2)

		result, err := batchEngine.SyncIssues(
			context.Background(),
			[]string{"TEST-1"},
			tempRepo,
		)

		require.NoError(t, err, "Regular batch sync failed")
		assert.Equal(t, 1, result.TotalIssues)
		assert.Equal(t, 1, result.ProcessedIssues)
		assert.Equal(t, 1, result.SuccessfulSync)
		assert.Equal(t, 0, result.FailedSync)
		assert.NotEmpty(t, result.ProcessedFiles)

		// Verify issue file exists
		issueFile := result.ProcessedFiles[0]
		assert.FileExists(t, issueFile)
	})

	t.Run("incremental_engine_with_existing_files", func(t *testing.T) {
		// Test that incremental engine can work with files created by regular engine
		stateManager := state.NewFileStateManager(state.FormatYAML)
		incrementalEngine := sync.NewIncrementalBatchSyncEngine(
			jiraClient, fileWriter, gitRepo, linkManager, stateManager, 2)

		// Sync with force to process existing file
		options := sync.IncrementalSyncOptions{
			Force:           true,
			DryRun:          false,
			IncludeNew:      true,
			IncludeModified: true,
		}

		result, err := incrementalEngine.SyncIssuesIncremental(
			context.Background(),
			[]string{"TEST-1"},
			tempRepo,
			options,
		)

		require.NoError(t, err, "Incremental sync with existing files failed")
		assert.Equal(t, 1, result.TotalIssues)
		assert.Equal(t, 1, result.ProcessedIssues)
		assert.Equal(t, 1, result.SuccessfulSync)
		assert.Equal(t, 0, result.FailedSync)

		// Verify state was created and tracks the existing file
		assert.NotNil(t, incrementalEngine.GetState())
		issueState, exists := stateManager.GetIssueState(incrementalEngine.GetState(), "TEST-1")
		assert.True(t, exists)
		assert.NotNil(t, issueState)
	})
}

// hasJIRAConfig checks if JIRA configuration is available for testing
func hasJIRAConfig() bool {
	// Check for .env file
	if _, err := os.Stat(".env"); err == nil {
		return true
	}

	// Check for environment variables
	return os.Getenv("JIRA_URL") != "" &&
		os.Getenv("JIRA_EMAIL") != "" &&
		os.Getenv("JIRA_TOKEN") != ""
}
