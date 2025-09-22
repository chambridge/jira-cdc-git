package client

import (
	"testing"
)

func TestMockClient_GetIssue_Success(t *testing.T) {
	mock := NewMockClient()

	// Create and add a test issue
	testIssue := CreateTestIssue("PROJ-123")
	mock.AddIssue(testIssue)

	// Test getting the issue
	issue, err := mock.GetIssue("PROJ-123")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if issue.Key != "PROJ-123" {
		t.Errorf("Expected issue key 'PROJ-123', got '%s'", issue.Key)
	}

	if mock.GetIssueCallCount != 1 {
		t.Errorf("Expected call count 1, got %d", mock.GetIssueCallCount)
	}

	if mock.LastRequestedIssue != "PROJ-123" {
		t.Errorf("Expected last requested issue 'PROJ-123', got '%s'", mock.LastRequestedIssue)
	}
}

func TestMockClient_GetIssue_NotFound(t *testing.T) {
	mock := NewMockClient()

	_, err := mock.GetIssue("NONEXISTENT-123")
	if err == nil {
		t.Fatal("Expected error for non-existent issue, got nil")
	}

	if !IsNotFoundError(err) {
		t.Errorf("Expected not found error, got: %v", err)
	}
}

func TestMockClient_GetIssue_AuthenticationError(t *testing.T) {
	mock := NewMockClient()
	authErr := &ClientError{
		Type:    "authentication_error",
		Message: "invalid credentials",
	}
	mock.SetAuthenticationError(authErr)

	_, err := mock.GetIssue("PROJ-123")
	if err == nil {
		t.Fatal("Expected authentication error, got nil")
	}

	if !IsAuthenticationError(err) {
		t.Errorf("Expected authentication error, got: %v", err)
	}
}

func TestMockClient_GetIssue_APIError(t *testing.T) {
	mock := NewMockClient()
	apiErr := &ClientError{
		Type:    "api_error",
		Message: "server error",
	}
	mock.SetAPIError(apiErr)

	_, err := mock.GetIssue("PROJ-123")
	if err == nil {
		t.Fatal("Expected API error, got nil")
	}

	if err != apiErr {
		t.Errorf("Expected API error to be returned, got: %v", err)
	}
}

func TestMockClient_Authenticate_Success(t *testing.T) {
	mock := NewMockClient()

	err := mock.Authenticate()
	if err != nil {
		t.Errorf("Expected no error for authentication, got: %v", err)
	}
}

func TestMockClient_Authenticate_Error(t *testing.T) {
	mock := NewMockClient()
	authErr := &ClientError{
		Type:    "authentication_error",
		Message: "invalid credentials",
	}
	mock.SetAuthenticationError(authErr)

	err := mock.Authenticate()
	if err == nil {
		t.Fatal("Expected authentication error, got nil")
	}

	if err != authErr {
		t.Errorf("Expected authentication error to be returned, got: %v", err)
	}
}

func TestMockClient_Reset(t *testing.T) {
	mock := NewMockClient()

	// Set up some state
	testIssue := CreateTestIssue("PROJ-123")
	mock.AddIssue(testIssue)
	mock.SetAuthenticationError(&ClientError{Type: "authentication_error"})
	_, _ = mock.GetIssue("PROJ-123") // This will increment call count

	// Verify state is set
	if len(mock.Issues) != 1 {
		t.Error("Expected issue to be added")
	}
	if mock.AuthenticationError == nil {
		t.Error("Expected authentication error to be set")
	}
	if mock.GetIssueCallCount != 1 {
		t.Error("Expected call count to be incremented")
	}

	// Reset and verify clean state
	mock.Reset()

	if len(mock.Issues) != 0 {
		t.Error("Expected issues to be cleared")
	}
	if mock.AuthenticationError != nil {
		t.Error("Expected authentication error to be cleared")
	}
	if mock.APIError != nil {
		t.Error("Expected API error to be cleared")
	}
	if mock.GetIssueCallCount != 0 {
		t.Error("Expected call count to be reset")
	}
	if mock.LastRequestedIssue != "" {
		t.Error("Expected last requested issue to be cleared")
	}
}

func TestCreateTestIssue(t *testing.T) {
	issue := CreateTestIssue("TEST-456")

	if issue.Key != "TEST-456" {
		t.Errorf("Expected key 'TEST-456', got '%s'", issue.Key)
	}

	if issue.Summary == "" {
		t.Error("Expected summary to be populated")
	}

	if issue.Status.Name == "" {
		t.Error("Expected status name to be populated")
	}

	if issue.Assignee.Name == "" {
		t.Error("Expected assignee name to be populated")
	}

	if issue.Reporter.Name == "" {
		t.Error("Expected reporter name to be populated")
	}

	if issue.Created == "" {
		t.Error("Expected created date to be populated")
	}

	if issue.Updated == "" {
		t.Error("Expected updated date to be populated")
	}
}

// JQL Search Mock Tests

func TestMockClient_SearchIssues_Success(t *testing.T) {
	mock := NewMockClient()

	// Create test issues
	issue1 := CreateTestIssue("PROJ-1")
	issue2 := CreateTestIssue("PROJ-2")
	mock.AddIssue(issue1)
	mock.AddIssue(issue2)

	// Configure JQL result
	jql := "project = PROJ"
	mock.AddJQLResult(jql, []string{"PROJ-1", "PROJ-2"})

	// Test search
	results, err := mock.SearchIssues(jql)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
	}

	if mock.SearchIssuesCallCount != 1 {
		t.Errorf("Expected call count 1, got %d", mock.SearchIssuesCallCount)
	}

	if mock.LastJQLQuery != jql {
		t.Errorf("Expected last JQL query '%s', got '%s'", jql, mock.LastJQLQuery)
	}

	// Verify issue contents
	if results[0].Key != "PROJ-1" {
		t.Errorf("Expected first result key 'PROJ-1', got '%s'", results[0].Key)
	}

	if results[1].Key != "PROJ-2" {
		t.Errorf("Expected second result key 'PROJ-2', got '%s'", results[1].Key)
	}
}

func TestMockClient_SearchIssues_EmptyJQL(t *testing.T) {
	mock := NewMockClient()

	_, err := mock.SearchIssues("")
	if err == nil {
		t.Fatal("Expected error for empty JQL query, got nil")
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

func TestMockClient_SearchIssues_NoResults(t *testing.T) {
	mock := NewMockClient()

	// Search with JQL that has no configured results
	results, err := mock.SearchIssues("project = NONEXISTENT")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(results) != 0 {
		t.Errorf("Expected 0 results, got %d", len(results))
	}
}

func TestMockClient_SearchIssues_JQLError(t *testing.T) {
	mock := NewMockClient()
	jqlErr := &ClientError{
		Type:    "jql_syntax_error",
		Message: "invalid JQL syntax",
	}
	mock.SetJQLError(jqlErr)

	_, err := mock.SearchIssues("invalid JQL")
	if err == nil {
		t.Fatal("Expected JQL error, got nil")
	}

	if err != jqlErr {
		t.Errorf("Expected specific JQL error, got: %v", err)
	}
}

func TestMockClient_SearchIssues_AuthenticationError(t *testing.T) {
	mock := NewMockClient()
	authErr := &ClientError{
		Type:    "authentication_error",
		Message: "invalid credentials",
	}
	mock.SetAuthenticationError(authErr)

	_, err := mock.SearchIssues("project = TEST")
	if err == nil {
		t.Fatal("Expected authentication error, got nil")
	}

	if err != authErr {
		t.Errorf("Expected specific authentication error, got: %v", err)
	}
}

func TestMockClient_SearchIssues_APIError(t *testing.T) {
	mock := NewMockClient()
	apiErr := &ClientError{
		Type:    "api_error",
		Message: "API request failed",
	}
	mock.SetAPIError(apiErr)

	_, err := mock.SearchIssues("project = TEST")
	if err == nil {
		t.Fatal("Expected API error, got nil")
	}

	if err != apiErr {
		t.Errorf("Expected specific API error, got: %v", err)
	}
}

func TestMockClient_SearchIssues_PartialResults(t *testing.T) {
	mock := NewMockClient()

	// Add only one of the two issues referenced in JQL result
	issue1 := CreateTestIssue("PROJ-1")
	mock.AddIssue(issue1)
	// PROJ-2 is not added, so it won't appear in results

	// Configure JQL result with two issues, but only one exists
	jql := "project = PROJ"
	mock.AddJQLResult(jql, []string{"PROJ-1", "PROJ-2"})

	results, err := mock.SearchIssues(jql)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Should only return the one issue that exists
	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	}

	if results[0].Key != "PROJ-1" {
		t.Errorf("Expected result key 'PROJ-1', got '%s'", results[0].Key)
	}
}

func TestMockClient_Reset_JQLFields(t *testing.T) {
	mock := NewMockClient()

	// Set up some state
	issue := CreateTestIssue("TEST-1")
	mock.AddIssue(issue)
	mock.AddJQLResult("project = TEST", []string{"TEST-1"})
	mock.SetJQLError(&ClientError{Type: "jql_syntax_error"})

	// Execute searches to populate counters
	_, _ = mock.SearchIssues("project = TEST")

	// Verify state is set
	if mock.SearchIssuesCallCount == 0 {
		t.Error("Expected search call count to be set")
	}
	if mock.LastJQLQuery == "" {
		t.Error("Expected last JQL query to be set")
	}

	// Reset
	mock.Reset()

	// Verify JQL-specific fields are reset
	if mock.SearchIssuesCallCount != 0 {
		t.Errorf("Expected search call count to be 0 after reset, got %d", mock.SearchIssuesCallCount)
	}

	if mock.LastJQLQuery != "" {
		t.Errorf("Expected last JQL query to be empty after reset, got '%s'", mock.LastJQLQuery)
	}

	if len(mock.JQLResults) != 0 {
		t.Errorf("Expected JQL results to be empty after reset, got %d entries", len(mock.JQLResults))
	}

	if mock.JQLError != nil {
		t.Errorf("Expected JQL error to be nil after reset, got: %v", mock.JQLError)
	}

	// Test that search works normally after reset
	results, err := mock.SearchIssues("project = TEST")
	if err != nil {
		t.Fatalf("Expected no error after reset, got: %v", err)
	}

	if len(results) != 0 {
		t.Errorf("Expected 0 results after reset, got %d", len(results))
	}
}
