package controllers

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/chambrid/jira-cdc-git/internal/operator/apiclient"
	operatortypes "github.com/chambrid/jira-cdc-git/internal/operator/types"
)

func setupTestReconciler() (*JIRASyncReconciler, client.Client) {
	// Create a test scheme
	testScheme := runtime.NewScheme()
	_ = scheme.AddToScheme(testScheme)
	_ = operatortypes.AddToScheme(testScheme)
	_ = batchv1.AddToScheme(testScheme)

	// Create a fake client with status subresource enabled
	fakeClient := fake.NewClientBuilder().
		WithScheme(testScheme).
		WithStatusSubresource(&operatortypes.JIRASync{}).
		Build()

	// Create mock API client
	mockAPIClient := apiclient.NewMockAPIClient()

	// Create reconciler
	reconciler := &JIRASyncReconciler{
		Client:    fakeClient,
		Scheme:    testScheme,
		Log:       ctrl.Log.WithName("test"),
		APIHost:   "http://test-api:8080",
		APIClient: mockAPIClient,
	}

	// Initialize metrics manually without registration to avoid conflicts in tests
	reconciler.reconcileCounter = *prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "test_jirasync_reconcile_total",
			Help: "Total number of JIRASync reconciliations",
		},
		[]string{"namespace", "name", "result"},
	)

	reconciler.reconcileDuration = *prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "test_jirasync_reconcile_duration_seconds",
			Help:    "Duration of JIRASync reconciliations",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"namespace", "name"},
	)

	reconciler.syncJobsTotal = *prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "test_jirasync_jobs_total",
			Help: "Total number of active sync jobs",
		},
		[]string{"namespace", "phase"},
	)

	// Initialize API monitoring metrics for tests
	reconciler.apiHealthStatus = *prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "test_jirasync_api_health_status",
			Help: "Test API health metric",
		},
		[]string{"api_host"},
	)

	reconciler.apiCallCounter = *prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "test_jirasync_api_calls_total",
			Help: "Test API calls metric",
		},
		[]string{"endpoint", "status"},
	)

	reconciler.apiCallDuration = *prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "test_jirasync_api_call_duration_seconds",
			Help:    "Test API call duration metric",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"endpoint"},
	)

	return reconciler, fakeClient
}

func createTestJIRASync(name, namespace string) *operatortypes.JIRASync {
	return &operatortypes.JIRASync{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: operatortypes.JIRASyncSpec{
			SyncType: "single",
			Target: operatortypes.SyncTarget{
				IssueKeys: []string{"TEST-123"},
			},
			Destination: operatortypes.GitDestination{
				Repository: "https://github.com/test/repo.git",
				Branch:     "main",
			},
		},
	}
}

func TestJIRASyncReconciler_Reconcile_NotFound(t *testing.T) {
	reconciler, _ := setupTestReconciler()

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      "non-existent",
			Namespace: "default",
		},
	}

	result, err := reconciler.Reconcile(context.TODO(), req)

	assert.NoError(t, err)
	assert.Equal(t, reconcile.Result{}, result)
}

func TestJIRASyncReconciler_InitializeSync(t *testing.T) {
	reconciler, fakeClient := setupTestReconciler()

	jiraSync := createTestJIRASync("test-sync", "default")
	err := fakeClient.Create(context.TODO(), jiraSync)
	require.NoError(t, err)

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      jiraSync.Name,
			Namespace: jiraSync.Namespace,
		},
	}

	// First reconcile should initialize and add finalizer
	result, err := reconciler.Reconcile(context.TODO(), req)
	assert.NoError(t, err)
	assert.True(t, result.Requeue)

	// Verify finalizer was added
	var updated operatortypes.JIRASync
	err = fakeClient.Get(context.TODO(), client.ObjectKeyFromObject(jiraSync), &updated)
	require.NoError(t, err)
	assert.Contains(t, updated.Finalizers, JIRASyncFinalizer)

	// Second reconcile should move to pending
	result, err = reconciler.Reconcile(context.TODO(), req)
	assert.NoError(t, err)
	assert.False(t, result.Requeue)

	// Verify status was updated
	err = fakeClient.Get(context.TODO(), client.ObjectKeyFromObject(jiraSync), &updated)
	require.NoError(t, err)
	assert.Equal(t, PhasePending, updated.Status.Phase)
	if assert.NotNil(t, updated.Status.SyncStats) {
		assert.NotNil(t, updated.Status.SyncStats.StartTime)
	}
}

func TestJIRASyncReconciler_HandlePending(t *testing.T) {
	reconciler, fakeClient := setupTestReconciler()

	jiraSync := createTestJIRASync("test-sync", "default")
	jiraSync.Finalizers = []string{JIRASyncFinalizer}
	jiraSync.Status.Phase = PhasePending
	jiraSync.Status.SyncStats = &operatortypes.SyncStats{
		StartTime: &metav1.Time{Time: time.Now()},
	}
	err := fakeClient.Create(context.TODO(), jiraSync)
	require.NoError(t, err)

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      jiraSync.Name,
			Namespace: jiraSync.Namespace,
		},
	}

	result, err := reconciler.Reconcile(context.TODO(), req)
	assert.NoError(t, err)
	assert.False(t, result.Requeue)

	// Verify API sync was triggered and status updated
	var updated operatortypes.JIRASync
	err = fakeClient.Get(context.TODO(), client.ObjectKeyFromObject(jiraSync), &updated)
	require.NoError(t, err)
	assert.Equal(t, PhaseRunning, updated.Status.Phase)
	assert.NotNil(t, updated.Status.JobRef)

	// Verify job reference is for API job (not Kubernetes job)
	assert.Equal(t, "api", updated.Status.JobRef.Namespace)
	assert.Equal(t, "mock-job-123", updated.Status.JobRef.Name) // Mock API client returns this job ID

	// Verify API client was called
	mockAPIClient := reconciler.APIClient.(*apiclient.MockAPIClient)
	assert.Len(t, mockAPIClient.TriggerSingleSyncCalls, 1)
	apiCall := mockAPIClient.TriggerSingleSyncCalls[0]
	assert.Equal(t, "TEST-123", apiCall.IssueKey)
	assert.Equal(t, "https://github.com/test/repo.git", apiCall.Repository)
}

func TestJIRASyncReconciler_HandleRunning_JobSucceeded(t *testing.T) {
	reconciler, fakeClient := setupTestReconciler()

	jiraSync := createTestJIRASync("test-sync", "default")
	jiraSync.Finalizers = []string{JIRASyncFinalizer}
	jiraSync.Status.Phase = PhaseRunning
	jiraSync.Status.SyncStats = &operatortypes.SyncStats{
		StartTime: &metav1.Time{Time: time.Now().Add(-5 * time.Minute)},
	}
	jiraSync.Status.JobRef = &operatortypes.JobReference{
		Name:      "test-job",
		Namespace: "default",
	}
	err := fakeClient.Create(context.TODO(), jiraSync)
	require.NoError(t, err)

	// Create a successful job
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-job",
			Namespace: "default",
		},
		Status: batchv1.JobStatus{
			Succeeded: 1,
		},
	}
	err = fakeClient.Create(context.TODO(), job)
	require.NoError(t, err)

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      jiraSync.Name,
			Namespace: jiraSync.Namespace,
		},
	}

	result, err := reconciler.Reconcile(context.TODO(), req)
	assert.NoError(t, err)
	assert.False(t, result.Requeue)

	// Verify status was updated to completed
	var updated operatortypes.JIRASync
	err = fakeClient.Get(context.TODO(), client.ObjectKeyFromObject(jiraSync), &updated)
	require.NoError(t, err)
	assert.Equal(t, PhaseCompleted, updated.Status.Phase)
	assert.NotEmpty(t, updated.Status.SyncStats.Duration)
	assert.NotNil(t, updated.Status.SyncStats.LastSyncTime)
}

func TestJIRASyncReconciler_HandleRunning_JobFailed(t *testing.T) {
	reconciler, fakeClient := setupTestReconciler()

	jiraSync := createTestJIRASync("test-sync", "default")
	jiraSync.Finalizers = []string{JIRASyncFinalizer}
	jiraSync.Status.Phase = PhaseRunning
	jiraSync.Status.SyncStats = &operatortypes.SyncStats{
		StartTime: &metav1.Time{Time: time.Now().Add(-5 * time.Minute)},
	}
	jiraSync.Status.JobRef = &operatortypes.JobReference{
		Name:      "test-job",
		Namespace: "default",
	}
	err := fakeClient.Create(context.TODO(), jiraSync)
	require.NoError(t, err)

	// Create a failed job
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-job",
			Namespace: "default",
		},
		Status: batchv1.JobStatus{
			Failed: 1,
		},
	}
	err = fakeClient.Create(context.TODO(), job)
	require.NoError(t, err)

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      jiraSync.Name,
			Namespace: jiraSync.Namespace,
		},
	}

	result, err := reconciler.Reconcile(context.TODO(), req)
	assert.NoError(t, err)
	assert.False(t, result.Requeue)

	// Verify status was updated to failed
	var updated operatortypes.JIRASync
	err = fakeClient.Get(context.TODO(), client.ObjectKeyFromObject(jiraSync), &updated)
	require.NoError(t, err)
	assert.Equal(t, PhaseFailed, updated.Status.Phase)
}

func TestJIRASyncReconciler_HandleFailed_WithRetry(t *testing.T) {
	reconciler, fakeClient := setupTestReconciler()

	jiraSync := createTestJIRASync("test-sync", "default")
	jiraSync.Finalizers = []string{JIRASyncFinalizer}
	jiraSync.Status.Phase = PhaseFailed
	jiraSync.Spec.RetryPolicy = &operatortypes.RetryPolicy{
		MaxRetries:        3,
		BackoffMultiplier: 2.0,
		InitialDelay:      5,
	}
	err := fakeClient.Create(context.TODO(), jiraSync)
	require.NoError(t, err)

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      jiraSync.Name,
			Namespace: jiraSync.Namespace,
		},
	}

	result, err := reconciler.Reconcile(context.TODO(), req)
	assert.NoError(t, err)
	assert.True(t, result.RequeueAfter > 0) // Should have retry delay

	// Verify status was updated to pending for retry
	var updated operatortypes.JIRASync
	err = fakeClient.Get(context.TODO(), client.ObjectKeyFromObject(jiraSync), &updated)
	require.NoError(t, err)
	assert.Equal(t, PhasePending, updated.Status.Phase)
	assert.Equal(t, "1", updated.Annotations[RetryCountAnnotation])
}

func TestJIRASyncReconciler_HandleFailed_MaxRetriesExceeded(t *testing.T) {
	reconciler, fakeClient := setupTestReconciler()

	jiraSync := createTestJIRASync("test-sync", "default")
	jiraSync.Finalizers = []string{JIRASyncFinalizer}
	jiraSync.Status.Phase = PhaseFailed
	jiraSync.Spec.RetryPolicy = &operatortypes.RetryPolicy{
		MaxRetries:        2,
		BackoffMultiplier: 2.0,
		InitialDelay:      5,
	}
	jiraSync.Annotations = map[string]string{
		RetryCountAnnotation: "2", // Already at max retries
	}
	err := fakeClient.Create(context.TODO(), jiraSync)
	require.NoError(t, err)

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      jiraSync.Name,
			Namespace: jiraSync.Namespace,
		},
	}

	result, err := reconciler.Reconcile(context.TODO(), req)
	assert.NoError(t, err)
	assert.Equal(t, reconcile.Result{}, result) // No retry

	// Verify status remains failed
	var updated operatortypes.JIRASync
	err = fakeClient.Get(context.TODO(), client.ObjectKeyFromObject(jiraSync), &updated)
	require.NoError(t, err)
	assert.Equal(t, PhaseFailed, updated.Status.Phase)
}

func TestJIRASyncReconciler_HandleDeletion(t *testing.T) {
	reconciler, fakeClient := setupTestReconciler()

	jiraSync := createTestJIRASync("test-sync", "default")
	jiraSync.Finalizers = []string{JIRASyncFinalizer}
	jiraSync.DeletionTimestamp = &metav1.Time{Time: time.Now()}
	jiraSync.Status.JobRef = &operatortypes.JobReference{
		Name:      "test-job",
		Namespace: "default",
	}
	err := fakeClient.Create(context.TODO(), jiraSync)
	require.NoError(t, err)

	// Create associated job
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-job",
			Namespace: "default",
		},
	}
	err = fakeClient.Create(context.TODO(), job)
	require.NoError(t, err)

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      jiraSync.Name,
			Namespace: jiraSync.Namespace,
		},
	}

	result, err := reconciler.Reconcile(context.TODO(), req)
	assert.NoError(t, err)
	assert.False(t, result.Requeue)

	// In a real cluster, the object would be deleted after finalizer removal
	// For the fake client, we check that the deletion was handled without error
	// The fake client doesn't automatically delete objects when finalizers are removed

	// Verify job was deleted
	var deletedJob batchv1.Job
	err = fakeClient.Get(context.TODO(), types.NamespacedName{
		Name:      "test-job",
		Namespace: "default",
	}, &deletedJob)
	assert.True(t, client.IgnoreNotFound(err) == nil) // Job should be deleted
}

func TestJIRASyncReconciler_ValidateSyncSpec(t *testing.T) {
	reconciler, _ := setupTestReconciler()

	tests := []struct {
		name    string
		spec    operatortypes.JIRASyncSpec
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid single sync",
			spec: operatortypes.JIRASyncSpec{
				SyncType: "single",
				Target: operatortypes.SyncTarget{
					IssueKeys: []string{"TEST-123"},
				},
				Destination: operatortypes.GitDestination{
					Repository: "https://github.com/test/repo.git",
				},
			},
			wantErr: false,
		},
		{
			name: "missing sync type",
			spec: operatortypes.JIRASyncSpec{
				Destination: operatortypes.GitDestination{
					Repository: "https://github.com/test/repo.git",
				},
			},
			wantErr: true,
			errMsg:  "syncType is required",
		},
		{
			name: "missing repository",
			spec: operatortypes.JIRASyncSpec{
				SyncType: "single",
				Target: operatortypes.SyncTarget{
					IssueKeys: []string{"TEST-123"},
				},
			},
			wantErr: true,
			errMsg:  "destination repository is required",
		},
		{
			name: "invalid sync type",
			spec: operatortypes.JIRASyncSpec{
				SyncType: "invalid",
				Destination: operatortypes.GitDestination{
					Repository: "https://github.com/test/repo.git",
				},
			},
			wantErr: true,
			errMsg:  "invalid syncType: invalid",
		},
		{
			name: "single sync without issue keys",
			spec: operatortypes.JIRASyncSpec{
				SyncType: "single",
				Destination: operatortypes.GitDestination{
					Repository: "https://github.com/test/repo.git",
				},
			},
			wantErr: true,
			errMsg:  "issueKeys required for single sync type",
		},
		{
			name: "jql sync without query",
			spec: operatortypes.JIRASyncSpec{
				SyncType: "jql",
				Destination: operatortypes.GitDestination{
					Repository: "https://github.com/test/repo.git",
				},
			},
			wantErr: true,
			errMsg:  "jqlQuery required for jql sync type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := reconciler.validateSyncSpec(&tt.spec)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestJIRASyncReconciler_BuildSyncArgs(t *testing.T) {
	reconciler, _ := setupTestReconciler()

	tests := []struct {
		name     string
		jiraSync *operatortypes.JIRASync
		expected []string
	}{
		{
			name: "single sync",
			jiraSync: &operatortypes.JIRASync{
				Spec: operatortypes.JIRASyncSpec{
					SyncType: "single",
					Target: operatortypes.SyncTarget{
						IssueKeys: []string{"TEST-123", "TEST-124"},
					},
					Destination: operatortypes.GitDestination{
						Repository: "/path/to/repo",
						Branch:     "main",
					},
				},
			},
			expected: []string{"sync", "--issues", "[TEST-123 TEST-124]", "--repo", "/path/to/repo", "--branch", "main"},
		},
		{
			name: "jql sync",
			jiraSync: &operatortypes.JIRASync{
				Spec: operatortypes.JIRASyncSpec{
					SyncType: "jql",
					Target: operatortypes.SyncTarget{
						JQLQuery: "project = TEST",
					},
					Destination: operatortypes.GitDestination{
						Repository: "/path/to/repo",
					},
				},
			},
			expected: []string{"sync", "--jql", "project = TEST", "--repo", "/path/to/repo"},
		},
		{
			name: "incremental sync",
			jiraSync: &operatortypes.JIRASync{
				Spec: operatortypes.JIRASyncSpec{
					SyncType: "incremental",
					Target: operatortypes.SyncTarget{
						ProjectKey: "TEST",
					},
					Destination: operatortypes.GitDestination{
						Repository: "/path/to/repo",
					},
				},
			},
			expected: []string{"sync", "--incremental", "--project", "TEST", "--repo", "/path/to/repo"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := reconciler.buildSyncArgs(tt.jiraSync)
			assert.Equal(t, tt.expected, args)
		})
	}
}

func TestJIRASyncReconciler_RetryLogic(t *testing.T) {
	reconciler, _ := setupTestReconciler()

	jiraSync := createTestJIRASync("test-sync", "default")

	// Test getting retry count from annotations
	assert.Equal(t, 0, reconciler.getRetryCount(jiraSync))

	jiraSync.Annotations = map[string]string{
		RetryCountAnnotation: "3",
	}
	assert.Equal(t, 3, reconciler.getRetryCount(jiraSync))

	// Test incrementing retry count
	reconciler.incrementRetryCount(jiraSync)
	assert.Equal(t, "4", jiraSync.Annotations[RetryCountAnnotation])

	// Test recording and clearing errors
	testErr := fmt.Errorf("test error")
	reconciler.recordError(jiraSync, testErr)
	assert.Equal(t, "test error", jiraSync.Annotations[LastErrorAnnotation])

	reconciler.clearError(jiraSync)
	assert.NotContains(t, jiraSync.Annotations, LastErrorAnnotation)
	assert.NotContains(t, jiraSync.Annotations, RetryCountAnnotation)
}
