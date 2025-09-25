package sync

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/chambrid/jira-cdc-git/pkg/client"
	"github.com/chambrid/jira-cdc-git/pkg/git"
	"github.com/chambrid/jira-cdc-git/pkg/links"
	"github.com/chambrid/jira-cdc-git/pkg/schema"
	"github.com/chambrid/jira-cdc-git/pkg/state"
)

// IncrementalBatchSyncEngine extends BatchSyncEngine with state management and incremental sync capabilities
type IncrementalBatchSyncEngine struct {
	*BatchSyncEngine
	stateManager state.StateManager
	state        *state.SyncState
}

// IncrementalSyncOptions contains options for incremental sync operations
type IncrementalSyncOptions struct {
	Force           bool          `json:"force"`
	DryRun          bool          `json:"dry_run"`
	Since           time.Time     `json:"since"`
	MaxAge          time.Duration `json:"max_age"`
	Projects        []string      `json:"projects"`
	IncludeNew      bool          `json:"include_new"`
	IncludeModified bool          `json:"include_modified"`
}

// NewIncrementalBatchSyncEngine creates a new incremental batch sync engine
func NewIncrementalBatchSyncEngine(
	client client.Client,
	fileWriter schema.FileWriter,
	gitRepo git.Repository,
	linkManager links.LinkManager,
	stateManager state.StateManager,
	concurrency int,
) *IncrementalBatchSyncEngine {

	batchEngine := NewBatchSyncEngine(client, fileWriter, gitRepo, linkManager, concurrency)

	return &IncrementalBatchSyncEngine{
		BatchSyncEngine: batchEngine,
		stateManager:    stateManager,
	}
}

// InitializeRepository initializes or loads the sync state for a repository
func (e *IncrementalBatchSyncEngine) InitializeRepository(repoPath string) error {
	// Try to load existing state
	existingState, err := e.stateManager.LoadState(repoPath)
	if err != nil {
		// State doesn't exist, create new one
		repoInfo := state.RepositoryInfo{
			Path:        repoPath,
			Branch:      "main", // TODO: Get actual branch from git
			InitialSync: true,
		}

		newState, initErr := e.stateManager.InitializeState(repoPath, repoInfo)
		if initErr != nil {
			return fmt.Errorf("failed to initialize state: %w", initErr)
		}
		e.state = newState
	} else {
		e.state = existingState
	}

	return nil
}

// SyncIssuesIncremental performs incremental sync for a list of issue keys
func (e *IncrementalBatchSyncEngine) SyncIssuesIncremental(
	ctx context.Context,
	issues []string,
	repoPath string,
	options IncrementalSyncOptions,
) (*BatchResult, error) {

	// Initialize repository state
	if err := e.InitializeRepository(repoPath); err != nil {
		return nil, fmt.Errorf("failed to initialize repository: %w", err)
	}

	// Create sync configuration
	syncConfig := state.SyncConfig{
		Concurrency:  e.concurrency,
		Incremental:  true,
		Force:        options.Force,
		DryRun:       options.DryRun,
		IncludeLinks: true,
	}

	// Start sync operation
	operation := e.stateManager.StartSyncOperation(e.state, state.SyncTypeIncremental, syncConfig)

	// Filter issues based on incremental options
	filteredIssues, err := e.filterIssuesForIncremental(ctx, issues, options)
	if err != nil {
		_ = e.stateManager.FailSyncOperation(e.state, operation, err)
		_ = e.stateManager.SaveState(repoPath, e.state)
		return nil, fmt.Errorf("failed to filter issues for incremental sync: %w", err)
	}

	operation.IssueKeys = filteredIssues

	// If no issues to sync, complete operation successfully
	if len(filteredIssues) == 0 {
		results := state.OperationResults{
			TotalIssues:     len(issues),
			ProcessedIssues: 0,
			SuccessfulSync:  0,
			FailedSync:      0,
			SkippedIssues:   len(issues),
			ProcessedFiles:  make([]string, 0),
			ErrorCount:      0,
		}

		_ = e.stateManager.CompleteSyncOperation(e.state, operation, results)
		_ = e.stateManager.SaveState(repoPath, e.state)

		return &BatchResult{
			TotalIssues:     len(issues),
			ProcessedIssues: 0,
			SuccessfulSync:  0,
			FailedSync:      0,
			ProcessedFiles:  make([]string, 0),
			Errors:          make([]BatchError, 0),
			Duration:        time.Since(operation.StartTime),
			Performance: PerformanceMetrics{
				IssuesPerSecond: 0,
				WorkerCount:     e.concurrency,
				AvgProcessTime:  0,
			},
		}, nil
	}

	// Perform the actual sync
	var result *BatchResult
	if options.DryRun {
		result = e.performDryRunSync(ctx, filteredIssues, repoPath)
	} else {
		result, err = e.performIncrementalSync(ctx, filteredIssues, repoPath)
		if err != nil {
			_ = e.stateManager.FailSyncOperation(e.state, operation, err)
			_ = e.stateManager.SaveState(repoPath, e.state)
			return nil, fmt.Errorf("incremental sync failed: %w", err)
		}
	}

	// Update operation results
	operationResults := state.OperationResults{
		TotalIssues:     len(issues),
		ProcessedIssues: result.ProcessedIssues,
		SuccessfulSync:  result.SuccessfulSync,
		FailedSync:      result.FailedSync,
		SkippedIssues:   len(issues) - len(filteredIssues),
		ProcessedFiles:  result.ProcessedFiles,
		ErrorCount:      len(result.Errors),
	}

	// Complete operation
	if result.FailedSync == 0 {
		_ = e.stateManager.CompleteSyncOperation(e.state, operation, operationResults)
	} else {
		_ = e.stateManager.FailSyncOperation(e.state, operation, fmt.Errorf("%d issues failed to sync", result.FailedSync))
	}

	// Save state
	if err := e.stateManager.SaveState(repoPath, e.state); err != nil {
		return result, fmt.Errorf("sync completed but failed to save state: %w", err)
	}

	return result, nil
}

// SyncJQLIncremental performs incremental sync for issues matching a JQL query
func (e *IncrementalBatchSyncEngine) SyncJQLIncremental(
	ctx context.Context,
	jql string,
	repoPath string,
	options IncrementalSyncOptions,
) (*BatchResult, error) {

	// Initialize repository state
	if err := e.InitializeRepository(repoPath); err != nil {
		return nil, fmt.Errorf("failed to initialize repository: %w", err)
	}

	// Create sync configuration
	syncConfig := state.SyncConfig{
		Concurrency:  e.concurrency,
		Incremental:  true,
		Force:        options.Force,
		DryRun:       options.DryRun,
		IncludeLinks: true,
	}

	// Start sync operation
	operation := e.stateManager.StartSyncOperation(e.state, state.SyncTypeJQL, syncConfig)
	operation.Query = jql

	// First, fetch all issues matching the JQL query
	jqlIssues, err := e.client.SearchIssues(jql)
	if err != nil {
		_ = e.stateManager.FailSyncOperation(e.state, operation, err)
		_ = e.stateManager.SaveState(repoPath, e.state)
		return nil, fmt.Errorf("failed to execute JQL search: %w", err)
	}

	// Extract issue keys
	issueKeys := make([]string, len(jqlIssues))
	for i, issue := range jqlIssues {
		issueKeys[i] = issue.Key
	}

	// Use incremental sync logic
	return e.SyncIssuesIncremental(ctx, issueKeys, repoPath, options)
}

// GetIncrementalSyncCandidates returns issues that should be synced based on state
func (e *IncrementalBatchSyncEngine) GetIncrementalSyncCandidates(options state.IncrementalSyncOptions) ([]string, error) {
	if e.state == nil {
		return nil, fmt.Errorf("state not initialized")
	}

	return e.stateManager.GetChangedIssues(e.state, options)
}

// ValidateRepositoryState validates the current repository state
func (e *IncrementalBatchSyncEngine) ValidateRepositoryState(repoPath string) (*state.StateValidationResult, error) {
	if e.state == nil {
		if err := e.InitializeRepository(repoPath); err != nil {
			return nil, fmt.Errorf("failed to initialize repository for validation: %w", err)
		}
	}

	return e.stateManager.ValidateState(e.state, repoPath)
}

// RecoverRepositoryState attempts to recover from state issues
func (e *IncrementalBatchSyncEngine) RecoverRepositoryState(repoPath string, options state.StateRecoveryOptions) (*state.StateValidationResult, error) {
	if e.state == nil {
		if err := e.InitializeRepository(repoPath); err != nil {
			return nil, fmt.Errorf("failed to initialize repository for recovery: %w", err)
		}
	}

	result, err := e.stateManager.RecoverState(e.state, repoPath, options)
	if err != nil {
		return result, err
	}

	// Save recovered state
	if !options.DryRun {
		if saveErr := e.stateManager.SaveState(repoPath, e.state); saveErr != nil {
			return result, fmt.Errorf("recovery completed but failed to save state: %w", saveErr)
		}
	}

	return result, nil
}

// GetSyncHistory returns the sync operation history
func (e *IncrementalBatchSyncEngine) GetSyncHistory(limit int) []state.SyncOperation {
	if e.state == nil {
		return make([]state.SyncOperation, 0)
	}

	return e.stateManager.GetHistoryReport(e.state, limit)
}

// GetSyncStatistics returns current sync statistics
func (e *IncrementalBatchSyncEngine) GetSyncStatistics() state.SyncStatistics {
	if e.state == nil {
		return state.SyncStatistics{}
	}

	return e.stateManager.GetSyncStatistics(e.state)
}

// GetLastSyncTime returns the timestamp of the last successful sync
func (e *IncrementalBatchSyncEngine) GetLastSyncTime() time.Time {
	if e.state == nil {
		return time.Time{}
	}

	return e.stateManager.GetLastSyncTime(e.state)
}

// Helper methods

// filterIssuesForIncremental filters issues based on incremental sync options
func (e *IncrementalBatchSyncEngine) filterIssuesForIncremental(
	ctx context.Context,
	issues []string,
	options IncrementalSyncOptions,
) ([]string, error) {

	if options.Force {
		return issues, nil
	}

	var filteredIssues []string

	for _, issueKey := range issues {
		// Check if issue should be synced based on state
		issueState, exists := e.stateManager.GetIssueState(e.state, issueKey)
		if !exists && options.IncludeNew {
			// New issue, include it
			filteredIssues = append(filteredIssues, issueKey)
			continue
		}

		if exists && options.IncludeModified {
			// Fetch current issue to check if it was updated
			issue, err := e.client.GetIssue(issueKey)
			if err != nil {
				// If we can't fetch the issue, include it to be safe
				filteredIssues = append(filteredIssues, issueKey)
				continue
			}

			if e.stateManager.ShouldSyncIssue(e.state, issue) {
				filteredIssues = append(filteredIssues, issueKey)
			}
		}

		// Check age constraints
		if exists && options.MaxAge > 0 {
			if time.Since(issueState.LastUpdated) > options.MaxAge {
				continue
			}
		}

		// Check since time
		if exists && !options.Since.IsZero() {
			if issueState.LastUpdated.Before(options.Since) {
				continue
			}
		}

		// Check project filter
		if len(options.Projects) > 0 && exists {
			projectMatch := false
			for _, project := range options.Projects {
				if issueState.ProjectKey == project {
					projectMatch = true
					break
				}
			}
			if !projectMatch {
				continue
			}
		}
	}

	return filteredIssues, nil
}

// performIncrementalSync performs the actual incremental sync
func (e *IncrementalBatchSyncEngine) performIncrementalSync(
	ctx context.Context,
	issues []string,
	repoPath string,
) (*BatchResult, error) {

	// Use the parent BatchSyncEngine for the actual sync
	//nolint:staticcheck // Explicit field access needed to avoid method overriding issues
	result, err := e.BatchSyncEngine.SyncIssues(ctx, issues, repoPath)
	if err != nil {
		return result, err
	}

	// Update state for successfully synced issues
	for _, filePath := range result.ProcessedFiles {
		// Extract issue key from file path
		issueKey := e.extractIssueKeyFromFilePath(filePath)
		if issueKey == "" {
			continue
		}

		// Get issue data
		issue, fetchErr := e.client.GetIssue(issueKey)
		if fetchErr != nil {
			continue
		}

		// Update issue state
		if updateErr := e.stateManager.UpdateIssueState(e.state, issue, filePath); updateErr != nil {
			// Log warning but don't fail the sync
			continue
		}
	}

	return result, nil
}

// performDryRunSync simulates sync without making changes
func (e *IncrementalBatchSyncEngine) performDryRunSync(
	ctx context.Context,
	issues []string,
	repoPath string,
) *BatchResult {

	startTime := time.Now()

	result := &BatchResult{
		TotalIssues:    len(issues),
		ProcessedFiles: make([]string, 0, len(issues)),
		Errors:         make([]BatchError, 0),
		Performance: PerformanceMetrics{
			WorkerCount: e.concurrency,
		},
	}

	// Simulate processing each issue
	for _, issueKey := range issues {
		select {
		case <-ctx.Done():
			result.FailedSync++
			result.Errors = append(result.Errors, BatchError{
				IssueKey: issueKey,
				Step:     "dry_run",
				Message:  "context cancelled",
			})
			continue
		default:
		}

		// Try to fetch issue to validate it exists
		_, err := e.client.GetIssue(issueKey)
		if err != nil {
			result.FailedSync++
			result.Errors = append(result.Errors, BatchError{
				IssueKey: issueKey,
				Step:     "fetch",
				Message:  err.Error(),
			})
		} else {
			result.SuccessfulSync++
			// Simulate file path
			projectKey := extractProjectKey(issueKey)
			filePath := fmt.Sprintf("%s/projects/%s/issues/%s.yaml", repoPath, projectKey, issueKey)
			result.ProcessedFiles = append(result.ProcessedFiles, filePath)
		}

		result.ProcessedIssues++
	}

	// Calculate performance metrics
	result.Duration = time.Since(startTime)
	if result.Duration > 0 {
		result.Performance.IssuesPerSecond = float64(result.ProcessedIssues) / result.Duration.Seconds()
	}
	if result.ProcessedIssues > 0 {
		result.Performance.AvgProcessTime = result.Duration / time.Duration(result.ProcessedIssues)
	}

	return result
}

// extractIssueKeyFromFilePath extracts issue key from a file path
func (e *IncrementalBatchSyncEngine) extractIssueKeyFromFilePath(filePath string) string {
	// File path format: {repo}/projects/{project-key}/issues/{issue-key}.yaml
	base := filepath.Base(filePath)
	if ext := filepath.Ext(base); ext == ".yaml" {
		return base[:len(base)-len(ext)]
	}
	return ""
}

// GetState returns the current sync state (for testing/debugging)
func (e *IncrementalBatchSyncEngine) GetState() *state.SyncState {
	return e.state
}

// SetState sets the sync state (for testing)
func (e *IncrementalBatchSyncEngine) SetState(s *state.SyncState) {
	e.state = s
}

// extractProjectKey extracts the project key from an issue key (e.g., "PROJ-123" -> "PROJ")
func extractProjectKey(issueKey string) string {
	// JIRA issue keys are in format: PROJECT-NUMBER
	for i, char := range issueKey {
		if char == '-' {
			return issueKey[:i]
		}
	}
	return issueKey // Fallback if no dash found
}
