package epic

import (
	"strings"
	"testing"

	"github.com/chambrid/jira-cdc-git/pkg/client"
)

func TestNewJIRAEpicAnalyzer(t *testing.T) {
	mockClient := client.NewMockClient()

	tests := []struct {
		name    string
		options *DiscoveryOptions
	}{
		{
			name:    "with default options",
			options: nil,
		},
		{
			name:    "with custom options",
			options: &DiscoveryOptions{Strategy: StrategyHybrid, MaxDepth: 3},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analyzer := NewJIRAEpicAnalyzer(mockClient, tt.options)

			jiraAnalyzer, ok := analyzer.(*JIRAEpicAnalyzer)
			if !ok {
				t.Fatal("Expected JIRAEpicAnalyzer instance")
			}

			if jiraAnalyzer.client != mockClient {
				t.Error("Expected client to be set")
			}

			if jiraAnalyzer.options == nil {
				t.Error("Expected options to be set")
			}

			if tt.options == nil && jiraAnalyzer.options.Strategy != StrategyEpicLink {
				t.Errorf("Expected default strategy %v, got %v", StrategyEpicLink, jiraAnalyzer.options.Strategy)
			}

			if tt.options != nil && jiraAnalyzer.options.Strategy != tt.options.Strategy {
				t.Errorf("Expected strategy %v, got %v", tt.options.Strategy, jiraAnalyzer.options.Strategy)
			}

			if jiraAnalyzer.cache == nil {
				t.Error("Expected cache to be initialized")
			}
		})
	}
}

func TestJIRAEpicAnalyzer_AnalyzeEpic(t *testing.T) {
	tests := []struct {
		name          string
		epicKey       string
		epicIssue     *client.Issue
		linkedIssues  []*client.Issue
		expectError   bool
		errorContains string
	}{
		{
			name:    "successful analysis",
			epicKey: "TEST-123",
			epicIssue: &client.Issue{
				Key:       "TEST-123",
				Summary:   "Test EPIC",
				IssueType: "Epic",
				Status:    client.Status{Name: "Open"},
			},
			linkedIssues: []*client.Issue{
				{
					Key:           "TEST-124",
					Summary:       "Story 1",
					IssueType:     "Story",
					Status:        client.Status{Name: "Open"},
					Relationships: &client.Relationships{EpicLink: "TEST-123"},
				},
				{
					Key:           "TEST-125",
					Summary:       "Task 1",
					IssueType:     "Task",
					Status:        client.Status{Name: "In Progress"},
					Relationships: &client.Relationships{EpicLink: "TEST-123"},
				},
			},
			expectError: false,
		},
		{
			name:    "not an epic issue",
			epicKey: "TEST-123",
			epicIssue: &client.Issue{
				Key:       "TEST-123",
				Summary:   "Not an EPIC",
				IssueType: "Story",
				Status:    client.Status{Name: "Open"},
			},
			expectError:   true,
			errorContains: "not an EPIC",
		},
		{
			name:          "epic not found",
			epicKey:       "NOTFOUND-123",
			expectError:   true,
			errorContains: "failed to get EPIC",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := client.NewMockClient()
			analyzer := NewJIRAEpicAnalyzer(mockClient, DefaultDiscoveryOptions())

			if tt.epicIssue != nil {
				mockClient.AddIssue(tt.epicIssue)
			}

			if tt.linkedIssues != nil {
				jql := `"Epic Link" = ` + tt.epicKey
				var issueKeys []string
				for _, issue := range tt.linkedIssues {
					mockClient.AddIssue(issue)
					issueKeys = append(issueKeys, issue.Key)
				}
				mockClient.AddJQLResult(jql, issueKeys)
			}

			result, err := analyzer.AnalyzeEpic(tt.epicKey)

			if tt.expectError {
				if err == nil {
					t.Fatal("Expected error but got none")
				}
				if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error to contain '%s', got: %v", tt.errorContains, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("Expected result but got nil")
			}

			if result.EpicKey != tt.epicKey {
				t.Errorf("Expected epic key %s, got %s", tt.epicKey, result.EpicKey)
			}

			if result.EpicSummary != tt.epicIssue.Summary {
				t.Errorf("Expected epic summary %s, got %s", tt.epicIssue.Summary, result.EpicSummary)
			}

			if result.TotalIssues != len(tt.linkedIssues) {
				t.Errorf("Expected %d total issues, got %d", len(tt.linkedIssues), result.TotalIssues)
			}

			if result.Performance == nil {
				t.Error("Expected performance metrics")
			}

			if result.Hierarchy == nil {
				t.Error("Expected hierarchy map")
			}

			if result.Completeness == nil {
				t.Error("Expected completeness report")
			}
		})
	}
}

func TestJIRAEpicAnalyzer_DiscoverEpicIssues(t *testing.T) {
	tests := []struct {
		name     string
		strategy EpicDiscoveryStrategy
		epicKey  string
		issues   []*client.Issue
	}{
		{
			name:     "epic link strategy",
			strategy: StrategyEpicLink,
			epicKey:  "TEST-123",
			issues: []*client.Issue{
				{Key: "TEST-124", IssueType: "Story"},
				{Key: "TEST-125", IssueType: "Task"},
			},
		},
		{
			name:     "custom field strategy",
			strategy: StrategyCustomField,
			epicKey:  "TEST-123",
			issues: []*client.Issue{
				{Key: "TEST-124", IssueType: "Story"},
			},
		},
		{
			name:     "parent link strategy",
			strategy: StrategyParentLink,
			epicKey:  "TEST-123",
			issues: []*client.Issue{
				{Key: "TEST-124", IssueType: "Sub-task"},
			},
		},
		{
			name:     "issue links strategy",
			strategy: StrategyIssueLinks,
			epicKey:  "TEST-123",
			issues: []*client.Issue{
				{Key: "TEST-124", IssueType: "Story"},
			},
		},
		{
			name:     "hybrid strategy",
			strategy: StrategyHybrid,
			epicKey:  "TEST-123",
			issues: []*client.Issue{
				{Key: "TEST-124", IssueType: "Story"},
				{Key: "TEST-125", IssueType: "Task"},
				{Key: "TEST-126", IssueType: "Bug"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := client.NewMockClient()
			options := &DiscoveryOptions{Strategy: tt.strategy}
			analyzer := NewJIRAEpicAnalyzer(mockClient, options)

			// Set up mock responses for different JQL queries
			jqlQueries := []string{
				`"Epic Link" = ` + tt.epicKey,
				`cf[12311140] = ` + tt.epicKey,
				`parent = ` + tt.epicKey,
				`issue in linkedIssues(` + tt.epicKey + `)`,
			}

			var issueKeys []string
			for _, issue := range tt.issues {
				mockClient.AddIssue(issue)
				issueKeys = append(issueKeys, issue.Key)
			}
			for _, jql := range jqlQueries {
				mockClient.AddJQLResult(jql, issueKeys)
			}

			result, err := analyzer.DiscoverEpicIssues(tt.epicKey)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if len(result) == 0 && len(tt.issues) > 0 {
				t.Error("Expected issues but got none")
			}

			// For hybrid strategy, we might get more results due to deduplication
			if tt.strategy != StrategyHybrid && len(result) != len(tt.issues) {
				t.Errorf("Expected %d issues, got %d", len(tt.issues), len(result))
			}
		})
	}
}

func TestJIRAEpicAnalyzer_GetEpicHierarchy(t *testing.T) {
	mockClient := client.NewMockClient()
	analyzer := NewJIRAEpicAnalyzer(mockClient, DefaultDiscoveryOptions())

	epicKey := "TEST-123"
	issues := []*client.Issue{
		{
			Key:           "TEST-124",
			Summary:       "Story 1",
			IssueType:     "Story",
			Status:        client.Status{Name: "Open"},
			Relationships: &client.Relationships{EpicLink: epicKey},
		},
		{
			Key:       "TEST-125",
			Summary:   "Task 1",
			IssueType: "Task",
			Status:    client.Status{Name: "In Progress"},
			Relationships: &client.Relationships{
				EpicLink: epicKey,
				Subtasks: []string{"TEST-126"},
			},
		},
		{
			Key:       "TEST-126",
			Summary:   "Subtask 1",
			IssueType: "Sub-task",
			Status:    client.Status{Name: "Done"},
			Relationships: &client.Relationships{
				EpicLink:    epicKey,
				ParentIssue: "TEST-125",
			},
		},
	}

	jql := `"Epic Link" = ` + epicKey
	var issueKeys []string
	for _, issue := range issues {
		mockClient.AddIssue(issue)
		issueKeys = append(issueKeys, issue.Key)
	}
	mockClient.AddJQLResult(jql, issueKeys)

	hierarchy, err := analyzer.GetEpicHierarchy(epicKey)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if hierarchy == nil {
		t.Fatal("Expected hierarchy but got nil")
	}

	if hierarchy.EpicKey != epicKey {
		t.Errorf("Expected epic key %s, got %s", epicKey, hierarchy.EpicKey)
	}

	if len(hierarchy.Stories) != 1 {
		t.Errorf("Expected 1 story, got %d", len(hierarchy.Stories))
	}

	if len(hierarchy.Tasks) != 1 {
		t.Errorf("Expected 1 task, got %d", len(hierarchy.Tasks))
	}

	if hierarchy.Levels < 2 {
		t.Errorf("Expected at least 2 levels, got %d", hierarchy.Levels)
	}

	// Check that the task has the subtask
	task := hierarchy.Tasks[0]
	if len(task.Subtasks) == 0 {
		t.Error("Expected task to have subtasks")
	}
}

func TestJIRAEpicAnalyzer_ValidateEpicCompleteness(t *testing.T) {
	mockClient := client.NewMockClient()
	analyzer := NewJIRAEpicAnalyzer(mockClient, DefaultDiscoveryOptions())

	epicKey := "TEST-123"
	issues := []*client.Issue{
		{
			Key:       "TEST-124",
			Summary:   "Story 1",
			IssueType: "Story",
			Status:    client.Status{Name: "Open"},
			Relationships: &client.Relationships{
				EpicLink:    epicKey,
				ParentIssue: "TEST-999", // Broken link - parent not in EPIC
			},
		},
	}

	jql := `"Epic Link" = ` + epicKey
	var issueKeys []string
	for _, issue := range issues {
		mockClient.AddIssue(issue)
		issueKeys = append(issueKeys, issue.Key)
	}
	mockClient.AddJQLResult(jql, issueKeys)

	report, err := analyzer.ValidateEpicCompleteness(epicKey)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if report == nil {
		t.Fatal("Expected completeness report but got nil")
	}

	if report.TotalFoundIssues != 1 {
		t.Errorf("Expected 1 found issue, got %d", report.TotalFoundIssues)
	}

	if len(report.BrokenLinks) == 0 {
		t.Error("Expected broken links to be detected")
	}

	if report.CompletenessPercent < 0 || report.CompletenessPercent > 100 {
		t.Errorf("Expected completeness percent between 0-100, got %.2f", report.CompletenessPercent)
	}
}

func TestJIRAEpicAnalyzer_AnalyzeIssues(t *testing.T) {
	mockClient := client.NewMockClient()
	analyzer := NewJIRAEpicAnalyzer(mockClient, DefaultDiscoveryOptions()).(*JIRAEpicAnalyzer)

	issues := []*client.Issue{
		{
			Key:       "TEST-124",
			IssueType: "Story",
			Status:    client.Status{Name: "Open"},
			Relationships: &client.Relationships{
				EpicLink:   "TEST-123",
				Subtasks:   []string{"TEST-126"},
				IssueLinks: []client.IssueLink{{Type: "Blocks", IssueKey: "TEST-127"}},
			},
		},
		{
			Key:       "TEST-125",
			IssueType: "Task",
			Status:    client.Status{Name: "In Progress"},
			Relationships: &client.Relationships{
				EpicLink: "TEST-123",
			},
		},
	}

	result := &AnalysisResult{
		IssuesByType:      make(map[string][]string),
		IssuesByStatus:    make(map[string][]string),
		RelationshipTypes: make(map[string]int),
	}

	analyzer.analyzeIssues(result, issues)

	if result.TotalIssues != 2 {
		t.Errorf("Expected 2 total issues, got %d", result.TotalIssues)
	}

	if len(result.IssuesByType["story"]) != 1 {
		t.Errorf("Expected 1 story, got %d", len(result.IssuesByType["story"]))
	}

	if len(result.IssuesByType["task"]) != 1 {
		t.Errorf("Expected 1 task, got %d", len(result.IssuesByType["task"]))
	}

	if len(result.IssuesByStatus["Open"]) != 1 {
		t.Errorf("Expected 1 open issue, got %d", len(result.IssuesByStatus["Open"]))
	}

	if result.RelationshipTypes["epic_link"] != 2 {
		t.Errorf("Expected 2 epic links, got %d", result.RelationshipTypes["epic_link"])
	}

	if result.RelationshipTypes["subtasks"] != 1 {
		t.Errorf("Expected 1 subtask relationship, got %d", result.RelationshipTypes["subtasks"])
	}

	if result.RelationshipTypes["issue_links"] != 1 {
		t.Errorf("Expected 1 issue link, got %d", result.RelationshipTypes["issue_links"])
	}
}

func TestJIRAEpicAnalyzer_isEpicIssue(t *testing.T) {
	analyzer := &JIRAEpicAnalyzer{}

	tests := []struct {
		issueType string
		expected  bool
	}{
		{"Epic", true},
		{"epic", true},
		{"EPIC", true},
		{"Story", false},
		{"Task", false},
		{"Bug", false},
		{"", false},
	}

	for _, tt := range tests {
		issue := &client.Issue{IssueType: tt.issueType}
		result := analyzer.isEpicIssue(issue)
		if result != tt.expected {
			t.Errorf("For issue type '%s', expected %v, got %v", tt.issueType, tt.expected, result)
		}
	}
}
