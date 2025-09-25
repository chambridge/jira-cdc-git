package jql

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/chambrid/jira-cdc-git/pkg/client"
	"github.com/chambrid/jira-cdc-git/pkg/epic"
)

// JIRAQueryBuilder implements QueryBuilder interface for JIRA
type JIRAQueryBuilder struct {
	client       client.Client
	epicAnalyzer epic.EpicAnalyzer
	options      *QueryBuilderOptions
	savedQueries []*SavedQuery
	queriesFile  string
}

// NewJIRAQueryBuilder creates a new JIRA query builder
func NewJIRAQueryBuilder(jiraClient client.Client, epicAnalyzer epic.EpicAnalyzer, options *QueryBuilderOptions) *JIRAQueryBuilder {
	if options == nil {
		options = DefaultQueryBuilderOptions()
	}

	// Default saved queries file location
	homeDir, _ := os.UserHomeDir()
	queriesFile := filepath.Join(homeDir, ".jira-sync", "saved_queries.json")

	builder := &JIRAQueryBuilder{
		client:       jiraClient,
		epicAnalyzer: epicAnalyzer,
		options:      options,
		queriesFile:  queriesFile,
	}

	// Load saved queries on initialization
	_ = builder.loadSavedQueries() // Ignore error during initialization

	return builder
}

// BuildEpicQuery builds a JQL query for an EPIC and all its related issues
func (qb *JIRAQueryBuilder) BuildEpicQuery(epicKey string) (*Query, error) {
	project, _, err := parseEpicKey(epicKey)
	if err != nil {
		return nil, fmt.Errorf("invalid epic key: %w", err)
	}

	// Use EPIC analysis to determine the best query strategy
	analysis, err := qb.epicAnalyzer.AnalyzeEpic(epicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze epic: %w", err)
	}

	// Build comprehensive JQL based on analysis results
	var jqlParts []string

	// Primary EPIC link query
	jqlParts = append(jqlParts, fmt.Sprintf(`"Epic Link" = %s`, epicKey))

	// Include subtasks of stories in the EPIC
	if analysis.TotalIssues > 0 {
		jqlParts = append(jqlParts, fmt.Sprintf(`parent in (issuesInEpic("%s"))`, epicKey))
	}

	// Combine with OR
	jql := strings.Join(jqlParts, " OR ")

	// Add project filter for performance
	jql = fmt.Sprintf("(%s) AND project = %s", jql, project)

	// Add ordering for consistent results
	jql += " ORDER BY key ASC"

	query := &Query{
		JQL:            jql,
		Description:    fmt.Sprintf("All issues related to EPIC %s", epicKey),
		Template:       "epic-all-issues",
		Parameters:     map[string]string{"epic_key": epicKey},
		Optimized:      true,
		EstimatedCount: analysis.TotalIssues,
		CreatedAt:      time.Now(),
	}

	return query, nil
}

// BuildFromTemplate builds a JQL query from a predefined template
func (qb *JIRAQueryBuilder) BuildFromTemplate(templateName string, params map[string]string) (*Query, error) {
	// Find the template
	var selectedTemplate *Template
	for _, tmpl := range GetBuiltInTemplates() {
		if tmpl.Name == templateName {
			selectedTemplate = tmpl
			break
		}
	}

	if selectedTemplate == nil {
		return nil, fmt.Errorf("template '%s' not found", templateName)
	}

	// Validate required parameters
	for _, param := range selectedTemplate.Parameters {
		if param.Required {
			if _, exists := params[param.Name]; !exists {
				return nil, fmt.Errorf("required parameter '%s' missing for template '%s'", param.Name, templateName)
			}
		}
	}

	// Apply default values for missing optional parameters
	finalParams := make(map[string]string)
	for key, value := range params {
		finalParams[key] = value
	}
	for _, param := range selectedTemplate.Parameters {
		if !param.Required && param.DefaultValue != "" {
			if _, exists := finalParams[param.Name]; !exists {
				finalParams[param.Name] = param.DefaultValue
			}
		}
	}

	// Parse and execute template
	tmpl, err := template.New(templateName).Parse(selectedTemplate.JQLTemplate)
	if err != nil {
		return nil, fmt.Errorf("failed to parse template: %w", err)
	}

	var jqlBuilder strings.Builder
	if err := tmpl.Execute(&jqlBuilder, finalParams); err != nil {
		return nil, fmt.Errorf("failed to execute template: %w", err)
	}

	jql := jqlBuilder.String()

	// Optimize if enabled
	if qb.options.OptimizeByDefault {
		optimized, err := qb.OptimizeQuery(jql)
		if err == nil {
			jql = optimized.JQL
		}
	}

	query := &Query{
		JQL:         jql,
		Description: selectedTemplate.Description,
		Template:    templateName,
		Parameters:  finalParams,
		Optimized:   qb.options.OptimizeByDefault,
		CreatedAt:   time.Now(),
	}

	return query, nil
}

// ValidateQuery validates a JQL query and provides suggestions
func (qb *JIRAQueryBuilder) ValidateQuery(jql string) (*ValidationResult, error) {
	result := &ValidationResult{
		JQL: jql,
	}

	// Basic syntax validation
	errors := validateJQLSyntax(jql)
	result.Errors = errors

	// Generate suggestions
	suggestions := generateJQLSuggestions(jql)
	result.Suggestions = suggestions

	// Check for common issues
	var warnings []string
	if len(jql) > 1000 {
		warnings = append(warnings, "very long JQL query may impact performance")
	}
	if strings.Count(jql, "OR") > 10 {
		warnings = append(warnings, "many OR conditions may impact performance")
	}
	result.Warnings = warnings

	// Overall validity
	result.Valid = len(errors) == 0

	return result, nil
}

// OptimizeQuery optimizes a JQL query for performance
func (qb *JIRAQueryBuilder) OptimizeQuery(jql string) (*Query, error) {
	optimizedJQL := jql

	// Optimization 1: Move project filters to the front
	if strings.Contains(jql, "project =") && !strings.HasPrefix(strings.TrimSpace(jql), "project =") {
		// Extract project filter and move to front
		projectParts := strings.Split(jql, " AND ")
		var projectFilter string
		var otherParts []string

		for _, part := range projectParts {
			part = strings.TrimSpace(part)
			if strings.HasPrefix(part, "project =") {
				projectFilter = part
			} else {
				otherParts = append(otherParts, part)
			}
		}

		if projectFilter != "" {
			optimizedJQL = projectFilter + " AND " + strings.Join(otherParts, " AND ")
		}
	}

	// Optimization 2: Consolidate multiple OR conditions into IN clauses where possible
	// This is a simplified optimization - more complex logic could be added

	// Optimization 3: Add ORDER BY if missing for consistent results
	if !strings.Contains(optimizedJQL, "ORDER BY") {
		optimizedJQL += " ORDER BY key ASC"
	}

	query := &Query{
		JQL:       optimizedJQL,
		Optimized: true,
		CreatedAt: time.Now(),
	}

	return query, nil
}

// PreviewQuery previews what issues a query would return without executing sync
func (qb *JIRAQueryBuilder) PreviewQuery(jql string) (*PreviewResult, error) {
	startTime := time.Now()

	// Execute search with limited results using pagination
	issues, totalCount, err := qb.client.SearchIssuesWithPagination(jql, 0, qb.options.MaxPreviewResults)
	if err != nil {
		return nil, fmt.Errorf("failed to preview query: %w", err)
	}

	// Build breakdowns
	projectBreakdown := make(map[string]int)
	statusBreakdown := make(map[string]int)
	typeBreakdown := make(map[string]int)

	for _, issue := range issues {
		project, _, _ := parseEpicKey(issue.Key)
		projectBreakdown[project]++
		statusBreakdown[issue.Status.Name]++
		typeBreakdown[issue.IssueType]++
	}

	result := &PreviewResult{
		Query:            jql,
		TotalCount:       totalCount,
		SampleIssues:     issues,
		ProjectBreakdown: projectBreakdown,
		StatusBreakdown:  statusBreakdown,
		TypeBreakdown:    typeBreakdown,
		ExecutionTimeMs:  time.Since(startTime).Milliseconds(),
	}

	return result, nil
}

// GetTemplates returns available query templates
func (qb *JIRAQueryBuilder) GetTemplates() []*Template {
	return GetBuiltInTemplates()
}

// SaveQuery saves a query as a favorite
func (qb *JIRAQueryBuilder) SaveQuery(name, description, jql string) error {
	// Check if query with same name already exists
	for i, saved := range qb.savedQueries {
		if saved.Name == name {
			// Update existing query
			qb.savedQueries[i] = &SavedQuery{
				Name:        name,
				Description: description,
				JQL:         jql,
				UsageCount:  saved.UsageCount,
				LastUsed:    saved.LastUsed,
				CreatedAt:   saved.CreatedAt,
			}
			return qb.saveToDisk()
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

	qb.savedQueries = append(qb.savedQueries, newQuery)
	return qb.saveToDisk()
}

// GetSavedQueries returns saved/favorite queries
func (qb *JIRAQueryBuilder) GetSavedQueries() ([]*SavedQuery, error) {
	// Return a copy to prevent external modification
	result := make([]*SavedQuery, len(qb.savedQueries))
	copy(result, qb.savedQueries)
	return result, nil
}

// loadSavedQueries loads saved queries from disk
func (qb *JIRAQueryBuilder) loadSavedQueries() error {
	// Create directory if it doesn't exist
	dir := filepath.Dir(qb.queriesFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Load file if it exists
	if _, err := os.Stat(qb.queriesFile); os.IsNotExist(err) {
		qb.savedQueries = []*SavedQuery{}
		return nil
	}

	data, err := os.ReadFile(qb.queriesFile)
	if err != nil {
		return err
	}

	var queries []*SavedQuery
	if err := json.Unmarshal(data, &queries); err != nil {
		return err
	}

	qb.savedQueries = queries
	return nil
}

// saveToDisk saves queries to disk
func (qb *JIRAQueryBuilder) saveToDisk() error {
	data, err := json.MarshalIndent(qb.savedQueries, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(qb.queriesFile, data, 0644)
}

// UpdateQueryUsage updates usage statistics for a saved query
func (qb *JIRAQueryBuilder) UpdateQueryUsage(name string) error {
	for i, query := range qb.savedQueries {
		if query.Name == name {
			qb.savedQueries[i].UsageCount++
			qb.savedQueries[i].LastUsed = time.Now()
			return qb.saveToDisk()
		}
	}
	return fmt.Errorf("saved query '%s' not found", name)
}
