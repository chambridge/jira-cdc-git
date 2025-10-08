package apiclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	operatortypes "github.com/chambrid/jira-cdc-git/internal/operator/types"
	"github.com/go-logr/logr"
)

func TestNewAPIClient(t *testing.T) {
	baseURL := "http://localhost:8080"
	timeout := 30 * time.Second
	log := logr.Discard()

	client := NewAPIClient(baseURL, timeout, log)

	if client == nil {
		t.Fatal("Expected client to be created, got nil")
	}

	// Type assertion to access private fields for testing
	c, ok := client.(*Client)
	if !ok {
		t.Fatal("Expected client to be of type *Client")
	}

	if c.baseURL != baseURL {
		t.Errorf("Expected baseURL to be %s, got %s", baseURL, c.baseURL)
	}

	if c.httpClient.Timeout != timeout {
		t.Errorf("Expected timeout to be %v, got %v", timeout, c.httpClient.Timeout)
	}
}

func TestClient_TriggerSingleSync(t *testing.T) {
	tests := []struct {
		name           string
		request        *SingleSyncRequest
		serverResponse interface{}
		statusCode     int
		expectError    bool
		expectedJobID  string
	}{
		{
			name: "successful single sync",
			request: &SingleSyncRequest{
				IssueKey:   "PROJ-123",
				Repository: "/tmp/repo",
				Branch:     "main",
			},
			serverResponse: map[string]interface{}{
				"success": true,
				"data": map[string]interface{}{
					"success": true,
					"job_id":  "job-123",
					"message": "Sync job created",
				},
			},
			statusCode:    http.StatusOK,
			expectError:   false,
			expectedJobID: "job-123",
		},
		{
			name: "API error response",
			request: &SingleSyncRequest{
				IssueKey:   "INVALID",
				Repository: "/tmp/repo",
			},
			serverResponse: map[string]interface{}{
				"success": false,
				"error": map[string]interface{}{
					"code":    "VALIDATION_ERROR",
					"message": "Invalid issue key format",
				},
			},
			statusCode:  http.StatusBadRequest,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/v1/sync/single" {
					t.Errorf("Expected path /api/v1/sync/single, got %s", r.URL.Path)
				}
				if r.Method != "POST" {
					t.Errorf("Expected POST method, got %s", r.Method)
				}

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.statusCode)
				if err := json.NewEncoder(w).Encode(tt.serverResponse); err != nil {
					t.Errorf("Failed to encode response: %v", err)
				}
			}))
			defer server.Close()

			client := NewAPIClient(server.URL, 30*time.Second, logr.Discard())

			response, err := client.TriggerSingleSync(context.Background(), tt.request)

			if tt.expectError {
				if err == nil {
					t.Fatal("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if response.JobID != tt.expectedJobID {
				t.Errorf("Expected job ID %s, got %s", tt.expectedJobID, response.JobID)
			}
		})
	}
}

func TestClient_TriggerBatchSync(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/sync/batch" {
			t.Errorf("Expected path /api/v1/sync/batch, got %s", r.URL.Path)
		}

		response := map[string]interface{}{
			"success": true,
			"data": map[string]interface{}{
				"success": true,
				"job_id":  "batch-job-456",
				"message": "Batch sync job created",
			},
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Errorf("Failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	client := NewAPIClient(server.URL, 30*time.Second, logr.Discard())

	request := &BatchSyncRequest{
		IssueKeys:   []string{"PROJ-1", "PROJ-2"},
		Repository:  "/tmp/repo",
		Parallelism: 2,
	}

	response, err := client.TriggerBatchSync(context.Background(), request)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if response.JobID != "batch-job-456" {
		t.Errorf("Expected job ID batch-job-456, got %s", response.JobID)
	}
}

func TestClient_TriggerJQLSync(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/sync/jql" {
			t.Errorf("Expected path /api/v1/sync/jql, got %s", r.URL.Path)
		}

		response := map[string]interface{}{
			"success": true,
			"data": map[string]interface{}{
				"success": true,
				"job_id":  "jql-job-789",
				"message": "JQL sync job created",
			},
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Errorf("Failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	client := NewAPIClient(server.URL, 30*time.Second, logr.Discard())

	request := &JQLSyncRequest{
		JQLQuery:   "project = PROJ",
		Repository: "/tmp/repo",
	}

	response, err := client.TriggerJQLSync(context.Background(), request)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if response.JobID != "jql-job-789" {
		t.Errorf("Expected job ID jql-job-789, got %s", response.JobID)
	}
}

func TestClient_GetJobStatus(t *testing.T) {
	tests := []struct {
		name           string
		jobID          string
		serverResponse interface{}
		statusCode     int
		expectError    bool
		expectedStatus string
	}{
		{
			name:  "successful job status query",
			jobID: "job-123",
			serverResponse: map[string]interface{}{
				"success": true,
				"data": map[string]interface{}{
					"job_id":   "job-123",
					"status":   "completed",
					"progress": 100,
					"message":  "Sync completed successfully",
				},
			},
			statusCode:     http.StatusOK,
			expectError:    false,
			expectedStatus: "completed",
		},
		{
			name:  "job not found",
			jobID: "nonexistent",
			serverResponse: map[string]interface{}{
				"success": false,
				"error": map[string]interface{}{
					"code":    "NOT_FOUND",
					"message": "Job not found",
				},
			},
			statusCode:  http.StatusNotFound,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				expectedPath := "/api/v1/jobs/" + tt.jobID
				if r.URL.Path != expectedPath {
					t.Errorf("Expected path %s, got %s", expectedPath, r.URL.Path)
				}

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.statusCode)
				if err := json.NewEncoder(w).Encode(tt.serverResponse); err != nil {
					t.Errorf("Failed to encode response: %v", err)
				}
			}))
			defer server.Close()

			client := NewAPIClient(server.URL, 30*time.Second, logr.Discard())

			response, err := client.GetJobStatus(context.Background(), tt.jobID)

			if tt.expectError {
				if err == nil {
					t.Fatal("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if response.Status != tt.expectedStatus {
				t.Errorf("Expected status %s, got %s", tt.expectedStatus, response.Status)
			}
		})
	}
}

func TestClient_HealthCheck(t *testing.T) {
	tests := []struct {
		name        string
		statusCode  int
		expectError bool
	}{
		{
			name:        "healthy server",
			statusCode:  http.StatusOK,
			expectError: false,
		},
		{
			name:        "unhealthy server",
			statusCode:  http.StatusServiceUnavailable,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/v1/health" {
					t.Errorf("Expected path /api/v1/health, got %s", r.URL.Path)
				}
				w.WriteHeader(tt.statusCode)
			}))
			defer server.Close()

			client := NewAPIClient(server.URL, 30*time.Second, logr.Discard())

			err := client.HealthCheck(context.Background())

			if tt.expectError && err == nil {
				t.Fatal("Expected error, got nil")
			}

			if !tt.expectError && err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
		})
	}
}

func TestConvertJIRASyncToAPIRequest(t *testing.T) {
	tests := []struct {
		name         string
		jiraSync     *operatortypes.JIRASync
		expectedType string
		expectError  bool
	}{
		{
			name: "single sync conversion",
			jiraSync: &operatortypes.JIRASync{
				Spec: operatortypes.JIRASyncSpec{
					SyncType: "single",
					Target: operatortypes.SyncTarget{
						IssueKeys: []string{"PROJ-123"},
					},
					Destination: operatortypes.GitDestination{
						Repository: "/tmp/repo",
						Branch:     "main",
					},
				},
			},
			expectedType: "single",
			expectError:  false,
		},
		{
			name: "batch sync conversion",
			jiraSync: &operatortypes.JIRASync{
				Spec: operatortypes.JIRASyncSpec{
					SyncType: "batch",
					Target: operatortypes.SyncTarget{
						IssueKeys: []string{"PROJ-1", "PROJ-2"},
					},
					Destination: operatortypes.GitDestination{
						Repository: "/tmp/repo",
					},
				},
			},
			expectedType: "batch",
			expectError:  false,
		},
		{
			name: "JQL sync conversion",
			jiraSync: &operatortypes.JIRASync{
				Spec: operatortypes.JIRASyncSpec{
					SyncType: "jql",
					Target: operatortypes.SyncTarget{
						JQLQuery: "project = PROJ",
					},
					Destination: operatortypes.GitDestination{
						Repository: "/tmp/repo",
					},
				},
			},
			expectedType: "jql",
			expectError:  false,
		},
		{
			name: "unsupported sync type",
			jiraSync: &operatortypes.JIRASync{
				Spec: operatortypes.JIRASyncSpec{
					SyncType: "unknown",
				},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request, requestType, err := ConvertJIRASyncToAPIRequest(tt.jiraSync)

			if tt.expectError {
				if err == nil {
					t.Fatal("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if requestType != tt.expectedType {
				t.Errorf("Expected request type %s, got %s", tt.expectedType, requestType)
			}

			if request == nil {
				t.Fatal("Expected request to be non-nil")
			}
		})
	}
}

func TestAPIError_Error(t *testing.T) {
	tests := []struct {
		name     string
		apiError *APIError
		expected string
	}{
		{
			name: "error with details",
			apiError: &APIError{
				Code:    "VALIDATION_ERROR",
				Message: "Invalid request",
				Details: "Missing required field",
			},
			expected: "API error VALIDATION_ERROR: Invalid request (Missing required field)",
		},
		{
			name: "error without details",
			apiError: &APIError{
				Code:    "NOT_FOUND",
				Message: "Resource not found",
			},
			expected: "API error NOT_FOUND: Resource not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.apiError.Error()
			if result != tt.expected {
				t.Errorf("Expected error message %q, got %q", tt.expected, result)
			}
		})
	}
}
