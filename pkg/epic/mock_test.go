package epic

import (
	"testing"

	"github.com/chambrid/jira-cdc-git/pkg/client"
)

func TestNewMockEpicAnalyzer(t *testing.T) {
	mock := NewMockEpicAnalyzer()

	if mock == nil {
		t.Fatal("Expected mock analyzer but got nil")
	}

	if mock.Issues == nil {
		t.Error("Expected Issues map to be initialized")
	}

	if mock.Analyses == nil {
		t.Error("Expected Analyses map to be initialized")
	}

	if mock.Hierarchies == nil {
		t.Error("Expected Hierarchies map to be initialized")
	}

	if mock.Reports == nil {
		t.Error("Expected Reports map to be initialized")
	}

	if mock.GetCallCount() != 0 {
		t.Errorf("Expected 0 initial calls, got %d", mock.GetCallCount())
	}
}

func TestMockEpicAnalyzer_AnalyzeEpic(t *testing.T) {
	mock := NewMockEpicAnalyzer()
	epicKey := "TEST-123"

	// Test default behavior
	result, err := mock.AnalyzeEpic(epicKey)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("Expected result but got nil")
	}

	if result.EpicKey != epicKey {
		t.Errorf("Expected epic key %s, got %s", epicKey, result.EpicKey)
	}

	if len(mock.AnalyzeEpicCalls) != 1 {
		t.Errorf("Expected 1 call, got %d", len(mock.AnalyzeEpicCalls))
	}

	if mock.AnalyzeEpicCalls[0] != epicKey {
		t.Errorf("Expected call with %s, got %s", epicKey, mock.AnalyzeEpicCalls[0])
	}

	// Test custom response
	customResult := &AnalysisResult{
		EpicKey:     epicKey,
		EpicSummary: "Custom EPIC",
		TotalIssues: 42,
	}
	mock.SetMockAnalysis(epicKey, customResult)

	result2, err := mock.AnalyzeEpic(epicKey)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result2.TotalIssues != 42 {
		t.Errorf("Expected 42 total issues, got %d", result2.TotalIssues)
	}

	if result2.EpicSummary != "Custom EPIC" {
		t.Errorf("Expected 'Custom EPIC', got %s", result2.EpicSummary)
	}
}

func TestMockEpicAnalyzer_DiscoverEpicIssues(t *testing.T) {
	mock := NewMockEpicAnalyzer()
	epicKey := "TEST-123"

	// Test default behavior
	issues, err := mock.DiscoverEpicIssues(epicKey)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(issues) != 3 {
		t.Errorf("Expected 3 default issues, got %d", len(issues))
	}

	if len(mock.DiscoverEpicIssuesCalls) != 1 {
		t.Errorf("Expected 1 call, got %d", len(mock.DiscoverEpicIssuesCalls))
	}

	// Test custom issues
	customIssues := []*client.Issue{
		{Key: "CUSTOM-1", IssueType: "Story"},
		{Key: "CUSTOM-2", IssueType: "Bug"},
	}
	mock.SetMockIssues(epicKey, customIssues)

	issues2, err := mock.DiscoverEpicIssues(epicKey)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(issues2) != 2 {
		t.Errorf("Expected 2 custom issues, got %d", len(issues2))
	}

	if issues2[0].Key != "CUSTOM-1" {
		t.Errorf("Expected CUSTOM-1, got %s", issues2[0].Key)
	}
}

func TestMockEpicAnalyzer_GetEpicHierarchy(t *testing.T) {
	mock := NewMockEpicAnalyzer()
	epicKey := "TEST-123"

	// Test default behavior
	hierarchy, err := mock.GetEpicHierarchy(epicKey)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if hierarchy == nil {
		t.Fatal("Expected hierarchy but got nil")
	}

	if hierarchy.EpicKey != epicKey {
		t.Errorf("Expected epic key %s, got %s", epicKey, hierarchy.EpicKey)
	}

	if len(hierarchy.Stories) != 2 {
		t.Errorf("Expected 2 stories in default hierarchy, got %d", len(hierarchy.Stories))
	}

	if len(hierarchy.Tasks) != 1 {
		t.Errorf("Expected 1 task in default hierarchy, got %d", len(hierarchy.Tasks))
	}

	// Test custom hierarchy
	customHierarchy := &HierarchyMap{
		EpicKey: epicKey,
		Stories: []*HierarchyNode{
			{IssueKey: "STORY-1", Summary: "Custom Story"},
		},
		Levels: 3,
	}
	mock.SetMockHierarchy(epicKey, customHierarchy)

	hierarchy2, err := mock.GetEpicHierarchy(epicKey)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if hierarchy2.Levels != 3 {
		t.Errorf("Expected 3 levels, got %d", hierarchy2.Levels)
	}

	if len(hierarchy2.Stories) != 1 {
		t.Errorf("Expected 1 custom story, got %d", len(hierarchy2.Stories))
	}
}

func TestMockEpicAnalyzer_ValidateEpicCompleteness(t *testing.T) {
	mock := NewMockEpicAnalyzer()
	epicKey := "TEST-123"

	// Test default behavior
	report, err := mock.ValidateEpicCompleteness(epicKey)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if report == nil {
		t.Fatal("Expected report but got nil")
	}

	if report.CompletenessPercent != 100.0 {
		t.Errorf("Expected 100%% completeness, got %.1f%%", report.CompletenessPercent)
	}

	// Test custom report
	customReport := &CompletenessReport{
		TotalExpectedIssues: 10,
		TotalFoundIssues:    8,
		CompletenessPercent: 80.0,
		MissingIssues:       []string{"MISSING-1", "MISSING-2"},
	}
	mock.SetMockCompletenessReport(epicKey, customReport)

	report2, err := mock.ValidateEpicCompleteness(epicKey)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if report2.CompletenessPercent != 80.0 {
		t.Errorf("Expected 80%% completeness, got %.1f%%", report2.CompletenessPercent)
	}

	if len(report2.MissingIssues) != 2 {
		t.Errorf("Expected 2 missing issues, got %d", len(report2.MissingIssues))
	}
}

func TestMockEpicAnalyzer_CallTracking(t *testing.T) {
	mock := NewMockEpicAnalyzer()
	epicKey1 := "TEST-123"
	epicKey2 := "TEST-456"

	// Make various calls
	_, _ = mock.AnalyzeEpic(epicKey1)
	_, _ = mock.DiscoverEpicIssues(epicKey1)
	_, _ = mock.GetEpicHierarchy(epicKey2)
	_, _ = mock.ValidateEpicCompleteness(epicKey2)

	// Check call counts
	if len(mock.AnalyzeEpicCalls) != 1 {
		t.Errorf("Expected 1 AnalyzeEpic call, got %d", len(mock.AnalyzeEpicCalls))
	}

	if len(mock.DiscoverEpicIssuesCalls) != 1 {
		t.Errorf("Expected 1 DiscoverEpicIssues call, got %d", len(mock.DiscoverEpicIssuesCalls))
	}

	if len(mock.GetEpicHierarchyCalls) != 1 {
		t.Errorf("Expected 1 GetEpicHierarchy call, got %d", len(mock.GetEpicHierarchyCalls))
	}

	if len(mock.ValidateEpicCompletenessCalls) != 1 {
		t.Errorf("Expected 1 ValidateEpicCompleteness call, got %d", len(mock.ValidateEpicCompletenessCalls))
	}

	if mock.GetCallCount() != 4 {
		t.Errorf("Expected 4 total calls, got %d", mock.GetCallCount())
	}

	// Check WasCalled method
	if !mock.WasCalled(epicKey1) {
		t.Errorf("Expected %s to have been called", epicKey1)
	}

	if !mock.WasCalled(epicKey2) {
		t.Errorf("Expected %s to have been called", epicKey2)
	}

	if mock.WasCalled("NEVER-CALLED") {
		t.Error("Expected NEVER-CALLED to not have been called")
	}
}

func TestMockEpicAnalyzer_Reset(t *testing.T) {
	mock := NewMockEpicAnalyzer()
	epicKey := "TEST-123"

	// Make some calls and set some data
	_, _ = mock.AnalyzeEpic(epicKey)
	mock.SetMockIssues(epicKey, []*client.Issue{{Key: "TEST-1"}})

	if mock.GetCallCount() == 0 {
		t.Fatal("Expected some calls before reset")
	}

	if len(mock.Issues) == 0 {
		t.Fatal("Expected some issues before reset")
	}

	// Reset
	mock.Reset()

	// Check everything is cleared
	if mock.GetCallCount() != 0 {
		t.Errorf("Expected 0 calls after reset, got %d", mock.GetCallCount())
	}

	if len(mock.Issues) != 0 {
		t.Errorf("Expected 0 issues after reset, got %d", len(mock.Issues))
	}

	if len(mock.Analyses) != 0 {
		t.Errorf("Expected 0 analyses after reset, got %d", len(mock.Analyses))
	}

	if mock.WasCalled(epicKey) {
		t.Error("Expected no calls to be tracked after reset")
	}
}

func TestMockEpicAnalyzer_CustomFunctions(t *testing.T) {
	mock := NewMockEpicAnalyzer()
	epicKey := "TEST-123"

	// Set custom function that returns an error
	mock.AnalyzeEpicFunc = func(key string) (*AnalysisResult, error) {
		if key == "ERROR-EPIC" {
			return nil, NewEpicError(ErrorTypeNotFound, "EPIC not found", key, nil)
		}
		return &AnalysisResult{EpicKey: key, EpicSummary: "Custom function result"}, nil
	}

	// Test normal case
	result, err := mock.AnalyzeEpic(epicKey)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result.EpicSummary != "Custom function result" {
		t.Errorf("Expected custom function result, got %s", result.EpicSummary)
	}

	// Test error case
	_, err = mock.AnalyzeEpic("ERROR-EPIC")
	if err == nil {
		t.Fatal("Expected error but got none")
	}

	if !IsNotFoundError(err) {
		t.Errorf("Expected NotFoundError, got %T", err)
	}
}

func TestCreateMockEpicIssues(t *testing.T) {
	epicKey := "TEST-123"

	issues := CreateMockEpicIssues(epicKey, 2, 1, 1)

	if len(issues) != 4 {
		t.Errorf("Expected 4 issues (2+1+1), got %d", len(issues))
	}

	storyCount := 0
	taskCount := 0
	bugCount := 0

	for _, issue := range issues {
		switch issue.IssueType {
		case "Story":
			storyCount++
		case "Task":
			taskCount++
		case "Bug":
			bugCount++
		}

		if issue.Relationships == nil || issue.Relationships.EpicLink != epicKey {
			t.Errorf("Expected issue %s to have epic link %s", issue.Key, epicKey)
		}
	}

	if storyCount != 2 {
		t.Errorf("Expected 2 stories, got %d", storyCount)
	}

	if taskCount != 1 {
		t.Errorf("Expected 1 task, got %d", taskCount)
	}

	if bugCount != 1 {
		t.Errorf("Expected 1 bug, got %d", bugCount)
	}
}
