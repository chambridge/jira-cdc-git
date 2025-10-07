package controllers

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/go-logr/logr"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/metrics"

	"github.com/prometheus/client_golang/prometheus"

	operatortypes "github.com/chambrid/jira-cdc-git/internal/operator/types"
)

// JIRASyncReconciler reconciles a JIRASync object
type JIRASyncReconciler struct {
	client.Client
	Scheme  *runtime.Scheme
	Log     logr.Logger
	APIHost string // v0.4.0 API server host for job triggering

	// Metrics
	reconcileCounter  prometheus.CounterVec
	reconcileDuration prometheus.HistogramVec
	syncJobsTotal     prometheus.GaugeVec
}

const (
	// Condition types
	ConditionTypeReady      = "Ready"
	ConditionTypeProcessing = "Processing"
	ConditionTypeFailed     = "Failed"

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
	reconciler := &JIRASyncReconciler{
		Client:  mgr.GetClient(),
		Scheme:  mgr.GetScheme(),
		Log:     ctrl.Log.WithName("controllers").WithName("JIRASync"),
		APIHost: apiHost,
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

	// Register metrics with controller-runtime's metrics registry
	metrics.Registry.MustRegister(&r.reconcileCounter, &r.reconcileDuration, &r.syncJobsTotal)
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

	// Validate the sync specification
	if err := r.validateSyncSpec(&jiraSync.Spec); err != nil {
		r.recordError(jiraSync, err)
		return r.updateStatus(ctx, jiraSync, PhaseFailed, "Validation failed: "+err.Error())
	}

	// Initialize statistics
	if jiraSync.Status.SyncStats == nil {
		jiraSync.Status.SyncStats = &operatortypes.SyncStats{
			StartTime: &metav1.Time{Time: time.Now()},
		}
	}

	// Set to pending and create the job
	return r.updateStatus(ctx, jiraSync, PhasePending, "Sync initialized and validated")
}

// handlePending processes a pending sync by creating a Job
func (r *JIRASyncReconciler) handlePending(ctx context.Context, jiraSync *operatortypes.JIRASync) (ctrl.Result, error) {
	log := r.Log.WithValues("jirasync", client.ObjectKeyFromObject(jiraSync))
	log.Info("Handling pending JIRASync")

	// Check if Job already exists
	var job batchv1.Job
	jobName := r.generateJobName(jiraSync)
	err := r.Get(ctx, types.NamespacedName{Name: jobName, Namespace: jiraSync.Namespace}, &job)

	if err != nil && apierrors.IsNotFound(err) {
		// Create new Job
		newJob, err := r.createSyncJob(ctx, jiraSync)
		if err != nil {
			r.recordError(jiraSync, err)
			return r.updateStatus(ctx, jiraSync, PhaseFailed, "Failed to create Job: "+err.Error())
		}

		// Update status with job reference
		jiraSync.Status.JobRef = &operatortypes.JobReference{
			Name:      newJob.Name,
			Namespace: newJob.Namespace,
		}

		return r.updateStatus(ctx, jiraSync, PhaseRunning, "Sync job created and running")
	} else if err != nil {
		log.Error(err, "Failed to check for existing Job")
		r.recordError(jiraSync, err)
		return ctrl.Result{}, err
	}

	// Job already exists, move to running
	jiraSync.Status.JobRef = &operatortypes.JobReference{
		Name:      job.Name,
		Namespace: job.Namespace,
	}
	return r.updateStatus(ctx, jiraSync, PhaseRunning, "Sync job found and running")
}

// handleRunning monitors a running sync job
func (r *JIRASyncReconciler) handleRunning(ctx context.Context, jiraSync *operatortypes.JIRASync) (ctrl.Result, error) {
	log := r.Log.WithValues("jirasync", client.ObjectKeyFromObject(jiraSync))
	log.Info("Handling running JIRASync")

	if jiraSync.Status.JobRef == nil {
		log.Info("No job reference found, moving back to pending")
		return r.updateStatus(ctx, jiraSync, PhasePending, "No job reference found")
	}

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
		duration := time.Since(jiraSync.Status.SyncStats.StartTime.Time)
		jiraSync.Status.SyncStats.Duration = duration.String()
		jiraSync.Status.SyncStats.LastSyncTime = &metav1.Time{Time: time.Now()}

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

// createSyncJob creates a Kubernetes Job for the sync operation
func (r *JIRASyncReconciler) createSyncJob(ctx context.Context, jiraSync *operatortypes.JIRASync) (*batchv1.Job, error) {
	log := r.Log.WithValues("jirasync", client.ObjectKeyFromObject(jiraSync))

	jobName := r.generateJobName(jiraSync)

	// Build command arguments based on sync type
	args := r.buildSyncArgs(jiraSync)

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: jiraSync.Namespace,
			Labels: map[string]string{
				"app":                     "jira-sync",
				"sync.jira.io/sync-type":  jiraSync.Spec.SyncType,
				"sync.jira.io/managed-by": "jira-sync-operator",
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: func() *int32 { i := int32(1); return &i }(), // Only 1 retry at job level
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{
						{
							Name:  "jira-sync",
							Image: "jira-sync:latest", // This would be configurable in production
							Args:  args,
							Env: []corev1.EnvVar{
								{
									Name: "JIRA_BASE_URL",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: "jira-credentials",
											},
											Key: "base-url",
										},
									},
								},
								{
									Name: "JIRA_EMAIL",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: "jira-credentials",
											},
											Key: "email",
										},
									},
								},
								{
									Name: "JIRA_PAT",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: "jira-credentials",
											},
											Key: "pat",
										},
									},
								},
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("100m"),
									corev1.ResourceMemory: resource.MustParse("128Mi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("500m"),
									corev1.ResourceMemory: resource.MustParse("512Mi"),
								},
							},
						},
					},
				},
			},
		},
	}

	// Set owner reference
	if err := controllerutil.SetControllerReference(jiraSync, job, r.Scheme); err != nil {
		return nil, err
	}

	// Create the Job
	if err := r.Create(ctx, job); err != nil {
		return nil, err
	}

	log.Info("Created sync job", "jobName", jobName)
	return job, nil
}

// Helper functions

func (r *JIRASyncReconciler) generateJobName(jiraSync *operatortypes.JIRASync) string {
	return fmt.Sprintf("%s-%d", jiraSync.Name, time.Now().Unix())
}

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
