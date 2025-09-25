package jql

import (
	"fmt"
	"strings"
	"time"

	"github.com/chambrid/jira-cdc-git/pkg/client"
)

// MockQueryBuilder implements QueryBuilder interface for testing
type MockQueryBuilder struct {
	// Mock data
	Queries      map[string]*Query
	Templates    []*Template
	SavedQueries []*SavedQuery
	Validations  map[string]*ValidationResult
	Previews     map[string]*PreviewResult

	// Error simulation
	Error error

	// Call tracking
	BuildEpicQueryCalls    []string
	BuildFromTemplateCalls []BuildFromTemplateCall
	ValidateQueryCalls     []string
	OptimizeQueryCalls     []string
	PreviewQueryCalls      []string
	SaveQueryCalls         []SaveQueryCall
	GetSavedQueriesCalls   int
	GetTemplatesCalls      int
	UpdateQueryUsageCalls  []string
}

// BuildFromTemplateCall represents a call to BuildFromTemplate
type BuildFromTemplateCall struct {
	TemplateName string
	Parameters   map[string]string
}

// SaveQueryCall represents a call to SaveQuery
type SaveQueryCall struct {
	Name        string
	Description string
	JQL         string
}

// NewMockQueryBuilder creates a new mock query builder for testing
func NewMockQueryBuilder() *MockQueryBuilder {
	return &MockQueryBuilder{
		Queries:      make(map[string]*Query),
		Templates:    GetBuiltInTemplates(),
		SavedQueries: []*SavedQuery{},
		Validations:  make(map[string]*ValidationResult),
		Previews:     make(map[string]*PreviewResult),
	}
}

// BuildEpicQuery builds a mock JQL query for an EPIC
func (m *MockQueryBuilder) BuildEpicQuery(epicKey string) (*Query, error) {
	m.BuildEpicQueryCalls = append(m.BuildEpicQueryCalls, epicKey)

	if m.Error != nil {
		return nil, m.Error
	}

	if query, exists := m.Queries[epicKey]; exists {
		return query, nil
	}

	// Default mock query
	project, _, err := parseEpicKey(epicKey)
	if err != nil {
		return nil, NewEpicAnalysisError("invalid epic key", epicKey, err)
	}

	query := &Query{
		JQL:            fmt.Sprintf(`"Epic Link" = %s OR parent in (issuesInEpic("%s")) AND project = %s ORDER BY key ASC`, epicKey, epicKey, project),
		Description:    fmt.Sprintf("All issues related to EPIC %s", epicKey),
		Template:       "epic-all-issues",
		Parameters:     map[string]string{"epic_key": epicKey},
		Optimized:      true,
		EstimatedCount: 10,
		CreatedAt:      time.Now(),
	}

	return query, nil
}

// BuildFromTemplate builds a mock JQL query from a template
func (m *MockQueryBuilder) BuildFromTemplate(templateName string, params map[string]string) (*Query, error) {
	m.BuildFromTemplateCalls = append(m.BuildFromTemplateCalls, BuildFromTemplateCall{
		TemplateName: templateName,
		Parameters:   params,
	})

	if m.Error != nil {
		return nil, m.Error
	}

	// Find template
	var template *Template
	for _, t := range m.Templates {
		if t.Name == templateName {
			template = t
			break
		}
	}

	if template == nil {
		return nil, NewTemplateNotFoundError(templateName)
	}

	// Check required parameters
	for _, param := range template.Parameters {
		if param.Required {
			if _, exists := params[param.Name]; !exists {
				return nil, NewParameterMissingError(param.Name, templateName)
			}
		}
	}

	// Mock JQL generation (simplified)
	jql := template.JQLTemplate
	for key, value := range params {
		placeholder := fmt.Sprintf("{{.%s}}", key)
		jql = strings.ReplaceAll(jql, placeholder, value)
	}

	query := &Query{
		JQL:         jql,
		Description: template.Description,
		Template:    templateName,
		Parameters:  params,
		Optimized:   false,
		CreatedAt:   time.Now(),
	}

	return query, nil
}

// ValidateQuery validates a mock JQL query
func (m *MockQueryBuilder) ValidateQuery(jql string) (*ValidationResult, error) {
	m.ValidateQueryCalls = append(m.ValidateQueryCalls, jql)

	if m.Error != nil {
		return nil, m.Error
	}

	if result, exists := m.Validations[jql]; exists {
		return result, nil
	}

	// Default validation
	errors := validateJQLSyntax(jql)
	suggestions := generateJQLSuggestions(jql)

	result := &ValidationResult{
		Valid:       len(errors) == 0,
		Errors:      errors,
		Warnings:    []string{},
		Suggestions: suggestions,
		JQL:         jql,
	}

	return result, nil
}

// OptimizeQuery optimizes a mock JQL query
func (m *MockQueryBuilder) OptimizeQuery(jql string) (*Query, error) {
	m.OptimizeQueryCalls = append(m.OptimizeQueryCalls, jql)

	if m.Error != nil {
		return nil, m.Error
	}

	// Simple optimization: add ORDER BY if missing
	optimizedJQL := jql
	if !strings.Contains(jql, "ORDER BY") {
		optimizedJQL += " ORDER BY key ASC"
	}

	query := &Query{
		JQL:       optimizedJQL,
		Optimized: true,
		CreatedAt: time.Now(),
	}

	return query, nil
}

// PreviewQuery previews a mock JQL query
func (m *MockQueryBuilder) PreviewQuery(jql string) (*PreviewResult, error) {
	m.PreviewQueryCalls = append(m.PreviewQueryCalls, jql)

	if m.Error != nil {
		return nil, m.Error
	}

	if result, exists := m.Previews[jql]; exists {
		return result, nil
	}

	// Default preview result
	result := &PreviewResult{
		Query:      jql,
		TotalCount: 5,
		SampleIssues: []*client.Issue{
			client.CreateTestIssue("PROJ-1"),
			client.CreateTestIssue("PROJ-2"),
		},
		ProjectBreakdown: map[string]int{
			"PROJ": 5,
		},
		StatusBreakdown: map[string]int{
			"To Do":       2,
			"In Progress": 2,
			"Done":        1,
		},
		TypeBreakdown: map[string]int{
			"Story": 3,
			"Bug":   2,
		},
		ExecutionTimeMs: 123,
	}

	return result, nil
}

// GetTemplates returns mock templates
func (m *MockQueryBuilder) GetTemplates() []*Template {
	m.GetTemplatesCalls++
	return m.Templates
}

// SaveQuery saves a mock query
func (m *MockQueryBuilder) SaveQuery(name, description, jql string) error {
	m.SaveQueryCalls = append(m.SaveQueryCalls, SaveQueryCall{
		Name:        name,
		Description: description,
		JQL:         jql,
	})

	if m.Error != nil {
		return m.Error
	}

	// Check if query exists
	for i, saved := range m.SavedQueries {
		if saved.Name == name {
			// Update existing
			m.SavedQueries[i] = &SavedQuery{
				Name:        name,
				Description: description,
				JQL:         jql,
				UsageCount:  saved.UsageCount,
				LastUsed:    saved.LastUsed,
				CreatedAt:   saved.CreatedAt,
			}
			return nil
		}
	}

	// Add new query
	newQuery := &SavedQuery{
		Name:        name,
		Description: description,
		JQL:         jql,
		UsageCount:  0,
		CreatedAt:   time.Now(),
	}

	m.SavedQueries = append(m.SavedQueries, newQuery)
	return nil
}

// GetSavedQueries returns mock saved queries
func (m *MockQueryBuilder) GetSavedQueries() ([]*SavedQuery, error) {
	m.GetSavedQueriesCalls++

	if m.Error != nil {
		return nil, m.Error
	}

	return m.SavedQueries, nil
}

// UpdateQueryUsage updates mock usage statistics
func (m *MockQueryBuilder) UpdateQueryUsage(name string) error {
	m.UpdateQueryUsageCalls = append(m.UpdateQueryUsageCalls, name)

	if m.Error != nil {
		return m.Error
	}

	for i, query := range m.SavedQueries {
		if query.Name == name {
			m.SavedQueries[i].UsageCount++
			m.SavedQueries[i].LastUsed = time.Now()
			return nil
		}
	}

	return NewSavedQueryError("query not found", name, nil)
}

// SetQuery sets a mock query for a specific epic key
func (m *MockQueryBuilder) SetQuery(epicKey string, query *Query) {
	m.Queries[epicKey] = query
}

// SetValidation sets a mock validation result for a specific JQL
func (m *MockQueryBuilder) SetValidation(jql string, result *ValidationResult) {
	m.Validations[jql] = result
}

// SetPreview sets a mock preview result for a specific JQL
func (m *MockQueryBuilder) SetPreview(jql string, result *PreviewResult) {
	m.Previews[jql] = result
}

// SetError configures the mock to return an error
func (m *MockQueryBuilder) SetError(err error) {
	m.Error = err
}

// Reset clears all mock state
func (m *MockQueryBuilder) Reset() {
	m.Queries = make(map[string]*Query)
	m.Templates = GetBuiltInTemplates()
	m.SavedQueries = []*SavedQuery{}
	m.Validations = make(map[string]*ValidationResult)
	m.Previews = make(map[string]*PreviewResult)
	m.Error = nil

	// Reset call tracking
	m.BuildEpicQueryCalls = []string{}
	m.BuildFromTemplateCalls = []BuildFromTemplateCall{}
	m.ValidateQueryCalls = []string{}
	m.OptimizeQueryCalls = []string{}
	m.PreviewQueryCalls = []string{}
	m.SaveQueryCalls = []SaveQueryCall{}
	m.GetSavedQueriesCalls = 0
	m.GetTemplatesCalls = 0
	m.UpdateQueryUsageCalls = []string{}
}

// CreateTestQuery creates a test query for testing
func CreateTestQuery(epicKey string) *Query {
	return &Query{
		JQL:            fmt.Sprintf(`"Epic Link" = %s ORDER BY key ASC`, epicKey),
		Description:    fmt.Sprintf("Test query for %s", epicKey),
		Template:       "epic-all-issues",
		Parameters:     map[string]string{"epic_key": epicKey},
		Optimized:      true,
		EstimatedCount: 5,
		CreatedAt:      time.Now(),
	}
}

// CreateTestValidationResult creates a test validation result
func CreateTestValidationResult(jql string, valid bool) *ValidationResult {
	var errors []string
	if !valid {
		errors = []string{"test validation error"}
	}

	return &ValidationResult{
		Valid:       valid,
		Errors:      errors,
		Warnings:    []string{},
		Suggestions: []string{"test suggestion"},
		JQL:         jql,
	}
}

// CreateTestPreviewResult creates a test preview result
func CreateTestPreviewResult(jql string, totalCount int) *PreviewResult {
	return &PreviewResult{
		Query:      jql,
		TotalCount: totalCount,
		SampleIssues: []*client.Issue{
			client.CreateTestIssue("PROJ-1"),
			client.CreateTestIssue("PROJ-2"),
		},
		ProjectBreakdown: map[string]int{
			"PROJ": totalCount,
		},
		StatusBreakdown: map[string]int{
			"To Do": totalCount / 2,
			"Done":  totalCount / 2,
		},
		TypeBreakdown: map[string]int{
			"Story": totalCount,
		},
		ExecutionTimeMs: 100,
	}
}

// CreateTestSavedQuery creates a test saved query
func CreateTestSavedQuery(name string) *SavedQuery {
	return &SavedQuery{
		Name:        name,
		Description: "Test saved query",
		JQL:         "project = PROJ AND assignee = currentUser()",
		Parameters:  map[string]string{"project_key": "PROJ"},
		UsageCount:  1,
		LastUsed:    time.Now(),
		CreatedAt:   time.Now().Add(-24 * time.Hour),
		Tags:        []string{"test", "favorite"},
	}
}
