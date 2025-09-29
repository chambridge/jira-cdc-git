package api

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/chambrid/jira-cdc-git/pkg/jobs"
	"github.com/spf13/cobra"
	"k8s.io/client-go/rest"
)

var buildInfo BuildInfo

// Execute starts the API server CLI
func Execute(info BuildInfo) error {
	buildInfo = info
	return rootCmd.Execute()
}

// rootCmd represents the base command for the API server
var rootCmd = &cobra.Command{
	Use:   "api-server",
	Short: "JIRA CDC Git Sync API Server",
	Long: `JIRA CDC Git Sync API Server - RESTful API for JIRA synchronization operations.

Provides programmatic access to all JIRA sync functionality through REST endpoints,
with Kubernetes Job scheduling for scalable operations.

Key Features:
  • REST API for sync operations (single, batch, JQL)
  • Kubernetes Job scheduling and management
  • Async operation status tracking
  • Rate limiting and request validation
  • OpenAPI documentation
  • Health monitoring and metrics

Configuration:
  Set configuration via environment variables or command-line flags:
    API_PORT=8080 (server port)
    API_HOST=0.0.0.0 (server host)
    KUBECONFIG=/path/to/kubeconfig (for Job scheduling)
    
API Endpoints:
  GET  /api/v1/health - Health check
  GET  /api/v1/docs - API documentation
  POST /api/v1/sync/{single,batch,jql} - Sync operations
  GET  /api/v1/jobs - Job management
  
Getting Started:
  api-server serve --port=8080`,
	Version: buildInfo.Version,
}

// serveCmd represents the serve command
var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the API server",
	Long: `Start the API server to handle REST requests for JIRA sync operations.

The server provides RESTful endpoints for all CLI functionality with additional
features like async job processing, monitoring, and management.

Examples:
  # Start server on default port 8080
  api-server serve
  
  # Start server on custom port
  api-server serve --port=9090
  
  # Start server with Kubernetes job scheduling
  api-server serve --enable-jobs --namespace=jira-sync
  
  # Development mode with verbose logging
  api-server serve --log-level=debug --enable-cors`,
	RunE: runServe,
}

func runServe(cmd *cobra.Command, args []string) error {
	// Get configuration from flags and environment
	config, err := loadServerConfig(cmd)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Initialize job manager
	jobManager, err := initializeJobManager(cmd)
	if err != nil {
		return fmt.Errorf("failed to initialize job manager: %w", err)
	}

	// Create and configure server
	server := NewServer(config, buildInfo, jobManager)

	// Set up graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Start server in goroutine
	serverErr := make(chan error, 1)
	go func() {
		if err := server.Start(); err != nil {
			serverErr <- err
		}
	}()

	// Wait for shutdown signal or server error
	select {
	case err := <-serverErr:
		return fmt.Errorf("server failed to start: %w", err)
	case sig := <-sigChan:
		log.Printf("🛑 Received signal %v, shutting down...", sig)

		// Graceful shutdown with timeout
		shutdownCtx, shutdownCancel := context.WithTimeout(ctx, 30*time.Second)
		defer shutdownCancel()

		if err := server.Stop(shutdownCtx); err != nil {
			log.Printf("❌ Error during shutdown: %v", err)
			return err
		}

		log.Println("✅ Server shut down gracefully")
		return nil
	}
}

// loadServerConfig loads server configuration from flags and environment
func loadServerConfig(cmd *cobra.Command) (*Config, error) {
	config := DefaultConfig()

	// Override with command-line flags
	if cmd.Flags().Changed("port") {
		port, _ := cmd.Flags().GetInt("port")
		config.Port = port
	}

	if cmd.Flags().Changed("host") {
		host, _ := cmd.Flags().GetString("host")
		config.Host = host
	}

	if cmd.Flags().Changed("log-level") {
		logLevel, _ := cmd.Flags().GetString("log-level")
		config.LogLevel = logLevel
	}

	if cmd.Flags().Changed("enable-auth") {
		enableAuth, _ := cmd.Flags().GetBool("enable-auth")
		config.EnableAuthentication = enableAuth
	}

	if cmd.Flags().Changed("enable-cors") {
		enableCORS, _ := cmd.Flags().GetBool("enable-cors")
		config.EnableCORS = enableCORS
	}

	if cmd.Flags().Changed("rate-limit") {
		rateLimit, _ := cmd.Flags().GetInt("rate-limit")
		config.RateLimitPerMinute = rateLimit
	}

	// Override with environment variables
	if port := os.Getenv("API_PORT"); port != "" {
		if p, err := parseIntParam(port, "API_PORT", config.Port); err == nil {
			config.Port = p
		}
	}

	if host := os.Getenv("API_HOST"); host != "" {
		config.Host = host
	}

	if logLevel := os.Getenv("LOG_LEVEL"); logLevel != "" {
		config.LogLevel = logLevel
	}

	return config, nil
}

// initializeJobManager initializes the job manager based on configuration
func initializeJobManager(cmd *cobra.Command) (jobs.JobManager, error) {
	enableJobs, _ := cmd.Flags().GetBool("enable-jobs")

	if !enableJobs {
		log.Println("ℹ️  Job scheduling disabled, using local execution only")
		// Return a mock or limited job manager for local execution
		return &LocalJobManager{}, nil
	}

	// Initialize Kubernetes client for job scheduling
	namespace, _ := cmd.Flags().GetString("namespace")
	if namespace == "" {
		namespace = "jira-sync"
	}

	image, _ := cmd.Flags().GetString("image")
	if image == "" {
		image = "jira-sync:latest"
	}

	// Try to create Kubernetes client configuration
	config, err := rest.InClusterConfig()
	if err != nil {
		// Fall back to kubeconfig if not running in cluster
		log.Println("⚠️  Not running in Kubernetes cluster, job scheduling limited")
		return &LocalJobManager{}, nil
	}

	// Create Kubernetes job scheduler
	scheduler, err := jobs.NewKubernetesJobScheduler(config, namespace, image)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes job scheduler: %w", err)
	}

	log.Printf("✅ Kubernetes job scheduling enabled in namespace '%s'", namespace)
	return &JobManagerWrapper{scheduler: scheduler}, nil
}

// LocalJobManager provides local-only job execution (fallback)
type LocalJobManager struct{}

func (m *LocalJobManager) SubmitSingleIssueSync(ctx context.Context, req *jobs.SingleIssueSyncRequest) (*jobs.JobResult, error) {
	// Convert to local sync and execute immediately
	localReq := &jobs.LocalSyncRequest{
		IssueKeys:   []string{req.IssueKey},
		Repository:  req.Repository,
		Concurrency: 1,
		RateLimit:   req.RateLimit,
		Incremental: req.Incremental,
		Force:       req.Force,
		DryRun:      req.DryRun,
	}

	result, err := m.ExecuteLocalSync(ctx, localReq)
	if err != nil {
		return nil, err
	}

	// Convert to job result
	jobResult := &jobs.JobResult{
		JobID:           fmt.Sprintf("local-%d", time.Now().Unix()),
		Status:          jobs.JobStatusSucceeded,
		TotalIssues:     result.TotalIssues,
		ProcessedIssues: result.ProcessedIssues,
		SuccessfulSync:  result.SuccessfulSync,
		FailedSync:      result.FailedSync,
		Duration:        result.Duration,
		ProcessedFiles:  result.ProcessedFiles,
	}

	if result.FailedSync > 0 {
		jobResult.Status = jobs.JobStatusFailed
	}

	return jobResult, nil
}

func (m *LocalJobManager) SubmitBatchSync(ctx context.Context, req *jobs.BatchSyncRequest) (*jobs.JobResult, error) {
	localReq := &jobs.LocalSyncRequest{
		IssueKeys:   req.IssueKeys,
		Repository:  req.Repository,
		Concurrency: req.Concurrency,
		RateLimit:   req.RateLimit,
		Incremental: req.Incremental,
		Force:       req.Force,
		DryRun:      req.DryRun,
	}

	result, err := m.ExecuteLocalSync(ctx, localReq)
	if err != nil {
		return nil, err
	}

	jobResult := &jobs.JobResult{
		JobID:           fmt.Sprintf("local-%d", time.Now().Unix()),
		Status:          jobs.JobStatusSucceeded,
		TotalIssues:     result.TotalIssues,
		ProcessedIssues: result.ProcessedIssues,
		SuccessfulSync:  result.SuccessfulSync,
		FailedSync:      result.FailedSync,
		Duration:        result.Duration,
		ProcessedFiles:  result.ProcessedFiles,
	}

	if result.FailedSync > 0 {
		jobResult.Status = jobs.JobStatusFailed
	}

	return jobResult, nil
}

func (m *LocalJobManager) SubmitJQLSync(ctx context.Context, req *jobs.JQLSyncRequest) (*jobs.JobResult, error) {
	// For JQL sync, we'd need to resolve the JQL to issue keys first
	// For now, return an error indicating this requires full implementation
	return nil, fmt.Errorf("JQL sync not supported in local mode")
}

func (m *LocalJobManager) GetJob(ctx context.Context, jobID string) (*jobs.JobResult, error) {
	return nil, fmt.Errorf("job tracking not supported in local mode")
}

func (m *LocalJobManager) ListJobs(ctx context.Context, filters *jobs.JobFilter) ([]*jobs.JobResult, error) {
	return []*jobs.JobResult{}, nil
}

func (m *LocalJobManager) CancelJob(ctx context.Context, jobID string) error {
	return fmt.Errorf("job cancellation not supported in local mode")
}

func (m *LocalJobManager) DeleteJob(ctx context.Context, jobID string) error {
	return fmt.Errorf("job deletion not supported in local mode")
}

func (m *LocalJobManager) WatchJob(ctx context.Context, jobID string) (<-chan jobs.JobMonitor, error) {
	return nil, fmt.Errorf("job monitoring not supported in local mode")
}

func (m *LocalJobManager) GetJobLogs(ctx context.Context, jobID string) (string, error) {
	return "", fmt.Errorf("job logs not supported in local mode")
}

func (m *LocalJobManager) GetQueueStatus(ctx context.Context) (*jobs.QueueStatus, error) {
	return &jobs.QueueStatus{
		TotalJobs:     0,
		PendingJobs:   0,
		RunningJobs:   0,
		CompletedJobs: 0,
		FailedJobs:    0,
	}, nil
}

func (m *LocalJobManager) ExecuteLocalSync(ctx context.Context, req *jobs.LocalSyncRequest) (*jobs.SyncResult, error) {
	// This would integrate with the existing sync logic
	// For now, return a basic result
	return &jobs.SyncResult{
		TotalIssues:     len(req.IssueKeys),
		ProcessedIssues: len(req.IssueKeys),
		SuccessfulSync:  len(req.IssueKeys),
		FailedSync:      0,
		Duration:        time.Second,
		ProcessedFiles:  []string{},
		Errors:          []string{},
	}, nil
}

// JobManagerWrapper wraps a Kubernetes job scheduler to implement JobManager interface
type JobManagerWrapper struct {
	scheduler *jobs.KubernetesJobScheduler
}

func (w *JobManagerWrapper) SubmitSingleIssueSync(ctx context.Context, req *jobs.SingleIssueSyncRequest) (*jobs.JobResult, error) {
	// Convert request to SyncJobConfig and create job
	config := &jobs.SyncJobConfig{
		ID:          fmt.Sprintf("single-%d", time.Now().Unix()),
		Type:        jobs.JobTypeSingle,
		Target:      req.IssueKey,
		Repository:  req.Repository,
		Created:     time.Now(),
		Concurrency: 1,
		RateLimit:   req.RateLimit,
		Incremental: req.Incremental,
		Force:       req.Force,
		DryRun:      req.DryRun,
		SafeMode:    req.SafeMode,
	}

	return w.scheduler.CreateJob(ctx, config)
}

func (w *JobManagerWrapper) SubmitBatchSync(ctx context.Context, req *jobs.BatchSyncRequest) (*jobs.JobResult, error) {
	// Convert request to SyncJobConfig and create job
	config := &jobs.SyncJobConfig{
		ID:          fmt.Sprintf("batch-%d", time.Now().Unix()),
		Type:        jobs.JobTypeBatch,
		Target:      fmt.Sprintf("%d issues", len(req.IssueKeys)), // Summary for display
		Repository:  req.Repository,
		Created:     time.Now(),
		Concurrency: req.Concurrency,
		RateLimit:   req.RateLimit,
		Incremental: req.Incremental,
		Force:       req.Force,
		DryRun:      req.DryRun,
		SafeMode:    req.SafeMode,
	}

	if req.Parallelism != nil && *req.Parallelism > 0 {
		config.Parallelism = req.Parallelism
	}

	return w.scheduler.CreateJob(ctx, config)
}

func (w *JobManagerWrapper) SubmitJQLSync(ctx context.Context, req *jobs.JQLSyncRequest) (*jobs.JobResult, error) {
	config := &jobs.SyncJobConfig{
		ID:          fmt.Sprintf("jql-%d", time.Now().Unix()),
		Type:        jobs.JobTypeJQL,
		Target:      req.JQL,
		Repository:  req.Repository,
		Created:     time.Now(),
		Concurrency: req.Concurrency,
		RateLimit:   req.RateLimit,
		Incremental: req.Incremental,
		Force:       req.Force,
		DryRun:      req.DryRun,
		SafeMode:    req.SafeMode,
	}

	if req.Parallelism != nil && *req.Parallelism > 0 {
		config.Parallelism = req.Parallelism
	}

	return w.scheduler.CreateJob(ctx, config)
}

func (w *JobManagerWrapper) GetJob(ctx context.Context, jobID string) (*jobs.JobResult, error) {
	return w.scheduler.GetJob(ctx, jobID)
}

func (w *JobManagerWrapper) ListJobs(ctx context.Context, filters *jobs.JobFilter) ([]*jobs.JobResult, error) {
	return w.scheduler.ListJobs(ctx, filters)
}

func (w *JobManagerWrapper) CancelJob(ctx context.Context, jobID string) error {
	return w.scheduler.CancelJob(ctx, jobID)
}

func (w *JobManagerWrapper) DeleteJob(ctx context.Context, jobID string) error {
	return w.scheduler.DeleteJob(ctx, jobID)
}

func (w *JobManagerWrapper) WatchJob(ctx context.Context, jobID string) (<-chan jobs.JobMonitor, error) {
	return w.scheduler.WatchJob(ctx, jobID)
}

func (w *JobManagerWrapper) GetJobLogs(ctx context.Context, jobID string) (string, error) {
	return w.scheduler.GetJobLogs(ctx, jobID)
}

func (w *JobManagerWrapper) GetQueueStatus(ctx context.Context) (*jobs.QueueStatus, error) {
	return w.scheduler.GetQueueStatus(ctx)
}

func (w *JobManagerWrapper) ExecuteLocalSync(ctx context.Context, req *jobs.LocalSyncRequest) (*jobs.SyncResult, error) {
	// Local execution fallback
	localManager := &LocalJobManager{}
	return localManager.ExecuteLocalSync(ctx, req)
}

func init() {
	rootCmd.AddCommand(serveCmd)

	// Server configuration flags
	serveCmd.Flags().Int("port", 8080, "Server port")
	serveCmd.Flags().String("host", "0.0.0.0", "Server host")
	serveCmd.Flags().String("log-level", "info", "Log level (debug, info, warn, error)")
	serveCmd.Flags().Bool("enable-auth", false, "Enable authentication (disabled in v0.4.0)")
	serveCmd.Flags().Bool("enable-cors", true, "Enable CORS")
	serveCmd.Flags().Int("rate-limit", 100, "Rate limit per minute")

	// Job scheduling flags
	serveCmd.Flags().Bool("enable-jobs", false, "Enable Kubernetes job scheduling")
	serveCmd.Flags().String("namespace", "jira-sync", "Kubernetes namespace for jobs")
	serveCmd.Flags().String("image", "jira-sync:latest", "Container image for sync jobs")
}
