package state

import (
	"time"

	"github.com/chambrid/jira-cdc-git/pkg/client"
)

// MockStateManager is a mock implementation of StateManager for testing
type MockStateManager struct {
	LoadStateFunc             func(repoPath string) (*SyncState, error)
	SaveStateFunc             func(repoPath string, state *SyncState) error
	InitializeStateFunc       func(repoPath string, repoInfo RepositoryInfo) (*SyncState, error)
	BackupStateFunc           func(repoPath string) error
	RestoreStateFunc          func(repoPath string) error
	StartSyncOperationFunc    func(state *SyncState, syncType SyncType, config SyncConfig) *SyncOperation
	CompleteSyncOperationFunc func(state *SyncState, operation *SyncOperation, results OperationResults) error
	FailSyncOperationFunc     func(state *SyncState, operation *SyncOperation, err error) error
	UpdateIssueStateFunc      func(state *SyncState, issue *client.Issue, filePath string) error
	GetIssueStateFunc         func(state *SyncState, issueKey string) (*IssueState, bool)
	RemoveIssueStateFunc      func(state *SyncState, issueKey string) error
	GetChangedIssuesFunc      func(state *SyncState, options IncrementalSyncOptions) ([]string, error)
	ShouldSyncIssueFunc       func(state *SyncState, issue *client.Issue) bool
	GetLastSyncTimeFunc       func(state *SyncState) time.Time
	ValidateStateFunc         func(state *SyncState, repoPath string) (*StateValidationResult, error)
	RecoverStateFunc          func(state *SyncState, repoPath string, options StateRecoveryOptions) (*StateValidationResult, error)
	GetSyncStatisticsFunc     func(state *SyncState) SyncStatistics
	UpdateStatisticsFunc      func(state *SyncState, operation *SyncOperation) error
	GetHistoryReportFunc      func(state *SyncState, limit int) []SyncOperation

	// Call tracking
	LoadStateCalls             []string
	SaveStateCalls             []SaveStateCall
	InitializeStateCalls       []InitializeStateCall
	BackupStateCalls           []string
	RestoreStateCalls          []string
	StartSyncOperationCalls    []StartSyncOperationCall
	CompleteSyncOperationCalls []CompleteSyncOperationCall
	FailSyncOperationCalls     []FailSyncOperationCall
	UpdateIssueStateCalls      []UpdateIssueStateCall
	GetIssueStateCalls         []GetIssueStateCall
	RemoveIssueStateCalls      []RemoveIssueStateCall
	GetChangedIssuesCalls      []GetChangedIssuesCall
	ShouldSyncIssueCalls       []ShouldSyncIssueCall
	GetLastSyncTimeCalls       []GetLastSyncTimeCall
	ValidateStateCalls         []ValidateStateCall
	RecoverStateCalls          []RecoverStateCall
	GetSyncStatisticsCalls     []GetSyncStatisticsCall
	UpdateStatisticsCalls      []UpdateStatisticsCall
	GetHistoryReportCalls      []GetHistoryReportCall

	// Mock state storage
	States map[string]*SyncState
}

// Call tracking types
type SaveStateCall struct {
	RepoPath string
	State    *SyncState
}

type InitializeStateCall struct {
	RepoPath string
	RepoInfo RepositoryInfo
}

type StartSyncOperationCall struct {
	State    *SyncState
	SyncType SyncType
	Config   SyncConfig
}

type CompleteSyncOperationCall struct {
	State     *SyncState
	Operation *SyncOperation
	Results   OperationResults
}

type FailSyncOperationCall struct {
	State     *SyncState
	Operation *SyncOperation
	Error     error
}

type UpdateIssueStateCall struct {
	State    *SyncState
	Issue    *client.Issue
	FilePath string
}

type GetIssueStateCall struct {
	State    *SyncState
	IssueKey string
}

type RemoveIssueStateCall struct {
	State    *SyncState
	IssueKey string
}

type GetChangedIssuesCall struct {
	State   *SyncState
	Options IncrementalSyncOptions
}

type ShouldSyncIssueCall struct {
	State *SyncState
	Issue *client.Issue
}

type GetLastSyncTimeCall struct {
	State *SyncState
}

type ValidateStateCall struct {
	State    *SyncState
	RepoPath string
}

type RecoverStateCall struct {
	State    *SyncState
	RepoPath string
	Options  StateRecoveryOptions
}

type GetSyncStatisticsCall struct {
	State *SyncState
}

type UpdateStatisticsCall struct {
	State     *SyncState
	Operation *SyncOperation
}

type GetHistoryReportCall struct {
	State *SyncState
	Limit int
}

// NewMockStateManager creates a new mock state manager
func NewMockStateManager() *MockStateManager {
	return &MockStateManager{
		States: make(map[string]*SyncState),
	}
}

// LoadState mock implementation
func (m *MockStateManager) LoadState(repoPath string) (*SyncState, error) {
	m.LoadStateCalls = append(m.LoadStateCalls, repoPath)

	if m.LoadStateFunc != nil {
		return m.LoadStateFunc(repoPath)
	}

	// Default behavior: return stored state or create new one
	if state, exists := m.States[repoPath]; exists {
		return state, nil
	}

	// Return a default state
	return &SyncState{
		Version: StateFileVersion,
		Repository: RepositoryInfo{
			Path: repoPath,
		},
		Issues:    make(map[string]IssueState),
		History:   make([]SyncOperation, 0),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}, nil
}

// SaveState mock implementation
func (m *MockStateManager) SaveState(repoPath string, state *SyncState) error {
	m.SaveStateCalls = append(m.SaveStateCalls, SaveStateCall{
		RepoPath: repoPath,
		State:    state,
	})

	if m.SaveStateFunc != nil {
		return m.SaveStateFunc(repoPath, state)
	}

	// Default behavior: store state
	m.States[repoPath] = state
	return nil
}

// InitializeState mock implementation
func (m *MockStateManager) InitializeState(repoPath string, repoInfo RepositoryInfo) (*SyncState, error) {
	m.InitializeStateCalls = append(m.InitializeStateCalls, InitializeStateCall{
		RepoPath: repoPath,
		RepoInfo: repoInfo,
	})

	if m.InitializeStateFunc != nil {
		return m.InitializeStateFunc(repoPath, repoInfo)
	}

	// Default behavior: create and store new state
	now := time.Now()
	state := &SyncState{
		Version:    StateFileVersion,
		Repository: repoInfo,
		Issues:     make(map[string]IssueState),
		History:    make([]SyncOperation, 0),
		Stats: SyncStatistics{
			ActiveProjects: make([]string, 0),
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	m.States[repoPath] = state
	return state, nil
}

// BackupState mock implementation
func (m *MockStateManager) BackupState(repoPath string) error {
	m.BackupStateCalls = append(m.BackupStateCalls, repoPath)

	if m.BackupStateFunc != nil {
		return m.BackupStateFunc(repoPath)
	}

	return nil
}

// RestoreState mock implementation
func (m *MockStateManager) RestoreState(repoPath string) error {
	m.RestoreStateCalls = append(m.RestoreStateCalls, repoPath)

	if m.RestoreStateFunc != nil {
		return m.RestoreStateFunc(repoPath)
	}

	return nil
}

// StartSyncOperation mock implementation
func (m *MockStateManager) StartSyncOperation(state *SyncState, syncType SyncType, config SyncConfig) *SyncOperation {
	m.StartSyncOperationCalls = append(m.StartSyncOperationCalls, StartSyncOperationCall{
		State:    state,
		SyncType: syncType,
		Config:   config,
	})

	if m.StartSyncOperationFunc != nil {
		return m.StartSyncOperationFunc(state, syncType, config)
	}

	// Default behavior
	now := time.Now()
	operation := &SyncOperation{
		ID:        "mock-operation",
		Type:      syncType,
		StartTime: now,
		Status:    SyncStatusRunning,
		Config:    config,
		Results:   OperationResults{},
		Metadata:  make(map[string]string),
	}

	state.LastSync = operation
	return operation
}

// CompleteSyncOperation mock implementation
func (m *MockStateManager) CompleteSyncOperation(state *SyncState, operation *SyncOperation, results OperationResults) error {
	m.CompleteSyncOperationCalls = append(m.CompleteSyncOperationCalls, CompleteSyncOperationCall{
		State:     state,
		Operation: operation,
		Results:   results,
	})

	if m.CompleteSyncOperationFunc != nil {
		return m.CompleteSyncOperationFunc(state, operation, results)
	}

	// Default behavior
	now := time.Now()
	operation.EndTime = now
	operation.Duration = now.Sub(operation.StartTime)
	operation.Status = SyncStatusCompleted
	operation.Results = results

	state.History = append(state.History, *operation)
	return nil
}

// FailSyncOperation mock implementation
func (m *MockStateManager) FailSyncOperation(state *SyncState, operation *SyncOperation, err error) error {
	m.FailSyncOperationCalls = append(m.FailSyncOperationCalls, FailSyncOperationCall{
		State:     state,
		Operation: operation,
		Error:     err,
	})

	if m.FailSyncOperationFunc != nil {
		return m.FailSyncOperationFunc(state, operation, err)
	}

	// Default behavior
	now := time.Now()
	operation.EndTime = now
	operation.Duration = now.Sub(operation.StartTime)
	operation.Status = SyncStatusFailed
	operation.Error = err.Error()

	state.History = append(state.History, *operation)
	return nil
}

// UpdateIssueState mock implementation
func (m *MockStateManager) UpdateIssueState(state *SyncState, issue *client.Issue, filePath string) error {
	m.UpdateIssueStateCalls = append(m.UpdateIssueStateCalls, UpdateIssueStateCall{
		State:    state,
		Issue:    issue,
		FilePath: filePath,
	})

	if m.UpdateIssueStateFunc != nil {
		return m.UpdateIssueStateFunc(state, issue, filePath)
	}

	// Default behavior
	now := time.Now()
	issueState := IssueState{
		Key:          issue.Key,
		ProjectKey:   extractProjectKey(issue.Key),
		LastSynced:   now,
		LastModified: now,
		LastUpdated:  now,
		Version:      1,
		FilePath:     filePath,
		SyncStatus:   "success",
		SyncCount:    1,
	}

	if state.Issues == nil {
		state.Issues = make(map[string]IssueState)
	}
	state.Issues[issue.Key] = issueState

	return nil
}

// GetIssueState mock implementation
func (m *MockStateManager) GetIssueState(state *SyncState, issueKey string) (*IssueState, bool) {
	m.GetIssueStateCalls = append(m.GetIssueStateCalls, GetIssueStateCall{
		State:    state,
		IssueKey: issueKey,
	})

	if m.GetIssueStateFunc != nil {
		return m.GetIssueStateFunc(state, issueKey)
	}

	// Default behavior
	if issueState, exists := state.Issues[issueKey]; exists {
		return &issueState, true
	}
	return nil, false
}

// RemoveIssueState mock implementation
func (m *MockStateManager) RemoveIssueState(state *SyncState, issueKey string) error {
	m.RemoveIssueStateCalls = append(m.RemoveIssueStateCalls, RemoveIssueStateCall{
		State:    state,
		IssueKey: issueKey,
	})

	if m.RemoveIssueStateFunc != nil {
		return m.RemoveIssueStateFunc(state, issueKey)
	}

	// Default behavior
	delete(state.Issues, issueKey)
	return nil
}

// GetChangedIssues mock implementation
func (m *MockStateManager) GetChangedIssues(state *SyncState, options IncrementalSyncOptions) ([]string, error) {
	m.GetChangedIssuesCalls = append(m.GetChangedIssuesCalls, GetChangedIssuesCall{
		State:   state,
		Options: options,
	})

	if m.GetChangedIssuesFunc != nil {
		return m.GetChangedIssuesFunc(state, options)
	}

	// Default behavior: return all tracked issues
	var issues []string
	for issueKey := range state.Issues {
		issues = append(issues, issueKey)
	}
	return issues, nil
}

// ShouldSyncIssue mock implementation
func (m *MockStateManager) ShouldSyncIssue(state *SyncState, issue *client.Issue) bool {
	m.ShouldSyncIssueCalls = append(m.ShouldSyncIssueCalls, ShouldSyncIssueCall{
		State: state,
		Issue: issue,
	})

	if m.ShouldSyncIssueFunc != nil {
		return m.ShouldSyncIssueFunc(state, issue)
	}

	// Default behavior: always sync
	return true
}

// GetLastSyncTime mock implementation
func (m *MockStateManager) GetLastSyncTime(state *SyncState) time.Time {
	m.GetLastSyncTimeCalls = append(m.GetLastSyncTimeCalls, GetLastSyncTimeCall{
		State: state,
	})

	if m.GetLastSyncTimeFunc != nil {
		return m.GetLastSyncTimeFunc(state)
	}

	// Default behavior
	if state.LastSync != nil && state.LastSync.Status == SyncStatusCompleted {
		return state.LastSync.EndTime
	}
	return time.Time{}
}

// ValidateState mock implementation
func (m *MockStateManager) ValidateState(state *SyncState, repoPath string) (*StateValidationResult, error) {
	m.ValidateStateCalls = append(m.ValidateStateCalls, ValidateStateCall{
		State:    state,
		RepoPath: repoPath,
	})

	if m.ValidateStateFunc != nil {
		return m.ValidateStateFunc(state, repoPath)
	}

	// Default behavior: return valid state
	return &StateValidationResult{
		Valid:              true,
		Errors:             make([]string, 0),
		Warnings:           make([]string, 0),
		MissingIssues:      make([]string, 0),
		OrphanedFiles:      make([]string, 0),
		CorruptedFiles:     make([]string, 0),
		RecommendedActions: make([]string, 0),
	}, nil
}

// RecoverState mock implementation
func (m *MockStateManager) RecoverState(state *SyncState, repoPath string, options StateRecoveryOptions) (*StateValidationResult, error) {
	m.RecoverStateCalls = append(m.RecoverStateCalls, RecoverStateCall{
		State:    state,
		RepoPath: repoPath,
		Options:  options,
	})

	if m.RecoverStateFunc != nil {
		return m.RecoverStateFunc(state, repoPath, options)
	}

	// Default behavior: return valid state
	return &StateValidationResult{
		Valid:              true,
		Errors:             make([]string, 0),
		Warnings:           make([]string, 0),
		MissingIssues:      make([]string, 0),
		OrphanedFiles:      make([]string, 0),
		CorruptedFiles:     make([]string, 0),
		RecommendedActions: make([]string, 0),
	}, nil
}

// GetSyncStatistics mock implementation
func (m *MockStateManager) GetSyncStatistics(state *SyncState) SyncStatistics {
	m.GetSyncStatisticsCalls = append(m.GetSyncStatisticsCalls, GetSyncStatisticsCall{
		State: state,
	})

	if m.GetSyncStatisticsFunc != nil {
		return m.GetSyncStatisticsFunc(state)
	}

	// Default behavior
	return state.Stats
}

// UpdateStatistics mock implementation
func (m *MockStateManager) UpdateStatistics(state *SyncState, operation *SyncOperation) error {
	m.UpdateStatisticsCalls = append(m.UpdateStatisticsCalls, UpdateStatisticsCall{
		State:     state,
		Operation: operation,
	})

	if m.UpdateStatisticsFunc != nil {
		return m.UpdateStatisticsFunc(state, operation)
	}

	// Default behavior: simple stats update
	stats := &state.Stats
	stats.TotalOperations++
	switch operation.Status {
	case SyncStatusCompleted:
		stats.SuccessfulOps++
		stats.LastSuccessfulOp = operation.EndTime
	case SyncStatusFailed:
		stats.FailedOps++
		stats.LastFailedOp = operation.EndTime
	}

	return nil
}

// GetHistoryReport mock implementation
func (m *MockStateManager) GetHistoryReport(state *SyncState, limit int) []SyncOperation {
	m.GetHistoryReportCalls = append(m.GetHistoryReportCalls, GetHistoryReportCall{
		State: state,
		Limit: limit,
	})

	if m.GetHistoryReportFunc != nil {
		return m.GetHistoryReportFunc(state, limit)
	}

	// Default behavior
	if limit <= 0 || limit > len(state.History) {
		limit = len(state.History)
	}

	if len(state.History) == 0 {
		return make([]SyncOperation, 0)
	}

	start := len(state.History) - limit
	return state.History[start:]
}
