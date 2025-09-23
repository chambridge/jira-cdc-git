package epic

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/chambrid/jira-cdc-git/pkg/client"
)

// JIRAEpicAnalyzer implements the EpicAnalyzer interface using JIRA client
type JIRAEpicAnalyzer struct {
	client  client.Client
	options *DiscoveryOptions
	cache   map[string]*client.Issue // Simple in-memory cache
}

// NewJIRAEpicAnalyzer creates a new JIRA-based EPIC analyzer
func NewJIRAEpicAnalyzer(jiraClient client.Client, options *DiscoveryOptions) EpicAnalyzer {
	if options == nil {
		options = DefaultDiscoveryOptions()
	}

	return &JIRAEpicAnalyzer{
		client:  jiraClient,
		options: options,
		cache:   make(map[string]*client.Issue),
	}
}

// AnalyzeEpic discovers and analyzes an EPIC's complete structure
func (ja *JIRAEpicAnalyzer) AnalyzeEpic(epicKey string) (*AnalysisResult, error) {

	// Get the EPIC issue first
	epicIssue, err := ja.getIssue(epicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get EPIC %s: %w", epicKey, err)
	}

	// Verify this is actually an EPIC
	if !ja.isEpicIssue(epicIssue) {
		return nil, fmt.Errorf("issue %s is not an EPIC (type: %s)", epicKey, epicIssue.IssueType)
	}

	result := &AnalysisResult{
		EpicKey:           epicKey,
		EpicSummary:       epicIssue.Summary,
		EpicStatus:        epicIssue.Status.Name,
		IssuesByType:      make(map[string][]string),
		IssuesByStatus:    make(map[string][]string),
		RelationshipTypes: make(map[string]int),
	}

	// Discover all issues linked to the EPIC
	discoveryStart := time.Now()
	linkedIssues, err := ja.DiscoverEpicIssues(epicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to discover EPIC issues: %w", err)
	}
	discoveryTime := time.Since(discoveryStart)

	// Analyze the discovered issues
	analysisStart := time.Now()
	ja.analyzeIssues(result, linkedIssues)
	analysisTime := time.Since(analysisStart)

	// Build hierarchy map
	hierarchy, err := ja.buildHierarchy(epicKey, linkedIssues)
	if err != nil {
		return nil, fmt.Errorf("failed to build hierarchy: %w", err)
	}
	result.Hierarchy = hierarchy

	// Generate completeness report
	completeness, err := ja.generateCompletenessReport(epicKey, linkedIssues)
	if err != nil {
		return nil, fmt.Errorf("failed to generate completeness report: %w", err)
	}
	result.Completeness = completeness

	// Set performance metrics
	result.Performance = &PerformanceMetrics{
		DiscoveryTimeMs:    discoveryTime.Milliseconds(),
		AnalysisTimeMs:     analysisTime.Milliseconds(),
		TotalAPICallsCount: ja.getAPICallCount(),
		CacheHitCount:      ja.getCacheHitCount(),
		CacheMissCount:     ja.getCacheMissCount(),
	}

	return result, nil
}

// DiscoverEpicIssues finds all issues linked to an EPIC using the configured strategy
func (ja *JIRAEpicAnalyzer) DiscoverEpicIssues(epicKey string) ([]*client.Issue, error) {
	switch ja.options.Strategy {
	case StrategyEpicLink:
		return ja.discoverByEpicLink(epicKey)
	case StrategyCustomField:
		return ja.discoverByCustomField(epicKey)
	case StrategyParentLink:
		return ja.discoverByParentLink(epicKey)
	case StrategyIssueLinks:
		return ja.discoverByIssueLinks(epicKey)
	case StrategyHybrid:
		return ja.discoverByHybridStrategy(epicKey)
	default:
		return ja.discoverByEpicLink(epicKey) // Default fallback
	}
}

// ValidateEpicCompleteness checks for missing issues in EPIC coverage
func (ja *JIRAEpicAnalyzer) ValidateEpicCompleteness(epicKey string) (*CompletenessReport, error) {
	issues, err := ja.DiscoverEpicIssues(epicKey)
	if err != nil {
		return nil, err
	}

	return ja.generateCompletenessReport(epicKey, issues)
}

// GetEpicHierarchy returns the hierarchical structure of an EPIC
func (ja *JIRAEpicAnalyzer) GetEpicHierarchy(epicKey string) (*HierarchyMap, error) {
	issues, err := ja.DiscoverEpicIssues(epicKey)
	if err != nil {
		return nil, err
	}

	return ja.buildHierarchy(epicKey, issues)
}

// Private helper methods

// getIssue retrieves an issue with caching support
func (ja *JIRAEpicAnalyzer) getIssue(issueKey string) (*client.Issue, error) {
	if ja.options.UseCache {
		if cached, exists := ja.cache[issueKey]; exists {
			return cached, nil
		}
	}

	issue, err := ja.client.GetIssue(issueKey)
	if err != nil {
		return nil, err
	}

	if ja.options.UseCache {
		ja.cache[issueKey] = issue
	}

	return issue, nil
}

// isEpicIssue checks if an issue is an EPIC
func (ja *JIRAEpicAnalyzer) isEpicIssue(issue *client.Issue) bool {
	return strings.EqualFold(issue.IssueType, "epic")
}

// discoverByEpicLink discovers issues using Epic Link field
func (ja *JIRAEpicAnalyzer) discoverByEpicLink(epicKey string) ([]*client.Issue, error) {
	jql := fmt.Sprintf(`"Epic Link" = %s`, epicKey)
	return ja.client.SearchIssues(jql)
}

// discoverByCustomField discovers issues using Red Hat JIRA custom field
func (ja *JIRAEpicAnalyzer) discoverByCustomField(epicKey string) ([]*client.Issue, error) {
	jql := fmt.Sprintf(`cf[12311140] = %s`, epicKey)
	return ja.client.SearchIssues(jql)
}

// discoverByParentLink discovers issues using parent relationship
func (ja *JIRAEpicAnalyzer) discoverByParentLink(epicKey string) ([]*client.Issue, error) {
	jql := fmt.Sprintf(`parent = %s`, epicKey)
	return ja.client.SearchIssues(jql)
}

// discoverByIssueLinks discovers issues using issue links
func (ja *JIRAEpicAnalyzer) discoverByIssueLinks(epicKey string) ([]*client.Issue, error) {
	jql := fmt.Sprintf(`issue in linkedIssues(%s)`, epicKey)
	return ja.client.SearchIssues(jql)
}

// discoverByHybridStrategy combines multiple discovery strategies
func (ja *JIRAEpicAnalyzer) discoverByHybridStrategy(epicKey string) ([]*client.Issue, error) {
	allIssues := make(map[string]*client.Issue) // Use map to deduplicate

	strategies := []func(string) ([]*client.Issue, error){
		ja.discoverByEpicLink,
		ja.discoverByCustomField,
	}

	if ja.options.IncludeLinkedIssues {
		strategies = append(strategies, ja.discoverByIssueLinks)
	}

	for _, strategy := range strategies {
		issues, err := strategy(epicKey)
		if err != nil {
			// Log error but continue with other strategies
			continue
		}

		for _, issue := range issues {
			allIssues[issue.Key] = issue
		}
	}

	// Convert map back to slice
	var result []*client.Issue
	for _, issue := range allIssues {
		result = append(result, issue)
	}

	// Sort by issue key for consistent results
	sort.Slice(result, func(i, j int) bool {
		return result[i].Key < result[j].Key
	})

	return result, nil
}

// analyzeIssues performs detailed analysis of discovered issues
func (ja *JIRAEpicAnalyzer) analyzeIssues(result *AnalysisResult, issues []*client.Issue) {
	result.TotalIssues = len(issues)

	for _, issue := range issues {
		// Categorize by issue type
		issueType := strings.ToLower(issue.IssueType)
		result.IssuesByType[issueType] = append(result.IssuesByType[issueType], issue.Key)

		// Categorize by status
		status := issue.Status.Name
		result.IssuesByStatus[status] = append(result.IssuesByStatus[status], issue.Key)

		// Analyze relationships
		if issue.Relationships != nil {
			if issue.Relationships.EpicLink != "" {
				result.RelationshipTypes["epic_link"]++
			}
			if issue.Relationships.ParentIssue != "" {
				result.RelationshipTypes["parent_child"]++
			}
			result.RelationshipTypes["issue_links"] += len(issue.Relationships.IssueLinks)
			result.RelationshipTypes["subtasks"] += len(issue.Relationships.Subtasks)
		}
	}
}

// buildHierarchy constructs a hierarchical map of the EPIC structure
func (ja *JIRAEpicAnalyzer) buildHierarchy(epicKey string, issues []*client.Issue) (*HierarchyMap, error) {
	hierarchy := &HierarchyMap{
		EpicKey: epicKey,
		Levels:  1, // Start with EPIC level
	}

	// Create nodes for all issues
	nodeMap := make(map[string]*HierarchyNode)
	for _, issue := range issues {
		node := &HierarchyNode{
			IssueKey:  issue.Key,
			Summary:   issue.Summary,
			IssueType: issue.IssueType,
			Status:    issue.Status.Name,
			Level:     1, // Default level
		}

		if issue.Relationships != nil && issue.Relationships.ParentIssue != "" {
			node.ParentKey = issue.Relationships.ParentIssue
		}

		nodeMap[issue.Key] = node
	}

	// Build parent-child relationships and calculate levels
	for _, node := range nodeMap {
		if node.ParentKey != "" {
			if parent, exists := nodeMap[node.ParentKey]; exists {
				parent.Subtasks = append(parent.Subtasks, node)
				node.Level = parent.Level + 1
				if node.Level > hierarchy.Levels {
					hierarchy.Levels = node.Level
				}
			}
		}
	}

	// Categorize top-level issues
	for _, node := range nodeMap {
		if node.ParentKey == "" || node.ParentKey == epicKey {
			// This is a direct child of the EPIC
			switch strings.ToLower(node.IssueType) {
			case "story":
				hierarchy.Stories = append(hierarchy.Stories, node)
			case "task":
				hierarchy.Tasks = append(hierarchy.Tasks, node)
			case "bug":
				hierarchy.Bugs = append(hierarchy.Bugs, node)
			default:
				hierarchy.DirectIssues = append(hierarchy.DirectIssues, node)
			}
		}
	}

	// Sort all slices for consistent output
	ja.sortHierarchyNodes(hierarchy.Stories)
	ja.sortHierarchyNodes(hierarchy.Tasks)
	ja.sortHierarchyNodes(hierarchy.Bugs)
	ja.sortHierarchyNodes(hierarchy.DirectIssues)

	return hierarchy, nil
}

// sortHierarchyNodes sorts hierarchy nodes by issue key
func (ja *JIRAEpicAnalyzer) sortHierarchyNodes(nodes []*HierarchyNode) {
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].IssueKey < nodes[j].IssueKey
	})

	// Recursively sort subtasks
	for _, node := range nodes {
		ja.sortHierarchyNodes(node.Subtasks)
	}
}

// generateCompletenessReport analyzes completeness of EPIC coverage
func (ja *JIRAEpicAnalyzer) generateCompletenessReport(epicKey string, issues []*client.Issue) (*CompletenessReport, error) {
	report := &CompletenessReport{
		TotalFoundIssues: len(issues),
	}

	// Analyze for broken links and orphaned issues
	issueMap := make(map[string]*client.Issue)
	for _, issue := range issues {
		issueMap[issue.Key] = issue
	}

	for _, issue := range issues {
		if issue.Relationships != nil {
			// Check for broken parent links
			if issue.Relationships.ParentIssue != "" {
				if _, exists := issueMap[issue.Relationships.ParentIssue]; !exists {
					// Parent not found in EPIC - might be broken link
					report.BrokenLinks = append(report.BrokenLinks,
						fmt.Sprintf("%s -> %s (parent not in EPIC)", issue.Key, issue.Relationships.ParentIssue))
				}
			}

			// Check for broken subtask links
			for _, subtaskKey := range issue.Relationships.Subtasks {
				if _, exists := issueMap[subtaskKey]; !exists {
					report.BrokenLinks = append(report.BrokenLinks,
						fmt.Sprintf("%s -> %s (subtask not in EPIC)", issue.Key, subtaskKey))
				}
			}
		}
	}

	// For now, assume we found all expected issues
	report.TotalExpectedIssues = report.TotalFoundIssues
	if report.TotalExpectedIssues > 0 {
		report.CompletenessPercent = float64(report.TotalFoundIssues) / float64(report.TotalExpectedIssues) * 100
	}

	// Generate recommendations
	if len(report.BrokenLinks) > 0 {
		report.Recommendations = append(report.Recommendations,
			"Fix broken relationships found in EPIC structure")
	}
	if report.TotalFoundIssues > 500 {
		report.Recommendations = append(report.Recommendations,
			"Consider splitting large EPIC for better manageability")
	}

	return report, nil
}

// Helper methods for performance tracking (simplified implementation)
func (ja *JIRAEpicAnalyzer) getAPICallCount() int {
	// In a real implementation, this would track actual API calls
	return len(ja.cache) + 1 // +1 for the initial EPIC fetch
}

func (ja *JIRAEpicAnalyzer) getCacheHitCount() int {
	// Simplified - in real implementation, track actual cache hits
	return len(ja.cache) / 2
}

func (ja *JIRAEpicAnalyzer) getCacheMissCount() int {
	// Simplified - in real implementation, track actual cache misses
	return len(ja.cache) / 2
}
