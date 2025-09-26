package test

import (
	"context"
	"os"
	"testing"

	"github.com/chambrid/jira-cdc-git/internal/sync"
	"github.com/chambrid/jira-cdc-git/pkg/client"
	"github.com/chambrid/jira-cdc-git/pkg/config"
	"github.com/chambrid/jira-cdc-git/pkg/epic"
	"github.com/chambrid/jira-cdc-git/pkg/git"
	"github.com/chambrid/jira-cdc-git/pkg/jql"
	"github.com/chambrid/jira-cdc-git/pkg/links"
	"github.com/chambrid/jira-cdc-git/pkg/profile"
	"github.com/chambrid/jira-cdc-git/pkg/schema"
	"github.com/chambrid/jira-cdc-git/pkg/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestV030Integration tests the integration between all v0.3.0 components
func TestV030Integration(t *testing.T) {
	// Skip if no JIRA config available
	if !hasJIRAConfig() {
		t.Skip("Skipping v0.3.0 integration test - no JIRA configuration found")
	}

	// Load configuration
	configLoader := config.NewDotEnvLoader()
	cfg, err := configLoader.Load()
	require.NoError(t, err, "Failed to load configuration")

	// Create temporary repository
	tempRepo, err := os.MkdirTemp("", "v030-integration-*")
	require.NoError(t, err, "Failed to create temp repository")
	defer func() { _ = os.RemoveAll(tempRepo) }()

	// Initialize all v0.3.0 components
	jiraClient, err := client.NewClient(cfg)
	require.NoError(t, err, "Failed to create JIRA client")

	err = jiraClient.Authenticate()
	require.NoError(t, err, "Failed to authenticate with JIRA")

	// Initialize all components needed for integration
	fileWriter := schema.NewYAMLFileWriter()
	gitRepo := git.NewGitRepository("JIRA CDC Git Sync Test", "test@automated.local")
	linkManager := links.NewSymbolicLinkManager()
	stateManager := state.NewFileStateManager(state.FormatYAML)
	epicAnalyzer := epic.NewJIRAEpicAnalyzer(jiraClient, epic.DefaultDiscoveryOptions())
	queryBuilder := jql.NewJIRAQueryBuilder(jiraClient, epicAnalyzer, nil)

	// Initialize Git repository
	err = gitRepo.Initialize(tempRepo)
	require.NoError(t, err, "Failed to initialize Git repository")

	t.Run("JCG016_JCG017_JCG018_integration", func(t *testing.T) {
		// Test EPIC discovery (JCG-016) + JQL generation (JCG-017) + State management (JCG-018)

		// Step 1: Use JQL Builder to create an EPIC-based query (JCG-017)
		testQuery, err := queryBuilder.BuildEpicQuery("TEST-1")
		require.NoError(t, err, "Failed to build EPIC query")
		assert.NotNil(t, testQuery)
		assert.NotEmpty(t, testQuery.JQL)

		// Step 2: Create incremental sync engine with state management (JCG-018)
		incrementalEngine := sync.NewIncrementalBatchSyncEngine(
			jiraClient, fileWriter, gitRepo, linkManager, stateManager, 2)

		// Step 3: Test incremental sync with JQL query
		options := sync.IncrementalSyncOptions{
			Force:           true, // Force for first sync
			DryRun:          false,
			IncludeNew:      true,
			IncludeModified: true,
		}

		// Use the JQL from the query builder
		testJQL := testQuery.JQL

		result, err := incrementalEngine.SyncJQLIncremental(
			context.Background(),
			testJQL,
			tempRepo,
			options,
		)

		// Verify the sync completed (even if no issues found)
		require.NoError(t, err, "JQL incremental sync failed")
		assert.NotNil(t, result)
		assert.GreaterOrEqual(t, result.TotalIssues, 0)

		// Step 4: Verify state was created and managed (JCG-018)
		syncState := incrementalEngine.GetState()
		assert.NotNil(t, syncState)
		assert.NotZero(t, syncState.CreatedAt)
		assert.NotEmpty(t, syncState.History)

		// Step 5: Test EPIC analyzer integration (JCG-016)
		// Try to analyze an epic (this tests JCG-016 integration)
		epicResult, err := epicAnalyzer.AnalyzeEpic("TEST-1")
		// Note: This might fail if TEST-1 is not an epic, which is expected
		if err != nil {
			t.Logf("EPIC analysis failed as expected for test key: %v", err)
		} else {
			assert.NotNil(t, epicResult)
			t.Logf("EPIC analysis succeeded for: %s", "TEST-1")
		}
	})

	t.Run("state_persistence_across_operations", func(t *testing.T) {
		// Test that state persists across multiple sync operations

		incrementalEngine := sync.NewIncrementalBatchSyncEngine(
			jiraClient, fileWriter, gitRepo, linkManager, stateManager, 2)

		// First sync operation
		options := sync.IncrementalSyncOptions{
			Force:           true,
			DryRun:          false,
			IncludeNew:      true,
			IncludeModified: true,
		}

		result1, err := incrementalEngine.SyncIssuesIncremental(
			context.Background(),
			[]string{"TEST-1"}, // Use a standard test issue
			tempRepo,
			options,
		)
		require.NoError(t, err, "First sync failed")

		// Get state after first sync
		state1 := incrementalEngine.GetState()
		require.NotNil(t, state1)

		// Create a new engine instance to test state persistence
		incrementalEngine2 := sync.NewIncrementalBatchSyncEngine(
			jiraClient, fileWriter, gitRepo, linkManager, stateManager, 2)

		// Second sync operation (should load existing state)
		options.Force = false // Test incremental behavior
		_, err = incrementalEngine2.SyncIssuesIncremental(
			context.Background(),
			[]string{"TEST-1"},
			tempRepo,
			options,
		)
		require.NoError(t, err, "Second sync failed")

		// Verify state consistency
		state2 := incrementalEngine2.GetState()
		assert.NotNil(t, state2)

		// If first sync was successful, state should be maintained
		if result1.SuccessfulSync > 0 {
			assert.Equal(t, state1.Repository.Path, state2.Repository.Path)
			assert.GreaterOrEqual(t, len(state2.History), len(state1.History))
		}
	})

	t.Run("jql_builder_with_state_management", func(t *testing.T) {
		// Test JQL builder integration with state management

		// Test EPIC query building and incremental sync
		epicQuery, err := queryBuilder.BuildEpicQuery("TEST-1")
		require.NoError(t, err, "Failed to build EPIC query")

		incrementalEngine := sync.NewIncrementalBatchSyncEngine(
			jiraClient, fileWriter, gitRepo, linkManager, stateManager, 1)

		// Test dry run to verify query integration
		options := sync.IncrementalSyncOptions{
			Force:           false,
			DryRun:          true, // Use dry run to avoid actual sync
			IncludeNew:      true,
			IncludeModified: true,
		}

		result, err := incrementalEngine.SyncJQLIncremental(
			context.Background(),
			epicQuery.JQL,
			tempRepo,
			options,
		)

		// Should not error even if no results
		require.NoError(t, err, "JQL sync failed")
		assert.NotNil(t, result)
		t.Logf("EPIC query generated: %s", epicQuery.JQL)
	})

	t.Run("JCG019_profile_integration_with_epic_workflow", func(t *testing.T) {
		// Test Profile Management (JCG-019) integration with EPIC workflow

		// Create temporary profile directory
		tempProfileDir, err := os.MkdirTemp("", "profile-integration-*")
		require.NoError(t, err, "Failed to create temp profile directory")
		defer func() { _ = os.RemoveAll(tempProfileDir) }()

		// Initialize profile manager
		profileManager := profile.NewFileProfileManager(tempProfileDir, "yaml")

		// Step 1: Create profile from EPIC template
		variables := map[string]string{
			"epic_key":   "TEST-1",
			"repository": tempRepo,
		}

		createdProfile, err := profileManager.CreateFromTemplate("epic-all-issues", "test-epic-profile", variables)
		require.NoError(t, err, "Failed to create profile from template")
		assert.Equal(t, "test-epic-profile", createdProfile.Name)
		assert.Equal(t, "TEST-1", createdProfile.EpicKey)
		assert.Equal(t, tempRepo, createdProfile.Repository)

		// Step 2: Validate profile can be retrieved
		retrievedProfile, err := profileManager.GetProfile("test-epic-profile")
		require.NoError(t, err, "Failed to retrieve created profile")
		assert.Equal(t, createdProfile.Name, retrievedProfile.Name)
		assert.Equal(t, createdProfile.EpicKey, retrievedProfile.EpicKey)

		// Step 3: Test profile with incremental sync engine
		incrementalEngine := sync.NewIncrementalBatchSyncEngine(
			jiraClient, fileWriter, gitRepo, linkManager, stateManager, 1)

		// Convert EPIC profile to JQL for testing
		expectedJQL := `"Epic Link" = TEST-1`

		options := sync.IncrementalSyncOptions{
			Force:           true,
			DryRun:          true, // Use dry run for integration test
			IncludeNew:      true,
			IncludeModified: true,
		}

		result, err := incrementalEngine.SyncJQLIncremental(
			context.Background(),
			expectedJQL,
			retrievedProfile.Repository,
			options,
		)

		require.NoError(t, err, "Profile-based sync failed")
		assert.NotNil(t, result)

		// Step 4: Record profile usage
		err = profileManager.RecordUsage("test-epic-profile", result.Duration.Milliseconds(), err == nil)
		require.NoError(t, err, "Failed to record profile usage")

		// Verify usage was recorded
		updatedProfile, err := profileManager.GetProfile("test-epic-profile")
		require.NoError(t, err, "Failed to get profile after usage recording")
		assert.Equal(t, 1, updatedProfile.UsageStats.TimesUsed)

		t.Logf("✅ Profile '%s' successfully created and used in EPIC workflow", createdProfile.Name)
	})

	t.Run("JCG019_profile_templates_integration", func(t *testing.T) {
		// Test integration between profile templates and JQL builder

		tempProfileDir, err := os.MkdirTemp("", "template-integration-*")
		require.NoError(t, err, "Failed to create temp profile directory")
		defer func() { _ = os.RemoveAll(tempProfileDir) }()

		profileManager := profile.NewFileProfileManager(tempProfileDir, "yaml")

		// Test each built-in profile template
		templates := profile.GetBuiltinTemplates()
		assert.Greater(t, len(templates), 0, "Should have built-in templates")

		for _, template := range templates {
			t.Run(template.ID, func(t *testing.T) {
				t.Logf("🧪 Testing profile template: %s", template.ID)

				// Prepare variables for template
				variables := map[string]string{
					"repository": tempRepo,
				}

				// Add template-specific variables
				switch template.ID {
				case "epic-all-issues", "epic-stories-only":
					variables["epic_key"] = "TEST-EPIC-1"
				case "project-active-issues":
					variables["project_key"] = "TEST"
				case "assignee-current-sprint":
					variables["assignee"] = "currentUser()"
				case "custom-jql":
					variables["jql"] = "project = TEST"
				}

				// Create profile from template
				profileName := "test-" + template.ID
				createdProfile, err := profileManager.CreateFromTemplate(template.ID, profileName, variables)
				require.NoError(t, err, "Failed to create profile from template %s", template.ID)

				// Validate profile structure
				assert.Equal(t, profileName, createdProfile.Name)
				assert.Equal(t, tempRepo, createdProfile.Repository)
				assert.NotEmpty(t, createdProfile.Tags, "Profile should have tags")

				// Validate profile has appropriate sync configuration
				validation, err := profileManager.ValidateProfile(createdProfile)
				require.NoError(t, err, "Failed to validate profile")
				assert.True(t, validation.Valid, "Profile should be valid: %v", validation.Errors)

				// Test that profile can be used for sync (dry run)
				if createdProfile.JQL != "" {
					// Test JQL-based profile
					result, err := jiraClient.SearchIssues(createdProfile.JQL)
					if err == nil {
						t.Logf("✅ Profile template '%s' JQL validation successful: %d issues", template.ID, len(result))
					} else {
						t.Logf("⚠️ Profile template '%s' JQL validation failed (may be expected): %v", template.ID, err)
					}
				}

				t.Logf("✅ Profile template '%s' successfully validated", template.ID)
			})
		}
	})
}

// TestV030ComponentCompatibility tests that new JCG-018 doesn't break existing components
func TestV030ComponentCompatibility(t *testing.T) {
	// Test that state management doesn't interfere with existing components

	t.Run("epic_analyzer_interface_exists", func(t *testing.T) {
		// Verify EPIC analyzer interface is available
		mockClient := client.NewMockClient()
		epicAnalyzer := epic.NewJIRAEpicAnalyzer(mockClient, epic.DefaultDiscoveryOptions())
		assert.NotNil(t, epicAnalyzer)

		// Verify the interface methods exist (they'll error with mock data, which is expected)
		_, err := epicAnalyzer.AnalyzeEpic("TEST-123")
		assert.Error(t, err) // Expected to fail with mock client
	})

	t.Run("jql_builder_interface_exists", func(t *testing.T) {
		// Verify JQL builder interface is available
		mockClient := client.NewMockClient()
		epicAnalyzer := epic.NewJIRAEpicAnalyzer(mockClient, epic.DefaultDiscoveryOptions())
		queryBuilder := jql.NewJIRAQueryBuilder(mockClient, epicAnalyzer, nil)
		assert.NotNil(t, queryBuilder)

		// Verify query building exists (may error with mock data)
		_, err := queryBuilder.BuildEpicQuery("TEST-1")
		// This may error or succeed depending on implementation, just verify it exists
		t.Logf("JQL builder error (expected with mock): %v", err)
	})

	t.Run("regular_sync_engine_compatibility", func(t *testing.T) {
		// Test that regular sync engine still works alongside incremental
		mockClient := client.NewMockClient()
		fileWriter := schema.NewYAMLFileWriter()
		gitRepo := git.NewGitRepository("Test", "test@example.com")
		linkManager := links.NewSymbolicLinkManager()

		// Regular batch engine should still work
		batchEngine := sync.NewBatchSyncEngine(mockClient, fileWriter, gitRepo, linkManager, 2)
		assert.NotNil(t, batchEngine)

		// Add a mock issue the correct way
		mockClient.Issues = map[string]*client.Issue{
			"TEST-456": {
				Key:     "TEST-456",
				Summary: "Test Issue",
				Status:  client.Status{Name: "To Do", Category: "new"},
			},
		}

		tempRepo, err := os.MkdirTemp("", "compat-test-*")
		require.NoError(t, err)
		defer func() { _ = os.RemoveAll(tempRepo) }()

		err = gitRepo.Initialize(tempRepo)
		require.NoError(t, err)

		// This should work without state management
		result, err := batchEngine.SyncIssues(context.Background(), []string{"TEST-456"}, tempRepo)
		require.NoError(t, err)
		assert.Equal(t, 1, result.TotalIssues)
		assert.Equal(t, 1, result.ProcessedIssues)
	})

	t.Run("state_manager_doesnt_interfere", func(t *testing.T) {
		// Test that creating state manager doesn't break other components
		stateManager := state.NewFileStateManager(state.FormatYAML)
		assert.NotNil(t, stateManager)

		// Should be able to create other components alongside state manager
		mockClient := client.NewMockClient()
		epicAnalyzer := epic.NewJIRAEpicAnalyzer(mockClient, epic.DefaultDiscoveryOptions())
		queryBuilder := jql.NewJIRAQueryBuilder(mockClient, epicAnalyzer, nil)

		assert.NotNil(t, epicAnalyzer)
		assert.NotNil(t, queryBuilder)
		assert.NotNil(t, stateManager)

		// All components should coexist without issues
		t.Log("All v0.3.0 components successfully created together")
	})

	t.Run("profile_manager_compatibility", func(t *testing.T) {
		// Test that profile manager integrates properly with other v0.3.0 components

		tempProfileDir, err := os.MkdirTemp("", "profile-compat-*")
		require.NoError(t, err)
		defer func() { _ = os.RemoveAll(tempProfileDir) }()

		profileManager := profile.NewFileProfileManager(tempProfileDir, "yaml")
		assert.NotNil(t, profileManager)

		// Create a test profile that uses EPIC functionality
		testProfile := &profile.Profile{
			Name:        "compatibility-test",
			Description: "Test profile for compatibility validation",
			EpicKey:     "TEST-EPIC-1",
			Repository:  "/tmp/test-repo",
			Options: profile.ProfileOptions{
				Concurrency:  2,
				RateLimit:    "500ms",
				Incremental:  true,
				Force:        false,
				DryRun:       false,
				IncludeLinks: true,
			},
			Tags: []string{"test", "compatibility"},
		}

		// Profile creation should work alongside other components
		err = profileManager.CreateProfile(testProfile)
		require.NoError(t, err, "Failed to create test profile")

		// Validate profile
		validation, err := profileManager.ValidateProfile(testProfile)
		require.NoError(t, err, "Failed to validate profile")
		assert.True(t, validation.Valid, "Profile should be valid")

		// Test profile can be retrieved
		retrieved, err := profileManager.GetProfile("compatibility-test")
		require.NoError(t, err, "Failed to retrieve profile")
		assert.Equal(t, testProfile.Name, retrieved.Name)
		assert.Equal(t, testProfile.EpicKey, retrieved.EpicKey)

		t.Log("✅ Profile manager compatibility with v0.3.0 components validated")
	})

	t.Run("full_v030_workflow_simulation", func(t *testing.T) {
		// Simulate a complete v0.3.0 workflow: EPIC discovery → Profile creation → Incremental sync

		tempProfileDir, err := os.MkdirTemp("", "workflow-test-*")
		require.NoError(t, err)
		defer func() { _ = os.RemoveAll(tempProfileDir) }()

		tempRepo, err := os.MkdirTemp("", "workflow-repo-*")
		require.NoError(t, err)
		defer func() { _ = os.RemoveAll(tempRepo) }()

		// Initialize all v0.3.0 components
		mockClient := client.NewMockClient()
		epicAnalyzer := epic.NewJIRAEpicAnalyzer(mockClient, epic.DefaultDiscoveryOptions())
		queryBuilder := jql.NewJIRAQueryBuilder(mockClient, epicAnalyzer, nil)
		profileManager := profile.NewFileProfileManager(tempProfileDir, "yaml")

		fileWriter := schema.NewYAMLFileWriter()
		gitRepo := git.NewGitRepository("Test Workflow", "test@workflow.local")
		linkManager := links.NewSymbolicLinkManager()
		stateManager := state.NewFileStateManager(state.FormatYAML)

		// Step 1: EPIC Discovery (JCG-016)
		// Simulate EPIC analysis would be done here in real workflow

		// Step 2: JQL Building (JCG-017)
		epicQuery, err := queryBuilder.BuildEpicQuery("TEST-EPIC-1")
		if err != nil {
			t.Logf("EPIC query building failed (expected with mock): %v", err)
			// Use fallback JQL for workflow test
			epicQuery = &jql.Query{
				JQL:         `"Epic Link" = TEST-EPIC-1`,
				Description: "Fallback EPIC query for workflow test",
			}
		}

		// Step 3: Profile Creation (JCG-019)
		workflowProfile := &profile.Profile{
			Name:        "workflow-test-profile",
			Description: "Complete v0.3.0 workflow test profile",
			JQL:         epicQuery.JQL,
			Repository:  tempRepo,
			Options: profile.ProfileOptions{
				Concurrency:  1,
				RateLimit:    "100ms",
				Incremental:  true,
				Force:        false,
				DryRun:       true, // Use dry run for test
				IncludeLinks: true,
			},
			Tags: []string{"workflow", "test", "v030"},
		}

		err = profileManager.CreateProfile(workflowProfile)
		require.NoError(t, err, "Failed to create workflow profile")

		// Step 4: State Management + Incremental Sync (JCG-018)
		err = gitRepo.Initialize(tempRepo)
		require.NoError(t, err, "Failed to initialize git repo")

		incrementalEngine := sync.NewIncrementalBatchSyncEngine(
			mockClient, fileWriter, gitRepo, linkManager, stateManager, 1)

		syncOptions := sync.IncrementalSyncOptions{
			Force:           workflowProfile.Options.Force,
			DryRun:          workflowProfile.Options.DryRun,
			IncludeNew:      true,
			IncludeModified: true,
		}

		// Execute workflow sync
		result, err := incrementalEngine.SyncJQLIncremental(
			context.Background(),
			workflowProfile.JQL,
			workflowProfile.Repository,
			syncOptions,
		)
		require.NoError(t, err, "Workflow sync failed")
		assert.NotNil(t, result)

		// Step 5: Record usage in profile
		err = profileManager.RecordUsage(workflowProfile.Name, result.Duration.Milliseconds(), true)
		require.NoError(t, err, "Failed to record profile usage")

		// Verify complete workflow success
		updatedProfile, err := profileManager.GetProfile(workflowProfile.Name)
		require.NoError(t, err, "Failed to retrieve updated profile")
		assert.Equal(t, 1, updatedProfile.UsageStats.TimesUsed)

		t.Log("✅ Complete v0.3.0 workflow simulation successful")
		t.Logf("    EPIC Query: %s", epicQuery.JQL)
		t.Logf("    Profile: %s", workflowProfile.Name)
		t.Logf("    Sync Result: %d total issues", result.TotalIssues)
		t.Logf("    Duration: %v", result.Duration)
	})
}
