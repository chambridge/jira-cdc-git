package client

import (
	"fmt"
	"sync"
)

// MockClient implements the Client interface for testing
// This enables comprehensive unit testing without external dependencies
type MockClient struct {
	// mu protects all fields for thread-safe concurrent access
	mu sync.RWMutex

	// Issues maps issue keys to Issue objects for testing
	Issues map[string]*Issue

	// JQLResults maps JQL queries to lists of issue keys for testing
	JQLResults map[string][]string

	// AuthenticationError simulates authentication failures when set
	AuthenticationError error

	// APIError simulates API failures when set
	APIError error

	// JQLError simulates JQL-specific errors when set
	JQLError error

	// GetIssueCallCount tracks how many times GetIssue was called
	GetIssueCallCount int

	// SearchIssuesCallCount tracks how many times SearchIssues was called
	SearchIssuesCallCount int

	// SearchIssuesWithPaginationCallCount tracks how many times SearchIssuesWithPagination was called
	SearchIssuesWithPaginationCallCount int

	// LastRequestedIssue tracks the last issue key requested
	LastRequestedIssue string

	// LastJQLQuery tracks the last JQL query executed
	LastJQLQuery string
}

// NewMockClient creates a new mock JIRA client for testing
func NewMockClient() *MockClient {
	return &MockClient{
		Issues:     make(map[string]*Issue),
		JQLResults: make(map[string][]string),
	}
}

// GetIssue retrieves a mock JIRA issue by key
func (m *MockClient) GetIssue(issueKey string) (*Issue, error) {
	m.mu.Lock()
	m.GetIssueCallCount++
	m.LastRequestedIssue = issueKey

	// Check for configured errors while holding lock
	apiError := m.APIError
	authError := m.AuthenticationError

	// Get issue copy while holding lock
	var issue *Issue
	if existingIssue, exists := m.Issues[issueKey]; exists {
		issue = existingIssue
	}
	m.mu.Unlock()

	// Simulate API error if configured
	if apiError != nil {
		return nil, apiError
	}

	// Simulate authentication error if configured
	if authError != nil {
		return nil, authError
	}

	// Return mock issue if found
	if issue != nil {
		return issue, nil
	}

	// Return not found error
	return nil, &ClientError{
		Type:    "not_found",
		Message: fmt.Sprintf("issue %s not found", issueKey),
		Context: issueKey,
	}
}

// SearchIssues simulates JQL search functionality for testing
func (m *MockClient) SearchIssues(jql string) ([]*Issue, error) {
	m.mu.Lock()
	m.SearchIssuesCallCount++
	m.LastJQLQuery = jql

	// Check for configured errors while holding lock
	jqlError := m.JQLError
	apiError := m.APIError
	authError := m.AuthenticationError

	// Get JQL results and issues while holding lock
	var issueKeys []string
	var exists bool
	if issueKeys, exists = m.JQLResults[jql]; exists {
		// Create a copy of issue keys to avoid holding lock during issue lookup
		issueKeysCopy := make([]string, len(issueKeys))
		copy(issueKeysCopy, issueKeys)
		issueKeys = issueKeysCopy
	}

	// Get copies of issues while holding lock
	var results []*Issue
	if exists {
		for _, key := range issueKeys {
			if issue, found := m.Issues[key]; found {
				results = append(results, issue)
			}
		}
	}
	m.mu.Unlock()

	// Simulate JQL-specific error if configured
	if jqlError != nil {
		return nil, jqlError
	}

	// Simulate API error if configured
	if apiError != nil {
		return nil, apiError
	}

	// Simulate authentication error if configured
	if authError != nil {
		return nil, authError
	}

	// Return empty result for empty JQL
	if jql == "" {
		return nil, &ClientError{
			Type:    "invalid_input",
			Message: "JQL query cannot be empty",
		}
	}

	// Return configured JQL results if available
	if exists {
		return results, nil
	}

	// Default: return empty results
	return []*Issue{}, nil
}

// SearchIssuesWithPagination simulates JQL search with pagination for testing
func (m *MockClient) SearchIssuesWithPagination(jql string, startAt, maxResults int) ([]*Issue, int, error) {
	m.mu.Lock()
	m.SearchIssuesWithPaginationCallCount++
	m.LastJQLQuery = jql

	// Check for configured errors while holding lock
	jqlError := m.JQLError
	apiError := m.APIError
	authError := m.AuthenticationError

	// Get all matching issues while holding lock
	var allIssues []*Issue
	if issueKeys, exists := m.JQLResults[jql]; exists {
		for _, key := range issueKeys {
			if issue, found := m.Issues[key]; found {
				allIssues = append(allIssues, issue)
			}
		}
	}
	m.mu.Unlock()

	// Simulate JQL-specific error if configured
	if jqlError != nil {
		return nil, 0, jqlError
	}

	// Simulate API error if configured
	if apiError != nil {
		return nil, 0, apiError
	}

	// Simulate authentication error if configured
	if authError != nil {
		return nil, 0, authError
	}

	// Return empty result for empty JQL
	if jql == "" {
		return nil, 0, &ClientError{
			Type:    "invalid_input",
			Message: "JQL query cannot be empty",
		}
	}

	totalCount := len(allIssues)

	// Apply pagination
	if startAt >= totalCount {
		return []*Issue{}, totalCount, nil
	}

	end := startAt + maxResults
	if end > totalCount {
		end = totalCount
	}

	paginatedIssues := allIssues[startAt:end]
	return paginatedIssues, totalCount, nil
}

// Authenticate simulates authentication check
func (m *MockClient) Authenticate() error {
	m.mu.RLock()
	authError := m.AuthenticationError
	m.mu.RUnlock()

	if authError != nil {
		return authError
	}
	return nil
}

// AddIssue adds a mock issue for testing
func (m *MockClient) AddIssue(issue *Issue) {
	m.mu.Lock()
	m.Issues[issue.Key] = issue
	m.mu.Unlock()
}

// SetAuthenticationError configures the mock to return an authentication error
func (m *MockClient) SetAuthenticationError(err error) {
	m.mu.Lock()
	m.AuthenticationError = err
	m.mu.Unlock()
}

// SetAPIError configures the mock to return an API error
func (m *MockClient) SetAPIError(err error) {
	m.mu.Lock()
	m.APIError = err
	m.mu.Unlock()
}

// SetJQLError configures the mock to return a JQL-specific error
func (m *MockClient) SetJQLError(err error) {
	m.mu.Lock()
	m.JQLError = err
	m.mu.Unlock()
}

// AddJQLResult configures the mock to return specific issues for a JQL query
func (m *MockClient) AddJQLResult(jql string, issueKeys []string) {
	m.mu.Lock()
	m.JQLResults[jql] = issueKeys
	m.mu.Unlock()
}

// Reset clears all mock state for clean test setup
func (m *MockClient) Reset() {
	m.mu.Lock()
	m.Issues = make(map[string]*Issue)
	m.JQLResults = make(map[string][]string)
	m.AuthenticationError = nil
	m.APIError = nil
	m.JQLError = nil
	m.GetIssueCallCount = 0
	m.SearchIssuesCallCount = 0
	m.SearchIssuesWithPaginationCallCount = 0
	m.LastRequestedIssue = ""
	m.LastJQLQuery = ""
	m.mu.Unlock()
}

// CreateTestIssue creates a sample issue for testing
func CreateTestIssue(key string) *Issue {
	return &Issue{
		Key:         key,
		Summary:     "Test Issue Summary",
		Description: "Test issue description for testing purposes",
		Status: Status{
			Name:     "In Progress",
			Category: "indeterminate",
		},
		Assignee: User{
			Name:  "John Doe",
			Email: "john.doe@example.com",
		},
		Reporter: User{
			Name:  "Jane Smith",
			Email: "jane.smith@example.com",
		},
		Created:   "2024-01-01T10:00:00Z",
		Updated:   "2024-01-02T15:30:00Z",
		Priority:  "High",
		IssueType: "Story",
	}
}

// CreateTestIssueWithRelationships creates a sample issue with relationships for testing
func CreateTestIssueWithRelationships(key string) *Issue {
	issue := CreateTestIssue(key)
	issue.Relationships = &Relationships{
		EpicLink:    "EPIC-123",
		ParentIssue: "",
		Subtasks:    []string{"SUB-1", "SUB-2"},
		IssueLinks: []IssueLink{
			{
				Type:      "Blocks",
				Direction: "outward",
				IssueKey:  "BLOCKED-456",
				Summary:   "Issue that is blocked by this one",
			},
			{
				Type:      "Clones",
				Direction: "inward",
				IssueKey:  "ORIGINAL-789",
				Summary:   "Original issue that this clones",
			},
		},
	}
	return issue
}

// CreateEpicIssue creates a sample epic issue for testing
func CreateEpicIssue(key string) *Issue {
	issue := CreateTestIssue(key)
	issue.IssueType = "Epic"
	issue.Summary = "Epic: " + issue.Summary
	issue.Relationships = &Relationships{
		Subtasks: []string{"STORY-1", "STORY-2", "STORY-3"},
	}
	return issue
}

// CreateSubtaskIssue creates a sample subtask issue for testing
func CreateSubtaskIssue(key, parentKey string) *Issue {
	issue := CreateTestIssue(key)
	issue.IssueType = "Sub-task"
	issue.Summary = "Subtask: " + issue.Summary
	issue.Relationships = &Relationships{
		ParentIssue: parentKey,
	}
	return issue
}
