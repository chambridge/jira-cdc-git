package client

import (
	"fmt"
)

// MockClient implements the Client interface for testing
// This enables comprehensive unit testing without external dependencies
type MockClient struct {
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
	m.GetIssueCallCount++
	m.LastRequestedIssue = issueKey

	// Simulate API error if configured
	if m.APIError != nil {
		return nil, m.APIError
	}

	// Simulate authentication error if configured
	if m.AuthenticationError != nil {
		return nil, m.AuthenticationError
	}

	// Return mock issue if found
	if issue, exists := m.Issues[issueKey]; exists {
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
	m.SearchIssuesCallCount++
	m.LastJQLQuery = jql

	// Simulate JQL-specific error if configured
	if m.JQLError != nil {
		return nil, m.JQLError
	}

	// Simulate API error if configured
	if m.APIError != nil {
		return nil, m.APIError
	}

	// Simulate authentication error if configured
	if m.AuthenticationError != nil {
		return nil, m.AuthenticationError
	}

	// Return empty result for empty JQL
	if jql == "" {
		return nil, &ClientError{
			Type:    "invalid_input",
			Message: "JQL query cannot be empty",
		}
	}

	// Return configured JQL results if available
	if issueKeys, exists := m.JQLResults[jql]; exists {
		var results []*Issue
		for _, key := range issueKeys {
			if issue, found := m.Issues[key]; found {
				results = append(results, issue)
			}
		}
		return results, nil
	}

	// Default: return empty results
	return []*Issue{}, nil
}

// Authenticate simulates authentication check
func (m *MockClient) Authenticate() error {
	if m.AuthenticationError != nil {
		return m.AuthenticationError
	}
	return nil
}

// AddIssue adds a mock issue for testing
func (m *MockClient) AddIssue(issue *Issue) {
	m.Issues[issue.Key] = issue
}

// SetAuthenticationError configures the mock to return an authentication error
func (m *MockClient) SetAuthenticationError(err error) {
	m.AuthenticationError = err
}

// SetAPIError configures the mock to return an API error
func (m *MockClient) SetAPIError(err error) {
	m.APIError = err
}

// SetJQLError configures the mock to return a JQL-specific error
func (m *MockClient) SetJQLError(err error) {
	m.JQLError = err
}

// AddJQLResult configures the mock to return specific issues for a JQL query
func (m *MockClient) AddJQLResult(jql string, issueKeys []string) {
	m.JQLResults[jql] = issueKeys
}

// Reset clears all mock state for clean test setup
func (m *MockClient) Reset() {
	m.Issues = make(map[string]*Issue)
	m.JQLResults = make(map[string][]string)
	m.AuthenticationError = nil
	m.APIError = nil
	m.JQLError = nil
	m.GetIssueCallCount = 0
	m.SearchIssuesCallCount = 0
	m.LastRequestedIssue = ""
	m.LastJQLQuery = ""
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
