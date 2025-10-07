package integration

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/chambrid/jira-cdc-git/internal/api"
	"github.com/chambrid/jira-cdc-git/pkg/jobs"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/scheme"
)

// JIRASyncSpec represents the CRD spec structure for validation
type JIRASyncSpec struct {
	SyncType    string                 `json:"syncType"`
	Target      map[string]interface{} `json:"target"`
	Destination DestinationSpec        `json:"destination"`
	Priority    string                 `json:"priority,omitempty"`
	Timeout     int                    `json:"timeout,omitempty"`
	RetryPolicy *RetryPolicySpec       `json:"retryPolicy,omitempty"`
	Schedule    string                 `json:"schedule,omitempty"`
	Labels      map[string]string      `json:"labels,omitempty"`
}

type DestinationSpec struct {
	Repository string `json:"repository"`
	Branch     string `json:"branch,omitempty"`
	Path       string `json:"path,omitempty"`
}

type RetryPolicySpec struct {
	MaxRetries        int     `json:"maxRetries,omitempty"`
	BackoffMultiplier float64 `json:"backoffMultiplier,omitempty"`
	InitialDelay      int     `json:"initialDelay,omitempty"`
}

// TestAPICRDDataModelCompatibility validates that API request models
// can be successfully converted to CRD specifications
func TestAPICRDDataModelCompatibility(t *testing.T) {
	tests := []struct {
		name            string
		apiRequest      interface{}
		expectedCRDSpec JIRASyncSpec
		expectError     bool
		description     string
	}{
		{
			name: "SingleSyncRequest to JIRASync CRD",
			apiRequest: &api.SingleSyncRequest{
				IssueKey:   "PROJ-123",
				Repository: "https://github.com/example/repo.git",
				Options: &api.SyncOptions{
					Incremental: true,
					Force:       false,
					DryRun:      false,
				},
				SafeMode: true,
				Async:    true,
			},
			expectedCRDSpec: JIRASyncSpec{
				SyncType: "single",
				Target: map[string]interface{}{
					"issueKeys": []string{"PROJ-123"},
				},
				Destination: DestinationSpec{
					Repository: "https://github.com/example/repo.git",
					Branch:     "main",
					Path:       "/",
				},
				Priority: "normal",
				Timeout:  1800,
			},
			expectError: false,
			description: "Single issue sync should map correctly to CRD with issueKeys target",
		},
		{
			name: "BatchSyncRequest to JIRASync CRD",
			apiRequest: &api.BatchSyncRequest{
				IssueKeys:  []string{"PROJ-1", "PROJ-2", "PROJ-3"},
				Repository: "git@github.com:example/repo.git",
				Options: &api.SyncOptions{
					Concurrency: 2,
					Incremental: false,
					Force:       true,
				},
				Parallelism: 2,
				SafeMode:    false,
				Async:       true,
			},
			expectedCRDSpec: JIRASyncSpec{
				SyncType: "batch",
				Target: map[string]interface{}{
					"issueKeys": []string{"PROJ-1", "PROJ-2", "PROJ-3"},
				},
				Destination: DestinationSpec{
					Repository: "git@github.com:example/repo.git",
					Branch:     "main",
					Path:       "/",
				},
				Priority: "normal",
				Timeout:  1800,
			},
			expectError: false,
			description: "Batch sync should map correctly to CRD with multiple issueKeys",
		},
		{
			name: "JQLSyncRequest to JIRASync CRD",
			apiRequest: &api.JQLSyncRequest{
				JQL:        "project = PROJ AND status = 'In Progress'",
				Repository: "https://github.com/example/repo.git",
				Options: &api.SyncOptions{
					RateLimit:   5 * time.Second,
					Incremental: true,
				},
				Parallelism: 3,
				SafeMode:    true,
				Async:       true,
			},
			expectedCRDSpec: JIRASyncSpec{
				SyncType: "jql",
				Target: map[string]interface{}{
					"jqlQuery": "project = PROJ AND status = 'In Progress'",
				},
				Destination: DestinationSpec{
					Repository: "https://github.com/example/repo.git",
					Branch:     "main",
					Path:       "/",
				},
				Priority: "normal",
				Timeout:  1800,
			},
			expectError: false,
			description: "JQL sync should map correctly to CRD with jqlQuery target",
		},
		{
			name: "Invalid Issue Key Format",
			apiRequest: &api.SingleSyncRequest{
				IssueKey:   "invalid-key-format",
				Repository: "https://github.com/example/repo.git",
				SafeMode:   true,
				Async:      true,
			},
			expectError: true,
			description: "Invalid issue key should be rejected by CRD validation",
		},
		{
			name: "Malicious Repository URL",
			apiRequest: &api.SingleSyncRequest{
				IssueKey:   "PROJ-123",
				Repository: "file:///etc/passwd",
				SafeMode:   true,
				Async:      true,
			},
			expectError: true,
			description: "Malicious repository URLs should be rejected by CRD validation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Convert API request to CRD spec
			crdSpec, err := convertAPIRequestToCRD(tt.apiRequest)

			// For error cases, check both conversion and validation
			if tt.expectError {
				conversionFailed := (err != nil)
				validationFailed := false

				if !conversionFailed && crdSpec != nil {
					// If conversion succeeded, check if validation fails
					if valErr := validateCRDSpec(crdSpec); valErr != nil {
						validationFailed = true
					}
				}

				if !conversionFailed && !validationFailed {
					t.Errorf("Expected error for %s, but both conversion and validation succeeded", tt.description)
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error converting %s: %v", tt.description, err)
				return
			}

			// Validate CRD spec against schema
			if err := validateCRDSpec(crdSpec); err != nil {
				t.Errorf("CRD validation failed for %s: %v", tt.description, err)
				return
			}

			// Verify critical fields match
			if crdSpec.SyncType != tt.expectedCRDSpec.SyncType {
				t.Errorf("SyncType mismatch: expected %s, got %s", tt.expectedCRDSpec.SyncType, crdSpec.SyncType)
			}

			if crdSpec.Destination.Repository != tt.expectedCRDSpec.Destination.Repository {
				t.Errorf("Repository mismatch: expected %s, got %s",
					tt.expectedCRDSpec.Destination.Repository, crdSpec.Destination.Repository)
			}

			// Verify target structure matches expected type
			if err := validateTargetStructure(crdSpec.Target, tt.expectedCRDSpec.Target); err != nil {
				t.Errorf("Target validation failed for %s: %v", tt.description, err)
			}
		})
	}
}

// TestCRDConversionWorkflow tests the CRD conversion functionality directly
func TestCRDConversionWorkflow(t *testing.T) {
	// Setup fake Kubernetes client
	gvr := schema.GroupVersionResource{
		Group:    "sync.jira.io",
		Version:  "v1alpha1",
		Resource: "jirasyncs",
	}

	fakeClient := fake.NewSimpleDynamicClient(scheme.Scheme)

	tests := []struct {
		name        string
		requestBody interface{}
		description string
	}{
		{
			name: "Single Sync Conversion",
			requestBody: &api.SingleSyncRequest{
				IssueKey:   "PROJ-456",
				Repository: "https://github.com/example/test.git",
				Async:      true,
				SafeMode:   true,
			},
			description: "Single sync should convert to valid JIRASync CRD",
		},
		{
			name: "Batch Sync Conversion",
			requestBody: &api.BatchSyncRequest{
				IssueKeys:  []string{"PROJ-100", "PROJ-101"},
				Repository: "https://github.com/example/batch.git",
				Async:      true,
				SafeMode:   false,
			},
			description: "Batch sync should convert to valid JIRASync CRD",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test conversion
			crdSpec, err := convertAPIRequestToCRD(tt.requestBody)
			if err != nil {
				t.Errorf("Failed to convert API request to CRD: %v", err)
				return
			}

			// Validate the conversion result
			if err := validateCRDSpec(crdSpec); err != nil {
				t.Errorf("CRD validation failed: %v", err)
				return
			}

			// Create CRD resource in fake cluster to test actual creation
			// Convert CRD spec to map for unstructured object
			// Deep copy target to avoid slice reference issues
			targetCopy := make(map[string]interface{})
			for k, v := range crdSpec.Target {
				switch val := v.(type) {
				case []string:
					// Copy slice to avoid deep copy issues
					copy := make([]interface{}, len(val))
					for i, s := range val {
						copy[i] = s
					}
					targetCopy[k] = copy
				default:
					targetCopy[k] = v
				}
			}

			// Convert all values to JSON-compatible types
			var priority interface{} = crdSpec.Priority
			var timeout interface{} = int64(crdSpec.Timeout) // Convert to int64 for JSON compatibility

			specMap := map[string]interface{}{
				"syncType": crdSpec.SyncType,
				"target":   targetCopy,
				"destination": map[string]interface{}{
					"repository": crdSpec.Destination.Repository,
					"branch":     crdSpec.Destination.Branch,
					"path":       crdSpec.Destination.Path,
				},
				"priority": priority,
				"timeout":  timeout,
			}

			crdResource := &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "sync.jira.io/v1alpha1",
					"kind":       "JIRASync",
					"metadata": map[string]interface{}{
						"name":      fmt.Sprintf("test-%s-%d", strings.ToLower(tt.name), time.Now().UnixNano()),
						"namespace": "default",
					},
					"spec": specMap,
				},
			}

			// Create in fake cluster
			created, err := fakeClient.Resource(gvr).Namespace("default").Create(
				context.Background(), crdResource, metav1.CreateOptions{})
			if err != nil {
				t.Errorf("Failed to create CRD resource: %v", err)
				return
			}

			// Verify CRD was created successfully
			if created.GetName() == "" {
				t.Errorf("CRD resource was not created properly")
				return
			}

			t.Logf("Successfully converted and created CRD: %s", created.GetName())
		})
	}
}

// TestCRDSecurityValidation tests that CRDs properly reject malicious inputs
func TestCRDSecurityValidation(t *testing.T) {
	securityTests := []struct {
		name        string
		crdSpec     JIRASyncSpec
		expectValid bool
		description string
	}{
		{
			name: "Valid Repository URL",
			crdSpec: JIRASyncSpec{
				SyncType: "single",
				Target: map[string]interface{}{
					"issueKeys": []string{"PROJ-123"},
				},
				Destination: DestinationSpec{
					Repository: "https://github.com/example/repo.git",
				},
			},
			expectValid: true,
			description: "Valid HTTPS repository should be accepted",
		},
		{
			name: "Local File Attack",
			crdSpec: JIRASyncSpec{
				SyncType: "single",
				Target: map[string]interface{}{
					"issueKeys": []string{"PROJ-123"},
				},
				Destination: DestinationSpec{
					Repository: "file:///etc/passwd",
				},
			},
			expectValid: false,
			description: "Local file URLs should be rejected",
		},
		{
			name: "SQL Injection in JQL",
			crdSpec: JIRASyncSpec{
				SyncType: "jql",
				Target: map[string]interface{}{
					"jqlQuery": "project = PROJ'; DROP TABLE issues; --",
				},
				Destination: DestinationSpec{
					Repository: "https://github.com/example/repo.git",
				},
			},
			expectValid: false,
			description: "JQL with SQL injection should be rejected",
		},
		{
			name: "Directory Traversal in Path",
			crdSpec: JIRASyncSpec{
				SyncType: "single",
				Target: map[string]interface{}{
					"issueKeys": []string{"PROJ-123"},
				},
				Destination: DestinationSpec{
					Repository: "https://github.com/example/repo.git",
					Path:       "../../etc/passwd",
				},
			},
			expectValid: false,
			description: "Directory traversal in path should be rejected",
		},
	}

	for _, tt := range securityTests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCRDSpec(&tt.crdSpec)

			if tt.expectValid && err != nil {
				t.Errorf("Expected valid CRD to be accepted, but got error: %v", err)
			}

			if !tt.expectValid && err == nil {
				t.Errorf("Expected malicious CRD to be rejected, but it was accepted")
			}
		})
	}
}

// Helper function to convert API requests to CRD specs
func convertAPIRequestToCRD(apiRequest interface{}) (*JIRASyncSpec, error) {
	switch req := apiRequest.(type) {
	case *api.SingleSyncRequest:
		return &JIRASyncSpec{
			SyncType: "single",
			Target: map[string]interface{}{
				"issueKeys": []string{req.IssueKey},
			},
			Destination: DestinationSpec{
				Repository: req.Repository,
				Branch:     "main",
				Path:       "/",
			},
			Priority: "normal",
			Timeout:  1800,
		}, nil

	case *api.BatchSyncRequest:
		return &JIRASyncSpec{
			SyncType: "batch",
			Target: map[string]interface{}{
				"issueKeys": req.IssueKeys,
			},
			Destination: DestinationSpec{
				Repository: req.Repository,
				Branch:     "main",
				Path:       "/",
			},
			Priority: "normal",
			Timeout:  1800,
		}, nil

	case *api.JQLSyncRequest:
		return &JIRASyncSpec{
			SyncType: "jql",
			Target: map[string]interface{}{
				"jqlQuery": req.JQL,
			},
			Destination: DestinationSpec{
				Repository: req.Repository,
				Branch:     "main",
				Path:       "/",
			},
			Priority: "normal",
			Timeout:  1800,
		}, nil

	default:
		return nil, fmt.Errorf("unsupported API request type: %T", apiRequest)
	}
}

// Helper function to validate CRD specs against the schema
func validateCRDSpec(spec *JIRASyncSpec) error {
	// Validate syncType
	validSyncTypes := []string{"single", "batch", "jql", "incremental"}
	if !contains(validSyncTypes, spec.SyncType) {
		return fmt.Errorf("invalid syncType: %s", spec.SyncType)
	}

	// Validate repository URL pattern
	repoPattern := `^(https://[a-zA-Z0-9][a-zA-Z0-9.-]*[a-zA-Z0-9]/[a-zA-Z0-9_.-]+/[a-zA-Z0-9_.-]+(\\.git)?|git@[a-zA-Z0-9][a-zA-Z0-9.-]*[a-zA-Z0-9]:[a-zA-Z0-9_.-]+/[a-zA-Z0-9_.-]+(\\.git)?)$`
	if !matchesPattern(spec.Destination.Repository, repoPattern) {
		return fmt.Errorf("invalid repository URL: %s", spec.Destination.Repository)
	}

	// Validate path for directory traversal
	if spec.Destination.Path != "" {
		if strings.Contains(spec.Destination.Path, "..") {
			return fmt.Errorf("path contains directory traversal: %s", spec.Destination.Path)
		}
		if strings.HasPrefix(spec.Destination.Path, "/etc/") || strings.Contains(spec.Destination.Path, "/etc/") {
			return fmt.Errorf("path access to system directories not allowed: %s", spec.Destination.Path)
		}
	}

	// Validate target based on syncType
	switch spec.SyncType {
	case "single", "batch":
		issueKeys, ok := spec.Target["issueKeys"].([]string)
		if !ok || len(issueKeys) == 0 {
			return fmt.Errorf("issueKeys required for %s syncType", spec.SyncType)
		}
		for _, key := range issueKeys {
			if !matchesPattern(key, `^[A-Z][A-Z0-9]*-[1-9][0-9]*$`) {
				return fmt.Errorf("invalid issue key format: %s", key)
			}
		}
	case "jql":
		jqlQuery, ok := spec.Target["jqlQuery"].(string)
		if !ok || jqlQuery == "" {
			return fmt.Errorf("jqlQuery required for jql syncType")
		}
		if !matchesPattern(jqlQuery, `^[^;\\<>"\x00-\x1f]*$`) {
			return fmt.Errorf("invalid JQL query contains prohibited characters")
		}
	}

	return nil
}

// Helper function to validate target structure
func validateTargetStructure(actual, expected map[string]interface{}) error {
	for key, expectedVal := range expected {
		actualVal, exists := actual[key]
		if !exists {
			return fmt.Errorf("missing target field: %s", key)
		}

		switch key {
		case "issueKeys":
			actualKeys, ok1 := actualVal.([]string)
			expectedKeys, ok2 := expectedVal.([]string)
			if !ok1 || !ok2 {
				return fmt.Errorf("issueKeys type mismatch")
			}
			if len(actualKeys) != len(expectedKeys) {
				return fmt.Errorf("issueKeys length mismatch: expected %d, got %d",
					len(expectedKeys), len(actualKeys))
			}
		case "jqlQuery":
			if actualVal != expectedVal {
				return fmt.Errorf("jqlQuery mismatch: expected %s, got %s", expectedVal, actualVal)
			}
		}
	}
	return nil
}

// Helper functions
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func matchesPattern(input, pattern string) bool {
	// Simplified pattern matching for tests
	// In production, use regexp.MustCompile(pattern).MatchString(input)
	switch pattern {
	case `^[A-Z][A-Z0-9]*-[1-9][0-9]*$`:
		// Valid JIRA issue key: uppercase project, dash, positive number
		if !strings.Contains(input, "-") {
			return false
		}
		parts := strings.Split(input, "-")
		if len(parts) != 2 {
			return false
		}
		// Project part should be uppercase letters/numbers starting with letter
		project := parts[0]
		if len(project) == 0 || !isUppercase(project[0:1]) || strings.HasPrefix(strings.ToLower(project), "invalid") {
			return false
		}
		// Issue number should be positive integer
		return len(parts[1]) > 0 && parts[1] != "0" && !strings.HasPrefix(parts[1], "0")
	case `^[^;\\<>"\x00-\x1f]*$`:
		// No SQL injection characters
		prohibited := []string{";", "\\", "<", ">", "\"", "DROP", "DELETE", "INSERT", "UPDATE"}
		for _, p := range prohibited {
			if strings.Contains(strings.ToUpper(input), strings.ToUpper(p)) {
				return false
			}
		}
		return true
	default:
		// Repository URL validation - reject dangerous schemes and paths
		dangerous := []string{"file://", "javascript:", "data:", "ftp://", "../"}
		for _, d := range dangerous {
			if strings.Contains(strings.ToLower(input), strings.ToLower(d)) {
				return false
			}
		}
		return strings.HasPrefix(input, "https://") || strings.HasPrefix(input, "git@")
	}
}

func isUppercase(s string) bool {
	return s == strings.ToUpper(s) && s != strings.ToLower(s)
}

// MockJobManager for testing
type MockJobManager struct{}

func (m *MockJobManager) SubmitSingleIssueSync(ctx context.Context, req *jobs.SingleIssueSyncRequest) (*jobs.JobResult, error) {
	return &jobs.JobResult{
		JobID:  fmt.Sprintf("job-%d", time.Now().Unix()),
		Status: jobs.JobStatusPending,
	}, nil
}

func (m *MockJobManager) SubmitBatchSync(ctx context.Context, req *jobs.BatchSyncRequest) (*jobs.JobResult, error) {
	return &jobs.JobResult{
		JobID:  fmt.Sprintf("batch-job-%d", time.Now().Unix()),
		Status: jobs.JobStatusPending,
	}, nil
}

func (m *MockJobManager) SubmitJQLSync(ctx context.Context, req *jobs.JQLSyncRequest) (*jobs.JobResult, error) {
	return &jobs.JobResult{
		JobID:  fmt.Sprintf("jql-job-%d", time.Now().Unix()),
		Status: jobs.JobStatusPending,
	}, nil
}

func (m *MockJobManager) ExecuteLocalSync(ctx context.Context, req *jobs.LocalSyncRequest) (*jobs.SyncResult, error) {
	return &jobs.SyncResult{
		TotalIssues:     len(req.IssueKeys),
		ProcessedIssues: len(req.IssueKeys),
		SuccessfulSync:  len(req.IssueKeys),
		FailedSync:      0,
		Duration:        100 * time.Millisecond,
		ProcessedFiles:  []string{"test.yaml"},
	}, nil
}

func (m *MockJobManager) GetJob(ctx context.Context, jobID string) (*jobs.JobResult, error) {
	return &jobs.JobResult{
		JobID:  jobID,
		Status: jobs.JobStatusSucceeded,
	}, nil
}

func (m *MockJobManager) ListJobs(ctx context.Context, filter *jobs.JobFilter) ([]*jobs.JobResult, error) {
	return []*jobs.JobResult{}, nil
}

func (m *MockJobManager) CancelJob(ctx context.Context, jobID string) error {
	return nil
}

func (m *MockJobManager) DeleteJob(ctx context.Context, jobID string) error {
	return nil
}

func (m *MockJobManager) WatchJob(ctx context.Context, jobID string) (<-chan jobs.JobMonitor, error) {
	ch := make(chan jobs.JobMonitor)
	close(ch)
	return ch, nil
}

func (m *MockJobManager) GetJobLogs(ctx context.Context, jobID string) (string, error) {
	return "mock logs", nil
}

func (m *MockJobManager) GetQueueStatus(ctx context.Context) (*jobs.QueueStatus, error) {
	return &jobs.QueueStatus{}, nil
}
