# JQL Query Builder Interface Specification

## Overview

The JQL Query Builder interface defines the contract for intelligent JIRA Query Language (JQL) operations in the jira-cdc-git system. This component provides template-based query generation, EPIC expansion, validation, optimization, and preview capabilities.

## Interface Definition

```go
type QueryBuilder interface {
    BuildEpicQuery(epicKey string) (*Query, error)
    BuildFromTemplate(templateName string, params map[string]string) (*Query, error)
    ValidateQuery(jql string) (*ValidationResult, error)
    OptimizeQuery(jql string) (*Query, error)
    PreviewQuery(jql string) (*PreviewResult, error)
    SaveQuery(name, description, jql string) error
    GetSavedQueries() ([]*SavedQuery, error)
    GetTemplates() []*Template
}
```

## Data Types

### Query Structure

```go
type Query struct {
    JQL            string `json:"jql"`
    EstimatedCount int    `json:"estimated_count"`
    Source         string `json:"source"`
    GeneratedAt    string `json:"generated_at"`
}
```

### ValidationResult Structure

```go
type ValidationResult struct {
    Valid       bool     `json:"valid"`
    Errors      []string `json:"errors,omitempty"`
    Warnings    []string `json:"warnings,omitempty"`
    Suggestions []string `json:"suggestions,omitempty"`
}
```

### PreviewResult Structure

```go
type PreviewResult struct {
    TotalCount        int            `json:"total_count"`
    ProjectBreakdown  map[string]int `json:"project_breakdown"`
    StatusBreakdown   map[string]int `json:"status_breakdown"`
    TypeBreakdown     map[string]int `json:"type_breakdown"`
    ExecutionTimeMs   int64          `json:"execution_time_ms"`
}
```

### Template Structure

```go
type Template struct {
    Name        string              `json:"name"`
    Description string              `json:"description"`
    JQL         string              `json:"jql"`
    Parameters  []TemplateParameter `json:"parameters"`
    Examples    []TemplateExample   `json:"examples"`
    Category    string              `json:"category"`
}

type TemplateParameter struct {
    Name        string `json:"name"`
    Description string `json:"description"`
    Required    bool   `json:"required"`
    Default     string `json:"default,omitempty"`
}

type TemplateExample struct {
    Description string            `json:"description"`
    Parameters  map[string]string `json:"parameters"`
}
```

### SavedQuery Structure

```go
type SavedQuery struct {
    Name        string `json:"name"`
    Description string `json:"description"`
    JQL         string `json:"jql"`
    CreatedAt   string `json:"created_at"`
    LastUsed    string `json:"last_used,omitempty"`
    UseCount    int    `json:"use_count"`
}
```

## Implementation Requirements

### EPIC Query Building
- **Smart Expansion**: Convert `--epic=PROJ-123` to comprehensive JQL queries
- **Project Extraction**: Parse project key from EPIC key using regex
- **Hierarchy Discovery**: Include stories, subtasks, and linked issues
- **Custom Field Support**: Handle both standard and custom EPIC field patterns

### Template System
- **Built-in Templates**: 5 pre-defined templates for common patterns
- **Parameter Substitution**: Replace template variables with user values
- **Validation**: Ensure all required parameters are provided
- **Examples**: Each template includes usage examples

### Query Validation
- **Syntax Checking**: Validate JQL syntax and structure
- **Quote Balance**: Check for properly balanced quotes and escaped characters
- **Field Validation**: Verify field names and operators
- **Suggestion Generation**: Provide helpful suggestions for common errors

### Query Optimization
- **Performance Improvements**: Optimize queries for better execution time
- **Field Ordering**: Order clauses for optimal JIRA performance
- **Redundancy Removal**: Remove duplicate conditions
- **Index Utilization**: Structure queries to use JIRA indexes effectively

### Query Preview
- **Fast Execution**: Quick preview without full result fetching
- **Issue Breakdown**: Show counts by project, status, and type
- **Performance Metrics**: Track query execution time
- **Pagination Support**: Use JIRA pagination for large result sets

### Saved Query Management
- **JSON Persistence**: Store saved queries in JSON format
- **Usage Tracking**: Track usage count and last used timestamp
- **Name Validation**: Ensure unique query names
- **Import/Export**: Support for sharing query configurations

## Error Handling

### Error Types
```go
// Validation errors
type ValidationError struct {
    JQL     string
    Errors  []string
    Context string
}

// Template errors
type TemplateError struct {
    TemplateName string
    Parameter    string
    Message      string
}

// Query execution errors
type QueryError struct {
    JQL     string
    Message string
    Code    int
}

// Preview errors
type PreviewError struct {
    JQL     string
    Message string
    Timeout bool
}

// Saved query errors
type SavedQueryError struct {
    Operation string
    Name      string
    Message   string
}
```

### Error Conditions
- **Invalid JQL Syntax**: Malformed queries with specific error locations
- **Missing Parameters**: Required template parameters not provided
- **EPIC Not Found**: EPIC key does not exist in JIRA
- **Query Timeout**: Preview or validation operations exceed time limits
- **Duplicate Names**: Saved query names already exist

## Built-in Templates

### 1. epic-all-issues
```jql
"Epic Link" = {epic_key} OR parent in (issuesInEpic("{epic_key}"))
```
**Parameters**: epic_key (required)
**Description**: All issues related to an EPIC

### 2. epic-stories-only
```jql
"Epic Link" = {epic_key} AND type = Story
```
**Parameters**: epic_key (required)
**Description**: Only stories in an EPIC

### 3. project-active-issues
```jql
project = {project_key} AND status in ("To Do", "In Progress", "In Review")
```
**Parameters**: project_key (required)
**Description**: Active issues in a project

### 4. my-current-sprint
```jql
assignee = currentUser() AND sprint in openSprints()
```
**Parameters**: None
**Description**: Current user's active sprint issues

### 5. recent-updates
```jql
project = {project_key} AND updated >= -{days}d ORDER BY updated DESC
```
**Parameters**: project_key (required), days (default: 7)
**Description**: Recently updated issues in a project

## Performance Requirements

### Query Building
- **EPIC Expansion**: < 100ms for typical EPIC analysis
- **Template Processing**: < 10ms for parameter substitution
- **Validation**: < 50ms for syntax checking

### Query Preview
- **Preview Execution**: < 3 seconds for most queries
- **Timeout Handling**: 10 second timeout for preview operations
- **Memory Usage**: < 10MB for preview operations

### Saved Queries
- **Load Time**: < 50ms to load all saved queries
- **Save Time**: < 100ms to persist new queries
- **Storage**: JSON files < 1MB for typical usage

## Security Requirements

### Input Validation
- **JQL Injection Prevention**: Sanitize all user inputs
- **Parameter Validation**: Validate template parameters
- **Path Traversal Protection**: Secure file operations for saved queries

### Query Safety
- **Resource Limits**: Prevent queries that could overload JIRA
- **Field Access Control**: Respect JIRA field permissions
- **Audit Logging**: Log query generation and usage (without sensitive data)

## Integration Requirements

### EPIC Analyzer Integration
```go
// JQL builder depends on EPIC analyzer for EPIC query expansion
type JIRAQueryBuilder struct {
    epicAnalyzer epic.EpicAnalyzer
    // ... other fields
}
```

### Client Integration
```go
// JQL builder uses client for query validation and preview
func (b *JIRAQueryBuilder) PreviewQuery(jql string) (*PreviewResult, error) {
    issues, totalCount, err := b.client.SearchIssuesWithPagination(jql, 0, 50)
    // ... implementation
}
```

### Sync Engine Integration
```go
// Batch sync engine uses JQL for issue discovery
func (e *BatchSyncEngine) SyncJQL(ctx context.Context, jql string, repoPath string) (*BatchResult, error) {
    // Execute JQL search and sync discovered issues
}
```

## Usage Examples

### EPIC Query Building
```go
// Create query builder
builder := jql.NewJIRAQueryBuilder(client, epicAnalyzer, nil)

// Build EPIC query
query, err := builder.BuildEpicQuery("RHOAIENG-123")
if err != nil {
    return fmt.Errorf("failed to build EPIC query: %w", err)
}

// Generated JQL: ("Epic Link" = RHOAIENG-123 OR parent in (issuesInEpic("RHOAIENG-123"))) AND project = RHOAIENG ORDER BY key ASC
```

### Template Usage
```go
// Build from template
query, err := builder.BuildFromTemplate("project-active-issues", map[string]string{
    "project_key": "RHOAIENG",
})
if err != nil {
    return fmt.Errorf("failed to build template query: %w", err)
}

// Generated JQL: project = RHOAIENG AND status in ("To Do", "In Progress", "In Review") ORDER BY key ASC
```

### Query Validation and Preview
```go
// Validate query
validation, err := builder.ValidateQuery(query.JQL)
if err != nil {
    return fmt.Errorf("failed to validate query: %w", err)
}

if !validation.Valid {
    return fmt.Errorf("invalid query: %v", validation.Errors)
}

// Preview results
preview, err := builder.PreviewQuery(query.JQL)
if err != nil {
    return fmt.Errorf("failed to preview query: %w", err)
}

fmt.Printf("Query will return %d issues\n", preview.TotalCount)
```

### Saved Query Management
```go
// Save query
err := builder.SaveQuery("my-epic-sync", "My EPIC sync configuration", query.JQL)
if err != nil {
    return fmt.Errorf("failed to save query: %w", err)
}

// Load saved queries
savedQueries, err := builder.GetSavedQueries()
if err != nil {
    return fmt.Errorf("failed to load saved queries: %w", err)
}
```

## Testing Requirements

### Unit Tests
- Mock client and EPIC analyzer for isolated testing
- Test all template parameter combinations
- Validate error handling for all error types
- Test quote balancing and escape character handling

### Integration Tests
- Test with real JIRA instance (skipped in CI)
- Validate EPIC expansion with actual EPIC data
- Test query preview performance and accuracy
- Validate template functionality with real parameters

### Performance Tests
- Measure query building performance
- Test preview operation timeouts
- Validate memory usage for large result sets

## Validation Requirements

1. **JQL Syntax**: All generated queries must be valid JQL
2. **Parameter Substitution**: Template parameters properly replaced
3. **Quote Handling**: Proper escaping of quotes in JQL strings
4. **Performance**: Operations complete within specified time limits
5. **Error Handling**: All error conditions properly handled and reported