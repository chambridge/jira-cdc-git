package jobs

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/chambrid/jira-cdc-git/internal/sync"
	"github.com/chambrid/jira-cdc-git/pkg/client"
	"github.com/chambrid/jira-cdc-git/pkg/config"
	"github.com/chambrid/jira-cdc-git/pkg/git"
	"github.com/chambrid/jira-cdc-git/pkg/links"
	"github.com/chambrid/jira-cdc-git/pkg/schema"
	"github.com/chambrid/jira-cdc-git/pkg/state"
)

// SyncJobOrchestrator coordinates between job scheduling and sync operations
type SyncJobOrchestrator struct {
	scheduler    JobScheduler
	idGenerator  JobIDGenerator
	configLoader config.Provider
}

// NewSyncJobOrchestrator creates a new sync job orchestrator
func NewSyncJobOrchestrator(scheduler JobScheduler) *SyncJobOrchestrator {
	return &SyncJobOrchestrator{
		scheduler:    scheduler,
		idGenerator:  NewJobIDGenerator(),
		configLoader: config.NewDotEnvLoader(),
	}
}

// SubmitSingleIssueSync submits a job for syncing a single JIRA issue
func (o *SyncJobOrchestrator) SubmitSingleIssueSync(ctx context.Context, req *SingleIssueSyncRequest) (*JobResult, error) {
	// Validate request
	if err := o.validateSingleIssueRequest(req); err != nil {
		return nil, NewValidationError("", "request", req, err.Error())
	}

	// Generate job ID
	jobID := o.idGenerator.GenerateWithType(JobTypeSingle)

	// Create job configuration
	config := &SyncJobConfig{
		ID:          jobID,
		Type:        JobTypeSingle,
		Name:        fmt.Sprintf("Single Issue Sync: %s", req.IssueKey),
		Created:     time.Now(),
		Target:      req.IssueKey,
		Repository:  req.Repository,
		Concurrency: 1, // Single issue sync uses 1 worker
		RateLimit:   req.RateLimit,
		Incremental: req.Incremental,
		Force:       req.Force,
		DryRun:      req.DryRun,
		SafeMode:    req.SafeMode,
		Namespace:   req.Namespace,
		Image:       req.Image,
		Resources:   req.Resources,
		TimeoutSec:  req.TimeoutSec,
	}

	// Submit job
	return o.scheduler.CreateJob(ctx, config)
}

// SubmitBatchSync submits a job for syncing multiple JIRA issues
func (o *SyncJobOrchestrator) SubmitBatchSync(ctx context.Context, req *BatchSyncRequest) (*JobResult, error) {
	// Validate request
	if err := o.validateBatchRequest(req); err != nil {
		return nil, NewValidationError("", "request", req, err.Error())
	}

	// Generate job ID
	jobID := o.idGenerator.GenerateWithType(JobTypeBatch)

	// Create job configuration
	config := &SyncJobConfig{
		ID:          jobID,
		Type:        JobTypeBatch,
		Name:        fmt.Sprintf("Batch Sync: %d issues", len(req.IssueKeys)),
		Created:     time.Now(),
		Target:      strings.Join(req.IssueKeys, ","),
		Repository:  req.Repository,
		BatchSize:   req.BatchSize,
		Concurrency: req.Concurrency,
		RateLimit:   req.RateLimit,
		Incremental: req.Incremental,
		Force:       req.Force,
		DryRun:      req.DryRun,
		SafeMode:    req.SafeMode,
		Namespace:   req.Namespace,
		Image:       req.Image,
		Resources:   req.Resources,
		Parallelism: req.Parallelism,
		Completions: req.Completions,
		TimeoutSec:  req.TimeoutSec,
	}

	// Submit job
	return o.scheduler.CreateJob(ctx, config)
}

// SubmitJQLSync submits a job for syncing issues matching a JQL query
func (o *SyncJobOrchestrator) SubmitJQLSync(ctx context.Context, req *JQLSyncRequest) (*JobResult, error) {
	// Validate request
	if err := o.validateJQLRequest(req); err != nil {
		return nil, NewValidationError("", "request", req, err.Error())
	}

	// Generate job ID
	jobID := o.idGenerator.GenerateWithType(JobTypeJQL)

	// Create job configuration
	config := &SyncJobConfig{
		ID:          jobID,
		Type:        JobTypeJQL,
		Name:        fmt.Sprintf("JQL Sync: %s", req.JQL),
		Created:     time.Now(),
		Target:      req.JQL,
		Repository:  req.Repository,
		BatchSize:   req.BatchSize,
		Concurrency: req.Concurrency,
		RateLimit:   req.RateLimit,
		Incremental: req.Incremental,
		Force:       req.Force,
		DryRun:      req.DryRun,
		SafeMode:    req.SafeMode,
		Namespace:   req.Namespace,
		Image:       req.Image,
		Resources:   req.Resources,
		Parallelism: req.Parallelism,
		Completions: req.Completions,
		TimeoutSec:  req.TimeoutSec,
	}

	// Submit job
	return o.scheduler.CreateJob(ctx, config)
}

// ExecuteLocalSync executes sync operation locally (non-Kubernetes)
func (o *SyncJobOrchestrator) ExecuteLocalSync(ctx context.Context, req *LocalSyncRequest) (*sync.BatchResult, error) {
	// Load configuration
	cfg, err := o.configLoader.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	// Apply rate limit override if provided
	if req.RateLimit > 0 {
		cfg.RateLimitDelay = req.RateLimit
	}

	// Initialize JIRA client
	jiraClient, err := client.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create JIRA client: %w", err)
	}

	if err := jiraClient.Authenticate(); err != nil {
		return nil, fmt.Errorf("failed to authenticate with JIRA: %w", err)
	}

	// Initialize Git repository
	gitRepo := git.NewGitRepository("JIRA CDC Git Sync", "jira-sync@automated.local")
	if err := gitRepo.Initialize(req.Repository); err != nil {
		return nil, fmt.Errorf("failed to initialize Git repository: %w", err)
	}

	if err := gitRepo.ValidateWorkingTree(req.Repository); err != nil {
		return nil, fmt.Errorf("git repository validation failed: %w", err)
	}

	// Initialize sync components
	fileWriter := schema.NewYAMLFileWriter()
	linkManager := links.NewSymbolicLinkManager()

	// Execute sync based on request type
	var result *sync.BatchResult

	if req.Incremental || req.Force || req.DryRun {
		// Use incremental engine
		stateManager := state.NewFileStateManager(state.FormatYAML)
		incrementalEngine := sync.NewIncrementalBatchSyncEngine(
			jiraClient, fileWriter, gitRepo, linkManager, stateManager, req.Concurrency)

		incrementalOptions := sync.IncrementalSyncOptions{
			Force:           req.Force,
			DryRun:          req.DryRun,
			IncludeNew:      true,
			IncludeModified: true,
		}

		if req.JQL != "" {
			result, err = incrementalEngine.SyncJQLIncremental(ctx, req.JQL, req.Repository, incrementalOptions)
		} else {
			result, err = incrementalEngine.SyncIssuesIncremental(ctx, req.IssueKeys, req.Repository, incrementalOptions)
		}
	} else {
		// Use regular batch engine
		batchEngine := sync.NewBatchSyncEngine(jiraClient, fileWriter, gitRepo, linkManager, req.Concurrency)

		if req.JQL != "" {
			result, err = batchEngine.SyncJQL(ctx, req.JQL, req.Repository)
		} else {
			result, err = batchEngine.SyncIssues(ctx, req.IssueKeys, req.Repository)
		}
	}

	if err != nil {
		return nil, fmt.Errorf("sync operation failed: %w", err)
	}

	return result, nil
}

// GetJob retrieves job status and results
func (o *SyncJobOrchestrator) GetJob(ctx context.Context, jobID string) (*JobResult, error) {
	if err := o.idGenerator.Validate(jobID); err != nil {
		return nil, NewValidationError(jobID, "job_id", jobID, err.Error())
	}

	return o.scheduler.GetJob(ctx, jobID)
}

// ListJobs lists jobs with optional filtering
func (o *SyncJobOrchestrator) ListJobs(ctx context.Context, filters *JobFilter) ([]*JobResult, error) {
	return o.scheduler.ListJobs(ctx, filters)
}

// CancelJob cancels a running job
func (o *SyncJobOrchestrator) CancelJob(ctx context.Context, jobID string) error {
	if err := o.idGenerator.Validate(jobID); err != nil {
		return NewValidationError(jobID, "job_id", jobID, err.Error())
	}

	return o.scheduler.CancelJob(ctx, jobID)
}

// DeleteJob deletes a job and its resources
func (o *SyncJobOrchestrator) DeleteJob(ctx context.Context, jobID string) error {
	if err := o.idGenerator.Validate(jobID); err != nil {
		return NewValidationError(jobID, "job_id", jobID, err.Error())
	}

	return o.scheduler.DeleteJob(ctx, jobID)
}

// WatchJob provides real-time monitoring of job status
func (o *SyncJobOrchestrator) WatchJob(ctx context.Context, jobID string) (<-chan JobMonitor, error) {
	if err := o.idGenerator.Validate(jobID); err != nil {
		return nil, NewValidationError(jobID, "job_id", jobID, err.Error())
	}

	return o.scheduler.WatchJob(ctx, jobID)
}

// GetJobLogs retrieves logs from a job
func (o *SyncJobOrchestrator) GetJobLogs(ctx context.Context, jobID string) (string, error) {
	if err := o.idGenerator.Validate(jobID); err != nil {
		return "", NewValidationError(jobID, "job_id", jobID, err.Error())
	}

	return o.scheduler.GetJobLogs(ctx, jobID)
}

// GetQueueStatus returns job queue information
func (o *SyncJobOrchestrator) GetQueueStatus(ctx context.Context) (*QueueStatus, error) {
	return o.scheduler.GetQueueStatus(ctx)
}

// Request types

// SingleIssueSyncRequest represents a request to sync a single JIRA issue
type SingleIssueSyncRequest struct {
	IssueKey    string                   `json:"issue_key"`
	Repository  string                   `json:"repository"`
	RateLimit   time.Duration            `json:"rate_limit,omitempty"`
	Incremental bool                     `json:"incremental,omitempty"`
	Force       bool                     `json:"force,omitempty"`
	DryRun      bool                     `json:"dry_run,omitempty"`
	SafeMode    bool                     `json:"safe_mode,omitempty"`
	Namespace   string                   `json:"namespace,omitempty"`
	Image       string                   `json:"image,omitempty"`
	Resources   *JobResourceRequirements `json:"resources,omitempty"`
	TimeoutSec  *int64                   `json:"timeout_sec,omitempty"`
}

// BatchSyncRequest represents a request to sync multiple JIRA issues
type BatchSyncRequest struct {
	IssueKeys   []string                 `json:"issue_keys"`
	Repository  string                   `json:"repository"`
	BatchSize   int                      `json:"batch_size,omitempty"`
	Concurrency int                      `json:"concurrency,omitempty"`
	RateLimit   time.Duration            `json:"rate_limit,omitempty"`
	Incremental bool                     `json:"incremental,omitempty"`
	Force       bool                     `json:"force,omitempty"`
	DryRun      bool                     `json:"dry_run,omitempty"`
	SafeMode    bool                     `json:"safe_mode,omitempty"`
	Namespace   string                   `json:"namespace,omitempty"`
	Image       string                   `json:"image,omitempty"`
	Resources   *JobResourceRequirements `json:"resources,omitempty"`
	Parallelism *int32                   `json:"parallelism,omitempty"`
	Completions *int32                   `json:"completions,omitempty"`
	TimeoutSec  *int64                   `json:"timeout_sec,omitempty"`
}

// JQLSyncRequest represents a request to sync issues matching a JQL query
type JQLSyncRequest struct {
	JQL         string                   `json:"jql"`
	Repository  string                   `json:"repository"`
	BatchSize   int                      `json:"batch_size,omitempty"`
	Concurrency int                      `json:"concurrency,omitempty"`
	RateLimit   time.Duration            `json:"rate_limit,omitempty"`
	Incremental bool                     `json:"incremental,omitempty"`
	Force       bool                     `json:"force,omitempty"`
	DryRun      bool                     `json:"dry_run,omitempty"`
	SafeMode    bool                     `json:"safe_mode,omitempty"`
	Namespace   string                   `json:"namespace,omitempty"`
	Image       string                   `json:"image,omitempty"`
	Resources   *JobResourceRequirements `json:"resources,omitempty"`
	Parallelism *int32                   `json:"parallelism,omitempty"`
	Completions *int32                   `json:"completions,omitempty"`
	TimeoutSec  *int64                   `json:"timeout_sec,omitempty"`
}

// LocalSyncRequest represents a request for local (non-Kubernetes) sync
type LocalSyncRequest struct {
	IssueKeys   []string      `json:"issue_keys,omitempty"`
	JQL         string        `json:"jql,omitempty"`
	Repository  string        `json:"repository"`
	Concurrency int           `json:"concurrency,omitempty"`
	RateLimit   time.Duration `json:"rate_limit,omitempty"`
	Incremental bool          `json:"incremental,omitempty"`
	Force       bool          `json:"force,omitempty"`
	DryRun      bool          `json:"dry_run,omitempty"`
}

// Validation methods

func (o *SyncJobOrchestrator) validateSingleIssueRequest(req *SingleIssueSyncRequest) error {
	if req.IssueKey == "" {
		return fmt.Errorf("issue key is required")
	}
	if req.Repository == "" {
		return fmt.Errorf("repository is required")
	}
	if req.Incremental && req.Force {
		return fmt.Errorf("cannot specify both incremental and force")
	}
	return nil
}

func (o *SyncJobOrchestrator) validateBatchRequest(req *BatchSyncRequest) error {
	if len(req.IssueKeys) == 0 {
		return fmt.Errorf("at least one issue key is required")
	}
	if req.Repository == "" {
		return fmt.Errorf("repository is required")
	}
	if req.Incremental && req.Force {
		return fmt.Errorf("cannot specify both incremental and force")
	}
	if req.Concurrency < 0 || req.Concurrency > 10 {
		return fmt.Errorf("concurrency must be between 0 and 10")
	}
	return nil
}

func (o *SyncJobOrchestrator) validateJQLRequest(req *JQLSyncRequest) error {
	if req.JQL == "" {
		return fmt.Errorf("JQL query is required")
	}
	if req.Repository == "" {
		return fmt.Errorf("repository is required")
	}
	if req.Incremental && req.Force {
		return fmt.Errorf("cannot specify both incremental and force")
	}
	if req.Concurrency < 0 || req.Concurrency > 10 {
		return fmt.Errorf("concurrency must be between 0 and 10")
	}
	return nil
}
