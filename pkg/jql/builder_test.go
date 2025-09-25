package jql

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseEpicKey(t *testing.T) {
	tests := []struct {
		name         string
		epicKey      string
		expectedProj string
		expectedNum  string
		expectError  bool
	}{
		{
			name:         "valid simple epic key",
			epicKey:      "PROJ-123",
			expectedProj: "PROJ",
			expectedNum:  "123",
			expectError:  false,
		},
		{
			name:         "valid complex epic key",
			epicKey:      "RHOAIENG-456",
			expectedProj: "RHOAIENG",
			expectedNum:  "456",
			expectError:  false,
		},
		{
			name:         "valid hyphenated project",
			epicKey:      "MY-PROJECT-789",
			expectedProj: "MY-PROJECT",
			expectedNum:  "789",
			expectError:  false,
		},
		{
			name:        "invalid no hyphen",
			epicKey:     "PROJ123",
			expectError: true,
		},
		{
			name:        "invalid empty key",
			epicKey:     "",
			expectError: true,
		},
		{
			name:        "invalid only hyphen",
			epicKey:     "-",
			expectError: true,
		},
		{
			name:        "invalid no number",
			epicKey:     "PROJ-",
			expectError: true,
		},
		{
			name:        "invalid no project",
			epicKey:     "-123",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			project, number, err := parseEpicKey(tt.epicKey)

			if tt.expectError {
				assert.Error(t, err)
				assert.Empty(t, project)
				assert.Empty(t, number)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedProj, project)
				assert.Equal(t, tt.expectedNum, number)
			}
		})
	}
}

func TestValidateJQLSyntax(t *testing.T) {
	tests := []struct {
		name           string
		jql            string
		expectedErrors []string
	}{
		{
			name:           "valid simple JQL",
			jql:            `project = PROJ AND status = "To Do"`,
			expectedErrors: []string{},
		},
		{
			name:           "unbalanced quotes",
			jql:            `project = PROJ AND status = "To Do`,
			expectedErrors: []string{"unbalanced quotes in JQL"},
		},
		{
			name:           "unbalanced parentheses - missing close",
			jql:            `project = PROJ AND (status = "To Do" OR status = "In Progress"`,
			expectedErrors: []string{"unbalanced parentheses in JQL"},
		},
		{
			name:           "unbalanced parentheses - missing open",
			jql:            `project = PROJ AND status = "To Do")`,
			expectedErrors: []string{"unbalanced parentheses in JQL"},
		},
		{
			name:           "duplicate AND operators",
			jql:            `project = PROJ AND AND status = "To Do"`,
			expectedErrors: []string{"duplicate logical operators detected"},
		},
		{
			name:           "duplicate OR operators",
			jql:            `project = PROJ OR OR status = "To Do"`,
			expectedErrors: []string{"duplicate logical operators detected"},
		},
		{
			name:           "multiple errors",
			jql:            `project = PROJ AND AND status = "To Do`,
			expectedErrors: []string{"unbalanced quotes in JQL", "duplicate logical operators detected"},
		},
		{
			name:           "valid complex JQL",
			jql:            `project = PROJ AND status IN ("To Do", "In Progress") AND assignee = currentUser() ORDER BY key ASC`,
			expectedErrors: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := validateJQLSyntax(tt.jql)

			assert.Equal(t, len(tt.expectedErrors), len(errors))
			for _, expectedError := range tt.expectedErrors {
				assert.Contains(t, errors, expectedError)
			}
		})
	}
}

func TestGenerateJQLSuggestions(t *testing.T) {
	tests := []struct {
		name                string
		jql                 string
		expectedSuggestions []string
	}{
		{
			name:                "wildcard without order by",
			jql:                 `project = PROJ AND summary ~ "*test*"`,
			expectedSuggestions: []string{"consider adding ORDER BY clause when using wildcards"},
		},
		{
			name:                "many OR conditions",
			jql:                 `project = PROJ OR project = TEAM OR project = DEV OR project = QA OR project = DOCS`,
			expectedSuggestions: []string{"consider using IN operator instead of multiple OR conditions"},
		},
		{
			name:                "hardcoded assignee",
			jql:                 `project = PROJ AND assignee = "john.doe"`,
			expectedSuggestions: []string{"consider using currentUser() function for dynamic assignee queries"},
		},
		{
			name: "multiple suggestions",
			jql:  `project = PROJ AND summary ~ "*test*" AND (assignee = "john.doe" OR assignee = "jane.smith" OR assignee = "bob.jones" OR assignee = "alice.brown" OR assignee = "charlie.wilson")`,
			expectedSuggestions: []string{
				"consider adding ORDER BY clause when using wildcards",
				"consider using IN operator instead of multiple OR conditions",
				"consider using currentUser() function for dynamic assignee queries",
			},
		},
		{
			name:                "optimal JQL",
			jql:                 `project = PROJ AND status IN ("To Do", "In Progress") AND assignee = currentUser() ORDER BY key ASC`,
			expectedSuggestions: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			suggestions := generateJQLSuggestions(tt.jql)

			assert.Equal(t, len(tt.expectedSuggestions), len(suggestions))
			for _, expectedSuggestion := range tt.expectedSuggestions {
				assert.Contains(t, suggestions, expectedSuggestion)
			}
		})
	}
}

func TestGetBuiltInTemplates(t *testing.T) {
	templates := GetBuiltInTemplates()

	// Should have at least 5 built-in templates
	assert.GreaterOrEqual(t, len(templates), 5)

	// Check for specific templates
	templateNames := make(map[string]*Template)
	for _, template := range templates {
		templateNames[template.Name] = template
	}

	// Verify epic-all-issues template
	epicTemplate, exists := templateNames["epic-all-issues"]
	require.True(t, exists, "epic-all-issues template should exist")
	assert.Equal(t, "epic", epicTemplate.Category)
	assert.Contains(t, epicTemplate.JQLTemplate, "{{.epic_key}}")
	assert.True(t, len(epicTemplate.Parameters) > 0)
	assert.True(t, epicTemplate.Parameters[0].Required)

	// Verify project-active-issues template
	projectTemplate, exists := templateNames["project-active-issues"]
	require.True(t, exists, "project-active-issues template should exist")
	assert.Equal(t, "project", projectTemplate.Category)
	assert.Contains(t, projectTemplate.JQLTemplate, "{{.project_key}}")

	// Verify assignee-current-sprint template
	assigneeTemplate, exists := templateNames["assignee-current-sprint"]
	require.True(t, exists, "assignee-current-sprint template should exist")
	assert.Equal(t, "assignee", assigneeTemplate.Category)
	assert.Contains(t, assigneeTemplate.JQLTemplate, "currentUser()")

	// All templates should have examples
	for _, template := range templates {
		assert.True(t, len(template.Examples) > 0, "template %s should have examples", template.Name)
		for _, example := range template.Examples {
			assert.NotEmpty(t, example.Description)
			assert.NotEmpty(t, example.ResultingJQL)
		}
	}
}

func TestDefaultQueryBuilderOptions(t *testing.T) {
	options := DefaultQueryBuilderOptions()

	assert.NotNil(t, options)
	assert.Equal(t, "customfield_12311140", options.EpicCustomField)
	assert.True(t, options.UseCache)
	assert.Equal(t, 10, options.MaxPreviewResults)
	assert.True(t, options.OptimizeByDefault)
}

func TestValidationResult(t *testing.T) {
	// Test valid result
	validResult := &ValidationResult{
		Valid:       true,
		Errors:      []string{},
		Warnings:    []string{},
		Suggestions: []string{},
		JQL:         "project = PROJ",
	}

	assert.True(t, validResult.Valid)
	assert.Empty(t, validResult.Errors)

	// Test invalid result
	invalidResult := &ValidationResult{
		Valid:       false,
		Errors:      []string{"unbalanced quotes"},
		Warnings:    []string{"performance warning"},
		Suggestions: []string{"use IN operator"},
		JQL:         "project = PROJ AND status = \"To Do",
	}

	assert.False(t, invalidResult.Valid)
	assert.Contains(t, invalidResult.Errors, "unbalanced quotes")
	assert.Contains(t, invalidResult.Warnings, "performance warning")
	assert.Contains(t, invalidResult.Suggestions, "use IN operator")
}

func TestQuery(t *testing.T) {
	query := &Query{
		JQL:         "project = PROJ",
		Description: "Test query",
		Template:    "test-template",
		Parameters:  map[string]string{"project_key": "PROJ"},
		Optimized:   true,
	}

	assert.Equal(t, "project = PROJ", query.JQL)
	assert.Equal(t, "Test query", query.Description)
	assert.Equal(t, "test-template", query.Template)
	assert.Equal(t, "PROJ", query.Parameters["project_key"])
	assert.True(t, query.Optimized)
}

func TestPreviewResult(t *testing.T) {
	preview := &PreviewResult{
		Query:      "project = PROJ",
		TotalCount: 42,
		ProjectBreakdown: map[string]int{
			"PROJ": 30,
			"TEAM": 12,
		},
		StatusBreakdown: map[string]int{
			"To Do":       15,
			"In Progress": 20,
			"Done":        7,
		},
		TypeBreakdown: map[string]int{
			"Story": 25,
			"Bug":   10,
			"Task":  7,
		},
		ExecutionTimeMs: 250,
	}

	assert.Equal(t, "project = PROJ", preview.Query)
	assert.Equal(t, 42, preview.TotalCount)
	assert.Equal(t, 30, preview.ProjectBreakdown["PROJ"])
	assert.Equal(t, 15, preview.StatusBreakdown["To Do"])
	assert.Equal(t, 25, preview.TypeBreakdown["Story"])
	assert.Equal(t, int64(250), preview.ExecutionTimeMs)
}

func TestSavedQuery(t *testing.T) {
	saved := &SavedQuery{
		Name:        "my-favorite-query",
		Description: "My frequently used query",
		JQL:         "project = PROJ AND assignee = currentUser()",
		Parameters:  map[string]string{"project_key": "PROJ"},
		UsageCount:  5,
		Tags:        []string{"favorite", "daily"},
	}

	assert.Equal(t, "my-favorite-query", saved.Name)
	assert.Equal(t, "My frequently used query", saved.Description)
	assert.Equal(t, 5, saved.UsageCount)
	assert.Contains(t, saved.Tags, "favorite")
	assert.Contains(t, saved.Tags, "daily")
}

func TestTemplate(t *testing.T) {
	template := &Template{
		Name:        "test-template",
		Description: "A test template",
		Category:    "test",
		JQLTemplate: "project = {{.project_key}} AND status = {{.status}}",
		Parameters: []TemplateParam{
			{
				Name:        "project_key",
				Description: "The project key",
				Required:    true,
				Examples:    []string{"PROJ", "TEAM"},
			},
			{
				Name:         "status",
				Description:  "The issue status",
				Required:     false,
				DefaultValue: "To Do",
				Examples:     []string{"To Do", "In Progress", "Done"},
			},
		},
		Examples: []TemplateExample{
			{
				Description:  "Get issues in PROJ project",
				Parameters:   map[string]string{"project_key": "PROJ"},
				ResultingJQL: "project = PROJ AND status = To Do",
			},
		},
	}

	assert.Equal(t, "test-template", template.Name)
	assert.Equal(t, "test", template.Category)
	assert.Contains(t, template.JQLTemplate, "{{.project_key}}")
	assert.True(t, template.Parameters[0].Required)
	assert.False(t, template.Parameters[1].Required)
	assert.Equal(t, "To Do", template.Parameters[1].DefaultValue)
	assert.NotEmpty(t, template.Examples)
}

func TestTemplateParameterValidation(t *testing.T) {
	param := TemplateParam{
		Name:        "test_param",
		Description: "A test parameter",
		Required:    true,
		Examples:    []string{"example1", "example2"},
	}

	assert.Equal(t, "test_param", param.Name)
	assert.True(t, param.Required)
	assert.Contains(t, param.Examples, "example1")

	optionalParam := TemplateParam{
		Name:         "optional_param",
		Description:  "An optional parameter",
		Required:     false,
		DefaultValue: "default",
	}

	assert.False(t, optionalParam.Required)
	assert.Equal(t, "default", optionalParam.DefaultValue)
}

func TestComplexJQLPatterns(t *testing.T) {
	tests := []struct {
		name  string
		jql   string
		valid bool
	}{
		{
			name:  "nested parentheses",
			jql:   `project = PROJ AND ((status = "To Do" OR status = "In Progress") AND (assignee = currentUser() OR assignee is EMPTY))`,
			valid: true,
		},
		{
			name:  "JQL functions",
			jql:   `project = PROJ AND updated >= startOfDay(-7d) AND assignee in membersOf("developers")`,
			valid: true,
		},
		{
			name:  "epic link query",
			jql:   `"Epic Link" = PROJ-123 OR parent in (issuesInEpic("PROJ-123"))`,
			valid: true,
		},
		{
			name:  "sprint queries",
			jql:   `project = PROJ AND sprint in openSprints() AND status not in (Done, "Won''t Do")`,
			valid: true,
		},
		{
			name:  "advanced search",
			jql:   `project = PROJ AND text ~ "database" AND created >= -30d ORDER BY priority DESC, updated ASC`,
			valid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := validateJQLSyntax(tt.jql)
			if tt.valid {
				assert.Empty(t, errors, "JQL should be valid: %s", tt.jql)
			} else {
				assert.NotEmpty(t, errors, "JQL should be invalid: %s", tt.jql)
			}
		})
	}
}

func TestJQLOptimizationHints(t *testing.T) {
	tests := []struct {
		name  string
		jql   string
		hints []string
	}{
		{
			name:  "should add order by",
			jql:   `project = PROJ AND summary ~ "*test*"`,
			hints: []string{"consider adding ORDER BY clause when using wildcards"},
		},
		{
			name:  "should use IN operator",
			jql:   `status = "To Do" OR status = "In Progress" OR status = "In Review" OR status = "Testing"`,
			hints: []string{"consider using IN operator instead of multiple OR conditions"},
		},
		{
			name:  "already optimized",
			jql:   `project = PROJ AND status IN ("To Do", "In Progress") ORDER BY key ASC`,
			hints: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			suggestions := generateJQLSuggestions(tt.jql)

			for _, hint := range tt.hints {
				assert.Contains(t, suggestions, hint)
			}
		})
	}
}
