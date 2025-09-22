package sync

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/chambrid/jira-cdc-git/pkg/client"
	"github.com/chambrid/jira-cdc-git/pkg/git"
	"github.com/chambrid/jira-cdc-git/pkg/schema"
)

func TestNewBatchSyncEngine(t *testing.T) {
	tests := []struct {
		name         string
		concurrency  int
		expectedConc int
	}{
		{
			name:         "valid concurrency",
			concurrency:  5,
			expectedConc: 5,
		},
		{
			name:         "zero concurrency defaults to 1",
			concurrency:  0,
			expectedConc: 1,
		},
		{
			name:         "negative concurrency defaults to 1",
			concurrency:  -1,
			expectedConc: 1,
		},
		{
			name:         "excessive concurrency capped at 10",
			concurrency:  15,
			expectedConc: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := client.NewMockClient()
			mockWriter := schema.NewMockFileWriter()
			mockGit := git.NewMockRepository()

			engine := NewBatchSyncEngine(mockClient, mockWriter, mockGit, tt.concurrency)

			if engine.concurrency != tt.expectedConc {
				t.Errorf("NewBatchSyncEngine() concurrency = %d, want %d", engine.concurrency, tt.expectedConc)
			}

			if engine.client != mockClient {
				t.Error("NewBatchSyncEngine() client not set correctly")
			}

			if engine.fileWriter != mockWriter {
				t.Error("NewBatchSyncEngine() fileWriter not set correctly")
			}

			if engine.gitRepo != mockGit {
				t.Error("NewBatchSyncEngine() gitRepo not set correctly")
			}

			if engine.progressChan == nil {
				t.Error("NewBatchSyncEngine() progressChan not initialized")
			}
		})
	}
}

func TestBatchSyncEngine_SyncIssues_Success(t *testing.T) {
	// Setup mocks
	mockClient := client.NewMockClient()
	mockWriter := schema.NewMockFileWriter()
	mockGit := git.NewMockRepository()

	// Configure mock data - add test issues to mock client
	issues := []string{"PROJ-1", "PROJ-2", "PROJ-3", "PROJ-4", "PROJ-5"}
	for _, issueKey := range issues {
		mockClient.Issues[issueKey] = &client.Issue{
			Key:     issueKey,
			Summary: "Test issue " + issueKey,
		}
	}

	// Configure git mock to recognize the test repo
	repoPath := "/test/repo"
	mockGit.Repositories[repoPath] = true

	engine := NewBatchSyncEngine(mockClient, mockWriter, mockGit, 1)

	ctx := context.Background()
	result, err := engine.SyncIssuesSync(ctx, issues, repoPath)

	if err != nil {
		t.Fatalf("SyncIssues() error = %v, want nil", err)
	}

	// Verify result
	if result.TotalIssues != len(issues) {
		t.Errorf("SyncIssues() TotalIssues = %d, want %d", result.TotalIssues, len(issues))
	}

	if result.ProcessedIssues != len(issues) {
		t.Errorf("SyncIssues() ProcessedIssues = %d, want %d", result.ProcessedIssues, len(issues))
	}

	if result.SuccessfulSync != len(issues) {
		t.Errorf("SyncIssues() SuccessfulSync = %d, want %d", result.SuccessfulSync, len(issues))
	}

	if result.FailedSync != 0 {
		t.Errorf("SyncIssues() FailedSync = %d, want 0", result.FailedSync)
	}

	if len(result.Errors) != 0 {
		t.Errorf("SyncIssues() Errors = %d, want 0", len(result.Errors))
	}

	if len(result.ProcessedFiles) != len(issues) {
		t.Errorf("SyncIssues() ProcessedFiles = %d, want %d", len(result.ProcessedFiles), len(issues))
	}

	// Verify performance metrics
	if result.Performance.WorkerCount != 1 {
		t.Errorf("SyncIssues() Performance.WorkerCount = %d, want 1", result.Performance.WorkerCount)
	}

	if result.Performance.IssuesPerSecond <= 0 {
		t.Errorf("SyncIssues() Performance.IssuesPerSecond = %f, want > 0", result.Performance.IssuesPerSecond)
	}

	// Verify mock calls
	if mockClient.GetIssueCallCount != len(issues) {
		t.Errorf("GetIssue called %d times, want %d", mockClient.GetIssueCallCount, len(issues))
	}

	if mockWriter.WriteIssueCallCount != len(issues) {
		t.Errorf("WriteIssueToYAML called %d times, want %d", mockWriter.WriteIssueCallCount, len(issues))
	}

	if mockGit.CommitCallCount != len(issues) {
		t.Errorf("CommitIssueFile called %d times, want %d", mockGit.CommitCallCount, len(issues))
	}
}

func TestBatchSyncEngine_SyncIssues_WithMissingIssues(t *testing.T) {
	// Setup mocks
	mockClient := client.NewMockClient()
	mockWriter := schema.NewMockFileWriter()
	mockGit := git.NewMockRepository()

	// Configure mock data with some issues missing (to simulate failures)
	issues := []string{"PROJ-1", "PROJ-2", "PROJ-3"}

	// Only add PROJ-1 to mock client (PROJ-2 and PROJ-3 will fail)
	mockClient.Issues["PROJ-1"] = &client.Issue{
		Key:     "PROJ-1",
		Summary: "Test issue PROJ-1",
	}

	engine := NewBatchSyncEngine(mockClient, mockWriter, mockGit, 1)
	repoPath := "/test/repo"

	ctx := context.Background()
	result, err := engine.SyncIssuesSync(ctx, issues, repoPath)

	if err != nil {
		t.Fatalf("SyncIssues() error = %v, want nil", err)
	}

	// Verify result - should have failures for missing issues
	if result.TotalIssues != len(issues) {
		t.Errorf("SyncIssues() TotalIssues = %d, want %d", result.TotalIssues, len(issues))
	}

	if result.ProcessedIssues != len(issues) {
		t.Errorf("SyncIssues() ProcessedIssues = %d, want %d", result.ProcessedIssues, len(issues))
	}

	// Should have some failures for missing issues
	if result.FailedSync == 0 {
		t.Error("Expected some failures for missing issues but got none")
	}

	if len(result.Errors) == 0 {
		t.Error("Expected some errors for missing issues but got none")
	}
}

func TestBatchSyncEngine_SyncJQL_Success(t *testing.T) {
	// Setup mocks
	mockClient := client.NewMockClient()
	mockWriter := schema.NewMockFileWriter()
	mockGit := git.NewMockRepository()

	// Configure mock JQL search - map JQL to issue keys
	jql := "project = PROJ AND status = 'To Do'"
	mockClient.JQLResults[jql] = []string{"PROJ-100", "PROJ-101"}

	// Add issues to mock client
	mockClient.Issues["PROJ-100"] = &client.Issue{
		Key:     "PROJ-100",
		Summary: "JQL Issue 1",
	}
	mockClient.Issues["PROJ-101"] = &client.Issue{
		Key:     "PROJ-101",
		Summary: "JQL Issue 2",
	}

	// Configure git mock to recognize the test repo
	repoPath := "/test/repo"
	mockGit.Repositories[repoPath] = true

	engine := NewBatchSyncEngine(mockClient, mockWriter, mockGit, 1)

	ctx := context.Background()
	result, err := engine.SyncJQLSync(ctx, jql, repoPath)

	if err != nil {
		t.Fatalf("SyncJQL() error = %v, want nil", err)
	}

	// Verify result
	if result.TotalIssues != 2 {
		t.Errorf("SyncJQL() TotalIssues = %d, want 2", result.TotalIssues)
	}

	if result.SuccessfulSync != 2 {
		t.Errorf("SyncJQL() SuccessfulSync = %d, want 2", result.SuccessfulSync)
	}

	// Verify SearchIssues was called
	if mockClient.SearchIssuesCallCount != 1 {
		t.Errorf("SearchIssues called %d times, want 1", mockClient.SearchIssuesCallCount)
	}

	if mockClient.LastJQLQuery != jql {
		t.Errorf("SearchIssues called with JQL %s, want %s", mockClient.LastJQLQuery, jql)
	}
}

func TestBatchSyncEngine_SyncJQL_SearchFailure(t *testing.T) {
	// Setup mocks
	mockClient := client.NewMockClient()
	mockWriter := schema.NewMockFileWriter()
	mockGit := git.NewMockRepository()

	// Configure mock JQL search to fail
	mockClient.JQLError = errors.New("JQL search failed")

	engine := NewBatchSyncEngine(mockClient, mockWriter, mockGit, 1)
	jql := "invalid JQL syntax"
	repoPath := "/test/repo"

	ctx := context.Background()
	result, err := engine.SyncJQLSync(ctx, jql, repoPath)

	if err == nil {
		t.Fatal("SyncJQL() expected error for failed search")
	}

	if result != nil {
		t.Error("SyncJQL() result should be nil on search failure")
	}

	// Verify SearchIssues was called
	if mockClient.SearchIssuesCallCount != 1 {
		t.Errorf("SearchIssues called %d times, want 1", mockClient.SearchIssuesCallCount)
	}
}

func TestBatchSyncEngine_GetProgressChannel(t *testing.T) {
	// Setup mocks
	mockClient := client.NewMockClient()
	mockWriter := schema.NewMockFileWriter()
	mockGit := git.NewMockRepository()

	engine := NewBatchSyncEngine(mockClient, mockWriter, mockGit, 1)

	// Verify progress channel is available
	progressChan := engine.GetProgressChannel()
	if progressChan == nil {
		t.Error("GetProgressChannel() returned nil")
	}
}

func TestBatchResult_PerformanceMetrics(t *testing.T) {
	result := &BatchResult{
		TotalIssues:     100,
		ProcessedIssues: 100,
		SuccessfulSync:  95,
		FailedSync:      5,
		Duration:        time.Second * 10,
		Performance: PerformanceMetrics{
			IssuesPerSecond: 10.0,
			MemoryUsageKB:   119000, // 1.19KB * 100 issues from SPIKE-005
			WorkerCount:     5,
			AvgProcessTime:  time.Millisecond * 100,
		},
	}

	// Verify performance metrics are reasonable
	if result.Performance.IssuesPerSecond != 10.0 {
		t.Errorf("Expected 10.0 issues/sec, got %f", result.Performance.IssuesPerSecond)
	}

	if result.Performance.MemoryUsageKB != 119000 {
		t.Errorf("Expected 119000 KB memory usage, got %d", result.Performance.MemoryUsageKB)
	}

	if result.Performance.WorkerCount != 5 {
		t.Errorf("Expected 5 workers, got %d", result.Performance.WorkerCount)
	}

	// Verify success rate calculation
	successRate := float64(result.SuccessfulSync) / float64(result.TotalIssues) * 100
	expectedSuccessRate := 95.0
	if successRate != expectedSuccessRate {
		t.Errorf("Expected %f%% success rate, got %f%%", expectedSuccessRate, successRate)
	}
}
