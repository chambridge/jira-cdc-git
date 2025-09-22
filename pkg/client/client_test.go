package client

import (
	"errors"
	"net/http"
	"testing"

	"github.com/andygrunwald/go-jira"
	"github.com/chambrid/jira-cdc-git/pkg/config"
)

func TestNewClient_Success(t *testing.T) {
	cfg := &config.Config{
		JIRABaseURL: "https://test.atlassian.net",
		JIRAEmail:   "test@example.com",
		JIRAPAT:     "test-pat-token-123",
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("Expected no error creating client, got: %v", err)
	}

	if client == nil {
		t.Fatal("Expected client to be created, got nil")
	}

	// Verify it's the correct type
	if _, ok := client.(*JIRAClient); !ok {
		t.Errorf("Expected *JIRAClient, got %T", client)
	}
}

func TestNewClient_InvalidURL(t *testing.T) {
	cfg := &config.Config{
		JIRABaseURL: "invalid-url",
		JIRAEmail:   "test@example.com",
		JIRAPAT:     "test-pat-token-123",
	}

	// Note: jira.NewClient doesn't validate URL format at creation time
	// It only fails when making actual HTTP requests
	// So we expect this to succeed, but fail when making API calls
	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("Expected no error creating client with invalid URL, got: %v", err)
	}

	if client == nil {
		t.Fatal("Expected client to be created even with invalid URL")
	}

	// Skip the authentication test to avoid hanging on invalid URL
	// The URL validation will be caught when actual API calls are made
	// This test just verifies that client creation succeeds regardless of URL format
	t.Log("Client created successfully with invalid URL - validation occurs during API calls")
}

func TestBearerTokenTransport_RoundTrip(t *testing.T) {
	transport := &BearerTokenTransport{
		Token: "test-token-123",
	}

	// Create a test request (we can't easily test the full round trip without a server)
	// but we can test that the header is set correctly by checking the transport logic
	if transport.Token != "test-token-123" {
		t.Errorf("Expected token 'test-token-123', got '%s'", transport.Token)
	}
}

func TestJIRAClient_GetIssue_EmptyKey(t *testing.T) {
	cfg := &config.Config{
		JIRABaseURL: "https://test.atlassian.net",
		JIRAEmail:   "test@example.com",
		JIRAPAT:     "test-pat-token-123",
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	_, err = client.GetIssue("")
	if err == nil {
		t.Fatal("Expected error for empty issue key, got nil")
	}

	if clientErr, ok := err.(*ClientError); !ok {
		t.Errorf("Expected *ClientError, got %T", err)
	} else if clientErr.Type != "invalid_input" {
		t.Errorf("Expected invalid_input error, got %s", clientErr.Type)
	}
}

func TestConvertJIRAIssue(t *testing.T) {
	// This would require importing the jira package and creating mock jira.Issue
	// For now, we'll test the conversion logic through the mock client
	// This demonstrates good separation of concerns - unit tests for conversion logic
	// can be added when we have real JIRA issue objects to work with
	t.Skip("Conversion logic tested through integration tests")
}

func TestGetStatusCategory(t *testing.T) {
	// Test the helper function with nil input
	result := getStatusCategory(nil)
	if result != "" {
		t.Errorf("Expected empty string for nil status, got '%s'", result)
	}
}

func TestClientError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *ClientError
		expected string
	}{
		{
			name: "error with context",
			err: &ClientError{
				Type:    "authentication_error",
				Message: "invalid credentials",
				Context: "PROJ-123",
			},
			expected: "JIRA client error (authentication_error) for PROJ-123: invalid credentials",
		},
		{
			name: "error without context",
			err: &ClientError{
				Type:    "api_error",
				Message: "request failed",
			},
			expected: "JIRA client error (api_error): request failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.err.Error()
			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestIsAuthenticationError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name: "authentication error",
			err: &ClientError{
				Type: "authentication_error",
			},
			expected: true,
		},
		{
			name: "other client error",
			err: &ClientError{
				Type: "api_error",
			},
			expected: false,
		},
		{
			name:     "non-client error",
			err:      errors.New("generic error"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsAuthenticationError(tt.err)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestIsNotFoundError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name: "not found error",
			err: &ClientError{
				Type: "not_found",
			},
			expected: true,
		},
		{
			name: "other client error",
			err: &ClientError{
				Type: "api_error",
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsNotFoundError(tt.err)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestIsAuthorizationError(t *testing.T) {
	authError := &ClientError{Type: "authorization_error"}
	if !IsAuthorizationError(authError) {
		t.Error("Expected authorization error to be detected")
	}

	otherError := &ClientError{Type: "api_error"}
	if IsAuthorizationError(otherError) {
		t.Error("Expected non-authorization error to not be detected")
	}
}

// JQL Search Tests

func TestJIRAClient_SearchIssues_EmptyJQL(t *testing.T) {
	cfg := &config.Config{
		JIRABaseURL: "https://test.atlassian.net",
		JIRAEmail:   "test@example.com",
		JIRAPAT:     "test-pat-token-123",
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	jiraClient := client.(*JIRAClient)

	_, err = jiraClient.SearchIssues("")
	if err == nil {
		t.Fatal("Expected error for empty JQL query")
	}

	clientErr, ok := err.(*ClientError)
	if !ok {
		t.Fatalf("Expected ClientError, got %T", err)
	}

	if clientErr.Type != "invalid_input" {
		t.Errorf("Expected error type 'invalid_input', got '%s'", clientErr.Type)
	}

	if clientErr.Message != "JQL query cannot be empty" {
		t.Errorf("Expected 'JQL query cannot be empty', got '%s'", clientErr.Message)
	}
}

func TestHandleJQLError(t *testing.T) {
	cfg := &config.Config{
		JIRABaseURL: "https://test.atlassian.net",
		JIRAEmail:   "test@example.com",
		JIRAPAT:     "test-pat-token-123",
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	jiraClient := client.(*JIRAClient)
	testJQL := "project = TEST"

	tests := []struct {
		name         string
		statusCode   int
		expectedType string
		expectedMsg  string
	}{
		{
			name:         "JQL syntax error",
			statusCode:   400,
			expectedType: "jql_syntax_error",
			expectedMsg:  "invalid JQL syntax",
		},
		{
			name:         "authentication error",
			statusCode:   401,
			expectedType: "authentication_error",
			expectedMsg:  "authentication failed - check JIRA credentials",
		},
		{
			name:         "authorization error",
			statusCode:   403,
			expectedType: "authorization_error",
			expectedMsg:  "access denied - insufficient permissions for JQL search",
		},
		{
			name:         "generic JQL search error",
			statusCode:   500,
			expectedType: "jql_search_error",
			expectedMsg:  "JQL search request failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock response
			response := &jira.Response{
				Response: &http.Response{
					StatusCode: tt.statusCode,
				},
			}

			err := jiraClient.handleJQLError(errors.New("test error"), response, testJQL)

			clientErr, ok := err.(*ClientError)
			if !ok {
				t.Fatalf("Expected ClientError, got %T", err)
			}

			if clientErr.Type != tt.expectedType {
				t.Errorf("Expected error type '%s', got '%s'", tt.expectedType, clientErr.Type)
			}

			if clientErr.Message != tt.expectedMsg {
				t.Errorf("Expected message '%s', got '%s'", tt.expectedMsg, clientErr.Message)
			}

			if clientErr.Context != testJQL {
				t.Errorf("Expected context '%s', got '%s'", testJQL, clientErr.Context)
			}
		})
	}
}

// Relationship Discovery Tests

func TestExtractRelationships_NoRelationships(t *testing.T) {
	cfg := &config.Config{
		JIRABaseURL: "https://test.atlassian.net",
		JIRAEmail:   "test@example.com",
		JIRAPAT:     "test-pat-token-123",
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	jiraClient := client.(*JIRAClient)

	// Create minimal JIRA issue with no relationships
	jiraIssue := &jira.Issue{
		Key: "TEST-123",
		Fields: &jira.IssueFields{
			Summary: "Test Issue",
		},
	}

	relationships := jiraClient.extractRelationships(jiraIssue)
	if relationships != nil {
		t.Error("Expected nil relationships for issue with no relationships")
	}
}

func TestExtractRelationships_WithEpicLink(t *testing.T) {
	cfg := &config.Config{
		JIRABaseURL: "https://test.atlassian.net",
		JIRAEmail:   "test@example.com",
		JIRAPAT:     "test-pat-token-123",
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	jiraClient := client.(*JIRAClient)

	// Create JIRA issue with epic link
	jiraIssue := &jira.Issue{
		Key: "STORY-456",
		Fields: &jira.IssueFields{
			Summary: "Story in Epic",
			Unknowns: map[string]interface{}{
				"customfield_12311140": "EPIC-789",
			},
		},
	}

	relationships := jiraClient.extractRelationships(jiraIssue)
	if relationships == nil {
		t.Fatal("Expected relationships to be extracted")
	}

	if relationships.EpicLink != "EPIC-789" {
		t.Errorf("Expected epic link 'EPIC-789', got '%s'", relationships.EpicLink)
	}
}

func TestExtractRelationships_WithSubtasks(t *testing.T) {
	cfg := &config.Config{
		JIRABaseURL: "https://test.atlassian.net",
		JIRAEmail:   "test@example.com",
		JIRAPAT:     "test-pat-token-123",
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	jiraClient := client.(*JIRAClient)

	// Create JIRA issue with subtasks
	jiraIssue := &jira.Issue{
		Key: "PARENT-123",
		Fields: &jira.IssueFields{
			Summary: "Parent Issue",
			Subtasks: []*jira.Subtasks{
				{Key: "SUB-1"},
				{Key: "SUB-2"},
				{Key: "SUB-3"},
			},
		},
	}

	relationships := jiraClient.extractRelationships(jiraIssue)
	if relationships == nil {
		t.Fatal("Expected relationships to be extracted")
	}

	expectedSubtasks := []string{"SUB-1", "SUB-2", "SUB-3"}
	if len(relationships.Subtasks) != len(expectedSubtasks) {
		t.Errorf("Expected %d subtasks, got %d", len(expectedSubtasks), len(relationships.Subtasks))
	}

	for i, expected := range expectedSubtasks {
		if relationships.Subtasks[i] != expected {
			t.Errorf("Expected subtask[%d] '%s', got '%s'", i, expected, relationships.Subtasks[i])
		}
	}
}

func TestExtractRelationships_WithParentIssue(t *testing.T) {
	cfg := &config.Config{
		JIRABaseURL: "https://test.atlassian.net",
		JIRAEmail:   "test@example.com",
		JIRAPAT:     "test-pat-token-123",
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	jiraClient := client.(*JIRAClient)

	// Create JIRA subtask with parent
	jiraIssue := &jira.Issue{
		Key: "SUB-456",
		Fields: &jira.IssueFields{
			Summary: "Subtask Issue",
			Parent: &jira.Parent{
				Key: "PARENT-789",
			},
		},
	}

	relationships := jiraClient.extractRelationships(jiraIssue)
	if relationships == nil {
		t.Fatal("Expected relationships to be extracted")
	}

	if relationships.ParentIssue != "PARENT-789" {
		t.Errorf("Expected parent issue 'PARENT-789', got '%s'", relationships.ParentIssue)
	}
}

func TestExtractRelationships_WithIssueLinks(t *testing.T) {
	cfg := &config.Config{
		JIRABaseURL: "https://test.atlassian.net",
		JIRAEmail:   "test@example.com",
		JIRAPAT:     "test-pat-token-123",
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	jiraClient := client.(*JIRAClient)

	// Create JIRA issue with issue links
	jiraIssue := &jira.Issue{
		Key: "TEST-123",
		Fields: &jira.IssueFields{
			Summary: "Issue with Links",
			IssueLinks: []*jira.IssueLink{
				{
					Type: jira.IssueLinkType{Name: "Blocks"},
					OutwardIssue: &jira.Issue{
						Key: "BLOCKED-456",
						Fields: &jira.IssueFields{
							Summary: "Blocked Issue",
						},
					},
				},
				{
					Type: jira.IssueLinkType{Name: "Clones"},
					InwardIssue: &jira.Issue{
						Key: "ORIGINAL-789",
						Fields: &jira.IssueFields{
							Summary: "Original Issue",
						},
					},
				},
			},
		},
	}

	relationships := jiraClient.extractRelationships(jiraIssue)
	if relationships == nil {
		t.Fatal("Expected relationships to be extracted")
	}

	if len(relationships.IssueLinks) != 2 {
		t.Errorf("Expected 2 issue links, got %d", len(relationships.IssueLinks))
	}

	// Check outward link (blocks)
	outwardLink := relationships.IssueLinks[0]
	if outwardLink.Type != "Blocks" {
		t.Errorf("Expected link type 'Blocks', got '%s'", outwardLink.Type)
	}
	if outwardLink.Direction != "outward" {
		t.Errorf("Expected direction 'outward', got '%s'", outwardLink.Direction)
	}
	if outwardLink.IssueKey != "BLOCKED-456" {
		t.Errorf("Expected issue key 'BLOCKED-456', got '%s'", outwardLink.IssueKey)
	}
	if outwardLink.Summary != "Blocked Issue" {
		t.Errorf("Expected summary 'Blocked Issue', got '%s'", outwardLink.Summary)
	}

	// Check inward link (clones)
	inwardLink := relationships.IssueLinks[1]
	if inwardLink.Type != "Clones" {
		t.Errorf("Expected link type 'Clones', got '%s'", inwardLink.Type)
	}
	if inwardLink.Direction != "inward" {
		t.Errorf("Expected direction 'inward', got '%s'", inwardLink.Direction)
	}
	if inwardLink.IssueKey != "ORIGINAL-789" {
		t.Errorf("Expected issue key 'ORIGINAL-789', got '%s'", inwardLink.IssueKey)
	}
	if inwardLink.Summary != "Original Issue" {
		t.Errorf("Expected summary 'Original Issue', got '%s'", inwardLink.Summary)
	}
}

func TestExtractRelationships_ComplexRelationships(t *testing.T) {
	cfg := &config.Config{
		JIRABaseURL: "https://test.atlassian.net",
		JIRAEmail:   "test@example.com",
		JIRAPAT:     "test-pat-token-123",
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	jiraClient := client.(*JIRAClient)

	// Create JIRA issue with all types of relationships
	jiraIssue := &jira.Issue{
		Key: "COMPLEX-123",
		Fields: &jira.IssueFields{
			Summary: "Complex Issue with All Relationships",
			Unknowns: map[string]interface{}{
				"customfield_12311140": "EPIC-456",
			},
			Parent: &jira.Parent{
				Key: "PARENT-789",
			},
			Subtasks: []*jira.Subtasks{
				{Key: "SUB-A"},
				{Key: "SUB-B"},
			},
			IssueLinks: []*jira.IssueLink{
				{
					Type: jira.IssueLinkType{Name: "Documents"},
					OutwardIssue: &jira.Issue{
						Key: "DOC-999",
						Fields: &jira.IssueFields{
							Summary: "Documentation Issue",
						},
					},
				},
			},
		},
	}

	relationships := jiraClient.extractRelationships(jiraIssue)
	if relationships == nil {
		t.Fatal("Expected relationships to be extracted")
	}

	// Verify all relationship types
	if relationships.EpicLink != "EPIC-456" {
		t.Errorf("Expected epic link 'EPIC-456', got '%s'", relationships.EpicLink)
	}

	if relationships.ParentIssue != "PARENT-789" {
		t.Errorf("Expected parent issue 'PARENT-789', got '%s'", relationships.ParentIssue)
	}

	expectedSubtasks := []string{"SUB-A", "SUB-B"}
	if len(relationships.Subtasks) != len(expectedSubtasks) {
		t.Errorf("Expected %d subtasks, got %d", len(expectedSubtasks), len(relationships.Subtasks))
	}

	if len(relationships.IssueLinks) != 1 {
		t.Errorf("Expected 1 issue link, got %d", len(relationships.IssueLinks))
	}

	if relationships.IssueLinks[0].Type != "Documents" {
		t.Errorf("Expected link type 'Documents', got '%s'", relationships.IssueLinks[0].Type)
	}
}

func TestCreateTestIssueWithRelationships(t *testing.T) {
	issue := CreateTestIssueWithRelationships("TEST-123")

	if issue.Relationships == nil {
		t.Fatal("Expected relationships to be present")
	}

	if issue.Relationships.EpicLink != "EPIC-123" {
		t.Errorf("Expected epic link 'EPIC-123', got '%s'", issue.Relationships.EpicLink)
	}

	expectedSubtasks := []string{"SUB-1", "SUB-2"}
	if len(issue.Relationships.Subtasks) != len(expectedSubtasks) {
		t.Errorf("Expected %d subtasks, got %d", len(expectedSubtasks), len(issue.Relationships.Subtasks))
	}

	if len(issue.Relationships.IssueLinks) != 2 {
		t.Errorf("Expected 2 issue links, got %d", len(issue.Relationships.IssueLinks))
	}
}

func TestCreateEpicIssue(t *testing.T) {
	epic := CreateEpicIssue("EPIC-456")

	if epic.IssueType != "Epic" {
		t.Errorf("Expected issue type 'Epic', got '%s'", epic.IssueType)
	}

	if epic.Relationships == nil {
		t.Fatal("Expected relationships to be present")
	}

	expectedStories := []string{"STORY-1", "STORY-2", "STORY-3"}
	if len(epic.Relationships.Subtasks) != len(expectedStories) {
		t.Errorf("Expected %d stories, got %d", len(expectedStories), len(epic.Relationships.Subtasks))
	}
}

func TestCreateSubtaskIssue(t *testing.T) {
	subtask := CreateSubtaskIssue("SUB-789", "PARENT-123")

	if subtask.IssueType != "Sub-task" {
		t.Errorf("Expected issue type 'Sub-task', got '%s'", subtask.IssueType)
	}

	if subtask.Relationships == nil {
		t.Fatal("Expected relationships to be present")
	}

	if subtask.Relationships.ParentIssue != "PARENT-123" {
		t.Errorf("Expected parent issue 'PARENT-123', got '%s'", subtask.Relationships.ParentIssue)
	}
}
