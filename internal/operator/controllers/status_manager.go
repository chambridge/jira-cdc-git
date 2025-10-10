package controllers

import (
	"context"
	"crypto/sha256"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatortypes "github.com/chambrid/jira-cdc-git/internal/operator/types"
)

// StatusManager handles comprehensive status updates and condition management
type StatusManager struct {
	client   client.Client
	recorder record.EventRecorder
	log      logr.Logger
}

// NewStatusManager creates a new StatusManager
func NewStatusManager(client client.Client, recorder record.EventRecorder, log logr.Logger) *StatusManager {
	return &StatusManager{
		client:   client,
		recorder: recorder,
		log:      log.WithName("status-manager"),
	}
}

// Condition types following Kubernetes conventions
const (
	// Core condition types
	ConditionTypeReady       = "Ready"
	ConditionTypeProcessing  = "Processing"
	ConditionTypeFailed      = "Failed"
	ConditionTypeProgressing = "Progressing"

	// Extended condition types
	ConditionTypeValidated = "Validated"
	ConditionTypeScheduled = "Scheduled"
	ConditionTypeHealthy   = "Healthy"
	ConditionTypeDegraded  = "Degraded"
)

// Standard condition reasons
const (
	ReasonInitializing     = "Initializing"
	ReasonValidating       = "Validating"
	ReasonScheduling       = "Scheduling"
	ReasonProcessing       = "Processing"
	ReasonCompleted        = "Completed"
	ReasonFailed           = "Failed"
	ReasonRetrying         = "Retrying"
	ReasonValidationFailed = "ValidationFailed"
	ReasonAPIError         = "APIError"
	ReasonJobError         = "JobError"
	ReasonConfigChanged    = "ConfigurationChanged"
	ReasonHealthCheck      = "HealthCheck"
)

// Sync stages for progress tracking
const (
	StageInitialization = "Initialization"
	StageValidation     = "Validation"
	StageScheduling     = "Scheduling"
	StageExecution      = "Execution"
	StageCompletion     = "Completion"
	StageCleanup        = "Cleanup"
)

// Health status values
const (
	HealthStatusHealthy   = "Healthy"
	HealthStatusDegraded  = "Degraded"
	HealthStatusUnhealthy = "Unhealthy"
	HealthStatusUnknown   = "Unknown"
)

// UpdateStatus provides a comprehensive status update with proper condition management
func (sm *StatusManager) UpdateStatus(ctx context.Context, jiraSync *operatortypes.JIRASync, update StatusUpdate) error {
	log := sm.log.WithValues("jirasync", client.ObjectKeyFromObject(jiraSync))

	// Update last status update timestamp
	now := metav1.Now()
	jiraSync.Status.LastStatusUpdate = &now

	// Update phase if provided
	if update.Phase != "" {
		previousPhase := jiraSync.Status.Phase
		jiraSync.Status.Phase = update.Phase

		// Emit event for phase changes
		if previousPhase != update.Phase {
			sm.emitPhaseChangeEvent(jiraSync, previousPhase, update.Phase)
		}
	}

	// Update conditions
	if len(update.Conditions) > 0 {
		for _, condition := range update.Conditions {
			sm.setCondition(&jiraSync.Status.Conditions, condition)
		}
	}

	// Update progress information
	if update.Progress != nil {
		if jiraSync.Status.Progress == nil {
			jiraSync.Status.Progress = &operatortypes.ProgressInfo{}
		}
		sm.updateProgress(jiraSync.Status.Progress, update.Progress)
	}

	// Update sync state
	if update.SyncState != nil {
		if jiraSync.Status.SyncState == nil {
			jiraSync.Status.SyncState = &operatortypes.SyncState{}
		}
		sm.updateSyncState(jiraSync.Status.SyncState, update.SyncState)
	}

	// Update error information
	if update.Error != nil {
		jiraSync.Status.LastError = update.Error.Error()
		jiraSync.Status.RetryCount = update.RetryCount
		sm.emitErrorEvent(jiraSync, update.Error)
	} else if update.ClearError {
		jiraSync.Status.LastError = ""
		jiraSync.Status.RetryCount = 0
	}

	// Update sync stats
	if update.SyncStats != nil {
		if jiraSync.Status.SyncStats == nil {
			jiraSync.Status.SyncStats = &operatortypes.SyncStats{}
		}
		sm.updateSyncStats(jiraSync.Status.SyncStats, update.SyncStats)
	}

	// Update job reference
	if update.JobRef != nil {
		jiraSync.Status.JobRef = update.JobRef
	}

	// Update health status
	if jiraSync.Status.SyncState == nil {
		jiraSync.Status.SyncState = &operatortypes.SyncState{}
	}
	jiraSync.Status.SyncState.HealthStatus = sm.calculateHealthStatus(jiraSync)

	// Update observed generation
	jiraSync.Status.ObservedGeneration = jiraSync.Generation

	// Persist status update
	if err := sm.client.Status().Update(ctx, jiraSync); err != nil {
		log.Error(err, "Failed to update JIRASync status")
		return fmt.Errorf("failed to update status: %w", err)
	}

	log.V(1).Info("Successfully updated JIRASync status",
		"phase", jiraSync.Status.Phase,
		"conditions", len(jiraSync.Status.Conditions),
		"healthStatus", jiraSync.Status.SyncState.HealthStatus)

	return nil
}

// StatusUpdate represents a comprehensive status update
type StatusUpdate struct {
	Phase      string
	Conditions []metav1.Condition
	Progress   *ProgressUpdate
	SyncState  *SyncStateUpdate
	SyncStats  *SyncStatsUpdate
	JobRef     *operatortypes.JobReference
	Error      error
	RetryCount int
	ClearError bool
}

// ProgressUpdate represents progress information updates
type ProgressUpdate struct {
	Percentage          *int
	CurrentOperation    string
	TotalOperations     *int
	CompletedOperations *int
	EstimatedCompletion *time.Time
	ProcessingRate      *float64
	Stage               string
}

// SyncStateUpdate represents sync state updates
type SyncStateUpdate struct {
	OperationID          string
	ConfigHash           string
	ActiveIssues         []string
	LastSuccessfulConfig string
	Metadata             map[string]string
	AddMetadata          map[string]string // Additional metadata to merge
}

// SyncStatsUpdate represents sync statistics updates
type SyncStatsUpdate struct {
	TotalIssues     *int
	ProcessedIssues *int
	FailedIssues    *int
	Duration        string
	StartTime       *time.Time
	LastSyncTime    *time.Time
}

// UpdateProgress updates progress information for long-running operations
func (sm *StatusManager) UpdateProgress(ctx context.Context, jiraSync *operatortypes.JIRASync,
	percentage int, operation string, stage string) error {

	update := StatusUpdate{
		Progress: &ProgressUpdate{
			Percentage:       &percentage,
			CurrentOperation: operation,
			Stage:            stage,
		},
	}

	// Add progressing condition
	condition := metav1.Condition{
		Type:               ConditionTypeProgressing,
		Status:             metav1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
		Reason:             ReasonProcessing,
		Message:            fmt.Sprintf("%s (%d%% complete)", operation, percentage),
	}
	update.Conditions = []metav1.Condition{condition}

	return sm.UpdateStatus(ctx, jiraSync, update)
}

// SetReadyCondition sets the Ready condition with proper status
func (sm *StatusManager) SetReadyCondition(ctx context.Context, jiraSync *operatortypes.JIRASync,
	ready bool, reason, message string) error {

	status := metav1.ConditionFalse
	if ready {
		status = metav1.ConditionTrue
	}

	condition := metav1.Condition{
		Type:               ConditionTypeReady,
		Status:             status,
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            message,
	}

	update := StatusUpdate{
		Conditions: []metav1.Condition{condition},
	}

	return sm.UpdateStatus(ctx, jiraSync, update)
}

// SetProcessingCondition sets the Processing condition
func (sm *StatusManager) SetProcessingCondition(ctx context.Context, jiraSync *operatortypes.JIRASync,
	processing bool, reason, message string) error {

	status := metav1.ConditionFalse
	if processing {
		status = metav1.ConditionTrue
	}

	condition := metav1.Condition{
		Type:               ConditionTypeProcessing,
		Status:             status,
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            message,
	}

	update := StatusUpdate{
		Conditions: []metav1.Condition{condition},
	}

	return sm.UpdateStatus(ctx, jiraSync, update)
}

// SetFailedCondition sets the Failed condition
func (sm *StatusManager) SetFailedCondition(ctx context.Context, jiraSync *operatortypes.JIRASync,
	failed bool, reason, message string) error {

	status := metav1.ConditionFalse
	if failed {
		status = metav1.ConditionTrue
	}

	condition := metav1.Condition{
		Type:               ConditionTypeFailed,
		Status:             status,
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            message,
	}

	update := StatusUpdate{
		Conditions: []metav1.Condition{condition},
		ClearError: !failed, // Clear error if not failed
	}

	return sm.UpdateStatus(ctx, jiraSync, update)
}

// ValidateStatus performs comprehensive status validation
func (sm *StatusManager) ValidateStatus(jiraSync *operatortypes.JIRASync) []string {
	var issues []string

	// Check phase consistency with conditions
	if jiraSync.Status.Phase == PhaseCompleted {
		if !sm.hasCondition(jiraSync.Status.Conditions, ConditionTypeReady, metav1.ConditionTrue) {
			issues = append(issues, "Phase is Completed but Ready condition is not True")
		}
	}

	if jiraSync.Status.Phase == PhaseFailed {
		if !sm.hasCondition(jiraSync.Status.Conditions, ConditionTypeFailed, metav1.ConditionTrue) {
			issues = append(issues, "Phase is Failed but Failed condition is not True")
		}
	}

	// Check progress consistency
	if jiraSync.Status.Progress != nil {
		if jiraSync.Status.Progress.Percentage < 0 || jiraSync.Status.Progress.Percentage > 100 {
			issues = append(issues, "Progress percentage must be between 0 and 100")
		}

		if jiraSync.Status.Progress.CompletedOperations > jiraSync.Status.Progress.TotalOperations {
			issues = append(issues, "Completed operations cannot exceed total operations")
		}
	}

	// Check sync stats consistency
	if jiraSync.Status.SyncStats != nil {
		total := jiraSync.Status.SyncStats.ProcessedIssues + jiraSync.Status.SyncStats.FailedIssues
		if total > jiraSync.Status.SyncStats.TotalIssues {
			issues = append(issues, "Processed + Failed issues cannot exceed total issues")
		}
	}

	// Check observed generation
	if jiraSync.Status.ObservedGeneration > jiraSync.Generation {
		issues = append(issues, "Observed generation cannot be greater than resource generation")
	}

	return issues
}

// Helper methods

func (sm *StatusManager) updateProgress(current *operatortypes.ProgressInfo, update *ProgressUpdate) {
	if update.Percentage != nil {
		current.Percentage = *update.Percentage
	}
	if update.CurrentOperation != "" {
		current.CurrentOperation = update.CurrentOperation
	}
	if update.TotalOperations != nil {
		current.TotalOperations = *update.TotalOperations
	}
	if update.CompletedOperations != nil {
		current.CompletedOperations = *update.CompletedOperations
	}
	if update.EstimatedCompletion != nil {
		current.EstimatedCompletion = &metav1.Time{Time: *update.EstimatedCompletion}
	}
	if update.ProcessingRate != nil {
		current.ProcessingRate = *update.ProcessingRate
	}
	if update.Stage != "" {
		current.Stage = update.Stage
	}
}

func (sm *StatusManager) updateSyncState(current *operatortypes.SyncState, update *SyncStateUpdate) {
	if update.OperationID != "" {
		current.OperationID = update.OperationID
	}
	if update.ConfigHash != "" {
		current.ConfigHash = update.ConfigHash
	}
	if update.ActiveIssues != nil {
		current.ActiveIssues = update.ActiveIssues
	}
	if update.LastSuccessfulConfig != "" {
		current.LastSuccessfulConfig = update.LastSuccessfulConfig
	}
	if update.Metadata != nil {
		current.Metadata = update.Metadata
	}
	if update.AddMetadata != nil {
		if current.Metadata == nil {
			current.Metadata = make(map[string]string)
		}
		for k, v := range update.AddMetadata {
			current.Metadata[k] = v
		}
	}
}

func (sm *StatusManager) updateSyncStats(current *operatortypes.SyncStats, update *SyncStatsUpdate) {
	if update.TotalIssues != nil {
		current.TotalIssues = *update.TotalIssues
	}
	if update.ProcessedIssues != nil {
		current.ProcessedIssues = *update.ProcessedIssues
	}
	if update.FailedIssues != nil {
		current.FailedIssues = *update.FailedIssues
	}
	if update.Duration != "" {
		current.Duration = update.Duration
	}
	if update.StartTime != nil {
		current.StartTime = &metav1.Time{Time: *update.StartTime}
	}
	if update.LastSyncTime != nil {
		current.LastSyncTime = &metav1.Time{Time: *update.LastSyncTime}
	}
}

func (sm *StatusManager) setCondition(conditions *[]metav1.Condition, newCondition metav1.Condition) {
	for i, condition := range *conditions {
		if condition.Type == newCondition.Type {
			// Only update if status or reason changed
			if condition.Status != newCondition.Status || condition.Reason != newCondition.Reason {
				newCondition.LastTransitionTime = metav1.Now()
			} else {
				newCondition.LastTransitionTime = condition.LastTransitionTime
			}
			(*conditions)[i] = newCondition
			return
		}
	}
	// Add new condition
	newCondition.LastTransitionTime = metav1.Now()
	*conditions = append(*conditions, newCondition)
}

func (sm *StatusManager) hasCondition(conditions []metav1.Condition, conditionType string, status metav1.ConditionStatus) bool {
	for _, condition := range conditions {
		if condition.Type == conditionType && condition.Status == status {
			return true
		}
	}
	return false
}

func (sm *StatusManager) calculateHealthStatus(jiraSync *operatortypes.JIRASync) string {
	// Check for critical failures
	if sm.hasCondition(jiraSync.Status.Conditions, ConditionTypeFailed, metav1.ConditionTrue) {
		return HealthStatusUnhealthy
	}

	// Check for degraded state (high retry count, etc.)
	if jiraSync.Status.RetryCount >= 3 {
		return HealthStatusDegraded
	}

	// Check if ready
	if sm.hasCondition(jiraSync.Status.Conditions, ConditionTypeReady, metav1.ConditionTrue) {
		return HealthStatusHealthy
	}

	// Processing state
	if sm.hasCondition(jiraSync.Status.Conditions, ConditionTypeProcessing, metav1.ConditionTrue) {
		return HealthStatusHealthy
	}

	return HealthStatusUnknown
}

// GenerateConfigHash creates a hash of the sync specification for change detection
func (sm *StatusManager) GenerateConfigHash(spec *operatortypes.JIRASyncSpec) string {
	hasher := sha256.New()
	_, _ = fmt.Fprintf(hasher, "%+v", spec)
	return fmt.Sprintf("%x", hasher.Sum(nil))[:16]
}

func (sm *StatusManager) emitPhaseChangeEvent(jiraSync *operatortypes.JIRASync, oldPhase, newPhase string) {
	message := fmt.Sprintf("Phase changed from %s to %s", oldPhase, newPhase)
	sm.recorder.Event(jiraSync, corev1.EventTypeNormal, "PhaseChanged", message)
}

func (sm *StatusManager) emitErrorEvent(jiraSync *operatortypes.JIRASync, err error) {
	sm.recorder.Event(jiraSync, corev1.EventTypeWarning, "SyncError", err.Error())
}

// GetProgressPercentage calculates progress percentage from sync stats
func (sm *StatusManager) GetProgressPercentage(stats *operatortypes.SyncStats) int {
	if stats == nil || stats.TotalIssues == 0 {
		return 0
	}

	completed := stats.ProcessedIssues + stats.FailedIssues
	return int((float64(completed) / float64(stats.TotalIssues)) * 100)
}

// EstimateCompletion estimates completion time based on current progress
func (sm *StatusManager) EstimateCompletion(stats *operatortypes.SyncStats, processingRate float64) *time.Time {
	if stats == nil || stats.TotalIssues == 0 || processingRate <= 0 {
		return nil
	}

	remaining := stats.TotalIssues - (stats.ProcessedIssues + stats.FailedIssues)
	if remaining <= 0 {
		return nil
	}

	minutesRemaining := float64(remaining) / processingRate
	completion := time.Now().Add(time.Duration(minutesRemaining) * time.Minute)
	return &completion
}
