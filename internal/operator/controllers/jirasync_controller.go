package controllers

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/go-logr/logr"
	batchv1 "k8s.io/api/batch/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/metrics"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/chambrid/jira-cdc-git/internal/operator/apiclient"
	operatortypes "github.com/chambrid/jira-cdc-git/internal/operator/types"
)

// JIRASyncReconciler reconciles a JIRASync object
type JIRASyncReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	Log           logr.Logger
	APIHost       string              // v0.4.0 API server host for job triggering
	APIClient     apiclient.APIClient // API client for triggering sync operations
	StatusManager *StatusManager      // Enhanced status management

	// Metrics
	reconcileCounter  prometheus.CounterVec
	reconcileDuration prometheus.HistogramVec
	syncJobsTotal     prometheus.GaugeVec
	apiHealthStatus   prometheus.GaugeVec
	apiCallCounter    prometheus.CounterVec
	apiCallDuration   prometheus.HistogramVec

	// Status-related metrics
	statusUpdateCounter prometheus.CounterVec
	conditionCounter    prometheus.GaugeVec
	progressGauge       prometheus.GaugeVec
}

const (
	// Phase constants
	PhasePending   = "Pending"
	PhaseRunning   = "Running"
	PhaseCompleted = "Completed"
	PhaseFailed    = "Failed"
	PhaseScheduled = "Scheduled"

	// Finalizer
	JIRASyncFinalizer = "sync.jira.io/jirasync-finalizer"

	// Annotations
	RetryCountAnnotation = "sync.jira.io/retry-count"
	LastErrorAnnotation  = "sync.jira.io/last-error"
)

// +kubebuilder:rbac:groups=sync.jira.io,resources=jirasyncs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=sync.jira.io,resources=jirasyncs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=sync.jira.io,resources=jirasyncs/finalizers,verbs=update
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// NewJIRASyncReconciler creates a new JIRASyncReconciler with metrics
func NewJIRASyncReconciler(mgr ctrl.Manager, apiHost string) *JIRASyncReconciler {
	log := ctrl.Log.WithName("controllers").WithName("JIRASync")

	// Create API client for v0.4.0 integration
	apiClient := apiclient.NewAPIClient(apiHost, 30*time.Second, log.WithName("api-client"))

	// Create event recorder
	recorder := mgr.GetEventRecorderFor("jirasync-controller")

	// Create status manager
	statusManager := NewStatusManager(mgr.GetClient(), recorder, log.WithName("status"))

	reconciler := &JIRASyncReconciler{
		Client:        mgr.GetClient(),
		Scheme:        mgr.GetScheme(),
		Log:           log,
		APIHost:       apiHost,
		APIClient:     apiClient,
		StatusManager: statusManager,
	}

	// Initialize metrics
	reconciler.initMetrics()

	return reconciler
}

// initMetrics initializes Prometheus metrics
func (r *JIRASyncReconciler) initMetrics() {
	r.reconcileCounter = *prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "jirasync_reconcile_total",
			Help: "Total number of JIRASync reconciliations",
		},
		[]string{"namespace", "name", "result"},
	)

	r.reconcileDuration = *prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "jirasync_reconcile_duration_seconds",
			Help:    "Duration of JIRASync reconciliations",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"namespace", "name"},
	)

	r.syncJobsTotal = *prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "jirasync_jobs_total",
			Help: "Total number of active sync jobs",
		},
		[]string{"namespace", "phase"},
	)

	r.apiHealthStatus = *prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "jirasync_api_health_status",
			Help: "Health status of the v0.4.0 API server (1=healthy, 0=unhealthy)",
		},
		[]string{"api_host"},
	)

	r.apiCallCounter = *prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "jirasync_api_calls_total",
			Help: "Total number of API calls made to v0.4.0 server",
		},
		[]string{"endpoint", "status"},
	)

	r.apiCallDuration = *prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "jirasync_api_call_duration_seconds",
			Help:    "Duration of API calls to v0.4.0 server",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"endpoint"},
	)

	r.statusUpdateCounter = *prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "jirasync_status_updates_total",
			Help: "Total number of status updates",
		},
		[]string{"namespace", "name", "phase"},
	)

	r.conditionCounter = *prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "jirasync_conditions",
			Help: "Current conditions status (1=True, 0=False)",
		},
		[]string{"namespace", "name", "type"},
	)

	r.progressGauge = *prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "jirasync_progress_percentage",
			Help: "Current progress percentage",
		},
		[]string{"namespace", "name", "stage"},
	)

	// Register metrics with controller-runtime's metrics registry
	metrics.Registry.MustRegister(&r.reconcileCounter, &r.reconcileDuration, &r.syncJobsTotal,
		&r.apiHealthStatus, &r.apiCallCounter, &r.apiCallDuration,
		&r.statusUpdateCounter, &r.conditionCounter, &r.progressGauge)
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *JIRASyncReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	start := time.Now()
	log := r.Log.WithValues("jirasync", req.NamespacedName)

	// Metrics tracking
	defer func() {
		duration := time.Since(start).Seconds()
		r.reconcileDuration.WithLabelValues(req.Namespace, req.Name).Observe(duration)
	}()

	// Fetch the JIRASync instance
	var jiraSync operatortypes.JIRASync
	if err := r.Get(ctx, req.NamespacedName, &jiraSync); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("JIRASync resource not found. Ignoring since object must be deleted")
			r.reconcileCounter.WithLabelValues(req.Namespace, req.Name, "not_found").Inc()
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get JIRASync")
		r.reconcileCounter.WithLabelValues(req.Namespace, req.Name, "error").Inc()
		return ctrl.Result{}, err
	}

	// Handle deletion
	if !jiraSync.DeletionTimestamp.IsZero() {
		result, err := r.handleDeletion(ctx, &jiraSync)
		if err != nil {
			r.reconcileCounter.WithLabelValues(req.Namespace, req.Name, "deletion_error").Inc()
		} else {
			r.reconcileCounter.WithLabelValues(req.Namespace, req.Name, "deleted").Inc()
		}
		return result, err
	}

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(&jiraSync, JIRASyncFinalizer) {
		controllerutil.AddFinalizer(&jiraSync, JIRASyncFinalizer)
		if err := r.Update(ctx, &jiraSync); err != nil {
			r.reconcileCounter.WithLabelValues(req.Namespace, req.Name, "finalizer_error").Inc()
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Update observed generation
	if jiraSync.Status.ObservedGeneration != jiraSync.Generation {
		jiraSync.Status.ObservedGeneration = jiraSync.Generation
		if err := r.Status().Update(ctx, &jiraSync); err != nil {
			r.reconcileCounter.WithLabelValues(req.Namespace, req.Name, "status_error").Inc()
			return ctrl.Result{}, err
		}
	}

	// Update metrics
	r.syncJobsTotal.WithLabelValues(req.Namespace, jiraSync.Status.Phase).Inc()

	// Reconcile based on current phase
	var result ctrl.Result
	var err error

	switch jiraSync.Status.Phase {
	case "":
		result, err = r.initializeSync(ctx, &jiraSync)
	case PhasePending:
		result, err = r.handlePending(ctx, &jiraSync)
	case PhaseRunning:
		result, err = r.handleRunning(ctx, &jiraSync)
	case PhaseCompleted:
		result, err = r.handleCompleted(ctx, &jiraSync)
	case PhaseFailed:
		result, err = r.handleFailed(ctx, &jiraSync)
	default:
		result, err = r.initializeSync(ctx, &jiraSync)
	}

	if err != nil {
		r.reconcileCounter.WithLabelValues(req.Namespace, req.Name, "reconcile_error").Inc()
	} else {
		r.reconcileCounter.WithLabelValues(req.Namespace, req.Name, "success").Inc()
	}

	return result, err
}

// initializeSync sets up a new sync operation
func (r *JIRASyncReconciler) initializeSync(ctx context.Context, jiraSync *operatortypes.JIRASync) (ctrl.Result, error) {
	log := r.Log.WithValues("jirasync", client.ObjectKeyFromObject(jiraSync))
	log.Info("Initializing JIRASync")

	// Set initialization progress
	if err := r.StatusManager.UpdateProgress(ctx, jiraSync, 10, "Validating sync specification", StageInitialization); err != nil {
		log.Error(err, "Failed to update initialization progress")
	}

	// Validate the sync specification
	if err := r.validateSyncSpec(&jiraSync.Spec); err != nil {
		update := StatusUpdate{
			Phase: PhaseFailed,
			Error: err,
			Conditions: []metav1.Condition{{
				Type:    ConditionTypeFailed,
				Status:  metav1.ConditionTrue,
				Reason:  ReasonValidationFailed,
				Message: "Sync specification validation failed: " + err.Error(),
			}},
		}
		if err := r.StatusManager.UpdateStatus(ctx, jiraSync, update); err != nil {
			log.Error(err, "Failed to update status")
		}
		r.updateStatusMetrics(jiraSync)
		return ctrl.Result{}, nil
	}

	// Initialize sync state and statistics
	startTime := time.Now()
	update := StatusUpdate{
		Phase: PhasePending,
		Progress: &ProgressUpdate{
			Percentage:       &[]int{25}[0],
			CurrentOperation: "Sync specification validated",
			Stage:            StageInitialization,
		},
		SyncStats: &SyncStatsUpdate{
			StartTime: &startTime,
		},
		SyncState: &SyncStateUpdate{
			OperationID: fmt.Sprintf("sync-%d", time.Now().Unix()),
			ConfigHash:  r.StatusManager.GenerateConfigHash(&jiraSync.Spec),
			Metadata: map[string]string{
				"syncType":  jiraSync.Spec.SyncType,
				"initiated": startTime.Format(time.RFC3339),
			},
		},
		Conditions: []metav1.Condition{
			{
				Type:    ConditionTypeValidated,
				Status:  metav1.ConditionTrue,
				Reason:  ReasonValidating,
				Message: "Sync specification validated successfully",
			},
			{
				Type:    ConditionTypeReady,
				Status:  metav1.ConditionFalse,
				Reason:  ReasonInitializing,
				Message: "Sync initialized, waiting for scheduling",
			},
		},
		ClearError: true,
	}

	if err := r.StatusManager.UpdateStatus(ctx, jiraSync, update); err != nil {
		log.Error(err, "Failed to update status during initialization")
		return ctrl.Result{}, err
	}

	r.updateStatusMetrics(jiraSync)
	return ctrl.Result{}, nil
}

// handlePending processes a pending sync by triggering API operations
func (r *JIRASyncReconciler) handlePending(ctx context.Context, jiraSync *operatortypes.JIRASync) (ctrl.Result, error) {
	log := r.Log.WithValues("jirasync", client.ObjectKeyFromObject(jiraSync))
	log.Info("Handling pending JIRASync - triggering API call")

	// Check if sync is already running (has a job ID)
	if jiraSync.Status.JobRef != nil && jiraSync.Status.JobRef.Name != "" {
		log.Info("Sync already has job ID, moving to running phase")
		return r.updateStatus(ctx, jiraSync, PhaseRunning, "API sync operation already triggered")
	}

	// Convert JIRASync to API request
	request, requestType, err := apiclient.ConvertJIRASyncToAPIRequest(jiraSync)
	if err != nil {
		r.recordError(jiraSync, err)
		return r.updateStatus(ctx, jiraSync, PhaseFailed, "Failed to convert sync spec: "+err.Error())
	}

	log.Info("Triggering API sync operation", "type", requestType)

	// Trigger the appropriate API call based on sync type with metrics
	var response *apiclient.SyncJobResponse
	var endpoint string
	startTime := time.Now()

	switch requestType {
	case "single":
		endpoint = "/api/v1/sync/single"
		response, err = r.APIClient.TriggerSingleSync(ctx, request.(*apiclient.SingleSyncRequest))
	case "batch":
		endpoint = "/api/v1/sync/batch"
		response, err = r.APIClient.TriggerBatchSync(ctx, request.(*apiclient.BatchSyncRequest))
	case "jql":
		endpoint = "/api/v1/sync/jql"
		response, err = r.APIClient.TriggerJQLSync(ctx, request.(*apiclient.JQLSyncRequest))
	default:
		err = fmt.Errorf("unsupported request type: %s", requestType)
	}

	// Record API call metrics
	duration := time.Since(startTime)
	status := "success"
	if err != nil {
		status = "error"
	}
	r.recordAPICall(endpoint, status, duration)

	if err != nil {
		log.Error(err, "Failed to trigger API sync operation")
		r.recordError(jiraSync, err)
		return r.updateStatus(ctx, jiraSync, PhaseFailed, "Failed to trigger sync: "+err.Error())
	}

	// Update status with API job reference
	jiraSync.Status.JobRef = &operatortypes.JobReference{
		Name:      response.JobID,
		Namespace: "api", // Special namespace indicating this is an API job
	}

	log.Info("API sync operation triggered successfully", "jobID", response.JobID)
	return r.updateStatus(ctx, jiraSync, PhaseRunning, fmt.Sprintf("API sync operation triggered: %s", response.JobID))
}

// handleRunning monitors a running sync operation via API
func (r *JIRASyncReconciler) handleRunning(ctx context.Context, jiraSync *operatortypes.JIRASync) (ctrl.Result, error) {
	log := r.Log.WithValues("jirasync", client.ObjectKeyFromObject(jiraSync))
	log.Info("Handling running JIRASync")

	if jiraSync.Status.JobRef == nil || jiraSync.Status.JobRef.Name == "" {
		log.Info("No job reference found, moving back to pending")
		return r.updateStatus(ctx, jiraSync, PhasePending, "No job reference found")
	}

	// Check if this is an API job (namespace = "api") or legacy Kubernetes job
	if jiraSync.Status.JobRef.Namespace == "api" {
		// This is an API job, check status via API
		return r.handleAPIJobStatus(ctx, jiraSync)
	}

	// Legacy Kubernetes job handling (for backward compatibility)
	return r.handleKubernetesJobStatus(ctx, jiraSync)
}

// handleAPIJobStatus checks the status of an API job
func (r *JIRASyncReconciler) handleAPIJobStatus(ctx context.Context, jiraSync *operatortypes.JIRASync) (ctrl.Result, error) {
	log := r.Log.WithValues("jirasync", client.ObjectKeyFromObject(jiraSync), "jobID", jiraSync.Status.JobRef.Name)
	log.Info("Checking API job status")

	// Get job status from API
	jobStatus, err := r.APIClient.GetJobStatus(ctx, jiraSync.Status.JobRef.Name)
	if err != nil {
		log.Error(err, "Failed to get job status from API")
		r.recordError(jiraSync, err)
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil // Retry after 30 seconds
	}

	log.Info("API job status received", "status", jobStatus.Status, "progress", jobStatus.Progress)

	// Update sync stats with progress
	// Note: API doesn't provide issue counts yet, so we keep existing values

	switch jobStatus.Status {
	case "completed":
		// Job completed successfully
		if jiraSync.Status.SyncStats != nil && jiraSync.Status.SyncStats.StartTime != nil {
			duration := time.Since(jiraSync.Status.SyncStats.StartTime.Time)
			jiraSync.Status.SyncStats.Duration = duration.String()
			jiraSync.Status.SyncStats.LastSyncTime = &metav1.Time{Time: time.Now()}
		}

		// Clear any previous error
		r.clearError(jiraSync)

		return r.updateStatus(ctx, jiraSync, PhaseCompleted, "API sync completed successfully")

	case "failed":
		// Job failed
		errorMsg := "API sync operation failed"
		if jobStatus.Message != "" {
			errorMsg += ": " + jobStatus.Message
		}
		r.recordError(jiraSync, fmt.Errorf("%s", errorMsg))
		return r.updateStatus(ctx, jiraSync, PhaseFailed, errorMsg)

	case "running", "pending":
		// Job still running, requeue for later check
		message := fmt.Sprintf("API sync in progress (status: %s)", jobStatus.Status)
		if jobStatus.Progress > 0 {
			message = fmt.Sprintf("API sync in progress (status: %s), progress: %d%%", jobStatus.Status, jobStatus.Progress)
		}

		// Update status without changing phase
		if err := r.Status().Update(ctx, jiraSync); err != nil {
			log.Error(err, "Failed to update status")
		}

		log.Info(message)
		return ctrl.Result{RequeueAfter: 15 * time.Second}, nil // Check again in 15 seconds

	default:
		// Unknown status
		log.Info("Unknown job status", "status", jobStatus.Status)
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}
}

// handleKubernetesJobStatus handles legacy Kubernetes job status checking
func (r *JIRASyncReconciler) handleKubernetesJobStatus(ctx context.Context, jiraSync *operatortypes.JIRASync) (ctrl.Result, error) {
	log := r.Log.WithValues("jirasync", client.ObjectKeyFromObject(jiraSync))
	log.Info("Checking Kubernetes job status (legacy mode)")

	// Fetch the Job
	var job batchv1.Job
	err := r.Get(ctx, types.NamespacedName{
		Name:      jiraSync.Status.JobRef.Name,
		Namespace: jiraSync.Status.JobRef.Namespace,
	}, &job)

	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Job not found, may have been deleted")
			return r.updateStatus(ctx, jiraSync, PhaseFailed, "Associated job not found")
		}
		log.Error(err, "Failed to get Job")
		r.recordError(jiraSync, err)
		return ctrl.Result{}, err
	}

	// Check job status
	if job.Status.Succeeded > 0 {
		// Job completed successfully
		if jiraSync.Status.SyncStats != nil && jiraSync.Status.SyncStats.StartTime != nil {
			duration := time.Since(jiraSync.Status.SyncStats.StartTime.Time)
			jiraSync.Status.SyncStats.Duration = duration.String()
			jiraSync.Status.SyncStats.LastSyncTime = &metav1.Time{Time: time.Now()}
		}

		// Clear any previous error
		r.clearError(jiraSync)

		return r.updateStatus(ctx, jiraSync, PhaseCompleted, "Sync completed successfully")
	}

	if job.Status.Failed > 0 {
		// Job failed
		err := fmt.Errorf("sync job failed after %d attempts", job.Status.Failed)
		r.recordError(jiraSync, err)
		return r.updateStatus(ctx, jiraSync, PhaseFailed, "Sync job failed")
	}

	// Job still running, requeue for later check
	return ctrl.Result{RequeueAfter: time.Minute * 2}, nil
}

// handleCompleted processes a completed sync
func (r *JIRASyncReconciler) handleCompleted(ctx context.Context, jiraSync *operatortypes.JIRASync) (ctrl.Result, error) {
	log := r.Log.WithValues("jirasync", client.ObjectKeyFromObject(jiraSync))
	log.Info("Handling completed JIRASync")

	// If this is a scheduled sync, we might want to clean up and prepare for next run
	// For now, just ensure status is correct
	return ctrl.Result{}, nil
}

// handleFailed processes a failed sync with enhanced retry logic
func (r *JIRASyncReconciler) handleFailed(ctx context.Context, jiraSync *operatortypes.JIRASync) (ctrl.Result, error) {
	log := r.Log.WithValues("jirasync", client.ObjectKeyFromObject(jiraSync))
	log.Info("Handling failed JIRASync")

	// Implement retry logic if configured
	if jiraSync.Spec.RetryPolicy != nil && jiraSync.Spec.RetryPolicy.MaxRetries > 0 {
		// Check if we should retry
		retryCount := r.getRetryCount(jiraSync)
		if retryCount < jiraSync.Spec.RetryPolicy.MaxRetries {
			log.Info("Retrying failed sync", "retryCount", retryCount, "maxRetries", jiraSync.Spec.RetryPolicy.MaxRetries)

			// Increment retry count
			r.incrementRetryCount(jiraSync)

			// Calculate backoff delay
			delay := time.Duration(jiraSync.Spec.RetryPolicy.InitialDelay) * time.Second
			for i := 0; i < retryCount; i++ {
				delay = time.Duration(float64(delay) * jiraSync.Spec.RetryPolicy.BackoffMultiplier)
			}

			// Update the JIRASync with the new retry count
			if err := r.Update(ctx, jiraSync); err != nil {
				return ctrl.Result{}, err
			}

			return r.updateStatusWithDelay(ctx, jiraSync, PhasePending,
				fmt.Sprintf("Retrying sync (attempt %d/%d)", retryCount+1, jiraSync.Spec.RetryPolicy.MaxRetries),
				delay)
		}
	}

	// No more retries, keep in failed state
	return ctrl.Result{}, nil
}

// handleDeletion handles cleanup when JIRASync is being deleted
func (r *JIRASyncReconciler) handleDeletion(ctx context.Context, jiraSync *operatortypes.JIRASync) (ctrl.Result, error) {
	log := r.Log.WithValues("jirasync", client.ObjectKeyFromObject(jiraSync))
	log.Info("Handling JIRASync deletion")

	// Clean up associated Job if it exists
	if jiraSync.Status.JobRef != nil {
		var job batchv1.Job
		err := r.Get(ctx, types.NamespacedName{
			Name:      jiraSync.Status.JobRef.Name,
			Namespace: jiraSync.Status.JobRef.Namespace,
		}, &job)

		if err == nil {
			// Delete the job
			if err := r.Delete(ctx, &job); err != nil && !apierrors.IsNotFound(err) {
				log.Error(err, "Failed to delete associated Job")
				return ctrl.Result{}, err
			}
			log.Info("Deleted associated Job", "jobName", job.Name)
		}
	}

	// Remove finalizer
	controllerutil.RemoveFinalizer(jiraSync, JIRASyncFinalizer)
	if err := r.Update(ctx, jiraSync); err != nil {
		return ctrl.Result{}, err
	}

	log.Info("Successfully deleted JIRASync")
	return ctrl.Result{}, nil
}

// Legacy functions for creating Kubernetes Jobs are no longer used in v0.4.1
// as we now use the API server for job triggering. These functions are kept
// for backward compatibility with legacy Kubernetes Job handling in handleKubernetesJobStatus.

func (r *JIRASyncReconciler) buildSyncArgs(jiraSync *operatortypes.JIRASync) []string {
	args := []string{"sync"}

	switch jiraSync.Spec.SyncType {
	case "single", "batch":
		if len(jiraSync.Spec.Target.IssueKeys) > 0 {
			args = append(args, "--issues", fmt.Sprintf("%v", jiraSync.Spec.Target.IssueKeys))
		}
	case "jql":
		if jiraSync.Spec.Target.JQLQuery != "" {
			args = append(args, "--jql", jiraSync.Spec.Target.JQLQuery)
		}
	case "incremental":
		args = append(args, "--incremental")
		if jiraSync.Spec.Target.ProjectKey != "" {
			args = append(args, "--project", jiraSync.Spec.Target.ProjectKey)
		}
	}

	args = append(args, "--repo", jiraSync.Spec.Destination.Repository)

	if jiraSync.Spec.Destination.Branch != "" {
		args = append(args, "--branch", jiraSync.Spec.Destination.Branch)
	}

	return args
}

func (r *JIRASyncReconciler) validateSyncSpec(spec *operatortypes.JIRASyncSpec) error {
	if spec.SyncType == "" {
		return fmt.Errorf("syncType is required")
	}

	if spec.Destination.Repository == "" {
		return fmt.Errorf("destination repository is required")
	}

	// Validate target based on sync type
	switch spec.SyncType {
	case "single", "batch":
		if len(spec.Target.IssueKeys) == 0 {
			return fmt.Errorf("issueKeys required for %s sync type", spec.SyncType)
		}
	case "jql":
		if spec.Target.JQLQuery == "" {
			return fmt.Errorf("jqlQuery required for jql sync type")
		}
	case "incremental":
		if spec.Target.ProjectKey == "" && spec.Target.JQLQuery == "" {
			return fmt.Errorf("projectKey or jqlQuery required for incremental sync type")
		}
	default:
		return fmt.Errorf("invalid syncType: %s", spec.SyncType)
	}

	return nil
}

func (r *JIRASyncReconciler) updateStatus(ctx context.Context, jiraSync *operatortypes.JIRASync, phase, message string) (ctrl.Result, error) {
	jiraSync.Status.Phase = phase

	// Update condition
	condition := metav1.Condition{
		Type:               ConditionTypeReady,
		Status:             metav1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
		Reason:             "SyncProgressing",
		Message:            message,
	}

	switch phase {
	case PhaseFailed:
		condition.Status = metav1.ConditionFalse
		condition.Reason = "SyncFailed"
	case PhaseCompleted:
		condition.Reason = "SyncCompleted"
	}

	// Update or add condition
	r.setCondition(&jiraSync.Status.Conditions, condition)

	if err := r.Status().Update(ctx, jiraSync); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *JIRASyncReconciler) updateStatusWithDelay(ctx context.Context, jiraSync *operatortypes.JIRASync, phase, message string, delay time.Duration) (ctrl.Result, error) {
	result, err := r.updateStatus(ctx, jiraSync, phase, message)
	if err != nil {
		return result, err
	}

	// Return with delay for retry
	return ctrl.Result{RequeueAfter: delay}, nil
}

func (r *JIRASyncReconciler) setCondition(conditions *[]metav1.Condition, newCondition metav1.Condition) {
	for i, condition := range *conditions {
		if condition.Type == newCondition.Type {
			(*conditions)[i] = newCondition
			return
		}
	}
	*conditions = append(*conditions, newCondition)
}

func (r *JIRASyncReconciler) getRetryCount(jiraSync *operatortypes.JIRASync) int {
	if retryAnnotation, exists := jiraSync.Annotations[RetryCountAnnotation]; exists {
		if count, err := strconv.Atoi(retryAnnotation); err == nil {
			return count
		}
	}
	return 0
}

func (r *JIRASyncReconciler) incrementRetryCount(jiraSync *operatortypes.JIRASync) {
	if jiraSync.Annotations == nil {
		jiraSync.Annotations = make(map[string]string)
	}

	currentCount := r.getRetryCount(jiraSync)
	jiraSync.Annotations[RetryCountAnnotation] = strconv.Itoa(currentCount + 1)
}

func (r *JIRASyncReconciler) recordError(jiraSync *operatortypes.JIRASync, err error) {
	if jiraSync.Annotations == nil {
		jiraSync.Annotations = make(map[string]string)
	}

	jiraSync.Annotations[LastErrorAnnotation] = err.Error()
}

func (r *JIRASyncReconciler) clearError(jiraSync *operatortypes.JIRASync) {
	if jiraSync.Annotations != nil {
		delete(jiraSync.Annotations, LastErrorAnnotation)
		delete(jiraSync.Annotations, RetryCountAnnotation)
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *JIRASyncReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&operatortypes.JIRASync{}).
		Owns(&batchv1.Job{}).
		Complete(r)
}

// recordAPICall records metrics for API calls
func (r *JIRASyncReconciler) recordAPICall(endpoint, status string, duration time.Duration) {
	r.apiCallCounter.WithLabelValues(endpoint, status).Inc()
	r.apiCallDuration.WithLabelValues(endpoint).Observe(duration.Seconds())
}

// performHealthCheck checks the health of the API server and updates metrics
func (r *JIRASyncReconciler) performHealthCheck(ctx context.Context) {
	log := r.Log.WithName("health-check")

	err := r.APIClient.HealthCheck(ctx)
	if err != nil {
		log.Error(err, "API health check failed")
		r.apiHealthStatus.WithLabelValues(r.APIHost).Set(0) // Unhealthy
	} else {
		log.V(1).Info("API health check passed")
		r.apiHealthStatus.WithLabelValues(r.APIHost).Set(1) // Healthy
	}
}

// StartHealthCheckRoutine starts a background goroutine for periodic health checks
func (r *JIRASyncReconciler) StartHealthCheckRoutine(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second) // Health check every 30 seconds
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				r.performHealthCheck(ctx)
			}
		}
	}()
}

// updateStatusMetrics updates Prometheus metrics based on current status
func (r *JIRASyncReconciler) updateStatusMetrics(jiraSync *operatortypes.JIRASync) {
	namespace := jiraSync.Namespace
	name := jiraSync.Name

	// Update status update counter
	r.statusUpdateCounter.WithLabelValues(namespace, name, jiraSync.Status.Phase).Inc()

	// Update condition metrics
	for _, condition := range jiraSync.Status.Conditions {
		value := 0.0
		if condition.Status == metav1.ConditionTrue {
			value = 1.0
		}
		r.conditionCounter.WithLabelValues(namespace, name, condition.Type).Set(value)
	}

	// Update progress metrics
	if jiraSync.Status.Progress != nil {
		stage := jiraSync.Status.Progress.Stage
		if stage == "" {
			stage = "unknown"
		}
		r.progressGauge.WithLabelValues(namespace, name, stage).Set(float64(jiraSync.Status.Progress.Percentage))
	}
}
