package jobs

import (
	"context"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// JobType represents the type of sync operation
type JobType string

const (
	JobTypeSingle JobType = "single"
	JobTypeBatch  JobType = "batch"
	JobTypeJQL    JobType = "jql"
)

// JobStatus represents the current status of a job
type JobStatus string

const (
	JobStatusPending   JobStatus = "pending"
	JobStatusRunning   JobStatus = "running"
	JobStatusSucceeded JobStatus = "succeeded"
	JobStatusFailed    JobStatus = "failed"
	JobStatusUnknown   JobStatus = "unknown"
)

// SyncJobConfig represents the configuration for a JIRA sync job
type SyncJobConfig struct {
	// Job identification
	ID      string    `json:"id"`
	Type    JobType   `json:"type"`
	Name    string    `json:"name,omitempty"`
	Created time.Time `json:"created"`

	// Sync parameters
	Target     string `json:"target"`     // Issue key, JQL query, or comma-separated issues
	Repository string `json:"repository"` // Target Git repository path

	// Sync options
	BatchSize   int           `json:"batch_size,omitempty"`
	Concurrency int           `json:"concurrency,omitempty"`
	RateLimit   time.Duration `json:"rate_limit,omitempty"`
	Incremental bool          `json:"incremental,omitempty"`
	Force       bool          `json:"force,omitempty"`
	DryRun      bool          `json:"dry_run,omitempty"`

	// Kubernetes options
	Namespace   string                   `json:"namespace,omitempty"`
	Image       string                   `json:"image,omitempty"`
	Resources   *JobResourceRequirements `json:"resources,omitempty"`
	Parallelism *int32                   `json:"parallelism,omitempty"`
	Completions *int32                   `json:"completions,omitempty"`
	TimeoutSec  *int64                   `json:"timeout_sec,omitempty"`

	// Security
	SafeMode bool `json:"safe_mode,omitempty"`
}

// JobResourceRequirements defines CPU and memory requirements for jobs
type JobResourceRequirements struct {
	RequestsCPU    string `json:"requests_cpu,omitempty"`
	RequestsMemory string `json:"requests_memory,omitempty"`
	LimitsCPU      string `json:"limits_cpu,omitempty"`
	LimitsMemory   string `json:"limits_memory,omitempty"`
}

// JobResult represents the result of a completed job
type JobResult struct {
	JobID          string        `json:"job_id"`
	Status         JobStatus     `json:"status"`
	StartTime      *time.Time    `json:"start_time,omitempty"`
	CompletionTime *time.Time    `json:"completion_time,omitempty"`
	Duration       time.Duration `json:"duration,omitempty"`

	// Sync results
	TotalIssues     int      `json:"total_issues,omitempty"`
	ProcessedIssues int      `json:"processed_issues,omitempty"`
	SuccessfulSync  int      `json:"successful_sync,omitempty"`
	FailedSync      int      `json:"failed_sync,omitempty"`
	ProcessedFiles  []string `json:"processed_files,omitempty"`

	// Error information
	ErrorMessage string   `json:"error_message,omitempty"`
	Errors       []string `json:"errors,omitempty"`

	// Kubernetes information
	PodName       string `json:"pod_name,omitempty"`
	ContainerLogs string `json:"container_logs,omitempty"`
}

// JobMonitor provides real-time job monitoring capabilities
type JobMonitor struct {
	JobID     string    `json:"job_id"`
	Status    JobStatus `json:"status"`
	Progress  float64   `json:"progress"`
	LastCheck time.Time `json:"last_check"`
	Message   string    `json:"message,omitempty"`
}

// JobScheduler defines the interface for creating and managing Kubernetes Jobs
type JobScheduler interface {
	// Job creation and management
	CreateJob(ctx context.Context, config *SyncJobConfig) (*JobResult, error)
	GetJob(ctx context.Context, jobID string) (*JobResult, error)
	ListJobs(ctx context.Context, filters *JobFilter) ([]*JobResult, error)
	DeleteJob(ctx context.Context, jobID string) error

	// Job monitoring
	WatchJob(ctx context.Context, jobID string) (<-chan JobMonitor, error)
	GetJobLogs(ctx context.Context, jobID string) (string, error)

	// Job queue management
	GetQueueStatus(ctx context.Context) (*QueueStatus, error)
	CancelJob(ctx context.Context, jobID string) error
}

// JobFilter provides filtering options for listing jobs
type JobFilter struct {
	Type          []JobType   `json:"type,omitempty"`
	Status        []JobStatus `json:"status,omitempty"`
	CreatedSince  *time.Time  `json:"created_since,omitempty"`
	CreatedBefore *time.Time  `json:"created_before,omitempty"`
	Namespace     string      `json:"namespace,omitempty"`
	Limit         int         `json:"limit,omitempty"`
	Offset        int         `json:"offset,omitempty"`
}

// QueueStatus provides information about the job queue
type QueueStatus struct {
	TotalJobs     int `json:"total_jobs"`
	PendingJobs   int `json:"pending_jobs"`
	RunningJobs   int `json:"running_jobs"`
	CompletedJobs int `json:"completed_jobs"`
	FailedJobs    int `json:"failed_jobs"`
}

// JobTemplate represents a Kubernetes Job template
type JobTemplate struct {
	JobType     JobType                  `json:"job_type"`
	Template    *batchv1.Job             `json:"template"`
	Resources   *JobResourceRequirements `json:"default_resources"`
	Parallelism *int32                   `json:"default_parallelism,omitempty"`
	Completions *int32                   `json:"default_completions,omitempty"`
	TimeoutSec  *int64                   `json:"default_timeout_sec,omitempty"`
}

// JobTemplateManager manages job templates
type JobTemplateManager interface {
	GetTemplate(jobType JobType) (*JobTemplate, error)
	LoadTemplate(jobType JobType, templatePath string) error
	ValidateTemplate(template *JobTemplate) error
}

// JobError represents job-specific errors
type JobError struct {
	JobID   string    `json:"job_id"`
	Type    string    `json:"type"`
	Message string    `json:"message"`
	Time    time.Time `json:"time"`
}

// Error implements the error interface
func (e *JobError) Error() string {
	return e.Message
}

// NewJobError creates a new job error
func NewJobError(jobID, errorType, message string) *JobError {
	return &JobError{
		JobID:   jobID,
		Type:    errorType,
		Message: message,
		Time:    time.Now(),
	}
}

// KubernetesJobInfo holds Kubernetes-specific job information
type KubernetesJobInfo struct {
	Name              string                 `json:"name"`
	Namespace         string                 `json:"namespace"`
	Labels            map[string]string      `json:"labels"`
	Annotations       map[string]string      `json:"annotations"`
	CreationTimestamp metav1.Time            `json:"creation_timestamp"`
	StartTime         *metav1.Time           `json:"start_time,omitempty"`
	CompletionTime    *metav1.Time           `json:"completion_time,omitempty"`
	Conditions        []batchv1.JobCondition `json:"conditions,omitempty"`
	Active            int32                  `json:"active"`
	Succeeded         int32                  `json:"succeeded"`
	Failed            int32                  `json:"failed"`
}
