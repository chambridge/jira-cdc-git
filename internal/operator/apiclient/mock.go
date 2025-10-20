package apiclient

import (
	"context"
	"fmt"
)

// MockAPIClient is a mock implementation of APIClient for testing
type MockAPIClient struct {
	TriggerSingleSyncFunc func(ctx context.Context, request *SingleSyncRequest) (*SyncJobResponse, error)
	TriggerBatchSyncFunc  func(ctx context.Context, request *BatchSyncRequest) (*SyncJobResponse, error)
	TriggerJQLSyncFunc    func(ctx context.Context, request *JQLSyncRequest) (*SyncJobResponse, error)
	GetJobStatusFunc      func(ctx context.Context, jobID string) (*JobStatusResponse, error)
	HealthCheckFunc       func(ctx context.Context) error
	DirectHealthCheckFunc func(ctx context.Context) error

	// Call tracking
	TriggerSingleSyncCalls []SingleSyncRequest
	TriggerBatchSyncCalls  []BatchSyncRequest
	TriggerJQLSyncCalls    []JQLSyncRequest
	GetJobStatusCalls      []string
	HealthCheckCalls       int
	DirectHealthCheckCalls int
}

// NewMockAPIClient creates a new mock API client
func NewMockAPIClient() *MockAPIClient {
	return &MockAPIClient{
		TriggerSingleSyncCalls: make([]SingleSyncRequest, 0),
		TriggerBatchSyncCalls:  make([]BatchSyncRequest, 0),
		TriggerJQLSyncCalls:    make([]JQLSyncRequest, 0),
		GetJobStatusCalls:      make([]string, 0),
	}
}

// TriggerSingleSync implements APIClient.TriggerSingleSync
func (m *MockAPIClient) TriggerSingleSync(ctx context.Context, request *SingleSyncRequest) (*SyncJobResponse, error) {
	if request != nil {
		m.TriggerSingleSyncCalls = append(m.TriggerSingleSyncCalls, *request)
	}

	if m.TriggerSingleSyncFunc != nil {
		return m.TriggerSingleSyncFunc(ctx, request)
	}

	// Default behavior
	return &SyncJobResponse{
		Success: true,
		JobID:   "mock-job-123",
		Message: "Mock sync job created",
	}, nil
}

// TriggerBatchSync implements APIClient.TriggerBatchSync
func (m *MockAPIClient) TriggerBatchSync(ctx context.Context, request *BatchSyncRequest) (*SyncJobResponse, error) {
	if request != nil {
		m.TriggerBatchSyncCalls = append(m.TriggerBatchSyncCalls, *request)
	}

	if m.TriggerBatchSyncFunc != nil {
		return m.TriggerBatchSyncFunc(ctx, request)
	}

	// Default behavior
	return &SyncJobResponse{
		Success: true,
		JobID:   "mock-batch-456",
		Message: "Mock batch sync job created",
	}, nil
}

// TriggerJQLSync implements APIClient.TriggerJQLSync
func (m *MockAPIClient) TriggerJQLSync(ctx context.Context, request *JQLSyncRequest) (*SyncJobResponse, error) {
	if request != nil {
		m.TriggerJQLSyncCalls = append(m.TriggerJQLSyncCalls, *request)
	}

	if m.TriggerJQLSyncFunc != nil {
		return m.TriggerJQLSyncFunc(ctx, request)
	}

	// Default behavior
	return &SyncJobResponse{
		Success: true,
		JobID:   "mock-jql-789",
		Message: "Mock JQL sync job created",
	}, nil
}

// GetJobStatus implements APIClient.GetJobStatus
func (m *MockAPIClient) GetJobStatus(ctx context.Context, jobID string) (*JobStatusResponse, error) {
	m.GetJobStatusCalls = append(m.GetJobStatusCalls, jobID)

	if m.GetJobStatusFunc != nil {
		return m.GetJobStatusFunc(ctx, jobID)
	}

	// Default behavior - job completed
	return &JobStatusResponse{
		JobID:    jobID,
		Status:   "completed",
		Progress: 100,
		Message:  "Mock job completed successfully",
	}, nil
}

// HealthCheck implements APIClient.HealthCheck
func (m *MockAPIClient) HealthCheck(ctx context.Context) error {
	m.HealthCheckCalls++

	if m.HealthCheckFunc != nil {
		return m.HealthCheckFunc(ctx)
	}

	// Default behavior - healthy
	return nil
}

// DirectHealthCheck implements APIClient.DirectHealthCheck
func (m *MockAPIClient) DirectHealthCheck(ctx context.Context) error {
	m.DirectHealthCheckCalls++

	if m.DirectHealthCheckFunc != nil {
		return m.DirectHealthCheckFunc(ctx)
	}

	// Default behavior - healthy
	return nil
}

// Reset clears all call tracking
func (m *MockAPIClient) Reset() {
	m.TriggerSingleSyncCalls = make([]SingleSyncRequest, 0)
	m.TriggerBatchSyncCalls = make([]BatchSyncRequest, 0)
	m.TriggerJQLSyncCalls = make([]JQLSyncRequest, 0)
	m.GetJobStatusCalls = make([]string, 0)
	m.HealthCheckCalls = 0
	m.DirectHealthCheckCalls = 0
}

// SetJobStatusResponse sets a custom response for GetJobStatus
func (m *MockAPIClient) SetJobStatusResponse(jobID, status string, progress int, message string) {
	m.GetJobStatusFunc = func(ctx context.Context, id string) (*JobStatusResponse, error) {
		if id == jobID {
			return &JobStatusResponse{
				JobID:    id,
				Status:   status,
				Progress: progress,
				Message:  message,
			}, nil
		}
		return nil, fmt.Errorf("job not found: %s", id)
	}
}

// SetSyncError sets the functions to return errors
func (m *MockAPIClient) SetSyncError(err error) {
	m.TriggerSingleSyncFunc = func(ctx context.Context, request *SingleSyncRequest) (*SyncJobResponse, error) {
		return nil, err
	}
	m.TriggerBatchSyncFunc = func(ctx context.Context, request *BatchSyncRequest) (*SyncJobResponse, error) {
		return nil, err
	}
	m.TriggerJQLSyncFunc = func(ctx context.Context, request *JQLSyncRequest) (*SyncJobResponse, error) {
		return nil, err
	}
}

// WithHost implements APIClient.WithHost for testing
func (m *MockAPIClient) WithHost(hostURL string) APIClient {
	// For mock client, just return self as we don't need to track host changes
	return m
}
