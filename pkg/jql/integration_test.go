package jql

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/chambrid/jira-cdc-git/pkg/client"
	"github.com/chambrid/jira-cdc-git/pkg/epic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJQLQueryBuilderIntegration(t *testing.T) {
	// Test complete workflow
	t.Run("Epic Query Building Workflow", func(t *testing.T) {
		// Create fresh mock dependencies for this test
		mockClient := client.NewMockClient()
		mockEpicAnalyzer := epic.NewMockEpicAnalyzer()
		setupTestData(mockClient, mockEpicAnalyzer)
		builder := NewJIRAQueryBuilder(mockClient, mockEpicAnalyzer, nil)

		testEpicQueryWorkflow(t, builder)
	})

	t.Run("Template Query Building Workflow", func(t *testing.T) {
		// Create fresh mock dependencies for this test
		mockClient := client.NewMockClient()
		mockEpicAnalyzer := epic.NewMockEpicAnalyzer()
		setupTestData(mockClient, mockEpicAnalyzer)
		builder := NewJIRAQueryBuilder(mockClient, mockEpicAnalyzer, nil)

		testTemplateQueryWorkflow(t, builder)
	})

	t.Run("Query Validation Workflow", func(t *testing.T) {
		// Create fresh mock dependencies for this test
		mockClient := client.NewMockClient()
		mockEpicAnalyzer := epic.NewMockEpicAnalyzer()
		setupTestData(mockClient, mockEpicAnalyzer)
		builder := NewJIRAQueryBuilder(mockClient, mockEpicAnalyzer, nil)

		testQueryValidationWorkflow(t, builder)
	})

	t.Run("Query Preview Workflow", func(t *testing.T) {
		// Create fresh mock dependencies for this test
		mockClient := client.NewMockClient()
		mockEpicAnalyzer := epic.NewMockEpicAnalyzer()
		setupTestData(mockClient, mockEpicAnalyzer)
		builder := NewJIRAQueryBuilder(mockClient, mockEpicAnalyzer, nil)

		testQueryPreviewWorkflow(t, builder, mockClient)
	})

	t.Run("Saved Query Management Workflow", func(t *testing.T) {
		// Create fresh mock dependencies for this test
		mockClient := client.NewMockClient()
		mockEpicAnalyzer := epic.NewMockEpicAnalyzer()
		setupTestData(mockClient, mockEpicAnalyzer)

		// Create builder with unique temp directory
		tempDir, err := os.MkdirTemp("", "jql_test_saved_*")
		require.NoError(t, err)
		defer func() { _ = os.RemoveAll(tempDir) }()

		options := DefaultQueryBuilderOptions()
		builder := NewJIRAQueryBuilder(mockClient, mockEpicAnalyzer, options)
		builder.queriesFile = filepath.Join(tempDir, "test_queries.json")
		// Reset saved queries for clean test
		builder.savedQueries = []*SavedQuery{}

		testSavedQueryWorkflow(t, builder)
	})

	t.Run("End-to-End Epic Analysis and Query Generation", func(t *testing.T) {
		// Create fresh mock dependencies for this test
		mockClient := client.NewMockClient()
		mockEpicAnalyzer := epic.NewMockEpicAnalyzer()
		setupTestData(mockClient, mockEpicAnalyzer)

		// Create builder with unique temp directory
		tempDir, err := os.MkdirTemp("", "jql_test_e2e_*")
		require.NoError(t, err)
		defer func() { _ = os.RemoveAll(tempDir) }()

		options := DefaultQueryBuilderOptions()
		builder := NewJIRAQueryBuilder(mockClient, mockEpicAnalyzer, options)
		builder.queriesFile = filepath.Join(tempDir, "test_queries.json")
		// Reset saved queries for clean test
		builder.savedQueries = []*SavedQuery{}

		testEndToEndEpicWorkflow(t, builder, mockClient)
	})
}

func setupTestData(mockClient *client.MockClient, mockEpicAnalyzer *epic.MockEpicAnalyzer) {
	// Set up test issues
	epic1 := client.CreateEpicIssue("PROJ-100")
	story1 := client.CreateTestIssue("PROJ-101")
	story1.IssueType = "Story"
	story1.Relationships = &client.Relationships{
		EpicLink: "PROJ-100",
	}

	story2 := client.CreateTestIssue("PROJ-102")
	story2.IssueType = "Story"
	story2.Relationships = &client.Relationships{
		EpicLink: "PROJ-100",
	}

	subtask1 := client.CreateSubtaskIssue("PROJ-103", "PROJ-101")

	mockClient.AddIssue(epic1)
	mockClient.AddIssue(story1)
	mockClient.AddIssue(story2)
	mockClient.AddIssue(subtask1)

	// Set up JQL search results
	mockClient.AddJQLResult(
		`("Epic Link" = PROJ-100 OR parent in (issuesInEpic("PROJ-100"))) AND project = PROJ ORDER BY key ASC`,
		[]string{"PROJ-101", "PROJ-102", "PROJ-103"},
	)

	mockClient.AddJQLResult(
		`"Epic Link" = PROJ-100 AND type = Story`,
		[]string{"PROJ-101", "PROJ-102"},
	)

	mockClient.AddJQLResult(
		`project = PROJ AND status in ("To Do", "In Progress", "In Review")`,
		[]string{"PROJ-101", "PROJ-102"},
	)

	// Set up epic analysis result
	analysisResult := &epic.AnalysisResult{
		EpicKey:     "PROJ-100",
		EpicSummary: "Test Epic",
		EpicStatus:  "In Progress",
		TotalIssues: 3,
		IssuesByType: map[string][]string{
			"Story":    {"PROJ-101", "PROJ-102"},
			"Sub-task": {"PROJ-103"},
		},
		IssuesByStatus: map[string][]string{
			"In Progress": {"PROJ-101", "PROJ-102", "PROJ-103"},
		},
		RelationshipTypes: map[string]int{
			"epic/story": 2,
			"subtask":    1,
		},
		Hierarchy: &epic.HierarchyMap{
			EpicKey: "PROJ-100",
			Stories: []*epic.HierarchyNode{
				{
					IssueKey:  "PROJ-101",
					Summary:   "Test Issue Summary",
					IssueType: "Story",
					Status:    "In Progress",
					Subtasks: []*epic.HierarchyNode{
						{
							IssueKey:  "PROJ-103",
							Summary:   "Subtask: Test Issue Summary",
							IssueType: "Sub-task",
							Status:    "In Progress",
							ParentKey: "PROJ-101",
						},
					},
				},
				{
					IssueKey:  "PROJ-102",
					Summary:   "Test Issue Summary",
					IssueType: "Story",
					Status:    "In Progress",
				},
			},
			Levels: 2,
		},
		Performance: &epic.PerformanceMetrics{
			DiscoveryTimeMs:    50,
			AnalysisTimeMs:     30,
			TotalAPICallsCount: 3,
			CacheHitCount:      0,
			CacheMissCount:     3,
		},
		Completeness: &epic.CompletenessReport{
			TotalExpectedIssues: 3,
			TotalFoundIssues:    3,
			CompletenessPercent: 100.0,
			MissingIssues:       []string{},
			OrphanedIssues:      []string{},
			BrokenLinks:         []string{},
		},
	}

	mockEpicAnalyzer.SetMockAnalysis("PROJ-100", analysisResult)
}

func testEpicQueryWorkflow(t *testing.T, builder *JIRAQueryBuilder) {
	// Build epic query
	epicQuery, err := builder.BuildEpicQuery("PROJ-100")
	require.NoError(t, err)
	assert.NotNil(t, epicQuery)

	// Verify query structure
	assert.Contains(t, epicQuery.JQL, `"Epic Link" = PROJ-100`)
	assert.Contains(t, epicQuery.JQL, `issuesInEpic("PROJ-100")`)
	assert.Contains(t, epicQuery.JQL, `project = PROJ`)
	assert.Contains(t, epicQuery.JQL, `ORDER BY key ASC`)

	// Verify metadata
	assert.Equal(t, "epic-all-issues", epicQuery.Template)
	assert.Equal(t, "PROJ-100", epicQuery.Parameters["epic_key"])
	assert.Equal(t, 3, epicQuery.EstimatedCount)
	assert.True(t, epicQuery.Optimized)

	// Validate the generated query
	validation, err := builder.ValidateQuery(epicQuery.JQL)
	require.NoError(t, err)
	assert.True(t, validation.Valid)
	assert.Empty(t, validation.Errors)

	// Test error case with invalid epic key (no hyphen)
	_, err = builder.BuildEpicQuery("invalidkey")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid epic key")
}

func testTemplateQueryWorkflow(t *testing.T, builder *JIRAQueryBuilder) {
	// Test epic stories template
	params := map[string]string{
		"epic_key": "PROJ-100",
	}

	query, err := builder.BuildFromTemplate("epic-stories-only", params)
	require.NoError(t, err)
	assert.Contains(t, query.JQL, `"Epic Link" = PROJ-100`)
	assert.Contains(t, query.JQL, `type = Story`)

	// Test project active issues template
	params = map[string]string{
		"project_key": "PROJ",
	}

	query, err = builder.BuildFromTemplate("project-active-issues", params)
	require.NoError(t, err)
	assert.Contains(t, query.JQL, `project = PROJ`)
	assert.Contains(t, query.JQL, `status in ("To Do", "In Progress", "In Review")`)

	// Test recent updates template with default parameter
	query, err = builder.BuildFromTemplate("recent-updates", params)
	require.NoError(t, err)
	assert.Contains(t, query.JQL, `project = PROJ`)
	assert.Contains(t, query.JQL, `-7d`) // Default value

	// Test template not found
	_, err = builder.BuildFromTemplate("non-existent", params)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "template 'non-existent' not found")

	// Test missing required parameter
	_, err = builder.BuildFromTemplate("epic-all-issues", map[string]string{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "required parameter 'epic_key' missing")
}

func testQueryValidationWorkflow(t *testing.T, builder *JIRAQueryBuilder) {
	testCases := []struct {
		name     string
		jql      string
		valid    bool
		hasError bool
	}{
		{
			name:  "valid simple query",
			jql:   `project = PROJ AND status = "To Do"`,
			valid: true,
		},
		{
			name:  "valid complex query",
			jql:   `project = PROJ AND status IN ("To Do", "In Progress") AND assignee = currentUser() ORDER BY key ASC`,
			valid: true,
		},
		{
			name:     "invalid unbalanced quotes",
			jql:      `project = PROJ AND status = "To Do`,
			valid:    false,
			hasError: true,
		},
		{
			name:     "invalid unbalanced parentheses",
			jql:      `project = PROJ AND (status = "To Do"`,
			valid:    false,
			hasError: true,
		},
		{
			name:     "invalid duplicate operators",
			jql:      `project = PROJ AND AND status = "To Do"`,
			valid:    false,
			hasError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := builder.ValidateQuery(tc.jql)
			require.NoError(t, err)

			assert.Equal(t, tc.valid, result.Valid)
			assert.Equal(t, tc.jql, result.JQL)

			if tc.hasError {
				assert.NotEmpty(t, result.Errors)
			} else {
				assert.Empty(t, result.Errors)
			}

			// Optimization suggestions
			optimized, err := builder.OptimizeQuery(tc.jql)
			require.NoError(t, err)
			assert.True(t, optimized.Optimized)
		})
	}
}

func testQueryPreviewWorkflow(t *testing.T, builder *JIRAQueryBuilder, mockClient *client.MockClient) {
	// Preview epic query
	jql := `("Epic Link" = PROJ-100 OR parent in (issuesInEpic("PROJ-100"))) AND project = PROJ ORDER BY key ASC`

	preview, err := builder.PreviewQuery(jql)
	require.NoError(t, err)
	assert.NotNil(t, preview)

	// Verify preview results
	assert.Equal(t, jql, preview.Query)
	assert.Equal(t, 3, preview.TotalCount)
	assert.NotEmpty(t, preview.SampleIssues)
	assert.Equal(t, 3, preview.ProjectBreakdown["PROJ"])
	assert.NotEmpty(t, preview.StatusBreakdown)
	assert.NotEmpty(t, preview.TypeBreakdown)
	assert.GreaterOrEqual(t, preview.ExecutionTimeMs, int64(0))

	// Verify API was called with pagination
	assert.Greater(t, mockClient.SearchIssuesWithPaginationCallCount, 0)

	// Test preview error case
	mockClient.SetAPIError(&client.ClientError{
		Type:    "api_error",
		Message: "JIRA API error",
	})

	_, err = builder.PreviewQuery("project = ERROR")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to preview query")

	// Reset error for other tests
	mockClient.SetAPIError(nil)
}

func testSavedQueryWorkflow(t *testing.T, builder *JIRAQueryBuilder) {
	// Save a query
	err := builder.SaveQuery("my-epic-query", "My favorite epic query", `"Epic Link" = PROJ-100`)
	require.NoError(t, err)

	// Get saved queries
	queries, err := builder.GetSavedQueries()
	require.NoError(t, err)
	assert.Len(t, queries, 1)
	assert.Equal(t, "my-epic-query", queries[0].Name)
	assert.Equal(t, "My favorite epic query", queries[0].Description)
	assert.Equal(t, `"Epic Link" = PROJ-100`, queries[0].JQL)
	assert.Equal(t, 0, queries[0].UsageCount)

	// Update query usage
	err = builder.UpdateQueryUsage("my-epic-query")
	require.NoError(t, err)

	// Verify usage updated
	queries, err = builder.GetSavedQueries()
	require.NoError(t, err)
	assert.Equal(t, 1, queries[0].UsageCount)
	assert.False(t, queries[0].LastUsed.IsZero())

	// Save another query with same name (should update)
	err = builder.SaveQuery("my-epic-query", "Updated description", `"Epic Link" = PROJ-200`)
	require.NoError(t, err)

	queries, err = builder.GetSavedQueries()
	require.NoError(t, err)
	assert.Len(t, queries, 1) // Still only one query
	assert.Equal(t, "Updated description", queries[0].Description)
	assert.Equal(t, `"Epic Link" = PROJ-200`, queries[0].JQL)
	assert.Equal(t, 1, queries[0].UsageCount) // Usage count preserved

	// Test updating non-existent query
	err = builder.UpdateQueryUsage("non-existent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "saved query 'non-existent' not found")
}

func testEndToEndEpicWorkflow(t *testing.T, builder *JIRAQueryBuilder, mockClient *client.MockClient) {
	// This test simulates a complete user workflow:
	// 1. User wants to sync an EPIC
	// 2. System builds the query
	// 3. User previews what will be synced
	// 4. User validates the query
	// 5. User saves the query for later use
	// 6. System optimizes the query for execution

	// Step 1: Build EPIC query
	epicQuery, err := builder.BuildEpicQuery("PROJ-100")
	require.NoError(t, err)
	assert.Contains(t, epicQuery.JQL, `"Epic Link" = PROJ-100`)

	// Step 2: Preview the query to see what issues would be synced
	preview, err := builder.PreviewQuery(epicQuery.JQL)
	require.NoError(t, err)
	assert.Equal(t, 3, preview.TotalCount)
	assert.NotEmpty(t, preview.SampleIssues)

	// Step 3: Validate the query to ensure it's syntactically correct
	validation, err := builder.ValidateQuery(epicQuery.JQL)
	require.NoError(t, err)
	assert.True(t, validation.Valid)
	assert.Empty(t, validation.Errors)

	// Step 4: Save the query for future use
	err = builder.SaveQuery("epic-proj-100", "PROJ-100 Epic with all stories and subtasks", epicQuery.JQL)
	require.NoError(t, err)

	// Step 5: Optimize the query for better performance
	optimized, err := builder.OptimizeQuery(epicQuery.JQL)
	require.NoError(t, err)
	assert.True(t, optimized.Optimized)
	assert.Contains(t, optimized.JQL, "ORDER BY key ASC")

	// Step 6: Verify we can retrieve and use the saved query
	savedQueries, err := builder.GetSavedQueries()
	require.NoError(t, err)
	assert.Len(t, savedQueries, 1)

	savedQuery := savedQueries[0]
	assert.Equal(t, "epic-proj-100", savedQuery.Name)

	// Step 7: Use the saved query for preview (simulating repeated use)
	err = builder.UpdateQueryUsage(savedQuery.Name)
	require.NoError(t, err)

	preview2, err := builder.PreviewQuery(savedQuery.JQL)
	require.NoError(t, err)
	assert.Equal(t, preview.TotalCount, preview2.TotalCount)

	// Verify usage was tracked
	updatedQueries, err := builder.GetSavedQueries()
	require.NoError(t, err)
	assert.Equal(t, 1, updatedQueries[0].UsageCount)
}

func TestJQLQueryBuilderErrorHandling(t *testing.T) {
	mockClient := client.NewMockClient()
	mockEpicAnalyzer := epic.NewMockEpicAnalyzer()
	builder := NewJIRAQueryBuilder(mockClient, mockEpicAnalyzer, nil)

	t.Run("Epic Analysis Error", func(t *testing.T) {
		mockEpicAnalyzer.AnalyzeEpicFunc = func(epicKey string) (*epic.AnalysisResult, error) {
			if epicKey == "PROJ-ERROR" {
				return nil, epic.NewEpicError(epic.ErrorTypeNotFound, "epic not found", epicKey, nil)
			}
			return nil, nil
		}

		_, err := builder.BuildEpicQuery("PROJ-ERROR")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to analyze epic")
	})

	t.Run("Client Error During Preview", func(t *testing.T) {
		mockClient.SetAPIError(&client.ClientError{
			Type:    "authentication_error",
			Message: "authentication failed",
		})

		_, err := builder.PreviewQuery("project = PROJ")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to preview query")

		// Reset error
		mockClient.SetAPIError(nil)
	})

	t.Run("Invalid Epic Key Format", func(t *testing.T) {
		_, err := builder.BuildEpicQuery("invalid")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid epic key")
	})
}

func TestJQLQueryBuilderTemplateSystem(t *testing.T) {
	builder := NewJIRAQueryBuilder(client.NewMockClient(), epic.NewMockEpicAnalyzer(), nil)

	templates := builder.GetTemplates()
	assert.NotEmpty(t, templates)

	// Verify all built-in templates are available and functional
	templateTests := []struct {
		name       string
		params     map[string]string
		shouldWork bool
	}{
		{
			name:       "epic-all-issues",
			params:     map[string]string{"epic_key": "PROJ-123"},
			shouldWork: true,
		},
		{
			name:       "epic-stories-only",
			params:     map[string]string{"epic_key": "PROJ-123"},
			shouldWork: true,
		},
		{
			name:       "project-active-issues",
			params:     map[string]string{"project_key": "PROJ"},
			shouldWork: true,
		},
		{
			name:       "assignee-current-sprint",
			params:     map[string]string{},
			shouldWork: true,
		},
		{
			name:       "recent-updates",
			params:     map[string]string{"project_key": "PROJ"},
			shouldWork: true,
		},
		{
			name:       "epic-all-issues",
			params:     map[string]string{}, // Missing required parameter
			shouldWork: false,
		},
	}

	for _, tc := range templateTests {
		t.Run(tc.name, func(t *testing.T) {
			query, err := builder.BuildFromTemplate(tc.name, tc.params)

			if tc.shouldWork {
				require.NoError(t, err)
				assert.NotNil(t, query)
				assert.NotEmpty(t, query.JQL)
				assert.Equal(t, tc.name, query.Template)

				// Validate the generated query
				validation, err := builder.ValidateQuery(query.JQL)
				require.NoError(t, err)
				assert.True(t, validation.Valid)
			} else {
				assert.Error(t, err)
			}
		})
	}
}
