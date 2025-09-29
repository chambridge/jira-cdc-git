// Package jobs provides Kubernetes Job scheduling capabilities for JIRA sync operations.
//
// This package implements JCG-023: Job Scheduling Engine, which provides:
// - Kubernetes Job creation and management for sync operations
// - Integration with existing CLI sync functionality
// - Job templates based on validated SPIKE-001 patterns
// - Comprehensive error handling and monitoring
//
// Architecture:
//
// The jobs package follows a clean architecture pattern with clear separation:
//
//   - JobScheduler interface defines the core scheduling operations
//   - KubernetesJobScheduler implements the interface using Kubernetes Jobs API
//   - JobTemplateManager handles job template loading and validation
//   - SyncJobOrchestrator coordinates between scheduling and sync operations
//   - Comprehensive error types for different failure scenarios
//
// Usage Example:
//
//	// Create Kubernetes client configuration
//	config, err := rest.InClusterConfig()
//	if err != nil {
//	    return err
//	}
//
//	// Create job scheduler
//	scheduler, err := jobs.NewKubernetesJobScheduler(config, "jira-sync", "jira-sync:latest")
//	if err != nil {
//	    return err
//	}
//
//	// Create orchestrator
//	orchestrator := jobs.NewSyncJobOrchestrator(scheduler)
//
//	// Submit a single issue sync job
//	request := &jobs.SingleIssueSyncRequest{
//	    IssueKey:   "PROJ-123",
//	    Repository: "/workspace/repo",
//	    SafeMode:   true,
//	}
//
//	result, err := orchestrator.SubmitSingleIssueSync(ctx, request)
//	if err != nil {
//	    return err
//	}
//
//	// Monitor job progress
//	monitor, err := orchestrator.WatchJob(ctx, result.JobID)
//	if err != nil {
//	    return err
//	}
//
//	for update := range monitor {
//	    fmt.Printf("Job %s: %s (%.1f%%)\n", update.JobID, update.Status, update.Progress)
//	}
//
// Job Types:
//
// The package supports three types of sync jobs:
//
//   - JobTypeSingle: Sync a single JIRA issue
//   - JobTypeBatch: Sync multiple JIRA issues with parallel processing
//   - JobTypeJQL: Sync issues matching a JQL query
//
// Each job type has optimized resource requirements and templates based on
// SPIKE-001 validation results.
//
// Integration with SPIKE-001:
//
// This package integrates the validated Kubernetes Job patterns from SPIKE-001:
//   - Single issue jobs: 100m CPU, 128Mi memory, 10-minute timeout
//   - Batch jobs: 200m CPU, 256Mi memory, 30-minute timeout, parallelism=2
//   - Security: Non-root execution, secret management, container isolation
//   - Monitoring: Structured logging, metrics collection, status tracking
//
// Error Handling:
//
// The package provides comprehensive error types:
//   - ValidationError: Configuration validation failures
//   - AuthenticationError: JIRA authentication issues
//   - KubernetesError: Kubernetes API operation failures
//   - TimeoutError: Job execution timeouts
//   - ResourceError: Resource allocation failures
//   - TemplateError: Job template issues
//   - ExecutionError: Job execution failures
//
// All errors implement JobErrorWithType interface and include:
//   - Error type classification
//   - Severity assessment
//   - Retry recommendations
//   - Remediation suggestions
//
// Performance Characteristics:
//
// Based on SPIKE-001 validation:
//   - Job creation: <200ms
//   - Single issue sync: <30 seconds
//   - Batch processing: 50+ issues in <5 minutes
//   - Container startup: <10 seconds
//   - Support for 10+ concurrent jobs
//
// Security Features:
//
//   - Non-root container execution
//   - Secure credential management via Kubernetes secrets
//   - Network isolation and security contexts
//   - Resource limits and quotas
//   - Audit logging and monitoring
//
// Monitoring and Observability:
//
//   - Real-time job status monitoring
//   - Progress tracking and reporting
//   - Container log access
//   - Queue status and metrics
//   - Error classification and alerting
package jobs

import (
	"context"
	"time"
)

// Version information
const (
	Version   = "0.4.0"
	Component = "job-scheduler"
)

// Default configuration values based on SPIKE-001 validation
const (
	DefaultNamespace   = "jira-sync"
	DefaultImage       = "jira-sync:latest"
	DefaultTimeout     = 30 * time.Minute
	DefaultRateLimit   = 500 * time.Millisecond
	DefaultConcurrency = 5

	// Resource defaults from SPIKE-001
	SingleJobCPURequest    = "100m"
	SingleJobMemoryRequest = "128Mi"
	SingleJobCPULimit      = "500m"
	SingleJobMemoryLimit   = "512Mi"

	BatchJobCPURequest    = "200m"
	BatchJobMemoryRequest = "256Mi"
	BatchJobCPULimit      = "1000m"
	BatchJobMemoryLimit   = "1Gi"

	// Job limits
	MaxConcurrency   = 10
	MaxBatchSize     = 100
	MaxJobNameLength = 63
)

// JobManager provides a high-level interface for job management
type JobManager interface {
	// Job submission
	SubmitSingleIssueSync(ctx context.Context, req *SingleIssueSyncRequest) (*JobResult, error)
	SubmitBatchSync(ctx context.Context, req *BatchSyncRequest) (*JobResult, error)
	SubmitJQLSync(ctx context.Context, req *JQLSyncRequest) (*JobResult, error)

	// Job management
	GetJob(ctx context.Context, jobID string) (*JobResult, error)
	ListJobs(ctx context.Context, filters *JobFilter) ([]*JobResult, error)
	CancelJob(ctx context.Context, jobID string) error
	DeleteJob(ctx context.Context, jobID string) error

	// Monitoring
	WatchJob(ctx context.Context, jobID string) (<-chan JobMonitor, error)
	GetJobLogs(ctx context.Context, jobID string) (string, error)
	GetQueueStatus(ctx context.Context) (*QueueStatus, error)

	// Local execution (fallback for non-Kubernetes environments)
	ExecuteLocalSync(ctx context.Context, req *LocalSyncRequest) (*SyncResult, error)
}

// SyncResult represents the result of a local sync operation
type SyncResult struct {
	TotalIssues     int           `json:"total_issues"`
	ProcessedIssues int           `json:"processed_issues"`
	SuccessfulSync  int           `json:"successful_sync"`
	FailedSync      int           `json:"failed_sync"`
	Duration        time.Duration `json:"duration"`
	ProcessedFiles  []string      `json:"processed_files"`
	Errors          []string      `json:"errors,omitempty"`
}

// JobConfiguration provides global job configuration
type JobConfiguration struct {
	DefaultNamespace string                   `json:"default_namespace"`
	DefaultImage     string                   `json:"default_image"`
	DefaultTimeout   time.Duration            `json:"default_timeout"`
	DefaultResources *JobResourceRequirements `json:"default_resources"`
	MaxConcurrency   int                      `json:"max_concurrency"`
	MaxBatchSize     int                      `json:"max_batch_size"`
	EnableSafeMode   bool                     `json:"enable_safe_mode"`
	EnableMonitoring bool                     `json:"enable_monitoring"`
	LogLevel         string                   `json:"log_level"`
}

// DefaultJobConfiguration returns the default job configuration
func DefaultJobConfiguration() *JobConfiguration {
	return &JobConfiguration{
		DefaultNamespace: DefaultNamespace,
		DefaultImage:     DefaultImage,
		DefaultTimeout:   DefaultTimeout,
		DefaultResources: &JobResourceRequirements{
			RequestsCPU:    SingleJobCPURequest,
			RequestsMemory: SingleJobMemoryRequest,
			LimitsCPU:      SingleJobCPULimit,
			LimitsMemory:   SingleJobMemoryLimit,
		},
		MaxConcurrency:   MaxConcurrency,
		MaxBatchSize:     MaxBatchSize,
		EnableSafeMode:   true,
		EnableMonitoring: true,
		LogLevel:         "INFO",
	}
}

// ValidateConfiguration validates job configuration
func ValidateConfiguration(config *JobConfiguration) error {
	if config.DefaultNamespace == "" {
		return NewValidationError("", "default_namespace", config.DefaultNamespace, "namespace cannot be empty")
	}

	if config.DefaultImage == "" {
		return NewValidationError("", "default_image", config.DefaultImage, "image cannot be empty")
	}

	if config.DefaultTimeout <= 0 {
		return NewValidationError("", "default_timeout", config.DefaultTimeout, "timeout must be positive")
	}

	if config.MaxConcurrency <= 0 || config.MaxConcurrency > MaxConcurrency {
		return NewValidationError("", "max_concurrency", config.MaxConcurrency,
			"concurrency must be between 1 and 10")
	}

	if config.MaxBatchSize <= 0 || config.MaxBatchSize > MaxBatchSize {
		return NewValidationError("", "max_batch_size", config.MaxBatchSize,
			"batch size must be between 1 and 100")
	}

	return nil
}

// JobMetrics provides job execution metrics
type JobMetrics struct {
	TotalJobs       int64         `json:"total_jobs"`
	SuccessfulJobs  int64         `json:"successful_jobs"`
	FailedJobs      int64         `json:"failed_jobs"`
	AverageExecTime time.Duration `json:"average_exec_time"`
	TotalExecTime   time.Duration `json:"total_exec_time"`
	JobsPerHour     float64       `json:"jobs_per_hour"`
	ErrorRate       float64       `json:"error_rate"`
	LastJobTime     time.Time     `json:"last_job_time"`
}

// HealthStatus represents the health of the job scheduling system
type HealthStatus struct {
	Status            string    `json:"status"` // "healthy", "degraded", "unhealthy"
	KubernetesHealthy bool      `json:"kubernetes_healthy"`
	TemplatesLoaded   bool      `json:"templates_loaded"`
	ActiveJobs        int       `json:"active_jobs"`
	QueueLength       int       `json:"queue_length"`
	LastHealthCheck   time.Time `json:"last_health_check"`
	Issues            []string  `json:"issues,omitempty"`
}

// JobSystemInfo provides system information
type JobSystemInfo struct {
	Version           string            `json:"version"`
	Component         string            `json:"component"`
	KubernetesVersion string            `json:"kubernetes_version,omitempty"`
	Namespace         string            `json:"namespace"`
	SupportedJobTypes []JobType         `json:"supported_job_types"`
	Configuration     *JobConfiguration `json:"configuration"`
	Metrics           *JobMetrics       `json:"metrics"`
	Health            *HealthStatus     `json:"health"`
}

// GetSystemInfo returns comprehensive system information
func GetSystemInfo(manager JobManager, config *JobConfiguration) *JobSystemInfo {
	return &JobSystemInfo{
		Version:   Version,
		Component: Component,
		Namespace: config.DefaultNamespace,
		SupportedJobTypes: []JobType{
			JobTypeSingle,
			JobTypeBatch,
			JobTypeJQL,
		},
		Configuration: config,
		Metrics:       &JobMetrics{}, // Would be populated from actual metrics
		Health: &HealthStatus{
			Status:          "healthy", // Would be determined by actual health checks
			LastHealthCheck: time.Now(),
		},
	}
}
