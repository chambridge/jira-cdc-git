package client

import (
	"fmt"
	"net/http"
	"time"

	"github.com/andygrunwald/go-jira"
	"github.com/chambrid/jira-cdc-git/pkg/config"
	"github.com/chambrid/jira-cdc-git/pkg/ratelimit"
)

// Client defines the interface for JIRA operations
// This enables dependency injection and testing with mock implementations
type Client interface {
	GetIssue(issueKey string) (*Issue, error)
	SearchIssues(jql string) ([]*Issue, error)
	Authenticate() error
}

// JIRAClient implements the Client interface using the go-jira library
type JIRAClient struct {
	client      *jira.Client
	config      *config.Config
	rateLimiter ratelimit.RateLimiter
}

// Issue represents a JIRA issue with essential fields and relationships
// Based on SPIKE-001 findings and SPIKE-003 relationship discovery
type Issue struct {
	Key           string         `json:"key" yaml:"key"`
	Summary       string         `json:"summary" yaml:"summary"`
	Description   string         `json:"description" yaml:"description"`
	Status        Status         `json:"status" yaml:"status"`
	Assignee      User           `json:"assignee" yaml:"assignee"`
	Reporter      User           `json:"reporter" yaml:"reporter"`
	Created       string         `json:"created" yaml:"created"`
	Updated       string         `json:"updated" yaml:"updated"`
	Priority      string         `json:"priority" yaml:"priority"`
	IssueType     string         `json:"issuetype" yaml:"issuetype"`
	Relationships *Relationships `json:"relationships,omitempty" yaml:"relationships,omitempty"`
}

// Status represents JIRA issue status information
type Status struct {
	Name     string `json:"name" yaml:"name"`
	Category string `json:"category,omitempty" yaml:"category,omitempty"`
}

// User represents JIRA user information
type User struct {
	Name  string `json:"name" yaml:"name"`
	Email string `json:"email,omitempty" yaml:"email,omitempty"`
}

// Relationships represents JIRA issue relationships
// Based on SPIKE-003 findings: epic links, issue links, and subtasks
type Relationships struct {
	EpicLink    string      `json:"epic_link,omitempty" yaml:"epic_link,omitempty"`
	ParentIssue string      `json:"parent_issue,omitempty" yaml:"parent_issue,omitempty"`
	Subtasks    []string    `json:"subtasks,omitempty" yaml:"subtasks,omitempty"`
	IssueLinks  []IssueLink `json:"issue_links,omitempty" yaml:"issue_links,omitempty"`
}

// IssueLink represents a JIRA issue link
// Based on SPIKE-003: clones, documents, blocks patterns with inward/outward directions
type IssueLink struct {
	Type      string `json:"type" yaml:"type"`
	Direction string `json:"direction" yaml:"direction"` // "inward" or "outward"
	IssueKey  string `json:"issue_key" yaml:"issue_key"`
	Summary   string `json:"summary,omitempty" yaml:"summary,omitempty"`
}

// RelationshipType represents the type of relationship between issues
type RelationshipType string

const (
	RelationshipTypeBlocks    RelationshipType = "blocks"
	RelationshipTypeClones    RelationshipType = "clones"
	RelationshipTypeDocuments RelationshipType = "documents"
	RelationshipTypeEpicStory RelationshipType = "epic/story"
	RelationshipTypeSubtask   RelationshipType = "subtask"
)

// BearerTokenTransport implements Bearer token authentication for HTTP requests
// Based on SPIKE-001 successful authentication approach
// NOTE: This is kept for backward compatibility but NewClient now uses rate-limited transport
type BearerTokenTransport struct {
	Token string
}

func (t *BearerTokenTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", "Bearer "+t.Token)
	return http.DefaultTransport.RoundTrip(req)
}

// NewClient creates a new JIRA client with the provided configuration
func NewClient(cfg *config.Config) (Client, error) {
	// Create rate limiter with configuration
	rateLimiter := ratelimit.NewRateLimiter(cfg)

	// Create rate-limited HTTP transport with Bearer token authentication
	transport := ratelimit.NewBearerTokenRateLimitedTransport(cfg.JIRAPAT, rateLimiter)

	httpClient := &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second, // 30-second timeout to prevent hanging requests
	}

	// Create JIRA client with rate-limited HTTP client
	jiraClient, err := jira.NewClient(httpClient, cfg.JIRABaseURL)
	if err != nil {
		return nil, &ClientError{
			Type:    "connection_error",
			Message: "failed to create JIRA client",
			Err:     err,
		}
	}

	return &JIRAClient{
		client:      jiraClient,
		config:      cfg,
		rateLimiter: rateLimiter,
	}, nil
}

// GetIssue retrieves a single JIRA issue by key
func (c *JIRAClient) GetIssue(issueKey string) (*Issue, error) {
	if issueKey == "" {
		return nil, &ClientError{
			Type:    "invalid_input",
			Message: "issue key cannot be empty",
		}
	}

	// Get the issue from JIRA API
	jiraIssue, response, err := c.client.Issue.Get(issueKey, nil)
	if err != nil {
		return nil, c.handleAPIError(err, response, issueKey)
	}

	// Convert JIRA issue to our internal Issue structure
	issue := c.convertJIRAIssue(jiraIssue)
	return issue, nil
}

// SearchIssues searches for JIRA issues using JQL query with pagination support
// Based on SPIKE-002 findings: supports StartAt/MaxResults parameters, handles 33k+ issues efficiently
func (c *JIRAClient) SearchIssues(jql string) ([]*Issue, error) {
	if jql == "" {
		return nil, &ClientError{
			Type:    "invalid_input",
			Message: "JQL query cannot be empty",
		}
	}

	var allIssues []*Issue
	startAt := 0
	maxResults := 100 // Default batch size for pagination

	for {
		// Create search options with pagination
		searchOptions := &jira.SearchOptions{
			StartAt:    startAt,
			MaxResults: maxResults,
		}

		// Execute JQL search
		issues, response, err := c.client.Issue.Search(jql, searchOptions)
		if err != nil {
			return nil, c.handleJQLError(err, response, jql)
		}

		// Convert JIRA issues to our internal Issue structure
		for _, jiraIssue := range issues {
			issue := c.convertJIRAIssue(&jiraIssue)
			allIssues = append(allIssues, issue)
		}

		// Check if we have retrieved all results
		if startAt+len(issues) >= response.Total {
			break
		}

		// Prepare for next page
		startAt += maxResults
	}

	return allIssues, nil
}

// Authenticate verifies the connection and credentials
func (c *JIRAClient) Authenticate() error {
	// Try to get current user info to validate authentication
	_, response, err := c.client.User.GetSelf()
	if err != nil {
		return c.handleAPIError(err, response, "authentication")
	}
	return nil
}

// convertJIRAIssue converts go-jira Issue to our internal Issue structure
// Based on SPIKE-001 field mapping analysis
func (c *JIRAClient) convertJIRAIssue(jiraIssue *jira.Issue) *Issue {
	issue := &Issue{
		Key:         jiraIssue.Key,
		Summary:     jiraIssue.Fields.Summary,
		Description: jiraIssue.Fields.Description,
		Created:     formatJIRATime(jiraIssue.Fields.Created),
		Updated:     formatJIRATime(jiraIssue.Fields.Updated),
	}

	// Extract status information
	if jiraIssue.Fields.Status != nil {
		issue.Status = Status{
			Name:     jiraIssue.Fields.Status.Name,
			Category: getStatusCategory(jiraIssue.Fields.Status),
		}
	}

	// Extract assignee information
	if jiraIssue.Fields.Assignee != nil {
		issue.Assignee = User{
			Name:  jiraIssue.Fields.Assignee.DisplayName,
			Email: jiraIssue.Fields.Assignee.EmailAddress,
		}
	}

	// Extract reporter information
	if jiraIssue.Fields.Reporter != nil {
		issue.Reporter = User{
			Name:  jiraIssue.Fields.Reporter.DisplayName,
			Email: jiraIssue.Fields.Reporter.EmailAddress,
		}
	}

	// Extract priority
	if jiraIssue.Fields.Priority != nil {
		issue.Priority = jiraIssue.Fields.Priority.Name
	}

	// Extract issue type
	issue.IssueType = jiraIssue.Fields.Type.Name

	// Extract relationships based on SPIKE-003 findings
	issue.Relationships = c.extractRelationships(jiraIssue)

	return issue
}

// Helper function to safely extract status category
func getStatusCategory(status *jira.Status) string {
	if status != nil && status.StatusCategory.Name != "" {
		return status.StatusCategory.Name
	}
	return ""
}

// Helper function to format JIRA time to ISO string
func formatJIRATime(jiraTime jira.Time) string {
	// Convert jira.Time to time.Time for formatting
	timeValue := time.Time(jiraTime)
	if timeValue.IsZero() {
		return ""
	}
	return timeValue.Format("2006-01-02T15:04:05.000Z")
}

// extractRelationships extracts relationship information from JIRA issue
// Based on SPIKE-003 findings: epic links in customfield_12311140, issue links, and subtasks
func (c *JIRAClient) extractRelationships(jiraIssue *jira.Issue) *Relationships {
	relationships := &Relationships{}
	hasRelationships := false

	// Extract epic link from customfield_12311140 (Red Hat JIRA specific)
	if epicLink := c.extractEpicLink(jiraIssue); epicLink != "" {
		relationships.EpicLink = epicLink
		hasRelationships = true
	}

	// Extract parent issue from subtask relationship
	if jiraIssue.Fields.Parent != nil {
		relationships.ParentIssue = jiraIssue.Fields.Parent.Key
		hasRelationships = true
	}

	// Extract subtasks
	if subtasks := c.extractSubtasks(jiraIssue); len(subtasks) > 0 {
		relationships.Subtasks = subtasks
		hasRelationships = true
	}

	// Extract issue links (blocks, clones, documents, etc.)
	if issueLinks := c.extractIssueLinks(jiraIssue); len(issueLinks) > 0 {
		relationships.IssueLinks = issueLinks
		hasRelationships = true
	}

	// Only return relationships if we found any
	if hasRelationships {
		return relationships
	}
	return nil
}

// extractEpicLink extracts epic link from Red Hat JIRA customfield_12311140
func (c *JIRAClient) extractEpicLink(jiraIssue *jira.Issue) string {
	if jiraIssue.Fields.Unknowns != nil {
		if epicLinkValue, exists := jiraIssue.Fields.Unknowns["customfield_12311140"]; exists {
			if epicLinkStr, ok := epicLinkValue.(string); ok && epicLinkStr != "" {
				return epicLinkStr
			}
		}
	}
	return ""
}

// extractSubtasks extracts subtask issue keys
func (c *JIRAClient) extractSubtasks(jiraIssue *jira.Issue) []string {
	var subtasks []string
	if jiraIssue.Fields.Subtasks != nil {
		for _, subtask := range jiraIssue.Fields.Subtasks {
			if subtask != nil && subtask.Key != "" {
				subtasks = append(subtasks, subtask.Key)
			}
		}
	}
	return subtasks
}

// extractIssueLinks extracts issue links with type and direction information
func (c *JIRAClient) extractIssueLinks(jiraIssue *jira.Issue) []IssueLink {
	var issueLinks []IssueLink

	if jiraIssue.Fields.IssueLinks != nil {
		for _, link := range jiraIssue.Fields.IssueLinks {
			if link != nil && link.Type.Name != "" {
				// Handle outward links
				if link.OutwardIssue != nil && link.OutwardIssue.Key != "" {
					issueLink := IssueLink{
						Type:      link.Type.Name,
						Direction: "outward",
						IssueKey:  link.OutwardIssue.Key,
						Summary:   link.OutwardIssue.Fields.Summary,
					}
					issueLinks = append(issueLinks, issueLink)
				}

				// Handle inward links
				if link.InwardIssue != nil && link.InwardIssue.Key != "" {
					issueLink := IssueLink{
						Type:      link.Type.Name,
						Direction: "inward",
						IssueKey:  link.InwardIssue.Key,
						Summary:   link.InwardIssue.Fields.Summary,
					}
					issueLinks = append(issueLinks, issueLink)
				}
			}
		}
	}

	return issueLinks
}

// handleJQLError creates appropriate error for JQL search operations
func (c *JIRAClient) handleJQLError(err error, response *jira.Response, jql string) error {
	if response != nil {
		switch response.StatusCode {
		case 400:
			return &ClientError{
				Type:    "jql_syntax_error",
				Message: "invalid JQL syntax",
				Err:     err,
				Context: jql,
			}
		case 401:
			return &ClientError{
				Type:    "authentication_error",
				Message: "authentication failed - check JIRA credentials",
				Err:     err,
				Context: jql,
			}
		case 403:
			return &ClientError{
				Type:    "authorization_error",
				Message: "access denied - insufficient permissions for JQL search",
				Err:     err,
				Context: jql,
			}
		}
	}

	return &ClientError{
		Type:    "jql_search_error",
		Message: "JQL search request failed",
		Err:     err,
		Context: jql,
	}
}

// handleAPIError creates appropriate error based on HTTP response
func (c *JIRAClient) handleAPIError(err error, response *jira.Response, context string) error {
	if response != nil {
		switch response.StatusCode {
		case 401:
			return &ClientError{
				Type:    "authentication_error",
				Message: "authentication failed - check JIRA credentials",
				Err:     err,
				Context: context,
			}
		case 403:
			return &ClientError{
				Type:    "authorization_error",
				Message: "access denied - insufficient permissions",
				Err:     err,
				Context: context,
			}
		case 404:
			return &ClientError{
				Type:    "not_found",
				Message: "issue not found",
				Err:     err,
				Context: context,
			}
		}
	}

	// Include more diagnostic information for debugging
	message := "JIRA API request failed"
	if response != nil {
		switch response.StatusCode {
		case 429:
			message = fmt.Sprintf("rate limit exceeded (HTTP %d) - consider increasing --rate-limit", response.StatusCode)
		case 500, 502, 503, 504:
			message = fmt.Sprintf("server error (HTTP %d) - JIRA server may be overloaded", response.StatusCode)
		default:
			message = fmt.Sprintf("HTTP %d error - %s", response.StatusCode, http.StatusText(response.StatusCode))
		}
	} else if err != nil {
		message = fmt.Sprintf("network/connection error: %v", err)
	}

	return &ClientError{
		Type:    "api_error",
		Message: message,
		Err:     err,
		Context: context,
	}
}
