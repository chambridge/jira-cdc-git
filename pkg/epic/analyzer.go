package epic

import (
	"github.com/chambrid/jira-cdc-git/pkg/client"
)

// EpicAnalyzer defines the interface for EPIC discovery and analysis operations
type EpicAnalyzer interface {
	// AnalyzeEpic discovers and analyzes an EPIC's complete structure
	AnalyzeEpic(epicKey string) (*AnalysisResult, error)

	// DiscoverEpicIssues finds all issues linked to an EPIC
	DiscoverEpicIssues(epicKey string) ([]*client.Issue, error)

	// ValidateEpicCompleteness checks for missing issues in EPIC coverage
	ValidateEpicCompleteness(epicKey string) (*CompletenessReport, error)

	// GetEpicHierarchy returns the hierarchical structure of an EPIC
	GetEpicHierarchy(epicKey string) (*HierarchyMap, error)
}

// AnalysisResult represents the complete analysis of an EPIC
type AnalysisResult struct {
	EpicKey           string              `json:"epic_key" yaml:"epic_key"`
	EpicSummary       string              `json:"epic_summary" yaml:"epic_summary"`
	EpicStatus        string              `json:"epic_status" yaml:"epic_status"`
	TotalIssues       int                 `json:"total_issues" yaml:"total_issues"`
	IssuesByType      map[string][]string `json:"issues_by_type" yaml:"issues_by_type"`
	IssuesByStatus    map[string][]string `json:"issues_by_status" yaml:"issues_by_status"`
	RelationshipTypes map[string]int      `json:"relationship_types" yaml:"relationship_types"`
	Hierarchy         *HierarchyMap       `json:"hierarchy" yaml:"hierarchy"`
	Performance       *PerformanceMetrics `json:"performance" yaml:"performance"`
	Completeness      *CompletenessReport `json:"completeness" yaml:"completeness"`
}

// HierarchyMap represents the hierarchical structure of issues in an EPIC
type HierarchyMap struct {
	EpicKey      string           `json:"epic_key" yaml:"epic_key"`
	Stories      []*HierarchyNode `json:"stories" yaml:"stories"`
	Tasks        []*HierarchyNode `json:"tasks" yaml:"tasks"`
	Bugs         []*HierarchyNode `json:"bugs" yaml:"bugs"`
	DirectIssues []*HierarchyNode `json:"direct_issues" yaml:"direct_issues"`
	Levels       int              `json:"levels" yaml:"levels"`
}

// HierarchyNode represents a single node in the EPIC hierarchy
type HierarchyNode struct {
	IssueKey  string           `json:"issue_key" yaml:"issue_key"`
	Summary   string           `json:"summary" yaml:"summary"`
	IssueType string           `json:"issue_type" yaml:"issue_type"`
	Status    string           `json:"status" yaml:"status"`
	Subtasks  []*HierarchyNode `json:"subtasks,omitempty" yaml:"subtasks,omitempty"`
	Level     int              `json:"level" yaml:"level"`
	ParentKey string           `json:"parent_key,omitempty" yaml:"parent_key,omitempty"`
}

// CompletenessReport analyzes gaps and completeness in EPIC coverage
type CompletenessReport struct {
	TotalExpectedIssues int      `json:"total_expected_issues" yaml:"total_expected_issues"`
	TotalFoundIssues    int      `json:"total_found_issues" yaml:"total_found_issues"`
	CompletenessPercent float64  `json:"completeness_percent" yaml:"completeness_percent"`
	MissingIssues       []string `json:"missing_issues,omitempty" yaml:"missing_issues,omitempty"`
	OrphanedIssues      []string `json:"orphaned_issues,omitempty" yaml:"orphaned_issues,omitempty"`
	BrokenLinks         []string `json:"broken_links,omitempty" yaml:"broken_links,omitempty"`
	Recommendations     []string `json:"recommendations,omitempty" yaml:"recommendations,omitempty"`
}

// PerformanceMetrics tracks performance data for EPIC analysis
type PerformanceMetrics struct {
	DiscoveryTimeMs    int64 `json:"discovery_time_ms" yaml:"discovery_time_ms"`
	AnalysisTimeMs     int64 `json:"analysis_time_ms" yaml:"analysis_time_ms"`
	TotalAPICallsCount int   `json:"total_api_calls" yaml:"total_api_calls"`
	CacheHitCount      int   `json:"cache_hits" yaml:"cache_hits"`
	CacheMissCount     int   `json:"cache_misses" yaml:"cache_misses"`
}

// EpicDiscoveryStrategy defines different strategies for discovering EPIC issues
type EpicDiscoveryStrategy string

const (
	StrategyEpicLink    EpicDiscoveryStrategy = "epic_link"    // Use Epic Link field
	StrategyCustomField EpicDiscoveryStrategy = "custom_field" // Use custom field directly
	StrategyParentLink  EpicDiscoveryStrategy = "parent_link"  // Use parent relationship
	StrategyIssueLinks  EpicDiscoveryStrategy = "issue_links"  // Use issue links
	StrategyHybrid      EpicDiscoveryStrategy = "hybrid"       // Combine multiple strategies
)

// DiscoveryOptions configures EPIC discovery behavior
type DiscoveryOptions struct {
	Strategy            EpicDiscoveryStrategy `json:"strategy" yaml:"strategy"`
	MaxDepth            int                   `json:"max_depth" yaml:"max_depth"`
	IncludeSubtasks     bool                  `json:"include_subtasks" yaml:"include_subtasks"`
	IncludeLinkedIssues bool                  `json:"include_linked_issues" yaml:"include_linked_issues"`
	BatchSize           int                   `json:"batch_size" yaml:"batch_size"`
	UseCache            bool                  `json:"use_cache" yaml:"use_cache"`
}

// DefaultDiscoveryOptions returns sensible default options for EPIC discovery
func DefaultDiscoveryOptions() *DiscoveryOptions {
	return &DiscoveryOptions{
		Strategy:            StrategyEpicLink,
		MaxDepth:            5,
		IncludeSubtasks:     true,
		IncludeLinkedIssues: true,
		BatchSize:           100,
		UseCache:            true,
	}
}
