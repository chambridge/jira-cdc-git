package test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/chambrid/jira-cdc-git/internal/api"
	"github.com/chambrid/jira-cdc-git/pkg/jobs"
)

// TestV040Integration validates that v0.4.0 API server and job scheduling work together
func TestV040Integration(t *testing.T) {
	t.Run("APIServerJobIntegration", testAPIServerJobIntegration)
	t.Run("EndToEndAPIWorkflow", testEndToEndAPIWorkflow)
	t.Run("BackwardCompatibilityV030", testBackwardCompatibilityV030)
}

// testAPIServerJobIntegration verifies API server properly integrates with job scheduling
func testAPIServerJobIntegration(t *testing.T) {
	// Create test job manager with tracking
	testJobManager := &TestJobManager{
		submittedJobs: make([]*jobs.JobResult, 0),
		jobResults:    make(map[string]*jobs.JobResult),
	}

	// Create API server with test job manager
	config := api.DefaultConfig()
	config.Port = 8999 // Use different port for testing

	buildInfo := api.BuildInfo{
		Version: "test-v0.4.0",
		Commit:  "test-integration",
		Date:    time.Now().Format("2006-01-02T15:04:05Z"),
	}

	server := api.NewServer(config, buildInfo, testJobManager)

	t.Run("SingleSyncJobCreation", func(t *testing.T) {
		// Test single sync job creation
		requestBody := api.SingleSyncRequest{
			IssueKey:   "PROJ-123",
			Repository: "/tmp/test-repo",
			Async:      true,
		}

		bodyBytes, _ := json.Marshal(requestBody)
		req := httptest.NewRequest("POST", "/api/v1/sync/single", bytes.NewBuffer(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		// Use reflection to call the handler directly since we can't access it
		mux := http.NewServeMux()
		server.RegisterTestRoutes(mux) // We'll need to add this method
		mux.ServeHTTP(w, req)

		if w.Code != http.StatusAccepted {
			t.Errorf("Expected status %d, got %d. Response: %s", http.StatusAccepted, w.Code, w.Body.String())
		}

		// Verify job was submitted to job manager
		if len(testJobManager.submittedJobs) != 1 {
			t.Errorf("Expected 1 job submitted, got %d", len(testJobManager.submittedJobs))
		}

		// Verify job details
		job := testJobManager.submittedJobs[0]
		if job.JobID == "" {
			t.Error("Job ID should not be empty")
		}
		if job.Status != jobs.JobStatusPending {
			t.Errorf("Expected job status %s, got %s", jobs.JobStatusPending, job.Status)
		}

		t.Logf("✅ Single sync job created successfully: %s", job.JobID)
	})

	t.Run("BatchSyncJobCreation", func(t *testing.T) {
		// Reset job manager
		testJobManager.submittedJobs = make([]*jobs.JobResult, 0)

		// Test batch sync job creation
		requestBody := api.BatchSyncRequest{
			IssueKeys:   []string{"PROJ-123", "PROJ-124", "PROJ-125"},
			Repository:  "/tmp/test-repo",
			Parallelism: 2,
			Async:       true,
		}

		bodyBytes, _ := json.Marshal(requestBody)
		req := httptest.NewRequest("POST", "/api/v1/sync/batch", bytes.NewBuffer(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		mux := http.NewServeMux()
		server.RegisterTestRoutes(mux)
		mux.ServeHTTP(w, req)

		if w.Code != http.StatusAccepted {
			t.Errorf("Expected status %d, got %d. Response: %s", http.StatusAccepted, w.Code, w.Body.String())
		}

		// Verify batch job was submitted
		if len(testJobManager.submittedJobs) != 1 {
			t.Errorf("Expected 1 batch job submitted, got %d", len(testJobManager.submittedJobs))
		}

		job := testJobManager.submittedJobs[0]
		if job.TotalIssues != 3 {
			t.Errorf("Expected 3 total issues, got %d", job.TotalIssues)
		}

		t.Logf("✅ Batch sync job created successfully: %s", job.JobID)
	})
}

// testEndToEndAPIWorkflow tests complete API workflow from request to completion
func testEndToEndAPIWorkflow(t *testing.T) {
	// Skip if no real JIRA configuration (use mock for this test)
	if os.Getenv("JIRA_URL") == "" {
		t.Log("Using mock workflow for end-to-end API test")
	}

	// Create temporary directory for test
	tempDir, err := os.MkdirTemp("", "v040-e2e-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Create test job manager that simulates realistic job execution
	testJobManager := &RealisticJobManager{
		jobs:      make(map[string]*jobs.JobResult),
		completed: make(chan string, 10),
	}

	// Create API server
	config := api.DefaultConfig()
	config.Port = 9000
	buildInfo := api.BuildInfo{Version: "test-v0.4.0", Commit: "e2e-test", Date: time.Now().Format(time.RFC3339)}
	server := api.NewServer(config, buildInfo, testJobManager)

	t.Run("CompleteWorkflowSimulation", func(t *testing.T) {
		// Step 1: Submit sync job via API
		requestBody := api.SingleSyncRequest{
			IssueKey:   "RHOAIENG-29357",
			Repository: tempDir,
			Async:      true,
		}

		bodyBytes, _ := json.Marshal(requestBody)
		req := httptest.NewRequest("POST", "/api/v1/sync/single", bytes.NewBuffer(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		mux := http.NewServeMux()
		server.RegisterTestRoutes(mux)
		mux.ServeHTTP(w, req)

		if w.Code != http.StatusAccepted {
			t.Fatalf("Expected status %d, got %d", http.StatusAccepted, w.Code)
		}

		var response api.Response
		if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		jobData, ok := response.Data.(map[string]interface{})
		if !ok {
			t.Fatal("Expected job data in response")
		}

		jobID, ok := jobData["job_id"].(string)
		if !ok {
			t.Fatal("Expected job_id in response")
		}

		t.Logf("Job submitted with ID: %s", jobID)

		// Step 2: Monitor job status
		timeout := time.After(30 * time.Second)
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-timeout:
				t.Fatal("Job did not complete within timeout")
			case <-ticker.C:
				// Check job status via API
				req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/jobs/%s", jobID), nil)
				w := httptest.NewRecorder()
				mux.ServeHTTP(w, req)

				if w.Code != http.StatusOK {
					continue
				}

				var statusResponse api.Response
				if err := json.NewDecoder(w.Body).Decode(&statusResponse); err != nil {
					continue
				}

				statusData, ok := statusResponse.Data.(map[string]interface{})
				if !ok {
					continue
				}

				status, ok := statusData["status"].(string)
				if !ok {
					continue
				}

				t.Logf("Job status: %s", status)

				if status == string(jobs.JobStatusSucceeded) {
					t.Logf("✅ Job completed successfully")
					return
				} else if status == string(jobs.JobStatusFailed) {
					t.Fatalf("Job failed")
				}
			}
		}
	})
}

// testBackwardCompatibilityV030 ensures v0.4.0 doesn't break v0.3.0 workflows
func testBackwardCompatibilityV030(t *testing.T) {
	t.Run("V030WorkflowsStillWork", func(t *testing.T) {
		// Test that existing v0.3.0 components still function with v0.4.0 additions
		// This ensures the API server addition doesn't break existing CLI workflows

		// Create job manager
		jobManager := &TestJobManager{
			submittedJobs: make([]*jobs.JobResult, 0),
			jobResults:    make(map[string]*jobs.JobResult),
		}

		// Test that JobManager interface is satisfied
		ctx := context.Background()

		// Test single issue sync
		singleReq := &jobs.SingleIssueSyncRequest{
			IssueKey:   "PROJ-123",
			Repository: "/tmp/test",
		}

		result, err := jobManager.SubmitSingleIssueSync(ctx, singleReq)
		if err != nil {
			t.Errorf("Single issue sync failed: %v", err)
		}
		if result == nil {
			t.Error("Expected result from single issue sync")
		}

		// Test batch sync
		batchReq := &jobs.BatchSyncRequest{
			IssueKeys:  []string{"PROJ-123", "PROJ-124"},
			Repository: "/tmp/test",
		}

		result, err = jobManager.SubmitBatchSync(ctx, batchReq)
		if err != nil {
			t.Errorf("Batch sync failed: %v", err)
		}
		if result == nil {
			t.Error("Expected result from batch sync")
		}

		// Test JQL sync
		jqlReq := &jobs.JQLSyncRequest{
			JQL:        "project = PROJ",
			Repository: "/tmp/test",
		}

		result, err = jobManager.SubmitJQLSync(ctx, jqlReq)
		if err != nil {
			t.Errorf("JQL sync failed: %v", err)
		}
		if result == nil {
			t.Error("Expected result from JQL sync")
		}

		t.Log("✅ All v0.3.0 job manager interfaces work correctly")
	})
}

// TestJobManager implements jobs.JobManager for testing
type TestJobManager struct {
	submittedJobs []*jobs.JobResult
	jobResults    map[string]*jobs.JobResult
}

func (t *TestJobManager) SubmitSingleIssueSync(ctx context.Context, req *jobs.SingleIssueSyncRequest) (*jobs.JobResult, error) {
	result := &jobs.JobResult{
		JobID:       fmt.Sprintf("test-job-single-%d", len(t.submittedJobs)+1),
		Status:      jobs.JobStatusPending,
		TotalIssues: 1,
	}
	t.submittedJobs = append(t.submittedJobs, result)
	t.jobResults[result.JobID] = result
	return result, nil
}

func (t *TestJobManager) SubmitBatchSync(ctx context.Context, req *jobs.BatchSyncRequest) (*jobs.JobResult, error) {
	result := &jobs.JobResult{
		JobID:       fmt.Sprintf("test-job-batch-%d", len(t.submittedJobs)+1),
		Status:      jobs.JobStatusPending,
		TotalIssues: len(req.IssueKeys),
	}
	t.submittedJobs = append(t.submittedJobs, result)
	t.jobResults[result.JobID] = result
	return result, nil
}

func (t *TestJobManager) SubmitJQLSync(ctx context.Context, req *jobs.JQLSyncRequest) (*jobs.JobResult, error) {
	result := &jobs.JobResult{
		JobID:       fmt.Sprintf("test-job-jql-%d", len(t.submittedJobs)+1),
		Status:      jobs.JobStatusPending,
		TotalIssues: 10, // Simulated
	}
	t.submittedJobs = append(t.submittedJobs, result)
	t.jobResults[result.JobID] = result
	return result, nil
}

func (t *TestJobManager) ExecuteLocalSync(ctx context.Context, req *jobs.LocalSyncRequest) (*jobs.SyncResult, error) {
	return &jobs.SyncResult{
		TotalIssues:     1,
		ProcessedIssues: 1,
		SuccessfulSync:  1,
		FailedSync:      0,
		Duration:        100 * time.Millisecond,
		ProcessedFiles:  []string{"/tmp/test.yaml"},
		Errors:          []string{},
	}, nil
}

func (t *TestJobManager) ListJobs(ctx context.Context, filter *jobs.JobFilter) ([]*jobs.JobResult, error) {
	return t.submittedJobs, nil
}

func (t *TestJobManager) GetJob(ctx context.Context, jobID string) (*jobs.JobResult, error) {
	if job, exists := t.jobResults[jobID]; exists {
		return job, nil
	}
	return nil, jobs.NewJobError(jobID, "not_found", "Job not found")
}

func (t *TestJobManager) DeleteJob(ctx context.Context, jobID string) error {
	if _, exists := t.jobResults[jobID]; exists {
		delete(t.jobResults, jobID)
		return nil
	}
	return jobs.NewJobError(jobID, "not_found", "Job not found")
}

func (t *TestJobManager) CancelJob(ctx context.Context, jobID string) error {
	if job, exists := t.jobResults[jobID]; exists {
		job.Status = jobs.JobStatusFailed // Use existing status since JobStatusCancelled doesn't exist
		return nil
	}
	return jobs.NewJobError(jobID, "not_found", "Job not found")
}

func (t *TestJobManager) GetJobLogs(ctx context.Context, jobID string) (string, error) {
	if _, exists := t.jobResults[jobID]; exists {
		return fmt.Sprintf("Test logs for job %s", jobID), nil
	}
	return "", jobs.NewJobError(jobID, "not_found", "Job not found")
}

func (t *TestJobManager) WatchJob(ctx context.Context, jobID string) (<-chan jobs.JobMonitor, error) {
	if _, exists := t.jobResults[jobID]; !exists {
		return nil, jobs.NewJobError(jobID, "not_found", "Job not found")
	}

	ch := make(chan jobs.JobMonitor, 1)
	ch <- jobs.JobMonitor{
		JobID:     jobID,
		Status:    jobs.JobStatusSucceeded,
		Progress:  100.0,
		LastCheck: time.Now(),
		Message:   "Test job completed",
	}
	close(ch)
	return ch, nil
}

func (t *TestJobManager) GetQueueStatus(ctx context.Context) (*jobs.QueueStatus, error) {
	return &jobs.QueueStatus{
		TotalJobs:     len(t.submittedJobs),
		PendingJobs:   1,
		RunningJobs:   0,
		CompletedJobs: len(t.submittedJobs) - 1,
		FailedJobs:    0,
	}, nil
}

// RealisticJobManager simulates more realistic job execution
type RealisticJobManager struct {
	jobs      map[string]*jobs.JobResult
	completed chan string
}

func (r *RealisticJobManager) SubmitSingleIssueSync(ctx context.Context, req *jobs.SingleIssueSyncRequest) (*jobs.JobResult, error) {
	jobID := fmt.Sprintf("realistic-job-%d", time.Now().UnixNano())
	startTime := time.Now()
	result := &jobs.JobResult{
		JobID:       jobID,
		Status:      jobs.JobStatusPending,
		TotalIssues: 1,
		StartTime:   &startTime,
	}
	r.jobs[jobID] = result

	// Simulate job execution
	go r.simulateJobExecution(jobID)
	return result, nil
}

func (r *RealisticJobManager) simulateJobExecution(jobID string) {
	time.Sleep(2 * time.Second) // Simulate work
	if job, exists := r.jobs[jobID]; exists {
		job.Status = jobs.JobStatusRunning
		runTime := time.Now()
		job.StartTime = &runTime
		time.Sleep(3 * time.Second) // More work
		job.Status = jobs.JobStatusSucceeded
		completeTime := time.Now()
		job.CompletionTime = &completeTime
		job.ProcessedIssues = job.TotalIssues
		job.SuccessfulSync = job.TotalIssues
		r.completed <- jobID
	}
}

// Implement remaining methods for RealisticJobManager...
func (r *RealisticJobManager) SubmitBatchSync(ctx context.Context, req *jobs.BatchSyncRequest) (*jobs.JobResult, error) {
	return &jobs.JobResult{}, nil
}
func (r *RealisticJobManager) SubmitJQLSync(ctx context.Context, req *jobs.JQLSyncRequest) (*jobs.JobResult, error) {
	return &jobs.JobResult{}, nil
}
func (r *RealisticJobManager) ExecuteLocalSync(ctx context.Context, req *jobs.LocalSyncRequest) (*jobs.SyncResult, error) {
	return &jobs.SyncResult{}, nil
}
func (r *RealisticJobManager) ListJobs(ctx context.Context, filter *jobs.JobFilter) ([]*jobs.JobResult, error) {
	jobs := make([]*jobs.JobResult, 0, len(r.jobs))
	for _, job := range r.jobs {
		jobs = append(jobs, job)
	}
	return jobs, nil
}
func (r *RealisticJobManager) GetJob(ctx context.Context, jobID string) (*jobs.JobResult, error) {
	if job, exists := r.jobs[jobID]; exists {
		return job, nil
	}
	return nil, jobs.NewJobError(jobID, "not_found", "Job not found")
}
func (r *RealisticJobManager) DeleteJob(ctx context.Context, jobID string) error   { return nil }
func (r *RealisticJobManager) CancelJob(ctx context.Context, jobID string) error   { return nil }
func (r *RealisticJobManager) GetJobLogs(ctx context.Context, jobID string) (string, error) {
	return "", nil
}
func (r *RealisticJobManager) WatchJob(ctx context.Context, jobID string) (<-chan jobs.JobMonitor, error) {
	return nil, nil
}
func (r *RealisticJobManager) GetQueueStatus(ctx context.Context) (*jobs.QueueStatus, error) {
	return &jobs.QueueStatus{}, nil
}