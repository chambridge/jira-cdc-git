package controllers

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/chambrid/jira-cdc-git/internal/operator/apiclient"
	operatortypes "github.com/chambrid/jira-cdc-git/internal/operator/types"
)

func TestAPIIntegration_SingleSyncWorkflow(t *testing.T) {
	reconciler, fakeClient := setupTestReconciler()
	mockAPI := reconciler.APIClient.(*apiclient.MockAPIClient)

	// Create test JIRASync resource
	jiraSync := createTestJIRASync("api-test-single", "default")
	err := fakeClient.Create(context.TODO(), jiraSync)
	require.NoError(t, err)

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      jiraSync.Name,
			Namespace: jiraSync.Namespace,
		},
	}

	// First reconcile - should add finalizer and requeue
	result, err := reconciler.Reconcile(context.TODO(), req)
	assert.NoError(t, err)
	assert.True(t, result.Requeue)

	// Second reconcile - should initialize and move to pending
	_, err = reconciler.Reconcile(context.TODO(), req)
	assert.NoError(t, err)

	// Third reconcile - should trigger API call
	_, err = reconciler.Reconcile(context.TODO(), req)
	assert.NoError(t, err)

	// Verify API call was made
	assert.Len(t, mockAPI.TriggerSingleSyncCalls, 1)
	apiCall := mockAPI.TriggerSingleSyncCalls[0]
	assert.Equal(t, "TEST-123", apiCall.IssueKey)
	assert.Equal(t, "https://github.com/test/repo.git", apiCall.Repository)

	// Verify status updated
	var updated operatortypes.JIRASync
	err = fakeClient.Get(context.TODO(), types.NamespacedName{
		Name: jiraSync.Name, Namespace: jiraSync.Namespace,
	}, &updated)
	require.NoError(t, err)
	assert.Equal(t, PhaseRunning, updated.Status.Phase)
	assert.Equal(t, "api", updated.Status.JobRef.Namespace)
	assert.Equal(t, "mock-job-123", updated.Status.JobRef.Name)

	// Fourth reconcile - should check API job status
	_, err = reconciler.Reconcile(context.TODO(), req)
	assert.NoError(t, err)

	// Verify job status was checked
	assert.Len(t, mockAPI.GetJobStatusCalls, 1)
	assert.Equal(t, "mock-job-123", mockAPI.GetJobStatusCalls[0])

	// Verify status updated to completed
	err = fakeClient.Get(context.TODO(), types.NamespacedName{
		Name: jiraSync.Name, Namespace: jiraSync.Namespace,
	}, &updated)
	require.NoError(t, err)
	assert.Equal(t, PhaseCompleted, updated.Status.Phase)
}

func TestAPIIntegration_BatchSyncWorkflow(t *testing.T) {
	reconciler, fakeClient := setupTestReconciler()
	mockAPI := reconciler.APIClient.(*apiclient.MockAPIClient)

	// Create test JIRASync resource for batch sync
	jiraSync := &operatortypes.JIRASync{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "api-test-batch",
			Namespace: "default",
		},
		Spec: operatortypes.JIRASyncSpec{
			SyncType: "batch",
			Target: operatortypes.SyncTarget{
				IssueKeys: []string{"TEST-1", "TEST-2", "TEST-3"},
			},
			Destination: operatortypes.GitDestination{
				Repository: "https://github.com/test/batch-repo.git",
				Branch:     "main",
			},
		},
	}
	err := fakeClient.Create(context.TODO(), jiraSync)
	require.NoError(t, err)

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      jiraSync.Name,
			Namespace: jiraSync.Namespace,
		},
	}

	// First reconcile - add finalizer and requeue
	_, err = reconciler.Reconcile(context.TODO(), req)
	assert.NoError(t, err)

	// Second reconcile - initialize and move to pending
	_, err = reconciler.Reconcile(context.TODO(), req)
	assert.NoError(t, err)

	// Third reconcile - trigger batch API call
	_, err = reconciler.Reconcile(context.TODO(), req)
	assert.NoError(t, err)

	// Verify batch API call was made
	assert.Len(t, mockAPI.TriggerBatchSyncCalls, 1)
	batchCall := mockAPI.TriggerBatchSyncCalls[0]
	assert.Equal(t, []string{"TEST-1", "TEST-2", "TEST-3"}, batchCall.IssueKeys)
	assert.Equal(t, "https://github.com/test/batch-repo.git", batchCall.Repository)
	assert.Equal(t, 1, batchCall.Parallelism) // Default parallelism
}

func TestAPIIntegration_JQLSyncWorkflow(t *testing.T) {
	reconciler, fakeClient := setupTestReconciler()
	mockAPI := reconciler.APIClient.(*apiclient.MockAPIClient)

	// Create test JIRASync resource for JQL sync
	jiraSync := &operatortypes.JIRASync{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "api-test-jql",
			Namespace: "default",
		},
		Spec: operatortypes.JIRASyncSpec{
			SyncType: "jql",
			Target: operatortypes.SyncTarget{
				JQLQuery: "project = TEST AND status = 'To Do'",
			},
			Destination: operatortypes.GitDestination{
				Repository: "https://github.com/test/jql-repo.git",
				Branch:     "feature",
			},
		},
	}
	err := fakeClient.Create(context.TODO(), jiraSync)
	require.NoError(t, err)

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      jiraSync.Name,
			Namespace: jiraSync.Namespace,
		},
	}

	// Process through reconciliation
	_, err = reconciler.Reconcile(context.TODO(), req) // Add finalizer
	assert.NoError(t, err)
	_, err = reconciler.Reconcile(context.TODO(), req) // Initialize
	assert.NoError(t, err)
	_, err = reconciler.Reconcile(context.TODO(), req) // Trigger API
	assert.NoError(t, err)

	// Verify JQL API call was made
	assert.Len(t, mockAPI.TriggerJQLSyncCalls, 1)
	jqlCall := mockAPI.TriggerJQLSyncCalls[0]
	assert.Equal(t, "project = TEST AND status = 'To Do'", jqlCall.JQLQuery)
	assert.Equal(t, "https://github.com/test/jql-repo.git", jqlCall.Repository)
	assert.Equal(t, "feature", jqlCall.Branch)
}

func TestAPIIntegration_APIErrorHandling(t *testing.T) {
	reconciler, fakeClient := setupTestReconciler()
	mockAPI := reconciler.APIClient.(*apiclient.MockAPIClient)

	// Configure mock to return error
	apiError := errors.New("API server unavailable")
	mockAPI.SetSyncError(apiError)

	// Create test JIRASync resource
	jiraSync := createTestJIRASync("api-test-error", "default")
	err := fakeClient.Create(context.TODO(), jiraSync)
	require.NoError(t, err)

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      jiraSync.Name,
			Namespace: jiraSync.Namespace,
		},
	}

	// Process through reconciliation
	_, err = reconciler.Reconcile(context.TODO(), req) // Add finalizer
	assert.NoError(t, err)
	_, err = reconciler.Reconcile(context.TODO(), req) // Initialize
	assert.NoError(t, err)
	_, err = reconciler.Reconcile(context.TODO(), req) // Trigger API (should fail)
	assert.NoError(t, err)                             // Reconcile should not return error, but update status

	// Verify status shows failure
	var updated operatortypes.JIRASync
	err = fakeClient.Get(context.TODO(), types.NamespacedName{
		Name: jiraSync.Name, Namespace: jiraSync.Namespace,
	}, &updated)
	require.NoError(t, err)
	assert.Equal(t, PhaseFailed, updated.Status.Phase)
}

func TestAPIIntegration_JobStatusProgression(t *testing.T) {
	reconciler, fakeClient := setupTestReconciler()
	mockAPI := reconciler.APIClient.(*apiclient.MockAPIClient)

	// Create test JIRASync resource
	jiraSync := createTestJIRASync("api-test-progress", "default")
	err := fakeClient.Create(context.TODO(), jiraSync)
	require.NoError(t, err)

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      jiraSync.Name,
			Namespace: jiraSync.Namespace,
		},
	}

	// Initialize and trigger API call
	_, err = reconciler.Reconcile(context.TODO(), req) // Add finalizer
	assert.NoError(t, err)
	_, err = reconciler.Reconcile(context.TODO(), req) // Initialize
	assert.NoError(t, err)
	_, err = reconciler.Reconcile(context.TODO(), req) // Trigger API
	assert.NoError(t, err)

	// Configure mock to return "running" status
	mockAPI.SetJobStatusResponse("mock-job-123", "running", 50, "Processing issues")

	// Check status while running
	result, err := reconciler.Reconcile(context.TODO(), req)
	assert.NoError(t, err)
	assert.True(t, result.RequeueAfter > 0) // Should requeue for status check

	// Verify status is still running
	var updated operatortypes.JIRASync
	err = fakeClient.Get(context.TODO(), types.NamespacedName{
		Name: jiraSync.Name, Namespace: jiraSync.Namespace,
	}, &updated)
	require.NoError(t, err)
	assert.Equal(t, PhaseRunning, updated.Status.Phase)

	// Configure mock to return "completed" status
	mockAPI.SetJobStatusResponse("mock-job-123", "completed", 100, "Sync completed")

	// Check status when completed
	result, err = reconciler.Reconcile(context.TODO(), req)
	assert.NoError(t, err)
	assert.False(t, result.Requeue)

	// Verify status is completed
	err = fakeClient.Get(context.TODO(), types.NamespacedName{
		Name: jiraSync.Name, Namespace: jiraSync.Namespace,
	}, &updated)
	require.NoError(t, err)
	assert.Equal(t, PhaseCompleted, updated.Status.Phase)
}

func TestAPIIntegration_HealthCheckFunctionality(t *testing.T) {
	reconciler, _ := setupTestReconciler()
	mockAPI := reconciler.APIClient.(*apiclient.MockAPIClient)

	// Test health check success
	reconciler.performHealthCheck(context.TODO())
	assert.Equal(t, 1, mockAPI.HealthCheckCalls)

	// Test health check failure
	mockAPI.HealthCheckFunc = func(ctx context.Context) error {
		return errors.New("API server down")
	}

	reconciler.performHealthCheck(context.TODO())
	assert.Equal(t, 2, mockAPI.HealthCheckCalls)
}

func TestAPIIntegration_CircuitBreakerSimulation(t *testing.T) {
	// This test simulates circuit breaker behavior by testing with
	// a real API client that has circuit breaker functionality
	reconciler, _ := setupTestReconciler()

	// Create a real API client with circuit breaker for testing
	apiClient := apiclient.NewAPIClient("http://nonexistent:8080", 1*time.Second, reconciler.Log)
	reconciler.APIClient = apiClient

	jiraSync := createTestJIRASync("circuit-test", "default")

	// Test multiple failures to trigger circuit breaker
	request, requestType, err := apiclient.ConvertJIRASyncToAPIRequest(jiraSync)
	assert.NoError(t, err)
	assert.Equal(t, "single", requestType)

	ctx := context.TODO()

	// Make multiple failing calls
	for i := 0; i < 4; i++ {
		_, err = apiClient.TriggerSingleSync(ctx, request.(*apiclient.SingleSyncRequest))
		// Expect errors due to nonexistent server
		assert.Error(t, err)
	}

	// Circuit should be open now, further calls should fail immediately
	_, err = apiClient.TriggerSingleSync(ctx, request.(*apiclient.SingleSyncRequest))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "circuit breaker is open")
}

func TestAPIIntegration_AuthenticationHeaders(t *testing.T) {
	reconciler, _ := setupTestReconciler()

	// Create API client with authentication
	apiClient := apiclient.NewAPIClientWithAuth("http://test:8080", 30*time.Second, "bearer", "test-token", reconciler.Log)

	// Test that client was created with auth
	assert.NotNil(t, apiClient)

	// Note: Testing actual auth headers would require inspecting HTTP requests,
	// which is complex in unit tests. The auth functionality is tested in
	// the apiclient package tests.
}
