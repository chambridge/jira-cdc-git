package apiclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	operatortypes "github.com/chambrid/jira-cdc-git/internal/operator/types"
	"github.com/go-logr/logr"
)

// APIClient represents a client for communicating with the v0.4.0 API server
type APIClient interface {
	// TriggerSingleSync triggers a single issue sync operation via the API
	TriggerSingleSync(ctx context.Context, request *SingleSyncRequest) (*SyncJobResponse, error)

	// TriggerBatchSync triggers a batch sync operation via the API
	TriggerBatchSync(ctx context.Context, request *BatchSyncRequest) (*SyncJobResponse, error)

	// TriggerJQLSync triggers a JQL-based sync operation via the API
	TriggerJQLSync(ctx context.Context, request *JQLSyncRequest) (*SyncJobResponse, error)

	// GetJobStatus retrieves the status of a sync job
	GetJobStatus(ctx context.Context, jobID string) (*JobStatusResponse, error)

	// HealthCheck performs a health check against the API server
	HealthCheck(ctx context.Context) error
}

// Client implements the APIClient interface
type Client struct {
	baseURL    string
	httpClient *http.Client
	log        logr.Logger

	// Authentication
	authToken string
	authType  string // "bearer", "api-key", or ""

	// Circuit breaker for error handling
	circuitBreaker *CircuitBreaker
}

// CircuitBreaker implements a simple circuit breaker pattern
type CircuitBreaker struct {
	maxFailures  int
	resetTimeout time.Duration
	failureCount int
	lastFailTime time.Time
	state        CircuitState
}

type CircuitState int

const (
	CircuitClosed CircuitState = iota
	CircuitOpen
	CircuitHalfOpen
)

// NewAPIClient creates a new API client instance
func NewAPIClient(baseURL string, timeout time.Duration, log logr.Logger) APIClient {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		log: log,
		circuitBreaker: &CircuitBreaker{
			maxFailures:  3,                // Open circuit after 3 failures
			resetTimeout: 60 * time.Second, // Try to reset after 60 seconds
			state:        CircuitClosed,
		},
	}
}

// NewAPIClientWithAuth creates a new API client with authentication
func NewAPIClientWithAuth(baseURL string, timeout time.Duration, authType, authToken string, log logr.Logger) APIClient {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		log:       log,
		authToken: authToken,
		authType:  authType,
		circuitBreaker: &CircuitBreaker{
			maxFailures:  3,
			resetTimeout: 60 * time.Second,
			state:        CircuitClosed,
		},
	}
}

// SingleSyncRequest represents a single issue sync request
type SingleSyncRequest struct {
	IssueKey   string `json:"issue_key"`
	Repository string `json:"repository"`
	Branch     string `json:"branch,omitempty"`
	DryRun     bool   `json:"dry_run,omitempty"`
}

// BatchSyncRequest represents a batch sync request
type BatchSyncRequest struct {
	IssueKeys   []string `json:"issue_keys"`
	Repository  string   `json:"repository"`
	Branch      string   `json:"branch,omitempty"`
	Parallelism int      `json:"parallelism,omitempty"`
	DryRun      bool     `json:"dry_run,omitempty"`
}

// JQLSyncRequest represents a JQL-based sync request
type JQLSyncRequest struct {
	JQLQuery   string `json:"jql_query"`
	Repository string `json:"repository"`
	Branch     string `json:"branch,omitempty"`
	DryRun     bool   `json:"dry_run,omitempty"`
}

// SyncJobResponse represents the response from a sync operation trigger
type SyncJobResponse struct {
	Success bool   `json:"success"`
	JobID   string `json:"job_id"`
	Message string `json:"message"`
}

// JobStatusResponse represents the response for job status queries
type JobStatusResponse struct {
	JobID     string            `json:"job_id"`
	Status    string            `json:"status"` // pending, running, completed, failed
	Progress  int               `json:"progress"`
	Message   string            `json:"message,omitempty"`
	StartTime *time.Time        `json:"start_time,omitempty"`
	EndTime   *time.Time        `json:"end_time,omitempty"`
	Results   map[string]string `json:"results,omitempty"`
}

// APIError represents an API error response
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

func (e *APIError) Error() string {
	if e.Details != "" {
		return fmt.Sprintf("API error %s: %s (%s)", e.Code, e.Message, e.Details)
	}
	return fmt.Sprintf("API error %s: %s", e.Code, e.Message)
}

// TriggerSingleSync implements APIClient.TriggerSingleSync
func (c *Client) TriggerSingleSync(ctx context.Context, request *SingleSyncRequest) (*SyncJobResponse, error) {
	endpoint := "/api/v1/sync/single"
	return c.makeRequest(ctx, "POST", endpoint, request)
}

// TriggerBatchSync implements APIClient.TriggerBatchSync
func (c *Client) TriggerBatchSync(ctx context.Context, request *BatchSyncRequest) (*SyncJobResponse, error) {
	endpoint := "/api/v1/sync/batch"
	return c.makeRequest(ctx, "POST", endpoint, request)
}

// TriggerJQLSync implements APIClient.TriggerJQLSync
func (c *Client) TriggerJQLSync(ctx context.Context, request *JQLSyncRequest) (*SyncJobResponse, error) {
	endpoint := "/api/v1/sync/jql"
	return c.makeRequest(ctx, "POST", endpoint, request)
}

// GetJobStatus implements APIClient.GetJobStatus
func (c *Client) GetJobStatus(ctx context.Context, jobID string) (*JobStatusResponse, error) {
	endpoint := fmt.Sprintf("/api/v1/jobs/%s", url.PathEscape(jobID))

	resp, err := c.makeHTTPRequest(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			c.log.Error(err, "Failed to close response body")
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, c.handleAPIError(resp)
	}

	var apiResponse struct {
		Success bool               `json:"success"`
		Data    *JobStatusResponse `json:"data"`
		Error   *APIError          `json:"error"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&apiResponse); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if !apiResponse.Success || apiResponse.Data == nil {
		if apiResponse.Error != nil {
			return nil, apiResponse.Error
		}
		return nil, fmt.Errorf("API request failed")
	}

	return apiResponse.Data, nil
}

// HealthCheck implements APIClient.HealthCheck
func (c *Client) HealthCheck(ctx context.Context) error {
	endpoint := "/api/v1/health"

	resp, err := c.makeHTTPRequest(ctx, "GET", endpoint, nil)
	if err != nil {
		return fmt.Errorf("health check request failed: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			c.log.Error(err, "Failed to close response body")
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check failed with status %d", resp.StatusCode)
	}

	return nil
}

// makeRequest is a helper method for making sync operation requests
func (c *Client) makeRequest(ctx context.Context, method, endpoint string, request interface{}) (*SyncJobResponse, error) {
	resp, err := c.makeHTTPRequest(ctx, method, endpoint, request)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			c.log.Error(err, "Failed to close response body")
		}
	}()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, c.handleAPIError(resp)
	}

	var apiResponse struct {
		Success bool             `json:"success"`
		Data    *SyncJobResponse `json:"data"`
		Error   *APIError        `json:"error"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&apiResponse); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if !apiResponse.Success {
		if apiResponse.Error != nil {
			return nil, apiResponse.Error
		}
		return nil, fmt.Errorf("API request failed")
	}

	// Handle case where Data might be null but Success is true
	if apiResponse.Data == nil {
		return &SyncJobResponse{
			Success: true,
			Message: "Request processed successfully",
		}, nil
	}

	return apiResponse.Data, nil
}

// makeHTTPRequest performs the actual HTTP request with circuit breaker and auth
func (c *Client) makeHTTPRequest(ctx context.Context, method, endpoint string, body interface{}) (*http.Response, error) {
	// Check circuit breaker state
	if err := c.checkCircuitBreaker(); err != nil {
		return nil, err
	}

	var reqBody io.Reader

	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(jsonBody)
	}

	fullURL := c.baseURL + endpoint
	req, err := http.NewRequestWithContext(ctx, method, fullURL, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "jira-sync-operator/v0.4.1")

	// Add authentication
	c.addAuthentication(req)

	c.log.V(1).Info("Making API request", "method", method, "url", fullURL)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.recordFailure()
		return nil, fmt.Errorf("request failed: %w", err)
	}

	// Check for errors that should trigger circuit breaker
	if resp.StatusCode >= 500 {
		c.recordFailure()
	} else {
		c.recordSuccess()
	}

	c.log.V(1).Info("API response received", "status", resp.StatusCode)

	return resp, nil
}

// addAuthentication adds authentication headers to the request
func (c *Client) addAuthentication(req *http.Request) {
	if c.authToken == "" {
		return
	}

	switch c.authType {
	case "bearer":
		req.Header.Set("Authorization", "Bearer "+c.authToken)
	case "api-key":
		req.Header.Set("X-API-Key", c.authToken)
	case "basic":
		req.Header.Set("Authorization", "Basic "+c.authToken)
	default:
		// Default to bearer
		req.Header.Set("Authorization", "Bearer "+c.authToken)
	}
}

// checkCircuitBreaker checks if requests should be allowed
func (c *Client) checkCircuitBreaker() error {
	switch c.circuitBreaker.state {
	case CircuitOpen:
		if time.Since(c.circuitBreaker.lastFailTime) > c.circuitBreaker.resetTimeout {
			c.circuitBreaker.state = CircuitHalfOpen
			return nil
		}
		return fmt.Errorf("circuit breaker is open - API requests blocked")
	case CircuitHalfOpen:
		// Allow one request to test if service is back
		return nil
	case CircuitClosed:
		return nil
	default:
		return nil
	}
}

// recordFailure records a failure for the circuit breaker
func (c *Client) recordFailure() {
	c.circuitBreaker.failureCount++
	c.circuitBreaker.lastFailTime = time.Now()

	if c.circuitBreaker.failureCount >= c.circuitBreaker.maxFailures {
		c.circuitBreaker.state = CircuitOpen
		c.log.Info("Circuit breaker opened due to failures", "failures", c.circuitBreaker.failureCount)
	}
}

// recordSuccess records a success for the circuit breaker
func (c *Client) recordSuccess() {
	c.circuitBreaker.failureCount = 0

	if c.circuitBreaker.state == CircuitHalfOpen {
		c.circuitBreaker.state = CircuitClosed
		c.log.Info("Circuit breaker closed - service recovered")
	}
}

// handleAPIError processes API error responses
func (c *Client) handleAPIError(resp *http.Response) error {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("API request failed with status %d (failed to read error response)", resp.StatusCode)
	}

	var apiResponse struct {
		Success bool      `json:"success"`
		Error   *APIError `json:"error"`
	}

	if err := json.Unmarshal(body, &apiResponse); err != nil {
		return fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	if apiResponse.Error != nil {
		return apiResponse.Error
	}

	return fmt.Errorf("API request failed with status %d", resp.StatusCode)
}

// ConvertJIRASyncToAPIRequest converts a JIRASync CRD to appropriate API request
func ConvertJIRASyncToAPIRequest(jiraSync *operatortypes.JIRASync) (interface{}, string, error) {
	switch jiraSync.Spec.SyncType {
	case "single":
		if len(jiraSync.Spec.Target.IssueKeys) == 0 {
			return nil, "", fmt.Errorf("single sync requires at least one issue key")
		}
		return &SingleSyncRequest{
			IssueKey:   jiraSync.Spec.Target.IssueKeys[0],
			Repository: jiraSync.Spec.Destination.Repository,
			Branch:     jiraSync.Spec.Destination.Branch,
			DryRun:     false, // DryRun not supported in CRD yet
		}, "single", nil

	case "batch":
		if len(jiraSync.Spec.Target.IssueKeys) == 0 {
			return nil, "", fmt.Errorf("batch sync requires at least one issue key")
		}
		return &BatchSyncRequest{
			IssueKeys:   jiraSync.Spec.Target.IssueKeys,
			Repository:  jiraSync.Spec.Destination.Repository,
			Branch:      jiraSync.Spec.Destination.Branch,
			Parallelism: 1,     // Default parallelism, not configurable in CRD yet
			DryRun:      false, // DryRun not supported in CRD yet
		}, "batch", nil

	case "jql", "incremental":
		if jiraSync.Spec.Target.JQLQuery == "" {
			return nil, "", fmt.Errorf("JQL sync requires a JQL query")
		}
		return &JQLSyncRequest{
			JQLQuery:   jiraSync.Spec.Target.JQLQuery,
			Repository: jiraSync.Spec.Destination.Repository,
			Branch:     jiraSync.Spec.Destination.Branch,
			DryRun:     false, // DryRun not supported in CRD yet
		}, "jql", nil

	default:
		return nil, "", fmt.Errorf("unsupported sync type: %s", jiraSync.Spec.SyncType)
	}
}
