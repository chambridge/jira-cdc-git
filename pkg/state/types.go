package state

import (
	"time"
)

// SyncState represents the complete sync state for a repository
type SyncState struct {
	Version    string                `json:"version" yaml:"version"`
	Repository RepositoryInfo        `json:"repository" yaml:"repository"`
	LastSync   *SyncOperation        `json:"last_sync" yaml:"last_sync"`
	History    []SyncOperation       `json:"history" yaml:"history"`
	Issues     map[string]IssueState `json:"issues" yaml:"issues"`
	Stats      SyncStatistics        `json:"stats" yaml:"stats"`
	CreatedAt  time.Time             `json:"created_at" yaml:"created_at"`
	UpdatedAt  time.Time             `json:"updated_at" yaml:"updated_at"`
}

// RepositoryInfo contains metadata about the target repository
type RepositoryInfo struct {
	Path        string `json:"path" yaml:"path"`
	Branch      string `json:"branch" yaml:"branch"`
	RemoteURL   string `json:"remote_url,omitempty" yaml:"remote_url,omitempty"`
	InitialSync bool   `json:"initial_sync" yaml:"initial_sync"`
}

// SyncOperation represents a single sync operation
type SyncOperation struct {
	ID        string            `json:"id" yaml:"id"`
	Type      SyncType          `json:"type" yaml:"type"`
	Query     string            `json:"query,omitempty" yaml:"query,omitempty"`
	IssueKeys []string          `json:"issue_keys,omitempty" yaml:"issue_keys,omitempty"`
	StartTime time.Time         `json:"start_time" yaml:"start_time"`
	EndTime   time.Time         `json:"end_time" yaml:"end_time"`
	Duration  time.Duration     `json:"duration" yaml:"duration"`
	Status    SyncStatus        `json:"status" yaml:"status"`
	Results   OperationResults  `json:"results" yaml:"results"`
	Config    SyncConfig        `json:"config" yaml:"config"`
	Error     string            `json:"error,omitempty" yaml:"error,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

// SyncType represents the type of sync operation
type SyncType string

const (
	SyncTypeIssues      SyncType = "issues"
	SyncTypeJQL         SyncType = "jql"
	SyncTypeIncremental SyncType = "incremental"
	SyncTypeFull        SyncType = "full"
)

// SyncStatus represents the status of a sync operation
type SyncStatus string

const (
	SyncStatusPending   SyncStatus = "pending"
	SyncStatusRunning   SyncStatus = "running"
	SyncStatusCompleted SyncStatus = "completed"
	SyncStatusFailed    SyncStatus = "failed"
	SyncStatusCancelled SyncStatus = "cancelled"
	SyncStatusPartial   SyncStatus = "partial"
)

// OperationResults contains the results of a sync operation
type OperationResults struct {
	TotalIssues     int      `json:"total_issues" yaml:"total_issues"`
	ProcessedIssues int      `json:"processed_issues" yaml:"processed_issues"`
	SuccessfulSync  int      `json:"successful_sync" yaml:"successful_sync"`
	FailedSync      int      `json:"failed_sync" yaml:"failed_sync"`
	SkippedIssues   int      `json:"skipped_issues" yaml:"skipped_issues"`
	ProcessedFiles  []string `json:"processed_files" yaml:"processed_files"`
	ErrorCount      int      `json:"error_count" yaml:"error_count"`
}

// SyncConfig contains configuration for a sync operation
type SyncConfig struct {
	Concurrency  int           `json:"concurrency" yaml:"concurrency"`
	RateLimit    time.Duration `json:"rate_limit" yaml:"rate_limit"`
	Incremental  bool          `json:"incremental" yaml:"incremental"`
	Force        bool          `json:"force" yaml:"force"`
	DryRun       bool          `json:"dry_run" yaml:"dry_run"`
	IncludeLinks bool          `json:"include_links" yaml:"include_links"`
}

// IssueState tracks the state of an individual issue
type IssueState struct {
	Key          string    `json:"key" yaml:"key"`
	ProjectKey   string    `json:"project_key" yaml:"project_key"`
	LastSynced   time.Time `json:"last_synced" yaml:"last_synced"`
	LastModified time.Time `json:"last_modified" yaml:"last_modified"`
	LastUpdated  time.Time `json:"last_updated" yaml:"last_updated"`
	Version      int       `json:"version" yaml:"version"`
	FilePath     string    `json:"file_path" yaml:"file_path"`
	FileSize     int64     `json:"file_size" yaml:"file_size"`
	Checksum     string    `json:"checksum" yaml:"checksum"`
	SyncStatus   string    `json:"sync_status" yaml:"sync_status"`
	ErrorMessage string    `json:"error_message,omitempty" yaml:"error_message,omitempty"`
	SyncCount    int       `json:"sync_count" yaml:"sync_count"`
}

// SyncStatistics contains aggregate statistics for sync operations
type SyncStatistics struct {
	TotalOperations   int           `json:"total_operations" yaml:"total_operations"`
	SuccessfulOps     int           `json:"successful_ops" yaml:"successful_ops"`
	FailedOps         int           `json:"failed_ops" yaml:"failed_ops"`
	TotalIssuesSynced int           `json:"total_issues_synced" yaml:"total_issues_synced"`
	TotalSyncTime     time.Duration `json:"total_sync_time" yaml:"total_sync_time"`
	AvgSyncTime       time.Duration `json:"avg_sync_time" yaml:"avg_sync_time"`
	LastSuccessfulOp  time.Time     `json:"last_successful_op" yaml:"last_successful_op"`
	LastFailedOp      time.Time     `json:"last_failed_op" yaml:"last_failed_op"`
	UniqueIssues      int           `json:"unique_issues" yaml:"unique_issues"`
	ActiveProjects    []string      `json:"active_projects" yaml:"active_projects"`
}

// IncrementalSyncOptions contains options for incremental sync
type IncrementalSyncOptions struct {
	Since           time.Time     `json:"since" yaml:"since"`
	Force           bool          `json:"force" yaml:"force"`
	IncludeNew      bool          `json:"include_new" yaml:"include_new"`
	IncludeModified bool          `json:"include_modified" yaml:"include_modified"`
	MaxAge          time.Duration `json:"max_age" yaml:"max_age"`
	Projects        []string      `json:"projects,omitempty" yaml:"projects,omitempty"`
}

// StateValidationResult contains the results of state validation
type StateValidationResult struct {
	Valid              bool     `json:"valid" yaml:"valid"`
	Errors             []string `json:"errors" yaml:"errors"`
	Warnings           []string `json:"warnings" yaml:"warnings"`
	MissingIssues      []string `json:"missing_issues" yaml:"missing_issues"`
	OrphanedFiles      []string `json:"orphaned_files" yaml:"orphaned_files"`
	CorruptedFiles     []string `json:"corrupted_files" yaml:"corrupted_files"`
	RecommendedActions []string `json:"recommended_actions" yaml:"recommended_actions"`
}

// RecoveryAction represents an action that can be taken to recover from state issues
type RecoveryAction string

const (
	ActionFullResync    RecoveryAction = "full_resync"
	ActionRemoveOrphans RecoveryAction = "remove_orphans"
	ActionRepairState   RecoveryAction = "repair_state"
	ActionSkipCorrupted RecoveryAction = "skip_corrupted"
	ActionValidateOnly  RecoveryAction = "validate_only"
)

// StateRecoveryOptions contains options for state recovery
type StateRecoveryOptions struct {
	Actions     []RecoveryAction `json:"actions" yaml:"actions"`
	DryRun      bool             `json:"dry_run" yaml:"dry_run"`
	BackupFirst bool             `json:"backup_first" yaml:"backup_first"`
	Force       bool             `json:"force" yaml:"force"`
	MaxRetries  int              `json:"max_retries" yaml:"max_retries"`
}
