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

func TestNewJIRAQueryBuilder(t *testing.T) {
	mockClient := client.NewMockClient()
	mockEpicAnalyzer := epic.NewMockEpicAnalyzer()

	builder := NewJIRAQueryBuilder(mockClient, mockEpicAnalyzer, nil)

	assert.NotNil(t, builder)
	assert.Equal(t, mockClient, builder.client)
	assert.Equal(t, mockEpicAnalyzer, builder.epicAnalyzer)
	assert.NotNil(t, builder.options)
	assert.Equal(t, "customfield_12311140", builder.options.EpicCustomField)
}

func TestNewJIRAQueryBuilderWithCustomOptions(t *testing.T) {
	mockClient := client.NewMockClient()
	mockEpicAnalyzer := epic.NewMockEpicAnalyzer()

	options := &QueryBuilderOptions{
		EpicCustomField:   "customfield_99999",
		UseCache:          false,
		MaxPreviewResults: 5,
		OptimizeByDefault: false,
	}

	builder := NewJIRAQueryBuilder(mockClient, mockEpicAnalyzer, options)

	assert.Equal(t, options, builder.options)
	assert.Equal(t, "customfield_99999", builder.options.EpicCustomField)
	assert.False(t, builder.options.UseCache)
	assert.Equal(t, 5, builder.options.MaxPreviewResults)
	assert.False(t, builder.options.OptimizeByDefault)
}

func TestBuildEpicQuery(t *testing.T) {
	mockClient := client.NewMockClient()
	mockEpicAnalyzer := epic.NewMockEpicAnalyzer()
	builder := NewJIRAQueryBuilder(mockClient, mockEpicAnalyzer, nil)

	// Set up mock epic analysis result
	analysisResult := &epic.AnalysisResult{
		EpicKey:     "PROJ-123",
		TotalIssues: 15,
	}
	mockEpicAnalyzer.SetMockAnalysis("PROJ-123", analysisResult)

	query, err := builder.BuildEpicQuery("PROJ-123")

	require.NoError(t, err)
	assert.NotNil(t, query)
	assert.Contains(t, query.JQL, `"Epic Link" = PROJ-123`)
	assert.Contains(t, query.JQL, `issuesInEpic("PROJ-123")`)
	assert.Contains(t, query.JQL, `project = PROJ`)
	assert.Contains(t, query.JQL, `ORDER BY key ASC`)
	assert.Equal(t, "epic-all-issues", query.Template)
	assert.Equal(t, "PROJ-123", query.Parameters["epic_key"])
	assert.Equal(t, 15, query.EstimatedCount)
	assert.True(t, query.Optimized)
}

func TestBuildEpicQueryInvalidKey(t *testing.T) {
	mockClient := client.NewMockClient()
	mockEpicAnalyzer := epic.NewMockEpicAnalyzer()
	builder := NewJIRAQueryBuilder(mockClient, mockEpicAnalyzer, nil)

	_, err := builder.BuildEpicQuery("invalidkey")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid epic key")
}

func TestBuildEpicQueryAnalysisError(t *testing.T) {
	mockClient := client.NewMockClient()
	mockEpicAnalyzer := epic.NewMockEpicAnalyzer()
	builder := NewJIRAQueryBuilder(mockClient, mockEpicAnalyzer, nil)

	// Set up mock to return error using function
	mockEpicAnalyzer.AnalyzeEpicFunc = func(epicKey string) (*epic.AnalysisResult, error) {
		return nil, epic.NewEpicError(epic.ErrorTypeNotFound, "epic not found", epicKey, nil)
	}

	_, err := builder.BuildEpicQuery("PROJ-123")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to analyze epic")
}

func TestBuildFromTemplate(t *testing.T) {
	mockClient := client.NewMockClient()
	mockEpicAnalyzer := epic.NewMockEpicAnalyzer()
	builder := NewJIRAQueryBuilder(mockClient, mockEpicAnalyzer, nil)

	params := map[string]string{
		"epic_key": "PROJ-123",
	}

	query, err := builder.BuildFromTemplate("epic-all-issues", params)

	require.NoError(t, err)
	assert.NotNil(t, query)
	assert.Contains(t, query.JQL, "PROJ-123")
	assert.Equal(t, "epic-all-issues", query.Template)
	assert.Equal(t, "PROJ-123", query.Parameters["epic_key"])
}

func TestBuildFromTemplateWithDefaults(t *testing.T) {
	mockClient := client.NewMockClient()
	mockEpicAnalyzer := epic.NewMockEpicAnalyzer()
	builder := NewJIRAQueryBuilder(mockClient, mockEpicAnalyzer, nil)

	params := map[string]string{
		"project_key": "PROJ",
	}

	query, err := builder.BuildFromTemplate("recent-updates", params)

	require.NoError(t, err)
	assert.Contains(t, query.JQL, "project = PROJ")
	assert.Contains(t, query.JQL, "-7d") // Default value for days parameter
}

func TestBuildFromTemplateNotFound(t *testing.T) {
	mockClient := client.NewMockClient()
	mockEpicAnalyzer := epic.NewMockEpicAnalyzer()
	builder := NewJIRAQueryBuilder(mockClient, mockEpicAnalyzer, nil)

	_, err := builder.BuildFromTemplate("non-existent-template", map[string]string{})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "template 'non-existent-template' not found")
}

func TestBuildFromTemplateMissingRequiredParam(t *testing.T) {
	mockClient := client.NewMockClient()
	mockEpicAnalyzer := epic.NewMockEpicAnalyzer()
	builder := NewJIRAQueryBuilder(mockClient, mockEpicAnalyzer, nil)

	_, err := builder.BuildFromTemplate("epic-all-issues", map[string]string{})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "required parameter 'epic_key' missing")
}

func TestValidateQuery(t *testing.T) {
	mockClient := client.NewMockClient()
	mockEpicAnalyzer := epic.NewMockEpicAnalyzer()
	builder := NewJIRAQueryBuilder(mockClient, mockEpicAnalyzer, nil)

	tests := []struct {
		name     string
		jql      string
		valid    bool
		hasError bool
	}{
		{
			name:  "valid JQL",
			jql:   `project = PROJ AND status = "To Do"`,
			valid: true,
		},
		{
			name:     "invalid JQL - unbalanced quotes",
			jql:      `project = PROJ AND status = "To Do`,
			valid:    false,
			hasError: true,
		},
		{
			name:  "complex valid JQL",
			jql:   `project = PROJ AND status IN ("To Do", "In Progress") AND assignee = currentUser()`,
			valid: true,
		},
		{
			name:     "invalid JQL - duplicate operators",
			jql:      `project = PROJ AND AND status = "To Do"`,
			valid:    false,
			hasError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := builder.ValidateQuery(tt.jql)

			require.NoError(t, err)
			assert.NotNil(t, result)
			assert.Equal(t, tt.valid, result.Valid)
			assert.Equal(t, tt.jql, result.JQL)

			if tt.hasError {
				assert.NotEmpty(t, result.Errors)
			} else {
				assert.Empty(t, result.Errors)
			}
		})
	}
}

func TestOptimizeQuery(t *testing.T) {
	mockClient := client.NewMockClient()
	mockEpicAnalyzer := epic.NewMockEpicAnalyzer()
	builder := NewJIRAQueryBuilder(mockClient, mockEpicAnalyzer, nil)

	tests := []struct {
		name            string
		inputJQL        string
		expectedChanges []string
	}{
		{
			name:            "add ORDER BY",
			inputJQL:        `project = PROJ AND status = "To Do"`,
			expectedChanges: []string{"ORDER BY key ASC"},
		},
		{
			name:            "move project filter to front",
			inputJQL:        `status = "To Do" AND project = PROJ`,
			expectedChanges: []string{"project = PROJ", "ORDER BY key ASC"},
		},
		{
			name:            "already optimized",
			inputJQL:        `project = PROJ AND status = "To Do" ORDER BY key ASC`,
			expectedChanges: []string{"project = PROJ", "ORDER BY key ASC"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query, err := builder.OptimizeQuery(tt.inputJQL)

			require.NoError(t, err)
			assert.NotNil(t, query)
			assert.True(t, query.Optimized)

			for _, expectedChange := range tt.expectedChanges {
				assert.Contains(t, query.JQL, expectedChange)
			}
		})
	}
}

func TestPreviewQuery(t *testing.T) {
	// Create temporary directory for test
	tempDir, err := os.MkdirTemp("", "jql_test_*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	mockClient := client.NewMockClient()
	mockEpicAnalyzer := epic.NewMockEpicAnalyzer()
	builder := NewJIRAQueryBuilder(mockClient, mockEpicAnalyzer, nil)
	builder.queriesFile = filepath.Join(tempDir, "test_queries.json")

	// Set up mock issues
	issue1 := client.CreateTestIssue("PROJ-1")
	issue1.IssueType = "Story"
	issue1.Status.Name = "To Do"

	issue2 := client.CreateTestIssue("PROJ-2")
	issue2.IssueType = "Bug"
	issue2.Status.Name = "In Progress"

	mockClient.AddIssue(issue1)
	mockClient.AddIssue(issue2)
	mockClient.AddJQLResult("project = PROJ", []string{"PROJ-1", "PROJ-2"})

	preview, err := builder.PreviewQuery("project = PROJ")

	require.NoError(t, err)
	assert.NotNil(t, preview)
	assert.Equal(t, "project = PROJ", preview.Query)
	assert.Equal(t, 2, preview.TotalCount)
	assert.Equal(t, 2, len(preview.SampleIssues))
	assert.Equal(t, 2, preview.ProjectBreakdown["PROJ"])
	assert.Equal(t, 1, preview.StatusBreakdown["To Do"])
	assert.Equal(t, 1, preview.StatusBreakdown["In Progress"])
	assert.Equal(t, 1, preview.TypeBreakdown["Story"])
	assert.Equal(t, 1, preview.TypeBreakdown["Bug"])
	assert.GreaterOrEqual(t, preview.ExecutionTimeMs, int64(0))
}

func TestPreviewQueryError(t *testing.T) {
	// Create temporary directory for test
	tempDir, err := os.MkdirTemp("", "jql_test_*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	mockClient := client.NewMockClient()
	mockEpicAnalyzer := epic.NewMockEpicAnalyzer()
	builder := NewJIRAQueryBuilder(mockClient, mockEpicAnalyzer, nil)
	builder.queriesFile = filepath.Join(tempDir, "test_queries.json")

	// Set up mock to return error
	mockClient.SetAPIError(&client.ClientError{
		Type:    "api_error",
		Message: "JIRA API error",
	})

	_, err = builder.PreviewQuery("project = PROJ")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to preview query")
}

func TestGetTemplates(t *testing.T) {
	mockClient := client.NewMockClient()
	mockEpicAnalyzer := epic.NewMockEpicAnalyzer()
	builder := NewJIRAQueryBuilder(mockClient, mockEpicAnalyzer, nil)

	templates := builder.GetTemplates()

	assert.NotEmpty(t, templates)
	assert.GreaterOrEqual(t, len(templates), 5)

	// Check for specific templates
	templateNames := make(map[string]bool)
	for _, template := range templates {
		templateNames[template.Name] = true
	}

	assert.True(t, templateNames["epic-all-issues"])
	assert.True(t, templateNames["project-active-issues"])
	assert.True(t, templateNames["assignee-current-sprint"])
}

func TestSaveAndGetSavedQueries(t *testing.T) {
	// Create temporary directory for test
	tempDir, err := os.MkdirTemp("", "jql_test_*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	mockClient := client.NewMockClient()
	mockEpicAnalyzer := epic.NewMockEpicAnalyzer()
	builder := NewJIRAQueryBuilder(mockClient, mockEpicAnalyzer, nil)
	builder.queriesFile = filepath.Join(tempDir, "test_queries.json")
	// Reset saved queries for clean test
	builder.savedQueries = []*SavedQuery{}

	// Save a query
	err = builder.SaveQuery("my-query", "My favorite query", "project = PROJ AND assignee = currentUser()")
	require.NoError(t, err)

	// Get saved queries
	queries, err := builder.GetSavedQueries()
	require.NoError(t, err)
	assert.Len(t, queries, 1)
	assert.Equal(t, "my-query", queries[0].Name)
	assert.Equal(t, "My favorite query", queries[0].Description)
	assert.Equal(t, "project = PROJ AND assignee = currentUser()", queries[0].JQL)
}

func TestSaveQueryUpdate(t *testing.T) {
	// Create temporary directory for test
	tempDir, err := os.MkdirTemp("", "jql_test_*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	mockClient := client.NewMockClient()
	mockEpicAnalyzer := epic.NewMockEpicAnalyzer()
	builder := NewJIRAQueryBuilder(mockClient, mockEpicAnalyzer, nil)
	builder.queriesFile = filepath.Join(tempDir, "test_queries.json")
	// Reset saved queries for clean test
	builder.savedQueries = []*SavedQuery{}

	// Save initial query
	err = builder.SaveQuery("my-query", "Original description", "project = PROJ")
	require.NoError(t, err)

	// Update the same query
	err = builder.SaveQuery("my-query", "Updated description", "project = PROJ AND status = \"To Do\"")
	require.NoError(t, err)

	// Verify only one query exists with updated values
	queries, err := builder.GetSavedQueries()
	require.NoError(t, err)
	assert.Len(t, queries, 1)
	assert.Equal(t, "Updated description", queries[0].Description)
	assert.Equal(t, "project = PROJ AND status = \"To Do\"", queries[0].JQL)
}

func TestUpdateQueryUsage(t *testing.T) {
	// Create temporary directory for test
	tempDir, err := os.MkdirTemp("", "jql_test_*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	mockClient := client.NewMockClient()
	mockEpicAnalyzer := epic.NewMockEpicAnalyzer()
	builder := NewJIRAQueryBuilder(mockClient, mockEpicAnalyzer, nil)
	builder.queriesFile = filepath.Join(tempDir, "test_queries.json")
	// Reset saved queries for clean test
	builder.savedQueries = []*SavedQuery{}

	// Save a query
	err = builder.SaveQuery("my-query", "Test query", "project = PROJ")
	require.NoError(t, err)

	// Update usage
	err = builder.UpdateQueryUsage("my-query")
	require.NoError(t, err)

	// Verify usage count incremented
	queries, err := builder.GetSavedQueries()
	require.NoError(t, err)
	assert.Len(t, queries, 1)
	assert.Equal(t, 1, queries[0].UsageCount)
	assert.False(t, queries[0].LastUsed.IsZero())

	// Update usage again
	err = builder.UpdateQueryUsage("my-query")
	require.NoError(t, err)

	queries, err = builder.GetSavedQueries()
	require.NoError(t, err)
	assert.Equal(t, 2, queries[0].UsageCount)
}

func TestUpdateQueryUsageNotFound(t *testing.T) {
	// Create temporary directory for test
	tempDir, err := os.MkdirTemp("", "jql_test_*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	mockClient := client.NewMockClient()
	mockEpicAnalyzer := epic.NewMockEpicAnalyzer()
	builder := NewJIRAQueryBuilder(mockClient, mockEpicAnalyzer, nil)
	builder.queriesFile = filepath.Join(tempDir, "test_queries.json")

	err = builder.UpdateQueryUsage("non-existent-query")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "saved query 'non-existent-query' not found")
}

func TestLoadSavedQueriesFileDoesNotExist(t *testing.T) {
	// Create temporary directory for test
	tempDir, err := os.MkdirTemp("", "jql_test_*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	mockClient := client.NewMockClient()
	mockEpicAnalyzer := epic.NewMockEpicAnalyzer()
	builder := NewJIRAQueryBuilder(mockClient, mockEpicAnalyzer, nil)
	builder.queriesFile = filepath.Join(tempDir, "non_existent.json")

	err = builder.loadSavedQueries()

	// Should not error, should just create empty list
	require.NoError(t, err)
	assert.Empty(t, builder.savedQueries)
}

func TestQueryBuilderIntegration(t *testing.T) {
	// Create temporary directory for test
	tempDir, err := os.MkdirTemp("", "jql_test_*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	mockClient := client.NewMockClient()
	mockEpicAnalyzer := epic.NewMockEpicAnalyzer()
	builder := NewJIRAQueryBuilder(mockClient, mockEpicAnalyzer, nil)
	builder.queriesFile = filepath.Join(tempDir, "test_queries.json")

	// Set up mock data
	analysisResult := &epic.AnalysisResult{
		EpicKey:     "PROJ-123",
		TotalIssues: 10,
	}
	mockEpicAnalyzer.SetMockAnalysis("PROJ-123", analysisResult)

	// Build epic query
	epicQuery, err := builder.BuildEpicQuery("PROJ-123")
	require.NoError(t, err)

	// Validate the query
	validation, err := builder.ValidateQuery(epicQuery.JQL)
	require.NoError(t, err)
	assert.True(t, validation.Valid)

	// Optimize the query
	optimized, err := builder.OptimizeQuery(epicQuery.JQL)
	require.NoError(t, err)
	assert.True(t, optimized.Optimized)

	// Build from template
	templateQuery, err := builder.BuildFromTemplate("epic-stories-only", map[string]string{
		"epic_key": "PROJ-123",
	})
	require.NoError(t, err)
	assert.Contains(t, templateQuery.JQL, "type = Story")

	// Get templates
	templates := builder.GetTemplates()
	assert.NotEmpty(t, templates)
}
