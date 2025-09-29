package api

import (
	"net/http"
	"strings"

	"github.com/chambrid/jira-cdc-git/pkg/jobs"
)

// JobResponse represents a job status response
type JobResponse struct {
	JobID           string                   `json:"job_id"`
	Status          string                   `json:"status"`
	Type            string                   `json:"type,omitempty"`
	TotalIssues     int                      `json:"total_issues,omitempty"`
	ProcessedIssues int                      `json:"processed_issues,omitempty"`
	SuccessfulSync  int                      `json:"successful_sync,omitempty"`
	FailedSync      int                      `json:"failed_sync,omitempty"`
	CreatedAt       string                   `json:"created_at,omitempty"`
	StartedAt       string                   `json:"started_at,omitempty"`
	CompletedAt     string                   `json:"completed_at,omitempty"`
	Duration        string                   `json:"duration,omitempty"`
	ProcessedFiles  []string                 `json:"processed_files,omitempty"`
	Errors          []jobs.JobExecutionError `json:"errors,omitempty"`
}

// JobListResponse represents a list of jobs response
type JobListResponse struct {
	Jobs       []JobResponse `json:"jobs"`
	TotalCount int           `json:"total_count"`
	Page       int           `json:"page"`
	PageSize   int           `json:"page_size"`
	HasMore    bool          `json:"has_more"`
}

// QueueStatusResponse represents queue status response
type QueueStatusResponse struct {
	TotalJobs     int `json:"total_jobs"`
	PendingJobs   int `json:"pending_jobs"`
	RunningJobs   int `json:"running_jobs"`
	CompletedJobs int `json:"completed_jobs"`
	FailedJobs    int `json:"failed_jobs"`
}

// handleListJobs handles job listing requests
func (s *Server) handleListJobs(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	query := r.URL.Query()

	page, err := parseIntParam(query.Get("page"), "page", 1)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "INVALID_PARAMETER", "Invalid page parameter", err.Error())
		return
	}

	pageSize, err := parseIntParam(query.Get("page_size"), "page_size", 20)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "INVALID_PARAMETER", "Invalid page_size parameter", err.Error())
		return
	}

	// Validate page parameters
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	// Parse filter parameters
	filters := &jobs.JobFilter{
		Limit: pageSize,
	}

	// Parse status filter
	if statusParam := query.Get("status"); statusParam != "" {
		statuses := strings.Split(statusParam, ",")
		for _, status := range statuses {
			switch strings.TrimSpace(status) {
			case "pending":
				filters.Status = append(filters.Status, jobs.JobStatusPending)
			case "running":
				filters.Status = append(filters.Status, jobs.JobStatusRunning)
			case "succeeded":
				filters.Status = append(filters.Status, jobs.JobStatusSucceeded)
			case "failed":
				filters.Status = append(filters.Status, jobs.JobStatusFailed)
			}
		}
	}

	// Parse type filter
	if typeParam := query.Get("type"); typeParam != "" {
		types := strings.Split(typeParam, ",")
		for _, jobType := range types {
			switch strings.TrimSpace(jobType) {
			case "single":
				filters.Type = append(filters.Type, jobs.JobTypeSingle)
			case "batch":
				filters.Type = append(filters.Type, jobs.JobTypeBatch)
			case "jql":
				filters.Type = append(filters.Type, jobs.JobTypeJQL)
			}
		}
	}

	// Get jobs from job manager
	jobResults, err := s.jobManager.ListJobs(r.Context(), filters)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "JOB_LIST_ERROR", "Failed to list jobs", err.Error())
		return
	}

	// Convert to response format
	jobs := make([]JobResponse, len(jobResults))
	for i, jobResult := range jobResults {
		jobs[i] = s.convertJobResultToResponse(jobResult)
	}

	// Calculate pagination info
	totalCount := len(jobs)
	hasMore := totalCount == pageSize // Simple heuristic

	response := JobListResponse{
		Jobs:       jobs,
		TotalCount: totalCount,
		Page:       page,
		PageSize:   pageSize,
		HasMore:    hasMore,
	}

	s.writeJSON(w, http.StatusOK, response)
}

// handleGetJob handles individual job status requests
func (s *Server) handleGetJob(w http.ResponseWriter, r *http.Request) {
	// Extract job ID from path
	jobID := s.extractJobIDFromPath(r.URL.Path)
	if jobID == "" {
		s.writeError(w, http.StatusBadRequest, "MISSING_JOB_ID", "Job ID is required", "")
		return
	}

	// Get job from job manager
	jobResult, err := s.jobManager.GetJob(r.Context(), jobID)
	if err != nil {
		s.writeError(w, http.StatusNotFound, "JOB_NOT_FOUND", "Job not found", err.Error())
		return
	}

	// Convert to response format
	response := s.convertJobResultToResponse(jobResult)

	s.writeJSON(w, http.StatusOK, response)
}

// handleDeleteJob handles job deletion requests
func (s *Server) handleDeleteJob(w http.ResponseWriter, r *http.Request) {
	// Extract job ID from path
	jobID := s.extractJobIDFromPath(r.URL.Path)
	if jobID == "" {
		s.writeError(w, http.StatusBadRequest, "MISSING_JOB_ID", "Job ID is required", "")
		return
	}

	// Delete job
	err := s.jobManager.DeleteJob(r.Context(), jobID)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "JOB_DELETE_ERROR", "Failed to delete job", err.Error())
		return
	}

	response := map[string]interface{}{
		"message": "Job deleted successfully",
		"job_id":  jobID,
	}

	s.writeJSON(w, http.StatusOK, response)
}

// handleCancelJob handles job cancellation requests
func (s *Server) handleCancelJob(w http.ResponseWriter, r *http.Request) {
	// Extract job ID from path
	jobID := s.extractJobIDFromPath(r.URL.Path)
	if jobID == "" {
		s.writeError(w, http.StatusBadRequest, "MISSING_JOB_ID", "Job ID is required", "")
		return
	}

	// Cancel job
	err := s.jobManager.CancelJob(r.Context(), jobID)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "JOB_CANCEL_ERROR", "Failed to cancel job", err.Error())
		return
	}

	response := map[string]interface{}{
		"message": "Job cancelled successfully",
		"job_id":  jobID,
	}

	s.writeJSON(w, http.StatusOK, response)
}

// handleGetJobLogs handles job log retrieval requests
func (s *Server) handleGetJobLogs(w http.ResponseWriter, r *http.Request) {
	// Extract job ID from path
	jobID := s.extractJobIDFromPath(r.URL.Path)
	if jobID == "" {
		s.writeError(w, http.StatusBadRequest, "MISSING_JOB_ID", "Job ID is required", "")
		return
	}

	// Get job logs
	logs, err := s.jobManager.GetJobLogs(r.Context(), jobID)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "JOB_LOGS_ERROR", "Failed to retrieve job logs", err.Error())
		return
	}

	response := map[string]interface{}{
		"job_id": jobID,
		"logs":   logs,
	}

	s.writeJSON(w, http.StatusOK, response)
}

// handleQueueStatus handles queue status requests
func (s *Server) handleQueueStatus(w http.ResponseWriter, r *http.Request) {
	// Get queue status from job manager
	queueStatus, err := s.jobManager.GetQueueStatus(r.Context())
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "QUEUE_STATUS_ERROR", "Failed to get queue status", err.Error())
		return
	}

	response := QueueStatusResponse{
		TotalJobs:     queueStatus.TotalJobs,
		PendingJobs:   queueStatus.PendingJobs,
		RunningJobs:   queueStatus.RunningJobs,
		CompletedJobs: queueStatus.CompletedJobs,
		FailedJobs:    queueStatus.FailedJobs,
	}

	s.writeJSON(w, http.StatusOK, response)
}

// extractJobIDFromPath extracts job ID from URL path
func (s *Server) extractJobIDFromPath(path string) string {
	// Simple path parsing - in production, use a proper router
	// Expected format: /api/v1/jobs/{id} or /api/v1/jobs/{id}/cancel or /api/v1/jobs/{id}/logs
	parts := strings.Split(strings.Trim(path, "/"), "/")

	// Find "jobs" in the path and get the next part
	for i, part := range parts {
		if part == "jobs" && i+1 < len(parts) {
			return parts[i+1]
		}
	}

	return ""
}

// convertJobResultToResponse converts a JobResult to JobResponse
func (s *Server) convertJobResultToResponse(jobResult *jobs.JobResult) JobResponse {
	response := JobResponse{
		JobID:           jobResult.JobID,
		Status:          string(jobResult.Status),
		TotalIssues:     jobResult.TotalIssues,
		ProcessedIssues: jobResult.ProcessedIssues,
		SuccessfulSync:  jobResult.SuccessfulSync,
		FailedSync:      jobResult.FailedSync,
		ProcessedFiles:  jobResult.ProcessedFiles,
		Errors:          jobResult.Errors,
	}

	// Format timestamps
	if jobResult.StartTime != nil {
		response.StartedAt = jobResult.StartTime.Format("2006-01-02T15:04:05Z")
	}

	if jobResult.CompletionTime != nil {
		response.CompletedAt = jobResult.CompletionTime.Format("2006-01-02T15:04:05Z")
	}

	if jobResult.Duration > 0 {
		response.Duration = jobResult.Duration.String()
	}

	return response
}
