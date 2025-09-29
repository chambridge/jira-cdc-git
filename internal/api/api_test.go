package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/chambrid/jira-cdc-git/pkg/jobs"
)

// TestAPIServer_HealthEndpoint tests the health check endpoint
func TestAPIServer_HealthEndpoint(t *testing.T) {
	server := createTestServer(t)
	req := httptest.NewRequest("GET", "/api/v1/health", nil)
	w := httptest.NewRecorder()

	server.handleHealth(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response Response
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if !response.Success {
		t.Error("Expected success to be true")
	}

	healthData, ok := response.Data.(map[string]interface{})
	if !ok {
		t.Fatal("Expected health data to be a map")
	}

	if healthData["status"] == nil {
		t.Error("Expected status field in health response")
	}
}

// TestAPIServer_SystemInfoEndpoint tests the system info endpoint
func TestAPIServer_SystemInfoEndpoint(t *testing.T) {
	server := createTestServer(t)
	req := httptest.NewRequest("GET", "/api/v1/system/info", nil)
	w := httptest.NewRecorder()

	server.handleSystemInfo(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response Response
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if !response.Success {
		t.Error("Expected success to be true")
	}

	systemData, ok := response.Data.(map[string]interface{})
	if !ok {
		t.Fatal("Expected system data to be a map")
	}

	requiredFields := []string{"version", "api_version", "platform", "capabilities"}
	for _, field := range requiredFields {
		if systemData[field] == nil {
			t.Errorf("Expected %s field in system info response", field)
		}
	}
}

// TestAPIServer_SingleSyncValidation tests single sync request validation
func TestAPIServer_SingleSyncValidation(t *testing.T) {
	server := createTestServer(t)

	tests := []struct {
		name           string
		request        SingleSyncRequest
		expectedStatus int
		expectedError  string
	}{
		{
			name: "valid request",
			request: SingleSyncRequest{
				IssueKey:   "PROJ-123",
				Repository: "/tmp/test-repo",
				Async:      false,
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "missing issue key",
			request: SingleSyncRequest{
				Repository: "/tmp/test-repo",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "VALIDATION_ERROR",
		},
		{
			name: "missing repository",
			request: SingleSyncRequest{
				IssueKey: "PROJ-123",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "VALIDATION_ERROR",
		},
		{
			name: "invalid issue key format",
			request: SingleSyncRequest{
				IssueKey:   "invalid-key",
				Repository: "/tmp/test-repo",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "VALIDATION_ERROR",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requestBody, _ := json.Marshal(tt.request)
			req := httptest.NewRequest("POST", "/api/v1/sync/single", bytes.NewBuffer(requestBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			server.handleSingleSync(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if tt.expectedError != "" {
				var response Response
				if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
					t.Fatalf("Failed to decode error response: %v", err)
				}

				if response.Success {
					t.Error("Expected success to be false for error case")
				}

				if response.Error == nil || response.Error.Code != tt.expectedError {
					t.Errorf("Expected error code %s, got %v", tt.expectedError, response.Error)
				}
			}
		})
	}
}

// TestAPIServer_BatchSyncValidation tests batch sync request validation
func TestAPIServer_BatchSyncValidation(t *testing.T) {
	server := createTestServer(t)

	tests := []struct {
		name           string
		request        BatchSyncRequest
		expectedStatus int
		expectedError  string
	}{
		{
			name: "valid request",
			request: BatchSyncRequest{
				IssueKeys:  []string{"PROJ-123", "PROJ-124"},
				Repository: "/tmp/test-repo",
				Async:      true,
			},
			expectedStatus: http.StatusAccepted,
		},
		{
			name: "empty issue keys",
			request: BatchSyncRequest{
				IssueKeys:  []string{},
				Repository: "/tmp/test-repo",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "VALIDATION_ERROR",
		},
		{
			name: "invalid parallelism",
			request: BatchSyncRequest{
				IssueKeys:   []string{"PROJ-123"},
				Repository:  "/tmp/test-repo",
				Parallelism: 15,
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "VALIDATION_ERROR",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requestBody, _ := json.Marshal(tt.request)
			req := httptest.NewRequest("POST", "/api/v1/sync/batch", bytes.NewBuffer(requestBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			server.handleBatchSync(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if tt.expectedError != "" {
				var response Response
				if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
					t.Fatalf("Failed to decode error response: %v", err)
				}

				if response.Success {
					t.Error("Expected success to be false for error case")
				}

				if response.Error == nil || response.Error.Code != tt.expectedError {
					t.Errorf("Expected error code %s, got %v", tt.expectedError, response.Error)
				}
			}
		})
	}
}

// TestAPIServer_ProfileEndpoints tests profile management endpoints
func TestAPIServer_ProfileEndpoints(t *testing.T) {
	server := createTestServer(t)

	t.Run("list profiles", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/profiles", nil)
		w := httptest.NewRecorder()

		server.handleListProfiles(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
		}

		var response Response
		if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if !response.Success {
			t.Error("Expected success to be true")
		}
	})

	t.Run("get profile not implemented", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/profiles/test-profile", nil)
		w := httptest.NewRecorder()

		server.handleGetProfile(w, req)

		if w.Code != http.StatusNotImplemented {
			t.Errorf("Expected status %d, got %d", http.StatusNotImplemented, w.Code)
		}

		var response Response
		if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if response.Success {
			t.Error("Expected success to be false for not implemented")
		}

		if response.Error == nil || response.Error.Code != "NOT_IMPLEMENTED" {
			t.Errorf("Expected NOT_IMPLEMENTED error, got %v", response.Error)
		}
	})
}

// TestAPIServer_ExtractPathParams tests path parameter extraction
func TestAPIServer_ExtractPathParams(t *testing.T) {
	server := createTestServer(t)

	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "job ID extraction",
			path:     "/api/v1/jobs/job-123",
			expected: "job-123",
		},
		{
			name:     "job ID from cancel path",
			path:     "/api/v1/jobs/job-456/cancel",
			expected: "job-456",
		},
		{
			name:     "profile name extraction",
			path:     "/api/v1/profiles/my-profile",
			expected: "my-profile",
		},
		{
			name:     "invalid path",
			path:     "/api/v1/invalid",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if strings.Contains(tt.path, "jobs") {
				result := server.extractJobIDFromPath(tt.path)
				if result != tt.expected {
					t.Errorf("Expected %s, got %s", tt.expected, result)
				}
			} else if strings.Contains(tt.path, "profiles") {
				result := server.extractProfileNameFromPath(tt.path)
				if result != tt.expected {
					t.Errorf("Expected %s, got %s", tt.expected, result)
				}
			}
		})
	}
}

// TestAPIServer_Middleware tests the middleware functionality
func TestAPIServer_Middleware(t *testing.T) {
	server := createTestServer(t)

	t.Run("CORS headers", func(t *testing.T) {
		mux := http.NewServeMux()
		mux.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		handler := server.withCORS(mux)
		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Header().Get("Access-Control-Allow-Origin") != "*" {
			t.Error("Expected CORS headers to be set")
		}
	})

	t.Run("OPTIONS preflight", func(t *testing.T) {
		mux := http.NewServeMux()
		handler := server.withCORS(mux)
		req := httptest.NewRequest("OPTIONS", "/test", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status %d for OPTIONS request, got %d", http.StatusOK, w.Code)
		}
	})
}

// TestAPIServer_ErrorHandling tests error response formatting
func TestAPIServer_ErrorHandling(t *testing.T) {
	server := createTestServer(t)

	w := httptest.NewRecorder()
	server.writeError(w, http.StatusBadRequest, "TEST_ERROR", "Test error message", "Error details")

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}

	var response Response
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode error response: %v", err)
	}

	if response.Success {
		t.Error("Expected success to be false for error response")
	}

	if response.Error == nil {
		t.Fatal("Expected error info to be present")
	}

	if response.Error.Code != "TEST_ERROR" {
		t.Errorf("Expected error code TEST_ERROR, got %s", response.Error.Code)
	}

	if response.Error.Message != "Test error message" {
		t.Errorf("Expected error message 'Test error message', got %s", response.Error.Message)
	}
}

// createTestServer creates a test server instance with mock dependencies
func createTestServer(t *testing.T) *Server {
	config := DefaultConfig()
	config.Port = 8888 // Use different port for testing

	buildInfo := BuildInfo{
		Version: "test-v0.4.0",
		Commit:  "test-commit",
		Date:    time.Now().Format("2006-01-02T15:04:05Z"),
	}

	// Create mock job manager
	mockJobManager := &MockJobManager{}

	return NewServer(config, buildInfo, mockJobManager)
}

// MockJobManager implements the jobs.JobManager interface for testing
type MockJobManager struct{}

func (m *MockJobManager) SubmitSingleIssueSync(ctx context.Context, req *jobs.SingleIssueSyncRequest) (*jobs.JobResult, error) {
	return &jobs.JobResult{
		JobID:  "test-job-single",
		Status: jobs.JobStatusPending,
	}, nil
}

func (m *MockJobManager) SubmitBatchSync(ctx context.Context, req *jobs.BatchSyncRequest) (*jobs.JobResult, error) {
	return &jobs.JobResult{
		JobID:  "test-job-batch",
		Status: jobs.JobStatusPending,
	}, nil
}

func (m *MockJobManager) SubmitJQLSync(ctx context.Context, req *jobs.JQLSyncRequest) (*jobs.JobResult, error) {
	return &jobs.JobResult{
		JobID:  "test-job-jql",
		Status: jobs.JobStatusPending,
	}, nil
}

func (m *MockJobManager) ExecuteLocalSync(ctx context.Context, req *jobs.LocalSyncRequest) (*jobs.SyncResult, error) {
	return &jobs.SyncResult{
		TotalIssues:     1,
		ProcessedIssues: 1,
		SuccessfulSync:  1,
		FailedSync:      0,
		Duration:        100 * time.Millisecond,
		ProcessedFiles:  []string{"/tmp/test-repo/projects/PROJ/issues/PROJ-123.yaml"},
		Errors:          []string{},
	}, nil
}

func (m *MockJobManager) ListJobs(ctx context.Context, filter *jobs.JobFilter) ([]*jobs.JobResult, error) {
	return []*jobs.JobResult{
		{
			JobID:           "test-job-1",
			Status:          jobs.JobStatusSucceeded,
			TotalIssues:     1,
			ProcessedIssues: 1,
			SuccessfulSync:  1,
			FailedSync:      0,
		},
	}, nil
}

func (m *MockJobManager) GetJob(ctx context.Context, jobID string) (*jobs.JobResult, error) {
	if jobID == "nonexistent" {
		return nil, jobs.NewJobError(jobID, "not_found", "Job not found")
	}

	return &jobs.JobResult{
		JobID:           jobID,
		Status:          jobs.JobStatusSucceeded,
		TotalIssues:     1,
		ProcessedIssues: 1,
		SuccessfulSync:  1,
		FailedSync:      0,
	}, nil
}

func (m *MockJobManager) DeleteJob(ctx context.Context, jobID string) error {
	if jobID == "nonexistent" {
		return jobs.NewJobError(jobID, "not_found", "Job not found")
	}
	return nil
}

func (m *MockJobManager) CancelJob(ctx context.Context, jobID string) error {
	if jobID == "nonexistent" {
		return jobs.NewJobError(jobID, "not_found", "Job not found")
	}
	return nil
}

func (m *MockJobManager) GetJobLogs(ctx context.Context, jobID string) (string, error) {
	if jobID == "nonexistent" {
		return "", jobs.NewJobError(jobID, "not_found", "Job not found")
	}

	return "2024-01-15T10:30:00Z INFO Starting sync operation\n2024-01-15T10:30:05Z INFO Sync completed successfully", nil
}

func (m *MockJobManager) WatchJob(ctx context.Context, jobID string) (<-chan jobs.JobMonitor, error) {
	if jobID == "nonexistent" {
		return nil, jobs.NewJobError(jobID, "not_found", "Job not found")
	}

	// Create a channel and immediately close it for testing
	ch := make(chan jobs.JobMonitor, 1)
	ch <- jobs.JobMonitor{
		JobID:     jobID,
		Status:    jobs.JobStatusSucceeded,
		Progress:  100.0,
		LastCheck: time.Now(),
		Message:   "Job completed successfully",
	}
	close(ch)
	return ch, nil
}

func (m *MockJobManager) GetQueueStatus(ctx context.Context) (*jobs.QueueStatus, error) {
	return &jobs.QueueStatus{
		TotalJobs:     10,
		PendingJobs:   2,
		RunningJobs:   1,
		CompletedJobs: 6,
		FailedJobs:    1,
	}, nil
}
