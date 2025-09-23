package epic

import (
	"testing"
)

func TestDefaultDiscoveryOptions(t *testing.T) {
	options := DefaultDiscoveryOptions()

	if options.Strategy != StrategyEpicLink {
		t.Errorf("Expected strategy %v, got %v", StrategyEpicLink, options.Strategy)
	}

	if options.MaxDepth != 5 {
		t.Errorf("Expected max depth 5, got %d", options.MaxDepth)
	}

	if !options.IncludeSubtasks {
		t.Error("Expected IncludeSubtasks to be true")
	}

	if !options.IncludeLinkedIssues {
		t.Error("Expected IncludeLinkedIssues to be true")
	}

	if options.BatchSize != 100 {
		t.Errorf("Expected batch size 100, got %d", options.BatchSize)
	}

	if !options.UseCache {
		t.Error("Expected UseCache to be true")
	}
}

func TestAnalysisResult_Structure(t *testing.T) {
	result := &AnalysisResult{
		EpicKey:           "TEST-123",
		EpicSummary:       "Test EPIC",
		EpicStatus:        "Open",
		TotalIssues:       5,
		IssuesByType:      map[string][]string{"story": {"TEST-124", "TEST-125"}},
		IssuesByStatus:    map[string][]string{"Open": {"TEST-124"}, "Done": {"TEST-125"}},
		RelationshipTypes: map[string]int{"epic_link": 2},
	}

	if result.EpicKey != "TEST-123" {
		t.Errorf("Expected epic key TEST-123, got %s", result.EpicKey)
	}

	if result.TotalIssues != 5 {
		t.Errorf("Expected total issues 5, got %d", result.TotalIssues)
	}

	if len(result.IssuesByType["story"]) != 2 {
		t.Errorf("Expected 2 stories, got %d", len(result.IssuesByType["story"]))
	}

	if result.RelationshipTypes["epic_link"] != 2 {
		t.Errorf("Expected 2 epic links, got %d", result.RelationshipTypes["epic_link"])
	}
}

func TestHierarchyMap_Structure(t *testing.T) {
	hierarchy := &HierarchyMap{
		EpicKey: "TEST-123",
		Stories: []*HierarchyNode{
			{
				IssueKey:  "TEST-124",
				Summary:   "Story 1",
				IssueType: "Story",
				Status:    "Open",
				Level:     1,
			},
		},
		Tasks: []*HierarchyNode{
			{
				IssueKey:  "TEST-125",
				Summary:   "Task 1",
				IssueType: "Task",
				Status:    "In Progress",
				Level:     1,
				Subtasks: []*HierarchyNode{
					{
						IssueKey:  "TEST-126",
						Summary:   "Subtask 1",
						IssueType: "Sub-task",
						Status:    "Done",
						Level:     2,
						ParentKey: "TEST-125",
					},
				},
			},
		},
		Levels: 2,
	}

	if hierarchy.EpicKey != "TEST-123" {
		t.Errorf("Expected epic key TEST-123, got %s", hierarchy.EpicKey)
	}

	if len(hierarchy.Stories) != 1 {
		t.Errorf("Expected 1 story, got %d", len(hierarchy.Stories))
	}

	if len(hierarchy.Tasks) != 1 {
		t.Errorf("Expected 1 task, got %d", len(hierarchy.Tasks))
	}

	if hierarchy.Levels != 2 {
		t.Errorf("Expected 2 levels, got %d", hierarchy.Levels)
	}

	task := hierarchy.Tasks[0]
	if len(task.Subtasks) != 1 {
		t.Errorf("Expected 1 subtask, got %d", len(task.Subtasks))
	}

	subtask := task.Subtasks[0]
	if subtask.Level != 2 {
		t.Errorf("Expected subtask level 2, got %d", subtask.Level)
	}

	if subtask.ParentKey != "TEST-125" {
		t.Errorf("Expected parent key TEST-125, got %s", subtask.ParentKey)
	}
}

func TestCompletenessReport_Structure(t *testing.T) {
	report := &CompletenessReport{
		TotalExpectedIssues: 10,
		TotalFoundIssues:    8,
		CompletenessPercent: 80.0,
		MissingIssues:       []string{"TEST-127", "TEST-128"},
		BrokenLinks:         []string{"TEST-124 -> TEST-999 (parent not found)"},
		Recommendations:     []string{"Fix broken relationships", "Review missing issues"},
	}

	if report.TotalExpectedIssues != 10 {
		t.Errorf("Expected 10 expected issues, got %d", report.TotalExpectedIssues)
	}

	if report.TotalFoundIssues != 8 {
		t.Errorf("Expected 8 found issues, got %d", report.TotalFoundIssues)
	}

	if report.CompletenessPercent != 80.0 {
		t.Errorf("Expected 80%% completeness, got %.1f%%", report.CompletenessPercent)
	}

	if len(report.MissingIssues) != 2 {
		t.Errorf("Expected 2 missing issues, got %d", len(report.MissingIssues))
	}

	if len(report.BrokenLinks) != 1 {
		t.Errorf("Expected 1 broken link, got %d", len(report.BrokenLinks))
	}

	if len(report.Recommendations) != 2 {
		t.Errorf("Expected 2 recommendations, got %d", len(report.Recommendations))
	}
}

func TestPerformanceMetrics_Structure(t *testing.T) {
	metrics := &PerformanceMetrics{
		DiscoveryTimeMs:    1500,
		AnalysisTimeMs:     250,
		TotalAPICallsCount: 10,
		CacheHitCount:      3,
		CacheMissCount:     7,
	}

	if metrics.DiscoveryTimeMs != 1500 {
		t.Errorf("Expected discovery time 1500ms, got %d", metrics.DiscoveryTimeMs)
	}

	if metrics.AnalysisTimeMs != 250 {
		t.Errorf("Expected analysis time 250ms, got %d", metrics.AnalysisTimeMs)
	}

	if metrics.TotalAPICallsCount != 10 {
		t.Errorf("Expected 10 API calls, got %d", metrics.TotalAPICallsCount)
	}

	if metrics.CacheHitCount != 3 {
		t.Errorf("Expected 3 cache hits, got %d", metrics.CacheHitCount)
	}

	if metrics.CacheMissCount != 7 {
		t.Errorf("Expected 7 cache misses, got %d", metrics.CacheMissCount)
	}
}

func TestEpicDiscoveryStrategy_Constants(t *testing.T) {
	strategies := []EpicDiscoveryStrategy{
		StrategyEpicLink,
		StrategyCustomField,
		StrategyParentLink,
		StrategyIssueLinks,
		StrategyHybrid,
	}

	expectedValues := []string{
		"epic_link",
		"custom_field",
		"parent_link",
		"issue_links",
		"hybrid",
	}

	for i, strategy := range strategies {
		if string(strategy) != expectedValues[i] {
			t.Errorf("Expected strategy %s, got %s", expectedValues[i], string(strategy))
		}
	}
}

func TestDiscoveryOptions_Validation(t *testing.T) {
	tests := []struct {
		name    string
		options *DiscoveryOptions
		valid   bool
	}{
		{
			name:    "default options",
			options: DefaultDiscoveryOptions(),
			valid:   true,
		},
		{
			name: "custom valid options",
			options: &DiscoveryOptions{
				Strategy:            StrategyHybrid,
				MaxDepth:            3,
				IncludeSubtasks:     true,
				IncludeLinkedIssues: false,
				BatchSize:           50,
				UseCache:            false,
			},
			valid: true,
		},
		{
			name: "zero max depth",
			options: &DiscoveryOptions{
				Strategy:            StrategyEpicLink,
				MaxDepth:            0,
				IncludeSubtasks:     true,
				IncludeLinkedIssues: true,
				BatchSize:           100,
				UseCache:            true,
			},
			valid: true, // Zero depth should be allowed (only direct children)
		},
		{
			name: "negative max depth",
			options: &DiscoveryOptions{
				Strategy:            StrategyEpicLink,
				MaxDepth:            -1,
				IncludeSubtasks:     true,
				IncludeLinkedIssues: true,
				BatchSize:           100,
				UseCache:            true,
			},
			valid: false, // Negative depth should be invalid
		},
		{
			name: "zero batch size",
			options: &DiscoveryOptions{
				Strategy:            StrategyEpicLink,
				MaxDepth:            5,
				IncludeSubtasks:     true,
				IncludeLinkedIssues: true,
				BatchSize:           0,
				UseCache:            true,
			},
			valid: false, // Zero batch size should be invalid
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Basic validation logic
			valid := tt.options.MaxDepth >= 0 && tt.options.BatchSize > 0

			if valid != tt.valid {
				t.Errorf("Expected validity %v, got %v for options: %+v", tt.valid, valid, tt.options)
			}
		})
	}
}
