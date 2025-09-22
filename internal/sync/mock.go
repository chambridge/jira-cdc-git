package sync

import (
	"context"
	"time"
)

// MockBatchSyncOrchestrator provides a mock implementation for testing
type MockBatchSyncOrchestrator struct {
	SyncIssuesFunc func(ctx context.Context, issues []string, repoPath string) (*BatchResult, error)
	SyncJQLFunc    func(ctx context.Context, jql string, repoPath string) (*BatchResult, error)

	// Call tracking
	SyncIssuesCalls []SyncIssuesCall
	SyncJQLCalls    []SyncJQLCall
}

// SyncIssuesCall tracks calls to SyncIssues
type SyncIssuesCall struct {
	Issues   []string
	RepoPath string
	Result   *BatchResult
	Error    error
}

// SyncJQLCall tracks calls to SyncJQL
type SyncJQLCall struct {
	JQL      string
	RepoPath string
	Result   *BatchResult
	Error    error
}

// NewMockBatchSyncOrchestrator creates a new mock batch sync orchestrator
func NewMockBatchSyncOrchestrator() *MockBatchSyncOrchestrator {
	return &MockBatchSyncOrchestrator{
		SyncIssuesCalls: make([]SyncIssuesCall, 0),
		SyncJQLCalls:    make([]SyncJQLCall, 0),
	}
}

// SyncIssues implements the BatchSyncOrchestrator interface
func (m *MockBatchSyncOrchestrator) SyncIssues(ctx context.Context, issues []string, repoPath string) (*BatchResult, error) {
	call := SyncIssuesCall{
		Issues:   issues,
		RepoPath: repoPath,
	}

	if m.SyncIssuesFunc != nil {
		result, err := m.SyncIssuesFunc(ctx, issues, repoPath)
		call.Result = result
		call.Error = err
		m.SyncIssuesCalls = append(m.SyncIssuesCalls, call)
		return result, err
	}

	// Default mock behavior - successful sync
	result := &BatchResult{
		TotalIssues:     len(issues),
		ProcessedIssues: len(issues),
		SuccessfulSync:  len(issues),
		FailedSync:      0,
		ProcessedFiles:  make([]string, len(issues)),
		Errors:          make([]BatchError, 0),
		Duration:        time.Millisecond * 100, // Simulate processing time
		Performance: PerformanceMetrics{
			IssuesPerSecond: float64(len(issues)) / 0.1, // 100ms = 0.1s
			MemoryUsageKB:   int64(len(issues)) * 1190,  // 1.19KB per issue from SPIKE-005
			WorkerCount:     5,                          // Default worker count
			AvgProcessTime:  time.Millisecond * 20,      // Average process time per issue
		},
	}

	// Generate mock file paths
	for i, issue := range issues {
		result.ProcessedFiles[i] = "/mock/path/projects/PROJ/issues/" + issue + ".yaml"
	}

	call.Result = result
	m.SyncIssuesCalls = append(m.SyncIssuesCalls, call)
	return result, nil
}

// SyncJQL implements the BatchSyncOrchestrator interface
func (m *MockBatchSyncOrchestrator) SyncJQL(ctx context.Context, jql string, repoPath string) (*BatchResult, error) {
	call := SyncJQLCall{
		JQL:      jql,
		RepoPath: repoPath,
	}

	if m.SyncJQLFunc != nil {
		result, err := m.SyncJQLFunc(ctx, jql, repoPath)
		call.Result = result
		call.Error = err
		m.SyncJQLCalls = append(m.SyncJQLCalls, call)
		return result, err
	}

	// Default mock behavior - simulate JQL returning 3 issues
	mockIssues := []string{"PROJ-1", "PROJ-2", "PROJ-3"}
	result := &BatchResult{
		TotalIssues:     len(mockIssues),
		ProcessedIssues: len(mockIssues),
		SuccessfulSync:  len(mockIssues),
		FailedSync:      0,
		ProcessedFiles:  make([]string, len(mockIssues)),
		Errors:          make([]BatchError, 0),
		Duration:        time.Millisecond * 150, // Simulate JQL + processing time
		Performance: PerformanceMetrics{
			IssuesPerSecond: float64(len(mockIssues)) / 0.15, // 150ms = 0.15s
			MemoryUsageKB:   int64(len(mockIssues)) * 1190,   // 1.19KB per issue from SPIKE-005
			WorkerCount:     5,                               // Default worker count
			AvgProcessTime:  time.Millisecond * 30,           // Average process time per issue
		},
	}

	// Generate mock file paths
	for i, issue := range mockIssues {
		result.ProcessedFiles[i] = "/mock/path/projects/PROJ/issues/" + issue + ".yaml"
	}

	call.Result = result
	m.SyncJQLCalls = append(m.SyncJQLCalls, call)
	return result, nil
}

// Reset clears all recorded calls
func (m *MockBatchSyncOrchestrator) Reset() {
	m.SyncIssuesCalls = make([]SyncIssuesCall, 0)
	m.SyncJQLCalls = make([]SyncJQLCall, 0)
}

// GetSyncIssuesCallCount returns the number of calls made to SyncIssues
func (m *MockBatchSyncOrchestrator) GetSyncIssuesCallCount() int {
	return len(m.SyncIssuesCalls)
}

// GetSyncJQLCallCount returns the number of calls made to SyncJQL
func (m *MockBatchSyncOrchestrator) GetSyncJQLCallCount() int {
	return len(m.SyncJQLCalls)
}

// GetLastSyncIssuesCall returns the last call made to SyncIssues
func (m *MockBatchSyncOrchestrator) GetLastSyncIssuesCall() *SyncIssuesCall {
	if len(m.SyncIssuesCalls) == 0 {
		return nil
	}
	return &m.SyncIssuesCalls[len(m.SyncIssuesCalls)-1]
}

// GetLastSyncJQLCall returns the last call made to SyncJQL
func (m *MockBatchSyncOrchestrator) GetLastSyncJQLCall() *SyncJQLCall {
	if len(m.SyncJQLCalls) == 0 {
		return nil
	}
	return &m.SyncJQLCalls[len(m.SyncJQLCalls)-1]
}
