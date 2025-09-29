package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// ProfileResponse represents a profile response
type ProfileResponse struct {
	Name        string                  `json:"name"`
	Description string                  `json:"description"`
	Repository  string                  `json:"repository"`
	EpicKey     string                  `json:"epic_key,omitempty"`
	JQL         string                  `json:"jql,omitempty"`
	IssueKeys   []string                `json:"issue_keys,omitempty"`
	Options     *ProfileOptionsResponse `json:"options"`
	CreatedAt   string                  `json:"created_at"`
	UpdatedAt   string                  `json:"updated_at"`
	LastUsed    string                  `json:"last_used,omitempty"`
	UsageCount  int                     `json:"usage_count"`
}

// ProfileOptionsResponse represents profile options
type ProfileOptionsResponse struct {
	Concurrency  int    `json:"concurrency"`
	RateLimit    string `json:"rate_limit"`
	Incremental  bool   `json:"incremental"`
	Force        bool   `json:"force"`
	DryRun       bool   `json:"dry_run"`
	IncludeLinks bool   `json:"include_links"`
}

// ProfileListResponse represents a list of profiles
type ProfileListResponse struct {
	Profiles []ProfileResponse `json:"profiles"`
	Count    int               `json:"count"`
}

// CreateProfileRequest represents a profile creation request
type CreateProfileRequest struct {
	Name        string                 `json:"name" validate:"required"`
	Description string                 `json:"description"`
	Repository  string                 `json:"repository" validate:"required"`
	EpicKey     string                 `json:"epic_key,omitempty"`
	JQL         string                 `json:"jql,omitempty"`
	IssueKeys   []string               `json:"issue_keys,omitempty"`
	Options     *ProfileOptionsRequest `json:"options,omitempty"`
}

// UpdateProfileRequest represents a profile update request
type UpdateProfileRequest struct {
	Description string                 `json:"description,omitempty"`
	Repository  string                 `json:"repository,omitempty"`
	EpicKey     string                 `json:"epic_key,omitempty"`
	JQL         string                 `json:"jql,omitempty"`
	IssueKeys   []string               `json:"issue_keys,omitempty"`
	Options     *ProfileOptionsRequest `json:"options,omitempty"`
}

// ProfileOptionsRequest represents profile options in requests
type ProfileOptionsRequest struct {
	Concurrency  int    `json:"concurrency,omitempty"`
	RateLimit    string `json:"rate_limit,omitempty"`
	Incremental  bool   `json:"incremental,omitempty"`
	Force        bool   `json:"force,omitempty"`
	DryRun       bool   `json:"dry_run,omitempty"`
	IncludeLinks bool   `json:"include_links,omitempty"`
}

// handleListProfiles handles profile listing requests
func (s *Server) handleListProfiles(w http.ResponseWriter, r *http.Request) {
	// For now, return a simple response indicating profiles are not fully implemented
	// In the future, this would integrate with the profile.Manager

	response := ProfileListResponse{
		Profiles: []ProfileResponse{},
		Count:    0,
	}

	// TODO: Integrate with profile.Manager when profiles are fully implemented in API
	// manager := profile.NewFileProfileManager(".", "yaml")
	// profiles, err := manager.ListProfiles()
	// if err != nil {
	//     s.writeError(w, http.StatusInternalServerError, "PROFILE_LIST_ERROR", "Failed to list profiles", err.Error())
	//     return
	// }

	s.writeJSON(w, http.StatusOK, response)
}

// handleGetProfile handles individual profile retrieval requests
func (s *Server) handleGetProfile(w http.ResponseWriter, r *http.Request) {
	// Extract profile name from path
	profileName := s.extractProfileNameFromPath(r.URL.Path)
	if profileName == "" {
		s.writeError(w, http.StatusBadRequest, "MISSING_PROFILE_NAME", "Profile name is required", "")
		return
	}

	// TODO: Integrate with profile.Manager when profiles are fully implemented in API
	s.writeError(w, http.StatusNotImplemented, "NOT_IMPLEMENTED", "Profile management not yet implemented in API", "Profiles will be available in future API versions")
}

// handleCreateProfile handles profile creation requests
func (s *Server) handleCreateProfile(w http.ResponseWriter, r *http.Request) {
	var req CreateProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid JSON request body", err.Error())
		return
	}

	// Validate request
	if err := s.validateCreateProfileRequest(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Request validation failed", err.Error())
		return
	}

	// TODO: Integrate with profile.Manager when profiles are fully implemented in API
	s.writeError(w, http.StatusNotImplemented, "NOT_IMPLEMENTED", "Profile management not yet implemented in API", "Profiles will be available in future API versions")
}

// handleUpdateProfile handles profile update requests
func (s *Server) handleUpdateProfile(w http.ResponseWriter, r *http.Request) {
	// Extract profile name from path
	profileName := s.extractProfileNameFromPath(r.URL.Path)
	if profileName == "" {
		s.writeError(w, http.StatusBadRequest, "MISSING_PROFILE_NAME", "Profile name is required", "")
		return
	}

	var req UpdateProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid JSON request body", err.Error())
		return
	}

	// TODO: Integrate with profile.Manager when profiles are fully implemented in API
	s.writeError(w, http.StatusNotImplemented, "NOT_IMPLEMENTED", "Profile management not yet implemented in API", "Profiles will be available in future API versions")
}

// handleDeleteProfile handles profile deletion requests
func (s *Server) handleDeleteProfile(w http.ResponseWriter, r *http.Request) {
	// Extract profile name from path
	profileName := s.extractProfileNameFromPath(r.URL.Path)
	if profileName == "" {
		s.writeError(w, http.StatusBadRequest, "MISSING_PROFILE_NAME", "Profile name is required", "")
		return
	}

	// TODO: Integrate with profile.Manager when profiles are fully implemented in API
	s.writeError(w, http.StatusNotImplemented, "NOT_IMPLEMENTED", "Profile management not yet implemented in API", "Profiles will be available in future API versions")
}

// extractProfileNameFromPath extracts profile name from URL path
func (s *Server) extractProfileNameFromPath(path string) string {
	// Simple path parsing - in production, use a proper router
	// Expected format: /api/v1/profiles/{name}
	parts := strings.Split(strings.Trim(path, "/"), "/")

	// Find "profiles" in the path and get the next part
	for i, part := range parts {
		if part == "profiles" && i+1 < len(parts) {
			return parts[i+1]
		}
	}

	return ""
}

// validateCreateProfileRequest validates a profile creation request
func (s *Server) validateCreateProfileRequest(req *CreateProfileRequest) error {
	if req.Name == "" {
		return fmt.Errorf("name is required")
	}
	if req.Repository == "" {
		return fmt.Errorf("repository is required")
	}

	// Validate that at least one sync method is specified
	syncMethods := 0
	if req.EpicKey != "" {
		syncMethods++
	}
	if req.JQL != "" {
		syncMethods++
	}
	if len(req.IssueKeys) > 0 {
		syncMethods++
	}

	if syncMethods == 0 {
		return fmt.Errorf("at least one sync method must be specified (epic_key, jql, or issue_keys)")
	}
	if syncMethods > 1 {
		return fmt.Errorf("only one sync method can be specified (epic_key, jql, or issue_keys)")
	}

	// Validate options if provided
	if req.Options != nil {
		if req.Options.Concurrency < 0 || req.Options.Concurrency > 10 {
			return fmt.Errorf("concurrency must be between 0 and 10")
		}

		if req.Options.Incremental && req.Options.Force {
			return fmt.Errorf("incremental and force options are mutually exclusive")
		}
	}

	return nil
}
