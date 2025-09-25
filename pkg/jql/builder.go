package jql

import (
	"fmt"
	"strings"
	"time"

	"github.com/chambrid/jira-cdc-git/pkg/client"
)

// QueryBuilder defines the interface for JQL query building and management
type QueryBuilder interface {
	// BuildEpicQuery builds a JQL query for an EPIC and all its related issues
	BuildEpicQuery(epicKey string) (*Query, error)

	// BuildFromTemplate builds a JQL query from a predefined template
	BuildFromTemplate(templateName string, params map[string]string) (*Query, error)

	// ValidateQuery validates a JQL query and provides suggestions
	ValidateQuery(jql string) (*ValidationResult, error)

	// OptimizeQuery optimizes a JQL query for performance
	OptimizeQuery(jql string) (*Query, error)

	// PreviewQuery previews what issues a query would return without executing sync
	PreviewQuery(jql string) (*PreviewResult, error)

	// GetTemplates returns available query templates
	GetTemplates() []*Template

	// SaveQuery saves a query as a favorite
	SaveQuery(name, description, jql string) error

	// GetSavedQueries returns saved/favorite queries
	GetSavedQueries() ([]*SavedQuery, error)
}

// Query represents a constructed JQL query with metadata
type Query struct {
	JQL            string            `json:"jql" yaml:"jql"`
	Description    string            `json:"description" yaml:"description"`
	Parameters     map[string]string `json:"parameters,omitempty" yaml:"parameters,omitempty"`
	Template       string            `json:"template,omitempty" yaml:"template,omitempty"`
	Optimized      bool              `json:"optimized" yaml:"optimized"`
	EstimatedCount int               `json:"estimated_count,omitempty" yaml:"estimated_count,omitempty"`
	CreatedAt      time.Time         `json:"created_at" yaml:"created_at"`
}

// ValidationResult contains JQL validation results and suggestions
type ValidationResult struct {
	Valid       bool     `json:"valid" yaml:"valid"`
	Errors      []string `json:"errors,omitempty" yaml:"errors,omitempty"`
	Warnings    []string `json:"warnings,omitempty" yaml:"warnings,omitempty"`
	Suggestions []string `json:"suggestions,omitempty" yaml:"suggestions,omitempty"`
	JQL         string   `json:"jql" yaml:"jql"`
}

// PreviewResult shows what issues would be returned by a query
type PreviewResult struct {
	Query            string          `json:"query" yaml:"query"`
	TotalCount       int             `json:"total_count" yaml:"total_count"`
	SampleIssues     []*client.Issue `json:"sample_issues,omitempty" yaml:"sample_issues,omitempty"`
	ProjectBreakdown map[string]int  `json:"project_breakdown" yaml:"project_breakdown"`
	StatusBreakdown  map[string]int  `json:"status_breakdown" yaml:"status_breakdown"`
	TypeBreakdown    map[string]int  `json:"type_breakdown" yaml:"type_breakdown"`
	ExecutionTimeMs  int64           `json:"execution_time_ms" yaml:"execution_time_ms"`
}

// Template represents a predefined JQL query template
type Template struct {
	Name        string            `json:"name" yaml:"name"`
	Description string            `json:"description" yaml:"description"`
	Category    string            `json:"category" yaml:"category"`
	JQLTemplate string            `json:"jql_template" yaml:"jql_template"`
	Parameters  []TemplateParam   `json:"parameters" yaml:"parameters"`
	Examples    []TemplateExample `json:"examples" yaml:"examples"`
}

// TemplateParam represents a parameter in a query template
type TemplateParam struct {
	Name         string   `json:"name" yaml:"name"`
	Description  string   `json:"description" yaml:"description"`
	Required     bool     `json:"required" yaml:"required"`
	DefaultValue string   `json:"default_value,omitempty" yaml:"default_value,omitempty"`
	Examples     []string `json:"examples,omitempty" yaml:"examples,omitempty"`
}

// TemplateExample provides usage examples for templates
type TemplateExample struct {
	Description  string            `json:"description" yaml:"description"`
	Parameters   map[string]string `json:"parameters" yaml:"parameters"`
	ResultingJQL string            `json:"resulting_jql" yaml:"resulting_jql"`
}

// SavedQuery represents a user's saved/favorite query
type SavedQuery struct {
	Name        string            `json:"name" yaml:"name"`
	Description string            `json:"description" yaml:"description"`
	JQL         string            `json:"jql" yaml:"jql"`
	Parameters  map[string]string `json:"parameters,omitempty" yaml:"parameters,omitempty"`
	UsageCount  int               `json:"usage_count" yaml:"usage_count"`
	LastUsed    time.Time         `json:"last_used" yaml:"last_used"`
	CreatedAt   time.Time         `json:"created_at" yaml:"created_at"`
	Tags        []string          `json:"tags,omitempty" yaml:"tags,omitempty"`
}

// QueryBuilderOptions configures query builder behavior
type QueryBuilderOptions struct {
	EpicCustomField   string `json:"epic_custom_field" yaml:"epic_custom_field"`
	UseCache          bool   `json:"use_cache" yaml:"use_cache"`
	MaxPreviewResults int    `json:"max_preview_results" yaml:"max_preview_results"`
	OptimizeByDefault bool   `json:"optimize_by_default" yaml:"optimize_by_default"`
}

// DefaultQueryBuilderOptions returns sensible defaults
func DefaultQueryBuilderOptions() *QueryBuilderOptions {
	return &QueryBuilderOptions{
		EpicCustomField:   "customfield_12311140", // Red Hat JIRA Epic Link field
		UseCache:          true,
		MaxPreviewResults: 10,
		OptimizeByDefault: true,
	}
}

// GetBuiltInTemplates returns the predefined query templates
func GetBuiltInTemplates() []*Template {
	return []*Template{
		{
			Name:        "epic-all-issues",
			Description: "All issues linked to an EPIC (stories, tasks, bugs, subtasks)",
			Category:    "epic",
			JQLTemplate: `"Epic Link" = {{.epic_key}} OR parent in (issuesInEpic("{{.epic_key}}"))`,
			Parameters: []TemplateParam{
				{
					Name:        "epic_key",
					Description: "The EPIC issue key",
					Required:    true,
					Examples:    []string{"PROJ-123", "RHOAIENG-456"},
				},
			},
			Examples: []TemplateExample{
				{
					Description:  "Get all issues for EPIC PROJ-123",
					Parameters:   map[string]string{"epic_key": "PROJ-123"},
					ResultingJQL: `"Epic Link" = PROJ-123 OR parent in (issuesInEpic("PROJ-123"))`,
				},
			},
		},
		{
			Name:        "epic-stories-only",
			Description: "Only stories directly linked to an EPIC",
			Category:    "epic",
			JQLTemplate: `"Epic Link" = {{.epic_key}} AND type = Story`,
			Parameters: []TemplateParam{
				{
					Name:        "epic_key",
					Description: "The EPIC issue key",
					Required:    true,
					Examples:    []string{"PROJ-123", "RHOAIENG-456"},
				},
			},
			Examples: []TemplateExample{
				{
					Description:  "Get only stories for EPIC PROJ-123",
					Parameters:   map[string]string{"epic_key": "PROJ-123"},
					ResultingJQL: `"Epic Link" = PROJ-123 AND type = Story`,
				},
			},
		},
		{
			Name:        "project-active-issues",
			Description: "All active issues in a project",
			Category:    "project",
			JQLTemplate: `project = {{.project_key}} AND status in ("To Do", "In Progress", "In Review")`,
			Parameters: []TemplateParam{
				{
					Name:        "project_key",
					Description: "The project key",
					Required:    true,
					Examples:    []string{"PROJ", "RHOAIENG"},
				},
			},
			Examples: []TemplateExample{
				{
					Description:  "Get active issues in project PROJ",
					Parameters:   map[string]string{"project_key": "PROJ"},
					ResultingJQL: `project = PROJ AND status in ("To Do", "In Progress", "In Review")`,
				},
			},
		},
		{
			Name:        "assignee-current-sprint",
			Description: "Issues assigned to current user in active sprint",
			Category:    "assignee",
			JQLTemplate: `assignee = currentUser() AND sprint in openSprints(){{if .project_key}} AND project = {{.project_key}}{{end}}`,
			Parameters: []TemplateParam{
				{
					Name:        "project_key",
					Description: "Optional project key to filter by",
					Required:    false,
					Examples:    []string{"PROJ", "RHOAIENG"},
				},
			},
			Examples: []TemplateExample{
				{
					Description:  "Get my issues in current sprint",
					Parameters:   map[string]string{},
					ResultingJQL: `assignee = currentUser() AND sprint in openSprints()`,
				},
				{
					Description:  "Get my issues in current sprint for project PROJ",
					Parameters:   map[string]string{"project_key": "PROJ"},
					ResultingJQL: `assignee = currentUser() AND sprint in openSprints() AND project = PROJ`,
				},
			},
		},
		{
			Name:        "recent-updates",
			Description: "Recently updated issues in a project",
			Category:    "project",
			JQLTemplate: `project = {{.project_key}} AND updated >= -{{.days}}d ORDER BY updated DESC`,
			Parameters: []TemplateParam{
				{
					Name:        "project_key",
					Description: "The project key",
					Required:    true,
					Examples:    []string{"PROJ", "RHOAIENG"},
				},
				{
					Name:         "days",
					Description:  "Number of days to look back",
					Required:     false,
					DefaultValue: "7",
					Examples:     []string{"1", "7", "30"},
				},
			},
			Examples: []TemplateExample{
				{
					Description:  "Issues updated in last 7 days",
					Parameters:   map[string]string{"project_key": "PROJ", "days": "7"},
					ResultingJQL: `project = PROJ AND updated >= -7d ORDER BY updated DESC`,
				},
			},
		},
	}
}

// parseEpicKey extracts project and epic number from epic key
func parseEpicKey(epicKey string) (project string, number string, err error) {
	// JIRA issue key format: PROJECT-NUMBER
	parts := strings.Split(epicKey, "-")
	if len(parts) < 2 {
		return "", "", fmt.Errorf("invalid epic key format: %s", epicKey)
	}

	project = strings.Join(parts[:len(parts)-1], "-")
	number = parts[len(parts)-1]

	if project == "" || number == "" {
		return "", "", fmt.Errorf("invalid epic key format: %s", epicKey)
	}

	return project, number, nil
}

// validateJQLSyntax performs basic JQL syntax validation
func validateJQLSyntax(jql string) []string {
	var errors []string

	// Check for balanced quotes (handle escaped quotes properly)
	if !areQuotesBalanced(jql) {
		errors = append(errors, "unbalanced quotes in JQL")
	}

	// Check for balanced parentheses
	openParens := strings.Count(jql, "(")
	closeParens := strings.Count(jql, ")")
	if openParens != closeParens {
		errors = append(errors, "unbalanced parentheses in JQL")
	}

	// Check for common typos
	lowerJQL := strings.ToLower(jql)
	if strings.Contains(lowerJQL, " and and ") || strings.Contains(lowerJQL, " or or ") {
		errors = append(errors, "duplicate logical operators detected")
	}

	return errors
}

// areQuotesBalanced checks if quotes are properly balanced, handling escaped quotes
func areQuotesBalanced(jql string) bool {
	doubleQuoteCount := 0
	singleQuoteCount := 0

	i := 0
	for i < len(jql) {
		char := jql[i]
		switch char {
		case '"':
			// Check if this is an escaped quote (doubled)
			if i+1 < len(jql) && jql[i+1] == '"' {
				// This is an escaped quote, skip both characters
				i += 2
				continue
			}
			doubleQuoteCount++
		case '\'':
			// Check if this is an escaped quote (doubled)
			if i+1 < len(jql) && jql[i+1] == '\'' {
				// This is an escaped quote, skip both characters
				i += 2
				continue
			}
			singleQuoteCount++
		}
		i++
	}

	return doubleQuoteCount%2 == 0 && singleQuoteCount%2 == 0
}

// generateJQLSuggestions provides helpful suggestions for JQL improvement
func generateJQLSuggestions(jql string) []string {
	var suggestions []string

	// Suggest optimization opportunities
	if strings.Contains(jql, "*") && !strings.Contains(jql, "ORDER BY") {
		suggestions = append(suggestions, "consider adding ORDER BY clause when using wildcards")
	}

	if strings.Count(jql, " OR ") >= 3 && !strings.Contains(jql, " IN ") {
		suggestions = append(suggestions, "consider using IN operator instead of multiple OR conditions")
	}

	if strings.Contains(jql, "assignee =") && !strings.Contains(jql, "currentUser()") {
		suggestions = append(suggestions, "consider using currentUser() function for dynamic assignee queries")
	}

	return suggestions
}
