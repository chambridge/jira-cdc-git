package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/chambrid/jira-cdc-git/pkg/jobs"
)

// SingleSyncRequest represents a single issue sync request
type SingleSyncRequest struct {
	IssueKey   string                        `json:"issue_key" validate:"required"`
	Repository string                        `json:"repository" validate:"required"`
	Options    *SyncOptions                  `json:"options,omitempty"`
	Resources  *jobs.JobResourceRequirements `json:"resources,omitempty"`
	SafeMode   bool                          `json:"safe_mode,omitempty"`
	Async      bool                          `json:"async,omitempty"`
}

// BatchSyncRequest represents a batch issue sync request
type BatchSyncRequest struct {
	IssueKeys   []string                      `json:"issue_keys" validate:"required,min=1"`
	Repository  string                        `json:"repository" validate:"required"`
	Options     *SyncOptions                  `json:"options,omitempty"`
	Resources   *jobs.JobResourceRequirements `json:"resources,omitempty"`
	Parallelism int                           `json:"parallelism,omitempty"`
	SafeMode    bool                          `json:"safe_mode,omitempty"`
	Async       bool                          `json:"async,omitempty"`
}

// JQLSyncRequest represents a JQL query-based sync request
type JQLSyncRequest struct {
	JQL         string                        `json:"jql" validate:"required"`
	Repository  string                        `json:"repository" validate:"required"`
	Options     *SyncOptions                  `json:"options,omitempty"`
	Resources   *jobs.JobResourceRequirements `json:"resources,omitempty"`
	Parallelism int                           `json:"parallelism,omitempty"`
	SafeMode    bool                          `json:"safe_mode,omitempty"`
	Async       bool                          `json:"async,omitempty"`
}

// SyncOptions represents sync operation options
type SyncOptions struct {
	Concurrency  int           `json:"concurrency,omitempty"`
	RateLimit    time.Duration `json:"rate_limit,omitempty"`
	Incremental  bool          `json:"incremental,omitempty"`
	Force        bool          `json:"force,omitempty"`
	DryRun       bool          `json:"dry_run,omitempty"`
	IncludeLinks bool          `json:"include_links,omitempty"`
}

// SyncResponse represents a sync operation response
type SyncResponse struct {
	JobID     string      `json:"job_id"`
	Status    string      `json:"status"`
	CreatedAt time.Time   `json:"created_at"`
	StartedAt *time.Time  `json:"started_at,omitempty"`
	Result    *SyncResult `json:"result,omitempty"`
}

// SyncResult represents sync operation results (for synchronous operations)
type SyncResult struct {
	TotalIssues     int           `json:"total_issues"`
	ProcessedIssues int           `json:"processed_issues"`
	SuccessfulSync  int           `json:"successful_sync"`
	FailedSync      int           `json:"failed_sync"`
	Duration        time.Duration `json:"duration"`
	ProcessedFiles  []string      `json:"processed_files,omitempty"`
	Errors          []SyncError   `json:"errors,omitempty"`
}

// SyncError represents a sync operation error
type SyncError struct {
	IssueKey string `json:"issue_key"`
	Step     string `json:"step"`
	Message  string `json:"message"`
}

// handleSingleSync handles single issue sync requests
func (s *Server) handleSingleSync(w http.ResponseWriter, r *http.Request) {
	var req SingleSyncRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid JSON request body", err.Error())
		return
	}

	// Validate request
	if err := s.validateSingleSyncRequest(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Request validation failed", err.Error())
		return
	}

	// Check if async operation is requested
	if req.Async {
		response, err := s.createAsyncSingleSync(r.Context(), &req)
		if err != nil {
			s.writeError(w, http.StatusInternalServerError, "SYNC_ERROR", "Failed to create sync job", err.Error())
			return
		}
		s.writeJSON(w, http.StatusAccepted, response)
		return
	}

	// Perform synchronous sync (for small operations)
	response, err := s.performSyncSingleSync(r.Context(), &req)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "SYNC_ERROR", "Sync operation failed", err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, response)
}

// handleBatchSync handles batch issue sync requests
func (s *Server) handleBatchSync(w http.ResponseWriter, r *http.Request) {
	var req BatchSyncRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid JSON request body", err.Error())
		return
	}

	// Validate request
	if err := s.validateBatchSyncRequest(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Request validation failed", err.Error())
		return
	}

	// Batch operations are always async for scalability
	response, err := s.createAsyncBatchSync(r.Context(), &req)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "SYNC_ERROR", "Failed to create batch sync job", err.Error())
		return
	}

	s.writeJSON(w, http.StatusAccepted, response)
}

// handleJQLSync handles JQL query-based sync requests
func (s *Server) handleJQLSync(w http.ResponseWriter, r *http.Request) {
	var req JQLSyncRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid JSON request body", err.Error())
		return
	}

	// Validate request
	if err := s.validateJQLSyncRequest(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Request validation failed", err.Error())
		return
	}

	// JQL operations are always async due to potentially large result sets
	response, err := s.createAsyncJQLSync(r.Context(), &req)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "SYNC_ERROR", "Failed to create JQL sync job", err.Error())
		return
	}

	s.writeJSON(w, http.StatusAccepted, response)
}

// validateSingleSyncRequest validates a single sync request
func (s *Server) validateSingleSyncRequest(req *SingleSyncRequest) error {
	if req.IssueKey == "" {
		return fmt.Errorf("issue_key is required")
	}
	if req.Repository == "" {
		return fmt.Errorf("repository is required")
	}

	// Validate issue key format (basic validation)
	if !isValidIssueKey(req.IssueKey) {
		return fmt.Errorf("invalid issue key format: %s", req.IssueKey)
	}

	return s.validateSyncOptions(req.Options)
}

// validateBatchSyncRequest validates a batch sync request
func (s *Server) validateBatchSyncRequest(req *BatchSyncRequest) error {
	if len(req.IssueKeys) == 0 {
		return fmt.Errorf("issue_keys is required and cannot be empty")
	}
	if req.Repository == "" {
		return fmt.Errorf("repository is required")
	}

	// Validate issue key formats
	for _, issueKey := range req.IssueKeys {
		if !isValidIssueKey(issueKey) {
			return fmt.Errorf("invalid issue key format: %s", issueKey)
		}
	}

	// Validate parallelism
	if req.Parallelism < 0 || req.Parallelism > 10 {
		return fmt.Errorf("parallelism must be between 0 and 10")
	}

	return s.validateSyncOptions(req.Options)
}

// validateJQLSyncRequest validates a JQL sync request
func (s *Server) validateJQLSyncRequest(req *JQLSyncRequest) error {
	if req.JQL == "" {
		return fmt.Errorf("jql is required")
	}
	if req.Repository == "" {
		return fmt.Errorf("repository is required")
	}

	// Basic JQL validation (more sophisticated validation would be in JIRA client)
	if len(req.JQL) < 5 {
		return fmt.Errorf("JQL query too short, minimum 5 characters")
	}

	// Validate parallelism
	if req.Parallelism < 0 || req.Parallelism > 10 {
		return fmt.Errorf("parallelism must be between 0 and 10")
	}

	return s.validateSyncOptions(req.Options)
}

// validateSyncOptions validates sync options
func (s *Server) validateSyncOptions(options *SyncOptions) error {
	if options == nil {
		return nil
	}

	if options.Concurrency < 0 || options.Concurrency > 10 {
		return fmt.Errorf("concurrency must be between 0 and 10")
	}

	if options.Incremental && options.Force {
		return fmt.Errorf("incremental and force options are mutually exclusive")
	}

	return nil
}

// isValidIssueKey performs basic JIRA issue key validation
func isValidIssueKey(issueKey string) bool {
	// Basic validation: PROJECT-NUMBER format
	parts := strings.Split(issueKey, "-")
	if len(parts) < 2 {
		return false
	}

	// Last part should be a number
	lastPart := parts[len(parts)-1]
	for _, char := range lastPart {
		if char < '0' || char > '9' {
			return false
		}
	}

	return len(issueKey) > 0 && len(issueKey) < 50
}

// createAsyncSingleSync creates an async single issue sync job
func (s *Server) createAsyncSingleSync(ctx context.Context, req *SingleSyncRequest) (*SyncResponse, error) {
	// Create job request
	jobRequest := &jobs.SingleIssueSyncRequest{
		IssueKey:   req.IssueKey,
		Repository: req.Repository,
		SafeMode:   req.SafeMode,
	}

	// Apply options
	if req.Options != nil {
		if req.Options.RateLimit > 0 {
			jobRequest.RateLimit = req.Options.RateLimit
		}
		jobRequest.Incremental = req.Options.Incremental
		jobRequest.Force = req.Options.Force
		jobRequest.DryRun = req.Options.DryRun
	}

	// Submit job
	result, err := s.jobManager.SubmitSingleIssueSync(ctx, jobRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to submit single issue sync job: %w", err)
	}

	response := &SyncResponse{
		JobID:     result.JobID,
		Status:    string(result.Status),
		CreatedAt: time.Now(),
	}

	if result.StartTime != nil {
		response.StartedAt = result.StartTime
	}

	return response, nil
}

// createAsyncBatchSync creates an async batch sync job
func (s *Server) createAsyncBatchSync(ctx context.Context, req *BatchSyncRequest) (*SyncResponse, error) {
	// Create job request
	jobRequest := &jobs.BatchSyncRequest{
		IssueKeys:  req.IssueKeys,
		Repository: req.Repository,
		SafeMode:   req.SafeMode,
	}

	// Convert parallelism from int to *int32
	if req.Parallelism > 0 {
		parallelism := int32(req.Parallelism)
		jobRequest.Parallelism = &parallelism
	}

	// Apply options
	if req.Options != nil {
		if req.Options.Concurrency > 0 {
			jobRequest.Concurrency = req.Options.Concurrency
		}
		if req.Options.RateLimit > 0 {
			jobRequest.RateLimit = req.Options.RateLimit
		}
		jobRequest.Incremental = req.Options.Incremental
		jobRequest.Force = req.Options.Force
		jobRequest.DryRun = req.Options.DryRun
	}

	// Submit job
	result, err := s.jobManager.SubmitBatchSync(ctx, jobRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to submit batch sync job: %w", err)
	}

	response := &SyncResponse{
		JobID:     result.JobID,
		Status:    string(result.Status),
		CreatedAt: time.Now(),
	}

	if result.StartTime != nil {
		response.StartedAt = result.StartTime
	}

	return response, nil
}

// createAsyncJQLSync creates an async JQL sync job
func (s *Server) createAsyncJQLSync(ctx context.Context, req *JQLSyncRequest) (*SyncResponse, error) {
	// Create job request
	jobRequest := &jobs.JQLSyncRequest{
		JQL:        req.JQL,
		Repository: req.Repository,
		SafeMode:   req.SafeMode,
	}

	// Convert parallelism from int to *int32
	if req.Parallelism > 0 {
		parallelism := int32(req.Parallelism)
		jobRequest.Parallelism = &parallelism
	}

	// Apply options
	if req.Options != nil {
		if req.Options.Concurrency > 0 {
			jobRequest.Concurrency = req.Options.Concurrency
		}
		if req.Options.RateLimit > 0 {
			jobRequest.RateLimit = req.Options.RateLimit
		}
		jobRequest.Incremental = req.Options.Incremental
		jobRequest.Force = req.Options.Force
		jobRequest.DryRun = req.Options.DryRun
	}

	// Submit job
	result, err := s.jobManager.SubmitJQLSync(ctx, jobRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to submit JQL sync job: %w", err)
	}

	response := &SyncResponse{
		JobID:     result.JobID,
		Status:    string(result.Status),
		CreatedAt: time.Now(),
	}

	if result.StartTime != nil {
		response.StartedAt = result.StartTime
	}

	return response, nil
}

// performSyncSingleSync performs a synchronous single issue sync (for small operations)
func (s *Server) performSyncSingleSync(ctx context.Context, req *SingleSyncRequest) (*SyncResponse, error) {
	// For synchronous operations, we can use the local execution capability
	localRequest := &jobs.LocalSyncRequest{
		IssueKeys:  []string{req.IssueKey},
		Repository: req.Repository,
	}

	// Apply options
	if req.Options != nil {
		if req.Options.Concurrency > 0 {
			localRequest.Concurrency = req.Options.Concurrency
		}
		if req.Options.RateLimit > 0 {
			localRequest.RateLimit = req.Options.RateLimit
		}
		localRequest.Incremental = req.Options.Incremental
		localRequest.Force = req.Options.Force
		localRequest.DryRun = req.Options.DryRun
	}

	// Execute local sync
	result, err := s.jobManager.ExecuteLocalSync(ctx, localRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to execute local sync: %w", err)
	}

	// Convert result to sync response
	syncResult := &SyncResult{
		TotalIssues:     result.TotalIssues,
		ProcessedIssues: result.ProcessedIssues,
		SuccessfulSync:  result.SuccessfulSync,
		FailedSync:      result.FailedSync,
		Duration:        result.Duration,
		ProcessedFiles:  result.ProcessedFiles,
	}

	// Convert errors
	for _, errMsg := range result.Errors {
		syncResult.Errors = append(syncResult.Errors, SyncError{
			IssueKey: req.IssueKey,
			Step:     "sync",
			Message:  errMsg,
		})
	}

	response := &SyncResponse{
		JobID:     fmt.Sprintf("local-%d", time.Now().Unix()),
		Status:    "completed",
		CreatedAt: time.Now(),
		Result:    syncResult,
	}

	return response, nil
}
