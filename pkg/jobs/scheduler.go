package jobs

import (
	"context"
	"fmt"
	"strings"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// KubernetesJobScheduler implements JobScheduler interface using Kubernetes Jobs
type KubernetesJobScheduler struct {
	clientset         kubernetes.Interface
	namespace         string
	defaultImage      string
	templateManager   JobTemplateManager
	credentialsSecret string
	configMapName     string
	pvcName           string
}

// NewKubernetesJobScheduler creates a new Kubernetes-based job scheduler
func NewKubernetesJobScheduler(config *rest.Config, namespace, image string) (*KubernetesJobScheduler, error) {
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	templateManager := NewFileJobTemplateManager()

	return &KubernetesJobScheduler{
		clientset:         clientset,
		namespace:         namespace,
		defaultImage:      image,
		templateManager:   templateManager,
		credentialsSecret: "jira-credentials",
		configMapName:     "jira-sync-config",
		pvcName:           "git-repo-pvc",
	}, nil
}

// CreateJob creates a new Kubernetes Job for JIRA sync
func (s *KubernetesJobScheduler) CreateJob(ctx context.Context, config *SyncJobConfig) (*JobResult, error) {
	// Validate config
	if err := s.validateJobConfig(config); err != nil {
		return nil, fmt.Errorf("invalid job configuration: %w", err)
	}

	// Get job template
	template, err := s.templateManager.GetTemplate(config.Type)
	if err != nil {
		return nil, fmt.Errorf("failed to get job template: %w", err)
	}

	// Create Kubernetes Job from template
	job, err := s.createKubernetesJob(config, template)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes job: %w", err)
	}

	// Submit job to Kubernetes
	createdJob, err := s.clientset.BatchV1().Jobs(s.namespace).Create(ctx, job, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to submit job to Kubernetes: %w", err)
	}

	// Create job result
	result := &JobResult{
		JobID:     config.ID,
		Status:    JobStatusPending,
		StartTime: &config.Created,
	}

	// Update with Kubernetes job information
	if createdJob.Status.StartTime != nil {
		startTime := createdJob.Status.StartTime.Time
		result.StartTime = &startTime
	}

	return result, nil
}

// GetJob retrieves job status and results
func (s *KubernetesJobScheduler) GetJob(ctx context.Context, jobID string) (*JobResult, error) {
	jobName := s.generateJobName(jobID)

	job, err := s.clientset.BatchV1().Jobs(s.namespace).Get(ctx, jobName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get job %s: %w", jobID, err)
	}

	return s.convertJobToResult(job, jobID)
}

// ListJobs lists jobs with optional filtering
func (s *KubernetesJobScheduler) ListJobs(ctx context.Context, filters *JobFilter) ([]*JobResult, error) {
	listOptions := metav1.ListOptions{
		LabelSelector: "app=jira-sync",
	}

	if filters != nil && filters.Limit > 0 {
		listOptions.Limit = int64(filters.Limit)
	}

	jobs, err := s.clientset.BatchV1().Jobs(s.namespace).List(ctx, listOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to list jobs: %w", err)
	}

	results := make([]*JobResult, 0, len(jobs.Items))
	for _, job := range jobs.Items {
		jobID := s.extractJobID(&job)
		if jobID == "" {
			continue
		}

		result, err := s.convertJobToResult(&job, jobID)
		if err != nil {
			continue // Skip jobs that can't be converted
		}

		// Apply filters
		if filters != nil && !s.matchesFilter(result, filters) {
			continue
		}

		results = append(results, result)
	}

	return results, nil
}

// DeleteJob deletes a job and its associated resources
func (s *KubernetesJobScheduler) DeleteJob(ctx context.Context, jobID string) error {
	jobName := s.generateJobName(jobID)

	deletePolicy := metav1.DeletePropagationForeground
	deleteOptions := metav1.DeleteOptions{
		PropagationPolicy: &deletePolicy,
	}

	return s.clientset.BatchV1().Jobs(s.namespace).Delete(ctx, jobName, deleteOptions)
}

// WatchJob provides real-time monitoring of job status
func (s *KubernetesJobScheduler) WatchJob(ctx context.Context, jobID string) (<-chan JobMonitor, error) {
	jobName := s.generateJobName(jobID)

	// Create watch for the specific job
	watcher, err := s.clientset.BatchV1().Jobs(s.namespace).Watch(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("metadata.name=%s", jobName),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create job watcher: %w", err)
	}

	monitorChan := make(chan JobMonitor, 10)

	go func() {
		defer close(monitorChan)
		defer watcher.Stop()

		for event := range watcher.ResultChan() {
			job, ok := event.Object.(*batchv1.Job)
			if !ok {
				continue
			}

			monitor := JobMonitor{
				JobID:     jobID,
				Status:    s.getJobStatus(job),
				LastCheck: time.Now(),
			}

			// Calculate progress based on completions
			if job.Spec.Completions != nil && *job.Spec.Completions > 0 {
				monitor.Progress = float64(job.Status.Succeeded) / float64(*job.Spec.Completions) * 100
			}

			// Add status message
			monitor.Message = s.getJobStatusMessage(job)

			select {
			case monitorChan <- monitor:
			case <-ctx.Done():
				return
			}
		}
	}()

	return monitorChan, nil
}

// GetJobLogs retrieves logs from job pods
func (s *KubernetesJobScheduler) GetJobLogs(ctx context.Context, jobID string) (string, error) {
	jobName := s.generateJobName(jobID)

	// Get pods for this job
	pods, err := s.clientset.CoreV1().Pods(s.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("job-name=%s", jobName),
	})
	if err != nil {
		return "", fmt.Errorf("failed to get pods for job %s: %w", jobID, err)
	}

	if len(pods.Items) == 0 {
		return "", fmt.Errorf("no pods found for job %s", jobID)
	}

	// Get logs from the first pod
	pod := pods.Items[0]
	logOptions := &corev1.PodLogOptions{
		Container: "sync-worker",
	}

	req := s.clientset.CoreV1().Pods(s.namespace).GetLogs(pod.Name, logOptions)
	logs, err := req.Stream(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get logs: %w", err)
	}
	defer func() {
		if err := logs.Close(); err != nil {
			// Log error but don't fail the operation - silently ignore
			_ = err
		}
	}()

	buf := new(strings.Builder)
	_, err = fmt.Fprint(buf, logs)
	if err != nil {
		return "", fmt.Errorf("failed to read logs: %w", err)
	}

	return buf.String(), nil
}

// GetQueueStatus returns information about the job queue
func (s *KubernetesJobScheduler) GetQueueStatus(ctx context.Context) (*QueueStatus, error) {
	jobs, err := s.clientset.BatchV1().Jobs(s.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app=jira-sync",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list jobs for queue status: %w", err)
	}

	status := &QueueStatus{}
	status.TotalJobs = len(jobs.Items)

	for _, job := range jobs.Items {
		jobStatus := s.getJobStatus(&job)
		switch jobStatus {
		case JobStatusPending:
			status.PendingJobs++
		case JobStatusRunning:
			status.RunningJobs++
		case JobStatusSucceeded:
			status.CompletedJobs++
		case JobStatusFailed:
			status.FailedJobs++
		}
	}

	return status, nil
}

// CancelJob cancels a running job
func (s *KubernetesJobScheduler) CancelJob(ctx context.Context, jobID string) error {
	jobName := s.generateJobName(jobID)

	// Get the job first
	job, err := s.clientset.BatchV1().Jobs(s.namespace).Get(ctx, jobName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get job for cancellation: %w", err)
	}

	// Set parallelism to 0 to stop creating new pods
	parallelism := int32(0)
	job.Spec.Parallelism = &parallelism

	// Update the job
	_, err = s.clientset.BatchV1().Jobs(s.namespace).Update(ctx, job, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to cancel job: %w", err)
	}

	// Delete running pods
	deleteOptions := metav1.DeleteOptions{}
	return s.clientset.CoreV1().Pods(s.namespace).DeleteCollection(ctx, deleteOptions, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("job-name=%s", jobName),
	})
}

// Helper methods

func (s *KubernetesJobScheduler) validateJobConfig(config *SyncJobConfig) error {
	if config.ID == "" {
		return fmt.Errorf("job ID is required")
	}
	if config.Target == "" {
		return fmt.Errorf("target is required")
	}
	if config.Repository == "" {
		return fmt.Errorf("repository is required")
	}
	if config.Type == "" {
		return fmt.Errorf("job type is required")
	}
	return nil
}

func (s *KubernetesJobScheduler) createKubernetesJob(config *SyncJobConfig, template *JobTemplate) (*batchv1.Job, error) {
	jobName := s.generateJobName(config.ID)

	// Clone the template
	job := template.Template.DeepCopy()

	// Update metadata
	job.Name = jobName
	job.Namespace = s.namespace
	job.Labels = s.generateJobLabels(config)
	job.Annotations = s.generateJobAnnotations(config)

	// Update container args
	container := &job.Spec.Template.Spec.Containers[0]
	container.Args = s.generateContainerArgs(config)

	// Set image
	if config.Image != "" {
		container.Image = config.Image
	} else {
		container.Image = s.defaultImage
	}

	// Apply resource requirements
	if config.Resources != nil {
		container.Resources = s.buildResourceRequirements(config.Resources)
	}

	// Apply job spec overrides
	if config.Parallelism != nil {
		job.Spec.Parallelism = config.Parallelism
	}
	if config.Completions != nil {
		job.Spec.Completions = config.Completions
	}
	if config.TimeoutSec != nil {
		job.Spec.ActiveDeadlineSeconds = config.TimeoutSec
	}

	// Add environment variables
	container.Env = append(container.Env, s.generateEnvironmentVars(config)...)

	return job, nil
}

func (s *KubernetesJobScheduler) generateJobName(jobID string) string {
	return fmt.Sprintf("jira-sync-%s", strings.ToLower(strings.ReplaceAll(jobID, "_", "-")))
}

func (s *KubernetesJobScheduler) generateJobLabels(config *SyncJobConfig) map[string]string {
	return map[string]string{
		"app":        "jira-sync",
		"sync-type":  string(config.Type),
		"sync-id":    config.ID,
		"managed-by": "jira-sync-scheduler",
	}
}

func (s *KubernetesJobScheduler) generateJobAnnotations(config *SyncJobConfig) map[string]string {
	return map[string]string{
		"jira-sync/target":     config.Target,
		"jira-sync/repository": config.Repository,
		"jira-sync/created":    config.Created.Format(time.RFC3339),
	}
}

func (s *KubernetesJobScheduler) generateContainerArgs(config *SyncJobConfig) []string {
	args := []string{"sync"}

	// Add sync parameters based on type
	switch config.Type {
	case JobTypeSingle, JobTypeBatch:
		args = append(args, "--issues="+config.Target)
	case JobTypeJQL:
		args = append(args, "--jql="+config.Target)
	}

	args = append(args, "--repo="+config.Repository)

	// Add optional parameters
	if config.Concurrency > 0 {
		args = append(args, fmt.Sprintf("--concurrency=%d", config.Concurrency))
	}
	if config.RateLimit > 0 {
		args = append(args, fmt.Sprintf("--rate-limit=%v", config.RateLimit))
	}
	if config.Incremental {
		args = append(args, "--incremental")
	}
	if config.Force {
		args = append(args, "--force")
	}
	if config.DryRun {
		args = append(args, "--dry-run")
	}

	return args
}

func (s *KubernetesJobScheduler) generateEnvironmentVars(config *SyncJobConfig) []corev1.EnvVar {
	envVars := []corev1.EnvVar{
		{
			Name:  "SYNC_JOB_ID",
			Value: config.ID,
		},
		{
			Name:  "LOG_LEVEL",
			Value: "INFO",
		},
	}

	if config.SafeMode {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "SPIKE_SAFE_MODE",
			Value: "true",
		})
	}

	return envVars
}

func (s *KubernetesJobScheduler) buildResourceRequirements(req *JobResourceRequirements) corev1.ResourceRequirements {
	resources := corev1.ResourceRequirements{
		Requests: make(corev1.ResourceList),
		Limits:   make(corev1.ResourceList),
	}

	if req.RequestsCPU != "" {
		resources.Requests[corev1.ResourceCPU] = resource.MustParse(req.RequestsCPU)
	}
	if req.RequestsMemory != "" {
		resources.Requests[corev1.ResourceMemory] = resource.MustParse(req.RequestsMemory)
	}
	if req.LimitsCPU != "" {
		resources.Limits[corev1.ResourceCPU] = resource.MustParse(req.LimitsCPU)
	}
	if req.LimitsMemory != "" {
		resources.Limits[corev1.ResourceMemory] = resource.MustParse(req.LimitsMemory)
	}

	return resources
}

func (s *KubernetesJobScheduler) convertJobToResult(job *batchv1.Job, jobID string) (*JobResult, error) {
	result := &JobResult{
		JobID:  jobID,
		Status: s.getJobStatus(job),
	}

	if job.Status.StartTime != nil {
		startTime := job.Status.StartTime.Time
		result.StartTime = &startTime
	}

	if job.Status.CompletionTime != nil {
		completionTime := job.Status.CompletionTime.Time
		result.CompletionTime = &completionTime

		if result.StartTime != nil {
			result.Duration = completionTime.Sub(*result.StartTime)
		}
	}

	// Extract sync results from job annotations if available
	if annotations := job.Annotations; annotations != nil {
		// These would be set by the job itself during execution
		// For now, we'll use placeholder logic
		result.TotalIssues = int(job.Status.Succeeded + job.Status.Failed)
		result.SuccessfulSync = int(job.Status.Succeeded)
		result.FailedSync = int(job.Status.Failed)
	}

	return result, nil
}

func (s *KubernetesJobScheduler) getJobStatus(job *batchv1.Job) JobStatus {
	for _, condition := range job.Status.Conditions {
		switch condition.Type {
		case batchv1.JobComplete:
			if condition.Status == corev1.ConditionTrue {
				return JobStatusSucceeded
			}
		case batchv1.JobFailed:
			if condition.Status == corev1.ConditionTrue {
				return JobStatusFailed
			}
		}
	}

	if job.Status.Active > 0 {
		return JobStatusRunning
	}

	return JobStatusPending
}

func (s *KubernetesJobScheduler) getJobStatusMessage(job *batchv1.Job) string {
	for _, condition := range job.Status.Conditions {
		if condition.Message != "" {
			return condition.Message
		}
	}

	if job.Status.Active > 0 {
		return fmt.Sprintf("Running with %d active pods", job.Status.Active)
	}

	return "Job pending"
}

func (s *KubernetesJobScheduler) extractJobID(job *batchv1.Job) string {
	if labels := job.Labels; labels != nil {
		return labels["sync-id"]
	}
	return ""
}

func (s *KubernetesJobScheduler) matchesFilter(result *JobResult, filters *JobFilter) bool {
	// Type filter
	if len(filters.Type) > 0 {
		found := false
		for _, t := range filters.Type {
			if string(t) == strings.ToLower(string(result.Status)) { // This is simplified
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Status filter
	if len(filters.Status) > 0 {
		found := false
		for _, s := range filters.Status {
			if s == result.Status {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Time filters would need job creation time from Kubernetes metadata

	return true
}
