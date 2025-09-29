package api

import (
	"fmt"
	"net/http"
	"runtime"
	"time"

	"github.com/chambrid/jira-cdc-git/pkg/jobs"
)

// HealthResponse represents the health check response
type HealthResponse struct {
	Status      string                     `json:"status"`
	Timestamp   time.Time                  `json:"timestamp"`
	Version     string                     `json:"version"`
	Uptime      string                     `json:"uptime"`
	Environment string                     `json:"environment"`
	Components  map[string]ComponentHealth `json:"components"`
}

// ComponentHealth represents the health of a system component
type ComponentHealth struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

// SystemInfoResponse represents system information response
type SystemInfoResponse struct {
	Version      string              `json:"version"`
	Commit       string              `json:"commit"`
	BuildDate    string              `json:"build_date"`
	GoVersion    string              `json:"go_version"`
	Platform     string              `json:"platform"`
	APIVersion   string              `json:"api_version"`
	Capabilities []string            `json:"capabilities"`
	JobSystem    *jobs.JobSystemInfo `json:"job_system,omitempty"`
	Config       *SystemConfigInfo   `json:"config,omitempty"`
}

// SystemConfigInfo represents sanitized system configuration
type SystemConfigInfo struct {
	Port                 int    `json:"port"`
	Host                 string `json:"host"`
	EnableAuthentication bool   `json:"enable_authentication"`
	EnableRateLimit      bool   `json:"enable_rate_limit"`
	RateLimitPerMinute   int    `json:"rate_limit_per_minute"`
	LogLevel             string `json:"log_level"`
	EnableCORS           bool   `json:"enable_cors"`
}

// APIDocsResponse represents API documentation response
type APIDocsResponse struct {
	Title       string                 `json:"title"`
	Description string                 `json:"description"`
	Version     string                 `json:"version"`
	BaseURL     string                 `json:"base_url"`
	Endpoints   []EndpointDoc          `json:"endpoints"`
	Examples    map[string]interface{} `json:"examples"`
}

// EndpointDoc represents documentation for an API endpoint
type EndpointDoc struct {
	Method      string                 `json:"method"`
	Path        string                 `json:"path"`
	Summary     string                 `json:"summary"`
	Description string                 `json:"description"`
	Parameters  []ParameterDoc         `json:"parameters,omitempty"`
	RequestBody *RequestBodyDoc        `json:"request_body,omitempty"`
	Responses   map[string]ResponseDoc `json:"responses"`
}

// ParameterDoc represents documentation for a parameter
type ParameterDoc struct {
	Name        string `json:"name"`
	In          string `json:"in"` // "path", "query", "header"
	Type        string `json:"type"`
	Required    bool   `json:"required"`
	Description string `json:"description"`
}

// RequestBodyDoc represents documentation for request body
type RequestBodyDoc struct {
	Required    bool        `json:"required"`
	ContentType string      `json:"content_type"`
	Schema      string      `json:"schema"`
	Example     interface{} `json:"example,omitempty"`
}

// ResponseDoc represents documentation for a response
type ResponseDoc struct {
	Description string      `json:"description"`
	Schema      string      `json:"schema"`
	Example     interface{} `json:"example,omitempty"`
}

var startTime = time.Now()

// handleHealth handles health check requests
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	uptime := time.Since(startTime)

	// Check component health
	components := make(map[string]ComponentHealth)

	// Check job manager health
	if s.jobManager != nil {
		// Try to get queue status as a health check
		_, err := s.jobManager.GetQueueStatus(r.Context())
		if err != nil {
			components["job_manager"] = ComponentHealth{
				Status:  "unhealthy",
				Message: fmt.Sprintf("Job manager error: %v", err),
			}
		} else {
			components["job_manager"] = ComponentHealth{
				Status: "healthy",
			}
		}
	} else {
		components["job_manager"] = ComponentHealth{
			Status:  "unavailable",
			Message: "Job manager not initialized",
		}
	}

	// Determine overall status
	overallStatus := "healthy"
	for _, component := range components {
		if component.Status == "unhealthy" {
			overallStatus = "unhealthy"
			break
		} else if component.Status == "unavailable" && overallStatus != "unhealthy" {
			overallStatus = "degraded"
		}
	}

	response := HealthResponse{
		Status:      overallStatus,
		Timestamp:   time.Now(),
		Version:     s.buildInfo.Version,
		Uptime:      uptime.String(),
		Environment: "production", // Could be configurable
		Components:  components,
	}

	// Set appropriate HTTP status based on health
	statusCode := http.StatusOK
	switch overallStatus {
	case "unhealthy":
		statusCode = http.StatusServiceUnavailable
	case "degraded":
		statusCode = http.StatusPartialContent
	}

	s.writeJSON(w, statusCode, response)
}

// handleSystemInfo handles system information requests
func (s *Server) handleSystemInfo(w http.ResponseWriter, r *http.Request) {
	// Get job system info if available
	var jobSystemInfo *jobs.JobSystemInfo
	if s.jobManager != nil {
		// This would require extending the JobManager interface to provide system info
		// For now, create a basic info structure
		jobSystemInfo = &jobs.JobSystemInfo{
			Version:   jobs.Version,
			Component: jobs.Component,
			SupportedJobTypes: []jobs.JobType{
				jobs.JobTypeSingle,
				jobs.JobTypeBatch,
				jobs.JobTypeJQL,
			},
		}
	}

	// Sanitize config for public exposure
	configInfo := &SystemConfigInfo{
		Port:                 s.config.Port,
		Host:                 s.config.Host,
		EnableAuthentication: s.config.EnableAuthentication,
		EnableRateLimit:      s.config.EnableRateLimit,
		RateLimitPerMinute:   s.config.RateLimitPerMinute,
		LogLevel:             s.config.LogLevel,
		EnableCORS:           s.config.EnableCORS,
	}

	response := SystemInfoResponse{
		Version:      s.buildInfo.Version,
		Commit:       s.buildInfo.Commit,
		BuildDate:    s.buildInfo.Date,
		GoVersion:    runtime.Version(),
		Platform:     fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
		APIVersion:   "v1",
		Capabilities: []string{"sync", "jobs", "profiles", "monitoring"},
		JobSystem:    jobSystemInfo,
		Config:       configInfo,
	}

	s.writeJSON(w, http.StatusOK, response)
}

// handleAPIDocs handles API documentation requests
func (s *Server) handleAPIDocs(w http.ResponseWriter, r *http.Request) {
	baseURL := fmt.Sprintf("http://%s:%d", s.config.Host, s.config.Port)
	if s.config.Host == "0.0.0.0" {
		baseURL = fmt.Sprintf("http://localhost:%d", s.config.Port)
	}

	response := APIDocsResponse{
		Title:       "JIRA CDC Git Sync API",
		Description: "REST API for JIRA CDC Git synchronization operations",
		Version:     s.buildInfo.Version,
		BaseURL:     baseURL,
		Endpoints:   s.getAPIEndpointDocs(),
		Examples:    s.getAPIExamples(),
	}

	s.writeJSON(w, http.StatusOK, response)
}

// getAPIEndpointDocs returns documentation for all API endpoints
func (s *Server) getAPIEndpointDocs() []EndpointDoc {
	return []EndpointDoc{
		{
			Method:      "GET",
			Path:        "/api/v1/health",
			Summary:     "Health check",
			Description: "Check the health status of the API server and its components",
			Responses: map[string]ResponseDoc{
				"200": {Description: "Service is healthy", Schema: "HealthResponse"},
				"503": {Description: "Service is unhealthy", Schema: "HealthResponse"},
			},
		},
		{
			Method:      "GET",
			Path:        "/api/v1/system/info",
			Summary:     "System information",
			Description: "Get detailed system information including version, capabilities, and configuration",
			Responses: map[string]ResponseDoc{
				"200": {Description: "System information", Schema: "SystemInfoResponse"},
			},
		},
		{
			Method:      "POST",
			Path:        "/api/v1/sync/single",
			Summary:     "Single issue sync",
			Description: "Synchronize a single JIRA issue to a Git repository",
			RequestBody: &RequestBodyDoc{
				Required:    true,
				ContentType: "application/json",
				Schema:      "SingleSyncRequest",
				Example: map[string]interface{}{
					"issue_key":  "PROJ-123",
					"repository": "/workspace/repo",
					"options": map[string]interface{}{
						"incremental": true,
						"dry_run":     false,
					},
					"async": false,
				},
			},
			Responses: map[string]ResponseDoc{
				"200": {Description: "Sync completed successfully", Schema: "SyncResponse"},
				"202": {Description: "Sync job created (async)", Schema: "SyncResponse"},
				"400": {Description: "Invalid request", Schema: "ErrorResponse"},
			},
		},
		{
			Method:      "POST",
			Path:        "/api/v1/sync/batch",
			Summary:     "Batch issue sync",
			Description: "Synchronize multiple JIRA issues to a Git repository",
			RequestBody: &RequestBodyDoc{
				Required:    true,
				ContentType: "application/json",
				Schema:      "BatchSyncRequest",
				Example: map[string]interface{}{
					"issue_keys": []string{"PROJ-123", "PROJ-124", "PROJ-125"},
					"repository": "/workspace/repo",
					"options": map[string]interface{}{
						"concurrency": 5,
						"incremental": true,
					},
					"parallelism": 2,
					"async":       true,
				},
			},
			Responses: map[string]ResponseDoc{
				"202": {Description: "Batch sync job created", Schema: "SyncResponse"},
				"400": {Description: "Invalid request", Schema: "ErrorResponse"},
			},
		},
		{
			Method:      "POST",
			Path:        "/api/v1/sync/jql",
			Summary:     "JQL query sync",
			Description: "Synchronize JIRA issues matching a JQL query to a Git repository",
			RequestBody: &RequestBodyDoc{
				Required:    true,
				ContentType: "application/json",
				Schema:      "JQLSyncRequest",
				Example: map[string]interface{}{
					"jql":        "project = PROJ AND status = 'To Do'",
					"repository": "/workspace/repo",
					"options": map[string]interface{}{
						"concurrency": 5,
						"force":       false,
					},
					"async": true,
				},
			},
			Responses: map[string]ResponseDoc{
				"202": {Description: "JQL sync job created", Schema: "SyncResponse"},
				"400": {Description: "Invalid request", Schema: "ErrorResponse"},
			},
		},
		{
			Method:      "GET",
			Path:        "/api/v1/jobs",
			Summary:     "List jobs",
			Description: "List all sync jobs with optional filtering",
			Parameters: []ParameterDoc{
				{Name: "status", In: "query", Type: "string", Description: "Filter by job status (pending,running,succeeded,failed)"},
				{Name: "type", In: "query", Type: "string", Description: "Filter by job type (single,batch,jql)"},
				{Name: "page", In: "query", Type: "integer", Description: "Page number (default: 1)"},
				{Name: "page_size", In: "query", Type: "integer", Description: "Page size (default: 20, max: 100)"},
			},
			Responses: map[string]ResponseDoc{
				"200": {Description: "List of jobs", Schema: "JobListResponse"},
			},
		},
		{
			Method:      "GET",
			Path:        "/api/v1/jobs/{id}",
			Summary:     "Get job status",
			Description: "Get detailed status and results for a specific job",
			Parameters: []ParameterDoc{
				{Name: "id", In: "path", Type: "string", Required: true, Description: "Job ID"},
			},
			Responses: map[string]ResponseDoc{
				"200": {Description: "Job details", Schema: "JobResponse"},
				"404": {Description: "Job not found", Schema: "ErrorResponse"},
			},
		},
		{
			Method:      "DELETE",
			Path:        "/api/v1/jobs/{id}",
			Summary:     "Delete job",
			Description: "Delete a job and its associated resources",
			Parameters: []ParameterDoc{
				{Name: "id", In: "path", Type: "string", Required: true, Description: "Job ID"},
			},
			Responses: map[string]ResponseDoc{
				"200": {Description: "Job deleted successfully"},
				"404": {Description: "Job not found", Schema: "ErrorResponse"},
			},
		},
		{
			Method:      "POST",
			Path:        "/api/v1/jobs/{id}/cancel",
			Summary:     "Cancel job",
			Description: "Cancel a running job",
			Parameters: []ParameterDoc{
				{Name: "id", In: "path", Type: "string", Required: true, Description: "Job ID"},
			},
			Responses: map[string]ResponseDoc{
				"200": {Description: "Job cancelled successfully"},
				"404": {Description: "Job not found", Schema: "ErrorResponse"},
			},
		},
		{
			Method:      "GET",
			Path:        "/api/v1/jobs/{id}/logs",
			Summary:     "Get job logs",
			Description: "Retrieve logs for a specific job",
			Parameters: []ParameterDoc{
				{Name: "id", In: "path", Type: "string", Required: true, Description: "Job ID"},
			},
			Responses: map[string]ResponseDoc{
				"200": {Description: "Job logs"},
				"404": {Description: "Job not found", Schema: "ErrorResponse"},
			},
		},
		{
			Method:      "GET",
			Path:        "/api/v1/jobs/queue/status",
			Summary:     "Queue status",
			Description: "Get current status of the job queue",
			Responses: map[string]ResponseDoc{
				"200": {Description: "Queue status", Schema: "QueueStatusResponse"},
			},
		},
	}
}

// getAPIExamples returns example requests and responses
func (s *Server) getAPIExamples() map[string]interface{} {
	return map[string]interface{}{
		"single_sync_request": map[string]interface{}{
			"issue_key":  "PROJ-123",
			"repository": "/workspace/repo",
			"options": map[string]interface{}{
				"incremental": true,
				"dry_run":     false,
			},
			"async": false,
		},
		"batch_sync_request": map[string]interface{}{
			"issue_keys": []string{"PROJ-123", "PROJ-124", "PROJ-125"},
			"repository": "/workspace/repo",
			"options": map[string]interface{}{
				"concurrency": 5,
				"incremental": true,
			},
			"parallelism": 2,
			"async":       true,
		},
		"jql_sync_request": map[string]interface{}{
			"jql":        "project = PROJ AND status = 'To Do'",
			"repository": "/workspace/repo",
			"options": map[string]interface{}{
				"concurrency": 5,
				"force":       false,
			},
			"async": true,
		},
		"sync_response": map[string]interface{}{
			"job_id":     "job-123456",
			"status":     "running",
			"created_at": "2024-01-15T10:30:00Z",
			"started_at": "2024-01-15T10:30:05Z",
		},
		"job_response": map[string]interface{}{
			"job_id":           "job-123456",
			"status":           "completed",
			"type":             "single",
			"total_issues":     1,
			"processed_issues": 1,
			"successful_sync":  1,
			"failed_sync":      0,
			"created_at":       "2024-01-15T10:30:00Z",
			"started_at":       "2024-01-15T10:30:05Z",
			"completed_at":     "2024-01-15T10:30:15Z",
			"duration":         "10s",
			"processed_files":  []string{"/workspace/repo/projects/PROJ/issues/PROJ-123.yaml"},
		},
	}
}
