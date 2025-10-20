package controllers

import (
	"context"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	operatortypes "github.com/chambrid/jira-cdc-git/internal/operator/types"
)

func TestStatusManager_UpdateStatus(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, operatortypes.AddToScheme(scheme))

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&operatortypes.JIRASync{}).
		Build()
	recorder := record.NewFakeRecorder(10)
	logger := logr.Discard()

	statusManager := NewStatusManager(fakeClient, recorder, logger)

	// Create a test JIRASync resource
	jiraSync := &operatortypes.JIRASync{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-sync",
			Namespace:  "default",
			Generation: 1,
		},
		Spec: operatortypes.JIRASyncSpec{
			SyncType: "single",
			Target: operatortypes.SyncTarget{
				IssueKeys: []string{"TEST-123"},
			},
			Destination: operatortypes.GitDestination{
				Repository: "/tmp/test-repo",
			},
		},
		Status: operatortypes.JIRASyncStatus{},
	}

	// Create the resource in the fake client
	ctx := context.Background()
	require.NoError(t, fakeClient.Create(ctx, jiraSync))

	t.Run("UpdateStatus with phase and conditions", func(t *testing.T) {
		update := StatusUpdate{
			Phase: PhasePending,
			Conditions: []metav1.Condition{
				{
					Type:    ConditionTypeReady,
					Status:  metav1.ConditionFalse,
					Reason:  ReasonInitializing,
					Message: "Sync is initializing",
				},
			},
		}

		err := statusManager.UpdateStatus(ctx, jiraSync, update)
		require.NoError(t, err)

		// Verify the status was updated
		assert.Equal(t, PhasePending, jiraSync.Status.Phase)
		assert.Len(t, jiraSync.Status.Conditions, 1)
		assert.Equal(t, ConditionTypeReady, jiraSync.Status.Conditions[0].Type)
		assert.Equal(t, metav1.ConditionFalse, jiraSync.Status.Conditions[0].Status)
		assert.Equal(t, ReasonInitializing, jiraSync.Status.Conditions[0].Reason)
		assert.Equal(t, int64(1), jiraSync.Status.ObservedGeneration)
		assert.NotNil(t, jiraSync.Status.LastStatusUpdate)
	})

	t.Run("UpdateStatus with progress information", func(t *testing.T) {
		percentage := 50
		totalOps := 100
		completedOps := 50

		update := StatusUpdate{
			Progress: &ProgressUpdate{
				Percentage:          &percentage,
				CurrentOperation:    "Processing issues",
				TotalOperations:     &totalOps,
				CompletedOperations: &completedOps,
				Stage:               StageExecution,
			},
		}

		err := statusManager.UpdateStatus(ctx, jiraSync, update)
		require.NoError(t, err)

		// Verify progress was updated
		require.NotNil(t, jiraSync.Status.Progress)
		assert.Equal(t, 50, jiraSync.Status.Progress.Percentage)
		assert.Equal(t, "Processing issues", jiraSync.Status.Progress.CurrentOperation)
		assert.Equal(t, 100, jiraSync.Status.Progress.TotalOperations)
		assert.Equal(t, 50, jiraSync.Status.Progress.CompletedOperations)
		assert.Equal(t, StageExecution, jiraSync.Status.Progress.Stage)
	})

	t.Run("UpdateStatus with sync state", func(t *testing.T) {
		update := StatusUpdate{
			SyncState: &SyncStateUpdate{
				OperationID:  "op-123",
				ConfigHash:   "abcd1234",
				ActiveIssues: []string{"TEST-123", "TEST-456"},
				Metadata: map[string]string{
					"syncType": "single",
					"started":  "2023-01-01T00:00:00Z",
				},
			},
		}

		err := statusManager.UpdateStatus(ctx, jiraSync, update)
		require.NoError(t, err)

		// Verify sync state was updated
		require.NotNil(t, jiraSync.Status.SyncState)
		assert.Equal(t, "op-123", jiraSync.Status.SyncState.OperationID)
		assert.Equal(t, "abcd1234", jiraSync.Status.SyncState.ConfigHash)
		assert.Equal(t, []string{"TEST-123", "TEST-456"}, jiraSync.Status.SyncState.ActiveIssues)
		assert.Equal(t, "single", jiraSync.Status.SyncState.Metadata["syncType"])
		assert.Equal(t, "2023-01-01T00:00:00Z", jiraSync.Status.SyncState.Metadata["started"])
	})

	t.Run("UpdateStatus with error", func(t *testing.T) {
		testError := assert.AnError
		update := StatusUpdate{
			Phase:      PhaseFailed,
			Error:      testError,
			RetryCount: 1,
		}

		err := statusManager.UpdateStatus(ctx, jiraSync, update)
		require.NoError(t, err)

		// Verify error was recorded
		assert.Equal(t, PhaseFailed, jiraSync.Status.Phase)
		assert.Equal(t, testError.Error(), jiraSync.Status.LastError)
		assert.Equal(t, 1, jiraSync.Status.RetryCount)
	})

	t.Run("UpdateStatus clear error", func(t *testing.T) {
		// First set an error
		jiraSync.Status.LastError = "some error"
		jiraSync.Status.RetryCount = 2

		update := StatusUpdate{
			Phase:      PhaseCompleted,
			ClearError: true,
		}

		err := statusManager.UpdateStatus(ctx, jiraSync, update)
		require.NoError(t, err)

		// Verify error was cleared
		assert.Equal(t, PhaseCompleted, jiraSync.Status.Phase)
		assert.Empty(t, jiraSync.Status.LastError)
		assert.Equal(t, 0, jiraSync.Status.RetryCount)
	})
}

func TestStatusManager_UpdateProgress(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, operatortypes.AddToScheme(scheme))

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&operatortypes.JIRASync{}).
		Build()
	recorder := record.NewFakeRecorder(10)
	logger := logr.Discard()

	statusManager := NewStatusManager(fakeClient, recorder, logger)

	jiraSync := &operatortypes.JIRASync{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-sync",
			Namespace: "default",
		},
		Status: operatortypes.JIRASyncStatus{},
	}

	ctx := context.Background()
	require.NoError(t, fakeClient.Create(ctx, jiraSync))

	err := statusManager.UpdateProgress(ctx, jiraSync, 75, "Syncing issues", StageExecution)
	require.NoError(t, err)

	// Verify progress was updated
	require.NotNil(t, jiraSync.Status.Progress)
	assert.Equal(t, 75, jiraSync.Status.Progress.Percentage)
	assert.Equal(t, "Syncing issues", jiraSync.Status.Progress.CurrentOperation)
	assert.Equal(t, StageExecution, jiraSync.Status.Progress.Stage)

	// Verify processing condition was set
	assert.Len(t, jiraSync.Status.Conditions, 1)
	assert.Equal(t, ConditionTypeProcessing, jiraSync.Status.Conditions[0].Type)
	assert.Equal(t, metav1.ConditionTrue, jiraSync.Status.Conditions[0].Status)
	assert.Equal(t, ReasonProcessing, jiraSync.Status.Conditions[0].Reason)
	assert.Contains(t, jiraSync.Status.Conditions[0].Message, "Syncing issues")
	assert.Contains(t, jiraSync.Status.Conditions[0].Message, "75%")
}

func TestStatusManager_SetConditions(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, operatortypes.AddToScheme(scheme))

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&operatortypes.JIRASync{}).
		Build()
	recorder := record.NewFakeRecorder(10)
	logger := logr.Discard()

	statusManager := NewStatusManager(fakeClient, recorder, logger)

	jiraSync := &operatortypes.JIRASync{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-sync",
			Namespace: "default",
		},
		Status: operatortypes.JIRASyncStatus{},
	}

	ctx := context.Background()
	require.NoError(t, fakeClient.Create(ctx, jiraSync))

	t.Run("SetReadyCondition true", func(t *testing.T) {
		err := statusManager.SetReadyCondition(ctx, jiraSync, true, ReasonCompleted, "Sync completed successfully")
		require.NoError(t, err)

		// Find the Ready condition
		var readyCondition *metav1.Condition
		for i := range jiraSync.Status.Conditions {
			if jiraSync.Status.Conditions[i].Type == ConditionTypeReady {
				readyCondition = &jiraSync.Status.Conditions[i]
				break
			}
		}

		require.NotNil(t, readyCondition)
		assert.Equal(t, ConditionTypeReady, readyCondition.Type)
		assert.Equal(t, metav1.ConditionTrue, readyCondition.Status)
		assert.Equal(t, ReasonCompleted, readyCondition.Reason)
		assert.Equal(t, "Sync completed successfully", readyCondition.Message)
	})

	t.Run("SetProcessingCondition true", func(t *testing.T) {
		err := statusManager.SetProcessingCondition(ctx, jiraSync, true, ReasonProcessing, "Sync is processing")
		require.NoError(t, err)

		// Find the Processing condition
		var processingCondition *metav1.Condition
		for i := range jiraSync.Status.Conditions {
			if jiraSync.Status.Conditions[i].Type == ConditionTypeProcessing {
				processingCondition = &jiraSync.Status.Conditions[i]
				break
			}
		}

		require.NotNil(t, processingCondition)
		assert.Equal(t, ConditionTypeProcessing, processingCondition.Type)
		assert.Equal(t, metav1.ConditionTrue, processingCondition.Status)
		assert.Equal(t, ReasonProcessing, processingCondition.Reason)
		assert.Equal(t, "Sync is processing", processingCondition.Message)
	})

	t.Run("SetFailedCondition true", func(t *testing.T) {
		err := statusManager.SetFailedCondition(ctx, jiraSync, true, ReasonFailed, "Sync failed due to error")
		require.NoError(t, err)

		// Find the Failed condition
		var failedCondition *metav1.Condition
		for i := range jiraSync.Status.Conditions {
			if jiraSync.Status.Conditions[i].Type == ConditionTypeFailed {
				failedCondition = &jiraSync.Status.Conditions[i]
				break
			}
		}

		require.NotNil(t, failedCondition)
		assert.Equal(t, ConditionTypeFailed, failedCondition.Type)
		assert.Equal(t, metav1.ConditionTrue, failedCondition.Status)
		assert.Equal(t, ReasonFailed, failedCondition.Reason)
		assert.Equal(t, "Sync failed due to error", failedCondition.Message)
	})
}

func TestStatusManager_ValidateStatus(t *testing.T) {
	recorder := record.NewFakeRecorder(10)
	logger := logr.Discard()

	statusManager := NewStatusManager(nil, recorder, logger)

	t.Run("Valid status", func(t *testing.T) {
		jiraSync := &operatortypes.JIRASync{
			ObjectMeta: metav1.ObjectMeta{
				Generation: 1,
			},
			Status: operatortypes.JIRASyncStatus{
				Phase:              PhaseCompleted,
				ObservedGeneration: 1,
				Conditions: []metav1.Condition{
					{
						Type:   ConditionTypeReady,
						Status: metav1.ConditionTrue,
					},
				},
				Progress: &operatortypes.ProgressInfo{
					Percentage:          100,
					TotalOperations:     10,
					CompletedOperations: 10,
				},
				SyncStats: &operatortypes.SyncStats{
					TotalIssues:     10,
					ProcessedIssues: 8,
					FailedIssues:    2,
				},
			},
		}

		issues := statusManager.ValidateStatus(jiraSync)
		assert.Empty(t, issues)
	})

	t.Run("Inconsistent phase and conditions", func(t *testing.T) {
		jiraSync := &operatortypes.JIRASync{
			Status: operatortypes.JIRASyncStatus{
				Phase: PhaseCompleted,
				Conditions: []metav1.Condition{
					{
						Type:   ConditionTypeReady,
						Status: metav1.ConditionFalse,
					},
				},
			},
		}

		issues := statusManager.ValidateStatus(jiraSync)
		assert.Contains(t, issues, "Phase is Completed but Ready condition is not True")
	})

	t.Run("Invalid progress percentage", func(t *testing.T) {
		jiraSync := &operatortypes.JIRASync{
			Status: operatortypes.JIRASyncStatus{
				Progress: &operatortypes.ProgressInfo{
					Percentage: 150, // Invalid: >100
				},
			},
		}

		issues := statusManager.ValidateStatus(jiraSync)
		assert.Contains(t, issues, "Progress percentage must be between 0 and 100")
	})

	t.Run("Inconsistent progress operations", func(t *testing.T) {
		jiraSync := &operatortypes.JIRASync{
			Status: operatortypes.JIRASyncStatus{
				Progress: &operatortypes.ProgressInfo{
					TotalOperations:     10,
					CompletedOperations: 15, // Invalid: >total
				},
			},
		}

		issues := statusManager.ValidateStatus(jiraSync)
		assert.Contains(t, issues, "Completed operations cannot exceed total operations")
	})

	t.Run("Inconsistent sync stats", func(t *testing.T) {
		jiraSync := &operatortypes.JIRASync{
			Status: operatortypes.JIRASyncStatus{
				SyncStats: &operatortypes.SyncStats{
					TotalIssues:     10,
					ProcessedIssues: 8,
					FailedIssues:    5, // Invalid: 8+5=13 > 10
				},
			},
		}

		issues := statusManager.ValidateStatus(jiraSync)
		assert.Contains(t, issues, "Processed + Failed issues cannot exceed total issues")
	})

	t.Run("Invalid observed generation", func(t *testing.T) {
		jiraSync := &operatortypes.JIRASync{
			ObjectMeta: metav1.ObjectMeta{
				Generation: 1,
			},
			Status: operatortypes.JIRASyncStatus{
				ObservedGeneration: 2, // Invalid: >generation
			},
		}

		issues := statusManager.ValidateStatus(jiraSync)
		assert.Contains(t, issues, "Observed generation cannot be greater than resource generation")
	})
}

func TestStatusManager_GenerateConfigHash(t *testing.T) {
	recorder := record.NewFakeRecorder(10)
	logger := logr.Discard()

	statusManager := NewStatusManager(nil, recorder, logger)

	spec1 := &operatortypes.JIRASyncSpec{
		SyncType: "single",
		Target: operatortypes.SyncTarget{
			IssueKeys: []string{"TEST-123"},
		},
		Destination: operatortypes.GitDestination{
			Repository: "/tmp/repo",
		},
	}

	spec2 := &operatortypes.JIRASyncSpec{
		SyncType: "single",
		Target: operatortypes.SyncTarget{
			IssueKeys: []string{"TEST-456"}, // Different issue key
		},
		Destination: operatortypes.GitDestination{
			Repository: "/tmp/repo",
		},
	}

	hash1 := statusManager.GenerateConfigHash(spec1)
	hash2 := statusManager.GenerateConfigHash(spec2)

	// Hashes should be different for different specs
	assert.NotEqual(t, hash1, hash2)

	// Hash should be consistent for same spec
	hash1Again := statusManager.GenerateConfigHash(spec1)
	assert.Equal(t, hash1, hash1Again)

	// Hash should be 16 characters (hex format)
	assert.Len(t, hash1, 16)
	assert.Len(t, hash2, 16)
}

func TestStatusManager_ProgressCalculations(t *testing.T) {
	recorder := record.NewFakeRecorder(10)
	logger := logr.Discard()

	statusManager := NewStatusManager(nil, recorder, logger)

	t.Run("GetProgressPercentage", func(t *testing.T) {
		tests := []struct {
			name     string
			stats    *operatortypes.SyncStats
			expected int
		}{
			{
				name:     "nil stats",
				stats:    nil,
				expected: 0,
			},
			{
				name: "zero total",
				stats: &operatortypes.SyncStats{
					TotalIssues: 0,
				},
				expected: 0,
			},
			{
				name: "half complete",
				stats: &operatortypes.SyncStats{
					TotalIssues:     10,
					ProcessedIssues: 4,
					FailedIssues:    1,
				},
				expected: 50, // (4+1)/10 * 100
			},
			{
				name: "fully complete",
				stats: &operatortypes.SyncStats{
					TotalIssues:     10,
					ProcessedIssues: 8,
					FailedIssues:    2,
				},
				expected: 100, // (8+2)/10 * 100
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := statusManager.GetProgressPercentage(tt.stats)
				assert.Equal(t, tt.expected, result)
			})
		}
	})

	t.Run("EstimateCompletion", func(t *testing.T) {
		stats := &operatortypes.SyncStats{
			TotalIssues:     100,
			ProcessedIssues: 60,
			FailedIssues:    10,
		}

		// 30 remaining issues at 10 issues/minute = 3 minutes
		completion := statusManager.EstimateCompletion(stats, 10.0)
		require.NotNil(t, completion)

		// Should be approximately 3 minutes from now
		expectedTime := time.Now().Add(3 * time.Minute)
		timeDiff := completion.Sub(expectedTime)
		assert.True(t, timeDiff < time.Minute && timeDiff > -time.Minute, "Completion time should be approximately 3 minutes from now")
	})

	t.Run("EstimateCompletion edge cases", func(t *testing.T) {
		// Nil stats
		completion := statusManager.EstimateCompletion(nil, 10.0)
		assert.Nil(t, completion)

		// Zero total
		stats := &operatortypes.SyncStats{TotalIssues: 0}
		completion = statusManager.EstimateCompletion(stats, 10.0)
		assert.Nil(t, completion)

		// Zero processing rate
		stats = &operatortypes.SyncStats{TotalIssues: 10, ProcessedIssues: 5}
		completion = statusManager.EstimateCompletion(stats, 0.0)
		assert.Nil(t, completion)

		// All issues completed
		stats = &operatortypes.SyncStats{
			TotalIssues:     10,
			ProcessedIssues: 8,
			FailedIssues:    2,
		}
		completion = statusManager.EstimateCompletion(stats, 10.0)
		assert.Nil(t, completion)
	})
}

func TestStatusManager_HealthStatus(t *testing.T) {
	recorder := record.NewFakeRecorder(10)
	logger := logr.Discard()

	statusManager := NewStatusManager(nil, recorder, logger)

	t.Run("Healthy when ready", func(t *testing.T) {
		jiraSync := &operatortypes.JIRASync{
			Status: operatortypes.JIRASyncStatus{
				Conditions: []metav1.Condition{
					{
						Type:   ConditionTypeReady,
						Status: metav1.ConditionTrue,
					},
				},
				SyncState: &operatortypes.SyncState{},
			},
		}

		health := statusManager.calculateHealthStatus(jiraSync)
		assert.Equal(t, HealthStatusHealthy, health)
	})

	t.Run("Unhealthy when failed", func(t *testing.T) {
		jiraSync := &operatortypes.JIRASync{
			Status: operatortypes.JIRASyncStatus{
				Conditions: []metav1.Condition{
					{
						Type:   ConditionTypeFailed,
						Status: metav1.ConditionTrue,
					},
				},
				SyncState: &operatortypes.SyncState{},
			},
		}

		health := statusManager.calculateHealthStatus(jiraSync)
		assert.Equal(t, HealthStatusUnhealthy, health)
	})

	t.Run("Degraded when high retry count", func(t *testing.T) {
		jiraSync := &operatortypes.JIRASync{
			Status: operatortypes.JIRASyncStatus{
				RetryCount: 5, // High retry count
				SyncState:  &operatortypes.SyncState{},
			},
		}

		health := statusManager.calculateHealthStatus(jiraSync)
		assert.Equal(t, HealthStatusDegraded, health)
	})

	t.Run("Healthy when processing", func(t *testing.T) {
		jiraSync := &operatortypes.JIRASync{
			Status: operatortypes.JIRASyncStatus{
				Conditions: []metav1.Condition{
					{
						Type:   ConditionTypeProcessing,
						Status: metav1.ConditionTrue,
					},
				},
				SyncState: &operatortypes.SyncState{},
			},
		}

		health := statusManager.calculateHealthStatus(jiraSync)
		assert.Equal(t, HealthStatusHealthy, health)
	})

	t.Run("Unknown when no clear status", func(t *testing.T) {
		jiraSync := &operatortypes.JIRASync{
			Status: operatortypes.JIRASyncStatus{
				SyncState: &operatortypes.SyncState{},
			},
		}

		health := statusManager.calculateHealthStatus(jiraSync)
		assert.Equal(t, HealthStatusUnknown, health)
	})
}
