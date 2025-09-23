package epic

import (
	"fmt"

	"github.com/chambrid/jira-cdc-git/pkg/client"
)

// MockEpicAnalyzer provides a mock implementation for testing
type MockEpicAnalyzer struct {
	// Function implementations
	AnalyzeEpicFunc              func(epicKey string) (*AnalysisResult, error)
	DiscoverEpicIssuesFunc       func(epicKey string) ([]*client.Issue, error)
	ValidateEpicCompletenessFunc func(epicKey string) (*CompletenessReport, error)
	GetEpicHierarchyFunc         func(epicKey string) (*HierarchyMap, error)

	// Call tracking
	AnalyzeEpicCalls              []string
	DiscoverEpicIssuesCalls       []string
	ValidateEpicCompletenessCalls []string
	GetEpicHierarchyCalls         []string

	// Pre-configured responses
	Issues      map[string][]*client.Issue
	Analyses    map[string]*AnalysisResult
	Hierarchies map[string]*HierarchyMap
	Reports     map[string]*CompletenessReport
}

// NewMockEpicAnalyzer creates a new mock EPIC analyzer
func NewMockEpicAnalyzer() *MockEpicAnalyzer {
	return &MockEpicAnalyzer{
		Issues:      make(map[string][]*client.Issue),
		Analyses:    make(map[string]*AnalysisResult),
		Hierarchies: make(map[string]*HierarchyMap),
		Reports:     make(map[string]*CompletenessReport),
	}
}

// AnalyzeEpic implements EpicAnalyzer interface
func (m *MockEpicAnalyzer) AnalyzeEpic(epicKey string) (*AnalysisResult, error) {
	m.AnalyzeEpicCalls = append(m.AnalyzeEpicCalls, epicKey)

	if m.AnalyzeEpicFunc != nil {
		return m.AnalyzeEpicFunc(epicKey)
	}

	if result, exists := m.Analyses[epicKey]; exists {
		return result, nil
	}

	// Return default mock result
	return &AnalysisResult{
		EpicKey:           epicKey,
		EpicSummary:       fmt.Sprintf("Mock EPIC %s", epicKey),
		EpicStatus:        "Open",
		TotalIssues:       3,
		IssuesByType:      map[string][]string{"story": {epicKey + "-1", epicKey + "-2"}, "task": {epicKey + "-3"}},
		IssuesByStatus:    map[string][]string{"Open": {epicKey + "-1"}, "In Progress": {epicKey + "-2"}, "Done": {epicKey + "-3"}},
		RelationshipTypes: map[string]int{"epic_link": 3},
		Performance: &PerformanceMetrics{
			DiscoveryTimeMs:    100,
			AnalysisTimeMs:     50,
			TotalAPICallsCount: 4,
			CacheHitCount:      1,
			CacheMissCount:     3,
		},
	}, nil
}

// DiscoverEpicIssues implements EpicAnalyzer interface
func (m *MockEpicAnalyzer) DiscoverEpicIssues(epicKey string) ([]*client.Issue, error) {
	m.DiscoverEpicIssuesCalls = append(m.DiscoverEpicIssuesCalls, epicKey)

	if m.DiscoverEpicIssuesFunc != nil {
		return m.DiscoverEpicIssuesFunc(epicKey)
	}

	if issues, exists := m.Issues[epicKey]; exists {
		return issues, nil
	}

	// Return default mock issues
	return []*client.Issue{
		{
			Key:       epicKey + "-1",
			Summary:   "Mock Story 1",
			IssueType: "Story",
			Status:    client.Status{Name: "Open"},
			Relationships: &client.Relationships{
				EpicLink: epicKey,
			},
		},
		{
			Key:       epicKey + "-2",
			Summary:   "Mock Story 2",
			IssueType: "Story",
			Status:    client.Status{Name: "In Progress"},
			Relationships: &client.Relationships{
				EpicLink: epicKey,
			},
		},
		{
			Key:       epicKey + "-3",
			Summary:   "Mock Task 1",
			IssueType: "Task",
			Status:    client.Status{Name: "Done"},
			Relationships: &client.Relationships{
				EpicLink: epicKey,
			},
		},
	}, nil
}

// ValidateEpicCompleteness implements EpicAnalyzer interface
func (m *MockEpicAnalyzer) ValidateEpicCompleteness(epicKey string) (*CompletenessReport, error) {
	m.ValidateEpicCompletenessCalls = append(m.ValidateEpicCompletenessCalls, epicKey)

	if m.ValidateEpicCompletenessFunc != nil {
		return m.ValidateEpicCompletenessFunc(epicKey)
	}

	if report, exists := m.Reports[epicKey]; exists {
		return report, nil
	}

	// Return default mock report
	return &CompletenessReport{
		TotalExpectedIssues: 3,
		TotalFoundIssues:    3,
		CompletenessPercent: 100.0,
		Recommendations:     []string{"EPIC structure looks complete"},
	}, nil
}

// GetEpicHierarchy implements EpicAnalyzer interface
func (m *MockEpicAnalyzer) GetEpicHierarchy(epicKey string) (*HierarchyMap, error) {
	m.GetEpicHierarchyCalls = append(m.GetEpicHierarchyCalls, epicKey)

	if m.GetEpicHierarchyFunc != nil {
		return m.GetEpicHierarchyFunc(epicKey)
	}

	if hierarchy, exists := m.Hierarchies[epicKey]; exists {
		return hierarchy, nil
	}

	// Return default mock hierarchy
	return &HierarchyMap{
		EpicKey: epicKey,
		Stories: []*HierarchyNode{
			{
				IssueKey:  epicKey + "-1",
				Summary:   "Mock Story 1",
				IssueType: "Story",
				Status:    "Open",
				Level:     1,
			},
			{
				IssueKey:  epicKey + "-2",
				Summary:   "Mock Story 2",
				IssueType: "Story",
				Status:    "In Progress",
				Level:     1,
			},
		},
		Tasks: []*HierarchyNode{
			{
				IssueKey:  epicKey + "-3",
				Summary:   "Mock Task 1",
				IssueType: "Task",
				Status:    "Done",
				Level:     1,
			},
		},
		Levels: 1,
	}, nil
}

// Helper methods for testing

// SetMockIssues configures mock issues for a specific EPIC
func (m *MockEpicAnalyzer) SetMockIssues(epicKey string, issues []*client.Issue) {
	m.Issues[epicKey] = issues
}

// SetMockAnalysis configures mock analysis result for a specific EPIC
func (m *MockEpicAnalyzer) SetMockAnalysis(epicKey string, result *AnalysisResult) {
	m.Analyses[epicKey] = result
}

// SetMockHierarchy configures mock hierarchy for a specific EPIC
func (m *MockEpicAnalyzer) SetMockHierarchy(epicKey string, hierarchy *HierarchyMap) {
	m.Hierarchies[epicKey] = hierarchy
}

// SetMockCompletenessReport configures mock completeness report for a specific EPIC
func (m *MockEpicAnalyzer) SetMockCompletenessReport(epicKey string, report *CompletenessReport) {
	m.Reports[epicKey] = report
}

// GetCallCount returns the total number of calls made to the mock
func (m *MockEpicAnalyzer) GetCallCount() int {
	return len(m.AnalyzeEpicCalls) + len(m.DiscoverEpicIssuesCalls) +
		len(m.ValidateEpicCompletenessCalls) + len(m.GetEpicHierarchyCalls)
}

// WasCalled checks if any method was called with the given EPIC key
func (m *MockEpicAnalyzer) WasCalled(epicKey string) bool {
	for _, call := range m.AnalyzeEpicCalls {
		if call == epicKey {
			return true
		}
	}
	for _, call := range m.DiscoverEpicIssuesCalls {
		if call == epicKey {
			return true
		}
	}
	for _, call := range m.ValidateEpicCompletenessCalls {
		if call == epicKey {
			return true
		}
	}
	for _, call := range m.GetEpicHierarchyCalls {
		if call == epicKey {
			return true
		}
	}
	return false
}

// Reset clears all call tracking and configured responses
func (m *MockEpicAnalyzer) Reset() {
	m.AnalyzeEpicCalls = nil
	m.DiscoverEpicIssuesCalls = nil
	m.ValidateEpicCompletenessCalls = nil
	m.GetEpicHierarchyCalls = nil

	m.Issues = make(map[string][]*client.Issue)
	m.Analyses = make(map[string]*AnalysisResult)
	m.Hierarchies = make(map[string]*HierarchyMap)
	m.Reports = make(map[string]*CompletenessReport)
}

// Helper to create mock EPIC issues for testing
func CreateMockEpicIssues(epicKey string, storyCount, taskCount, bugCount int) []*client.Issue {
	var issues []*client.Issue

	// Create stories
	for i := 1; i <= storyCount; i++ {
		issues = append(issues, &client.Issue{
			Key:       fmt.Sprintf("%s-S%d", epicKey, i),
			Summary:   fmt.Sprintf("Mock Story %d", i),
			IssueType: "Story",
			Status:    client.Status{Name: "Open"},
			Relationships: &client.Relationships{
				EpicLink: epicKey,
			},
		})
	}

	// Create tasks
	for i := 1; i <= taskCount; i++ {
		issues = append(issues, &client.Issue{
			Key:       fmt.Sprintf("%s-T%d", epicKey, i),
			Summary:   fmt.Sprintf("Mock Task %d", i),
			IssueType: "Task",
			Status:    client.Status{Name: "In Progress"},
			Relationships: &client.Relationships{
				EpicLink: epicKey,
			},
		})
	}

	// Create bugs
	for i := 1; i <= bugCount; i++ {
		issues = append(issues, &client.Issue{
			Key:       fmt.Sprintf("%s-B%d", epicKey, i),
			Summary:   fmt.Sprintf("Mock Bug %d", i),
			IssueType: "Bug",
			Status:    client.Status{Name: "Done"},
			Relationships: &client.Relationships{
				EpicLink: epicKey,
			},
		})
	}

	return issues
}
