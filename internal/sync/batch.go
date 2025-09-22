package sync

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/chambrid/jira-cdc-git/pkg/client"
	"github.com/chambrid/jira-cdc-git/pkg/git"
	"github.com/chambrid/jira-cdc-git/pkg/schema"
)

// BatchSyncOrchestrator defines the interface for batch sync operations
// Based on SPIKE-005 findings: 5 workers optimal, 15,023 issues/sec peak throughput
type BatchSyncOrchestrator interface {
	SyncIssues(ctx context.Context, issues []string, repoPath string) (*BatchResult, error)
	SyncJQL(ctx context.Context, jql string, repoPath string) (*BatchResult, error)
}

// BatchSyncEngine implements the BatchSyncOrchestrator interface
// Provides parallel processing with configurable concurrency (2-5 workers recommended)
type BatchSyncEngine struct {
	client       client.Client
	fileWriter   schema.FileWriter
	gitRepo      git.Repository
	concurrency  int
	progressChan chan ProgressUpdate
}

// BatchResult contains the results of a batch sync operation
type BatchResult struct {
	TotalIssues     int                `json:"total_issues"`
	ProcessedIssues int                `json:"processed_issues"`
	SuccessfulSync  int                `json:"successful_sync"`
	FailedSync      int                `json:"failed_sync"`
	ProcessedFiles  []string           `json:"processed_files"`
	Errors          []BatchError       `json:"errors"`
	Duration        time.Duration      `json:"duration"`
	Performance     PerformanceMetrics `json:"performance"`
}

// BatchError represents an error that occurred during batch processing
type BatchError struct {
	IssueKey string `json:"issue_key"`
	Step     string `json:"step"`
	Message  string `json:"message"`
	Error    error  `json:"-"`
}

// PerformanceMetrics contains performance statistics for batch operations
// Based on SPIKE-005 performance validation
type PerformanceMetrics struct {
	IssuesPerSecond float64       `json:"issues_per_second"`
	MemoryUsageKB   int64         `json:"memory_usage_kb"`
	WorkerCount     int           `json:"worker_count"`
	AvgProcessTime  time.Duration `json:"avg_process_time"`
}

// ProgressUpdate represents progress information for batch operations
type ProgressUpdate struct {
	CurrentIssue   string    `json:"current_issue"`
	ProcessedCount int       `json:"processed_count"`
	TotalCount     int       `json:"total_count"`
	Percentage     float64   `json:"percentage"`
	Step           string    `json:"step"`
	Timestamp      time.Time `json:"timestamp"`
	WorkerID       int       `json:"worker_id"`
}

// SyncTask represents a single issue sync task for worker processing
type SyncTask struct {
	IssueKey string
	Index    int
}

// SyncResult represents the result of a single issue sync operation
type SyncResult struct {
	IssueKey    string
	Index       int
	FilePath    string
	Error       error
	ProcessTime time.Duration
}

// NewBatchSyncEngine creates a new batch sync engine with configurable concurrency
// concurrency: number of parallel workers (recommended 2-5 based on SPIKE-005)
func NewBatchSyncEngine(client client.Client, fileWriter schema.FileWriter, gitRepo git.Repository, concurrency int) *BatchSyncEngine {
	// Validate concurrency based on SPIKE-005 findings
	if concurrency < 1 {
		concurrency = 1
	}
	if concurrency > 10 {
		concurrency = 10 // Cap at 10 to prevent resource exhaustion
	}

	return &BatchSyncEngine{
		client:       client,
		fileWriter:   fileWriter,
		gitRepo:      gitRepo,
		concurrency:  concurrency,
		progressChan: make(chan ProgressUpdate, concurrency*2), // Buffered to prevent blocking
	}
}

// SyncIssuesSync performs batch sync for a list of issue keys WITHOUT concurrency (for testing)
func (b *BatchSyncEngine) SyncIssuesSync(ctx context.Context, issues []string, repoPath string) (*BatchResult, error) {
	startTime := time.Now()

	result := &BatchResult{
		TotalIssues:    len(issues),
		ProcessedFiles: make([]string, 0, len(issues)),
		Errors:         make([]BatchError, 0),
		Performance: PerformanceMetrics{
			WorkerCount: 1, // Always 1 for sync mode
		},
	}

	// Process each issue sequentially
	var totalProcessTime time.Duration
	for _, issueKey := range issues {
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		default:
		}

		startTime := time.Now()
		filePath, err := b.processSingleIssue(ctx, issueKey, repoPath, 0)
		processTime := time.Since(startTime)

		result.ProcessedIssues++
		totalProcessTime += processTime

		if err != nil {
			result.FailedSync++
			result.Errors = append(result.Errors, BatchError{
				IssueKey: issueKey,
				Step:     "sync",
				Message:  err.Error(),
				Error:    err,
			})
		} else {
			result.SuccessfulSync++
			result.ProcessedFiles = append(result.ProcessedFiles, filePath)
		}

		// Send progress update (non-blocking)
		select {
		case b.progressChan <- ProgressUpdate{
			CurrentIssue:   issueKey,
			ProcessedCount: result.ProcessedIssues,
			TotalCount:     result.TotalIssues,
			Percentage:     float64(result.ProcessedIssues) / float64(result.TotalIssues) * 100,
			Step:           "processing",
			Timestamp:      time.Now(),
		}:
		default:
			// Non-blocking - skip if channel is full
		}
	}

	// Calculate performance metrics
	result.Duration = time.Since(startTime)
	if result.Duration > 0 {
		result.Performance.IssuesPerSecond = float64(result.ProcessedIssues) / result.Duration.Seconds()
	}
	if result.ProcessedIssues > 0 {
		result.Performance.AvgProcessTime = totalProcessTime / time.Duration(result.ProcessedIssues)
	}

	return result, nil
}

// SyncIssues performs batch sync for a list of issue keys with parallel processing
func (b *BatchSyncEngine) SyncIssues(ctx context.Context, issues []string, repoPath string) (*BatchResult, error) {
	startTime := time.Now()

	result := &BatchResult{
		TotalIssues:    len(issues),
		ProcessedFiles: make([]string, 0, len(issues)),
		Errors:         make([]BatchError, 0),
		Performance: PerformanceMetrics{
			WorkerCount: b.concurrency,
		},
	}

	// Create task channel and result channel
	taskChan := make(chan SyncTask, len(issues))
	resultChan := make(chan SyncResult, len(issues))

	// Start worker goroutines
	var wg sync.WaitGroup
	for i := 0; i < b.concurrency; i++ {
		wg.Add(1)
		go b.worker(ctx, i, taskChan, resultChan, repoPath, &wg)
	}

	// Send tasks to workers
	go func() {
		defer close(taskChan)
		for i, issueKey := range issues {
			select {
			case taskChan <- SyncTask{IssueKey: issueKey, Index: i}:
			case <-ctx.Done():
				return
			}
		}
	}()

	// Collect results
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Process results as they come in
	var totalProcessTime time.Duration
	for syncResult := range resultChan {
		result.ProcessedIssues++
		totalProcessTime += syncResult.ProcessTime

		if syncResult.Error != nil {
			result.FailedSync++
			result.Errors = append(result.Errors, BatchError{
				IssueKey: syncResult.IssueKey,
				Step:     "sync",
				Message:  syncResult.Error.Error(),
				Error:    syncResult.Error,
			})
		} else {
			result.SuccessfulSync++
			result.ProcessedFiles = append(result.ProcessedFiles, syncResult.FilePath)
		}

		// Send progress update
		select {
		case b.progressChan <- ProgressUpdate{
			CurrentIssue:   syncResult.IssueKey,
			ProcessedCount: result.ProcessedIssues,
			TotalCount:     result.TotalIssues,
			Percentage:     float64(result.ProcessedIssues) / float64(result.TotalIssues) * 100,
			Step:           "processing",
			Timestamp:      time.Now(),
		}:
		default:
			// Non-blocking send - skip if channel is full
		}
	}

	// Calculate performance metrics
	result.Duration = time.Since(startTime)
	if result.Duration > 0 {
		result.Performance.IssuesPerSecond = float64(result.ProcessedIssues) / result.Duration.Seconds()
	}
	if result.ProcessedIssues > 0 {
		result.Performance.AvgProcessTime = totalProcessTime / time.Duration(result.ProcessedIssues)
	}

	return result, nil
}

// SyncJQL performs batch sync for issues matching a JQL query
func (b *BatchSyncEngine) SyncJQL(ctx context.Context, jql string, repoPath string) (*BatchResult, error) {
	// First, fetch all issues matching the JQL query
	issues, err := b.client.SearchIssues(jql)
	if err != nil {
		return nil, fmt.Errorf("failed to execute JQL search: %w", err)
	}

	// Extract issue keys
	issueKeys := make([]string, len(issues))
	for i, issue := range issues {
		issueKeys[i] = issue.Key
	}

	// Use SyncIssues to process the results
	return b.SyncIssues(ctx, issueKeys, repoPath)
}

// SyncJQLSync performs batch sync for issues matching a JQL query WITHOUT concurrency (for testing)
func (b *BatchSyncEngine) SyncJQLSync(ctx context.Context, jql string, repoPath string) (*BatchResult, error) {
	// First, fetch all issues matching the JQL query
	issues, err := b.client.SearchIssues(jql)
	if err != nil {
		return nil, fmt.Errorf("failed to execute JQL search: %w", err)
	}

	// Extract issue keys
	issueKeys := make([]string, len(issues))
	for i, issue := range issues {
		issueKeys[i] = issue.Key
	}

	// Use SyncIssuesSync to process the results
	return b.SyncIssuesSync(ctx, issueKeys, repoPath)
}

// GetProgressChannel returns a channel for receiving progress updates
func (b *BatchSyncEngine) GetProgressChannel() <-chan ProgressUpdate {
	return b.progressChan
}

// worker processes sync tasks from the task channel
func (b *BatchSyncEngine) worker(ctx context.Context, workerID int, tasks <-chan SyncTask, results chan<- SyncResult, repoPath string, wg *sync.WaitGroup) {
	defer wg.Done()

	for {
		select {
		case task, ok := <-tasks:
			if !ok {
				return // Channel closed, worker done
			}

			startTime := time.Now()
			filePath, err := b.processSingleIssue(ctx, task.IssueKey, repoPath, workerID)
			processTime := time.Since(startTime)

			result := SyncResult{
				IssueKey:    task.IssueKey,
				Index:       task.Index,
				FilePath:    filePath,
				Error:       err,
				ProcessTime: processTime,
			}

			select {
			case results <- result:
			case <-ctx.Done():
				return
			}

		case <-ctx.Done():
			return
		}
	}
}

// processSingleIssue handles the sync of a single issue (fetch, write, commit)
func (b *BatchSyncEngine) processSingleIssue(ctx context.Context, issueKey, repoPath string, workerID int) (string, error) {
	// Send progress update for fetch step
	select {
	case b.progressChan <- ProgressUpdate{
		CurrentIssue: issueKey,
		Step:         "fetching",
		Timestamp:    time.Now(),
		WorkerID:     workerID,
	}:
	default:
	}

	// Fetch issue data
	issueData, err := b.client.GetIssue(issueKey)
	if err != nil {
		return "", fmt.Errorf("failed to fetch issue %s: %w", issueKey, err)
	}

	// Send progress update for write step
	select {
	case b.progressChan <- ProgressUpdate{
		CurrentIssue: issueKey,
		Step:         "writing",
		Timestamp:    time.Now(),
		WorkerID:     workerID,
	}:
	default:
	}

	// Write YAML file
	yamlFilePath, err := b.fileWriter.WriteIssueToYAML(issueData, repoPath)
	if err != nil {
		return "", fmt.Errorf("failed to write YAML for issue %s: %w", issueKey, err)
	}

	// Send progress update for commit step
	select {
	case b.progressChan <- ProgressUpdate{
		CurrentIssue: issueKey,
		Step:         "committing",
		Timestamp:    time.Now(),
		WorkerID:     workerID,
	}:
	default:
	}

	// Commit to Git
	if err := b.gitRepo.CommitIssueFile(repoPath, yamlFilePath, issueData); err != nil {
		return yamlFilePath, fmt.Errorf("failed to commit issue %s: %w", issueKey, err)
	}

	return yamlFilePath, nil
}
