# EPIC Analyzer Interface Specification

## Overview

The EPIC Analyzer interface defines the contract for intelligent EPIC discovery and analysis operations in the jira-cdc-git system. This component provides comprehensive EPIC structure analysis, relationship mapping, and completeness validation for EPIC-focused sync workflows.

## Interface Definition

```go
type EpicAnalyzer interface {
    AnalyzeEpic(epicKey string) (*AnalysisResult, error)
    DiscoverEpicIssues(epicKey string) ([]*client.Issue, error)
    GetEpicHierarchy(epicKey string) (*EpicHierarchy, error)
    ValidateEpicCompleteness(epicKey string) (*CompletenessReport, error)
}
```

## Data Types

### AnalysisResult Structure

```go
type AnalysisResult struct {
    EpicKey      string              `json:"epic_key"`
    EpicSummary  string              `json:"epic_summary"`
    TotalIssues  int                 `json:"total_issues"`
    IssuesByType map[string][]string `json:"issues_by_type"`
    Performance  PerformanceMetrics  `json:"performance"`
    Completeness CompletenessReport  `json:"completeness"`
}
```

### EpicHierarchy Structure

```go
type EpicHierarchy struct {
    EpicKey       string            `json:"epic_key"`
    Stories       []*HierarchyNode  `json:"stories"`
    Tasks         []*HierarchyNode  `json:"tasks"`
    Bugs          []*HierarchyNode  `json:"bugs"`
    DirectIssues  []*HierarchyNode  `json:"direct_issues"`
    Relationships map[string]string `json:"relationships"`
}

type HierarchyNode struct {
    IssueKey    string            `json:"issue_key"`
    Summary     string            `json:"summary"`
    IssueType   string            `json:"issue_type"`
    Status      string            `json:"status"`
    Children    []*HierarchyNode  `json:"children,omitempty"`
    Parent      string            `json:"parent,omitempty"`
    Relationships map[string][]string `json:"relationships,omitempty"`
}
```

### CompletenessReport Structure

```go
type CompletenessReport struct {
    Complete          bool     `json:"complete"`
    MissingIssues     []string `json:"missing_issues,omitempty"`
    OrphanedIssues    []string `json:"orphaned_issues,omitempty"`
    BrokenLinks       []string `json:"broken_links,omitempty"`
    Recommendations   []string `json:"recommendations,omitempty"`
    CompletionPercent float64  `json:"completion_percent"`
}
```

### PerformanceMetrics Structure

```go
type PerformanceMetrics struct {
    DiscoveryTimeMs int64 `json:"discovery_time_ms"`
    AnalysisTimeMs  int64 `json:"analysis_time_ms"`
    APICalls        int   `json:"api_calls"`
    CacheHits       int   `json:"cache_hits"`
    MemoryUsageKB   int64 `json:"memory_usage_kb"`
}
```

### DiscoveryOptions Structure

```go
type DiscoveryOptions struct {
    Strategy            DiscoveryStrategy `json:"strategy"`
    MaxDepth            int               `json:"max_depth"`
    IncludeSubtasks     bool              `json:"include_subtasks"`
    IncludeLinkedIssues bool              `json:"include_linked_issues"`
    BatchSize           int               `json:"batch_size"`
    UseCache            bool              `json:"use_cache"`
    CacheExpiration     time.Duration     `json:"cache_expiration"`
}

type DiscoveryStrategy string

const (
    StrategyEpicLink    DiscoveryStrategy = "epic_link"
    StrategyCustomField DiscoveryStrategy = "custom_field"
    StrategyParentLink  DiscoveryStrategy = "parent_link"
    StrategyHybrid      DiscoveryStrategy = "hybrid"
)
```

## Implementation Requirements

### EPIC Discovery Strategies
- **Epic Link Strategy**: Use standard "Epic Link" field
- **Custom Field Strategy**: Use custom field (e.g., `customfield_12311140`)
- **Parent Link Strategy**: Use parent-child relationships
- **Hybrid Strategy**: Combine multiple strategies for comprehensive discovery

### Hierarchy Mapping
- **Multi-level Relationships**: Support EPIC → Story → Subtask hierarchies
- **Relationship Types**: Handle epic links, parent-child, blocks, clones, etc.
- **Cross-references**: Maintain bidirectional relationship mapping
- **Validation**: Ensure hierarchy consistency and detect circular references

### Performance Optimization
- **Caching**: Cache analysis results with configurable expiration
- **Batch Processing**: Use batch API calls for large EPICs
- **Lazy Loading**: Load hierarchy details on demand
- **Memory Management**: Efficient memory usage for large EPIC structures

### Completeness Validation
- **Missing Issues**: Detect issues referenced but not accessible
- **Orphaned Issues**: Find issues that should belong to EPIC but don't
- **Broken Links**: Identify invalid relationship references
- **Health Scoring**: Calculate EPIC completeness percentage

## Error Handling

### Error Types
```go
// EPIC analysis errors
type EpicAnalysisError struct {
    EpicKey   string
    Operation string
    Message   string
    Cause     error
}

// Discovery errors
type DiscoveryError struct {
    Strategy DiscoveryStrategy
    EpicKey  string
    Message  string
}

// Hierarchy errors
type HierarchyError struct {
    EpicKey string
    IssueKey string
    Message  string
    Type     HierarchyErrorType
}

type HierarchyErrorType string

const (
    CircularReference HierarchyErrorType = "circular_reference"
    MissingParent     HierarchyErrorType = "missing_parent"
    InvalidRelation   HierarchyErrorType = "invalid_relationship"
)
```

### Error Conditions
- **EPIC Not Found**: EPIC key does not exist or is not accessible
- **Permission Denied**: Insufficient permissions to access EPIC or related issues
- **API Timeout**: JIRA API calls exceed timeout limits
- **Invalid Strategy**: Unsupported discovery strategy specified
- **Circular References**: Detected circular parent-child relationships

## Discovery Strategies Implementation

### Epic Link Strategy
```jql
"Epic Link" = {epic_key}
```
- Standard JIRA EPIC field
- Most common and reliable approach
- Works with all JIRA instances

### Custom Field Strategy
```jql
cf[12311140] = {epic_key}
```
- Red Hat JIRA specific custom field
- Higher performance for large instances
- Requires field ID knowledge

### Parent Link Strategy
```jql
parent = {epic_key} OR parent in (issuesInEpic("{epic_key}"))
```
- Uses parent-child relationships
- Handles nested hierarchies
- Good for complex EPIC structures

### Hybrid Strategy
Combines multiple strategies for comprehensive coverage:
1. Start with Epic Link strategy
2. Supplement with Custom Field strategy
3. Add Parent Link strategy for subtasks
4. Deduplicate and merge results

## Performance Requirements

### Discovery Performance
- **Small EPICs (< 50 issues)**: < 5 seconds discovery time
- **Medium EPICs (50-200 issues)**: < 15 seconds discovery time
- **Large EPICs (200+ issues)**: < 30 seconds discovery time
- **API Efficiency**: Minimize API calls through batching and caching

### Memory Usage
- **Small EPICs**: < 10MB memory usage
- **Medium EPICs**: < 50MB memory usage
- **Large EPICs**: < 200MB memory usage
- **Cache Management**: Configurable cache size limits

### Analysis Performance
- **Hierarchy Building**: < 2 seconds for typical EPICs
- **Completeness Check**: < 5 seconds for comprehensive validation
- **Relationship Mapping**: < 1 second for relationship extraction

## Caching Strategy

### Cache Key Structure
```go
type CacheKey struct {
    EpicKey   string
    Strategy  DiscoveryStrategy
    Options   string // hash of discovery options
    Timestamp time.Time
}
```

### Cache Implementation
- **In-Memory Cache**: LRU cache for frequently accessed EPICs
- **Expiration**: Configurable cache expiration (default: 1 hour)
- **Invalidation**: Manual cache invalidation for updated EPICs
- **Size Limits**: Maximum cache size to prevent memory exhaustion

## Usage Examples

### Basic EPIC Analysis
```go
// Create EPIC analyzer
analyzer := epic.NewJIRAEpicAnalyzer(client, epic.DefaultDiscoveryOptions())

// Analyze EPIC
result, err := analyzer.AnalyzeEpic("RHOAIENG-123")
if err != nil {
    return fmt.Errorf("failed to analyze EPIC: %w", err)
}

fmt.Printf("EPIC %s contains %d issues\n", result.EpicKey, result.TotalIssues)
for issueType, issues := range result.IssuesByType {
    fmt.Printf("  %s: %d issues\n", issueType, len(issues))
}
```

### Custom Discovery Options
```go
// Configure discovery options
options := &epic.DiscoveryOptions{
    Strategy:            epic.StrategyHybrid,
    MaxDepth:            3,
    IncludeSubtasks:     true,
    IncludeLinkedIssues: true,
    BatchSize:           50,
    UseCache:            true,
    CacheExpiration:     2 * time.Hour,
}

analyzer := epic.NewJIRAEpicAnalyzer(client, options)
```

### Hierarchy Navigation
```go
// Get EPIC hierarchy
hierarchy, err := analyzer.GetEpicHierarchy("RHOAIENG-123")
if err != nil {
    return fmt.Errorf("failed to get hierarchy: %w", err)
}

// Navigate hierarchy
for _, story := range hierarchy.Stories {
    fmt.Printf("Story: %s - %s\n", story.IssueKey, story.Summary)
    
    for _, subtask := range story.Children {
        fmt.Printf("  Subtask: %s - %s\n", subtask.IssueKey, subtask.Summary)
    }
}
```

### Completeness Validation
```go
// Validate EPIC completeness
report, err := analyzer.ValidateEpicCompleteness("RHOAIENG-123")
if err != nil {
    return fmt.Errorf("failed to validate completeness: %w", err)
}

fmt.Printf("EPIC Completeness: %.1f%%\n", report.CompletionPercent)

if !report.Complete {
    if len(report.MissingIssues) > 0 {
        fmt.Printf("Missing issues: %v\n", report.MissingIssues)
    }
    
    if len(report.OrphanedIssues) > 0 {
        fmt.Printf("Orphaned issues: %v\n", report.OrphanedIssues)
    }
    
    for _, recommendation := range report.Recommendations {
        fmt.Printf("Recommendation: %s\n", recommendation)
    }
}
```

## Integration Requirements

### Client Integration
```go
// EPIC analyzer depends on JIRA client for API access
type JIRAEpicAnalyzer struct {
    client client.Client
    // ... other fields
}
```

### JQL Builder Integration
```go
// JQL builder uses EPIC analyzer for EPIC query expansion
func (b *JIRAQueryBuilder) BuildEpicQuery(epicKey string) (*Query, error) {
    analysis, err := b.epicAnalyzer.AnalyzeEpic(epicKey)
    // ... use analysis results to build optimized JQL
}
```

### Sync Engine Integration
```go
// Sync engine can use EPIC analyzer for issue discovery
func (e *BatchSyncEngine) SyncEpic(ctx context.Context, epicKey, repoPath string) (*BatchResult, error) {
    issues, err := e.epicAnalyzer.DiscoverEpicIssues(epicKey)
    // ... sync discovered issues
}
```

## Testing Requirements

### Unit Tests
- Mock client for isolated testing
- Test all discovery strategies independently
- Validate error handling for all error types
- Test caching behavior and expiration

### Integration Tests
- Test with real JIRA EPICs (skipped in CI)
- Validate discovery accuracy with known EPIC structures
- Test performance with large EPICs
- Validate completeness checking with real data

### Performance Tests
- Measure discovery time for different EPIC sizes
- Test memory usage under load
- Validate cache effectiveness
- Test timeout handling

## Security Requirements

### Access Control
- **Permission Validation**: Verify access to EPIC and related issues
- **Field Access**: Respect JIRA field-level permissions
- **Rate Limiting**: Respect JIRA API rate limits

### Data Protection
- **Sensitive Data**: Avoid caching sensitive issue content
- **Audit Logging**: Log discovery operations (without sensitive data)
- **Error Information**: Limit error details to prevent information disclosure

## Default Configuration

```go
func DefaultDiscoveryOptions() *DiscoveryOptions {
    return &DiscoveryOptions{
        Strategy:            StrategyHybrid,
        MaxDepth:            3,
        IncludeSubtasks:     true,
        IncludeLinkedIssues: true,
        BatchSize:           50,
        UseCache:            true,
        CacheExpiration:     1 * time.Hour,
    }
}
```

This configuration provides a good balance of thoroughness and performance for most EPIC analysis scenarios.