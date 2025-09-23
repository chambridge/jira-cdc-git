package schema

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/chambrid/jira-cdc-git/pkg/client"
)

func TestToYAML_Success(t *testing.T) {
	// Create a test issue matching SPIKE-001 structure
	issue := &client.Issue{
		Key:         "PROJ-123",
		Summary:     "Test Issue Summary",
		Description: "Test issue description",
		Status: client.Status{
			Name:     "In Progress",
			Category: "indeterminate",
		},
		Assignee: client.User{
			Name:  "John Doe",
			Email: "john.doe@company.com",
		},
		Reporter: client.User{
			Name:  "Jane Smith",
			Email: "jane.smith@company.com",
		},
		Created:   "2024-01-01T10:00:00Z",
		Updated:   "2024-01-02T15:30:00Z",
		Priority:  "High",
		IssueType: "Story",
	}

	yamlData, err := ToYAML(issue)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	yamlString := string(yamlData)

	// Verify YAML contains expected fields based on SPIKE-001 recommendations
	expectedFields := []string{
		"key: PROJ-123",
		"summary: Test Issue Summary",
		"description: Test issue description",
		"name: In Progress",
		"name: John Doe",
		"email: john.doe@company.com",
		"created: \"2024-01-01T10:00:00Z\"",
		"priority: High",
		"issuetype: Story",
	}

	for _, field := range expectedFields {
		if !strings.Contains(yamlString, field) {
			t.Errorf("Expected YAML to contain '%s', but it didn't. YAML:\n%s", field, yamlString)
		}
	}
}

func TestToYAML_NilIssue(t *testing.T) {
	_, err := ToYAML(nil)
	if err == nil {
		t.Fatal("Expected error for nil issue, got nil")
	}

	if !IsInvalidInputError(err) {
		t.Errorf("Expected invalid input error, got: %v", err)
	}
}

func TestFromYAML_Success(t *testing.T) {
	// YAML matching SPIKE-001 recommendations
	yamlData := []byte(`
key: PROJ-123
summary: "Test Issue Summary"
description: "Test issue description"
status:
  name: "In Progress"
  category: "indeterminate"
assignee:
  name: "John Doe"
  email: "john.doe@company.com"
reporter:
  name: "Jane Smith"
  email: "jane.smith@company.com"
created: "2024-01-01T10:00:00Z"
updated: "2024-01-02T15:30:00Z"
priority: "High"
issuetype: "Story"
`)

	issue, err := FromYAML(yamlData)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify all fields are correctly parsed
	if issue.Key != "PROJ-123" {
		t.Errorf("Expected key 'PROJ-123', got '%s'", issue.Key)
	}
	if issue.Summary != "Test Issue Summary" {
		t.Errorf("Expected summary 'Test Issue Summary', got '%s'", issue.Summary)
	}
	if issue.Status.Name != "In Progress" {
		t.Errorf("Expected status name 'In Progress', got '%s'", issue.Status.Name)
	}
	if issue.Assignee.Name != "John Doe" {
		t.Errorf("Expected assignee name 'John Doe', got '%s'", issue.Assignee.Name)
	}
	if issue.Assignee.Email != "john.doe@company.com" {
		t.Errorf("Expected assignee email 'john.doe@company.com', got '%s'", issue.Assignee.Email)
	}
	if issue.Priority != "High" {
		t.Errorf("Expected priority 'High', got '%s'", issue.Priority)
	}
}

func TestFromYAML_EmptyData(t *testing.T) {
	_, err := FromYAML([]byte{})
	if err == nil {
		t.Fatal("Expected error for empty YAML data, got nil")
	}

	if !IsInvalidInputError(err) {
		t.Errorf("Expected invalid input error, got: %v", err)
	}
}

func TestFromYAML_InvalidYAML(t *testing.T) {
	invalidYAML := []byte(`
key: PROJ-123
summary: "Test
  invalid yaml structure
`)

	_, err := FromYAML(invalidYAML)
	if err == nil {
		t.Fatal("Expected error for invalid YAML, got nil")
	}

	if !IsSerializationError(err) {
		t.Errorf("Expected serialization error, got: %v", err)
	}
}

func TestExtractProjectKey(t *testing.T) {
	tests := []struct {
		name     string
		issueKey string
		expected string
	}{
		{"valid key", "PROJ-123", "PROJ"},
		{"complex key", "MY-PROJECT-456", "MY-PROJECT"},
		{"single part", "PROJ", ""},
		{"empty key", "", ""},
		{"no dash", "PROJ123", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractProjectKey(tt.issueKey)
			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestYAMLFileWriter_GetIssueFilePath(t *testing.T) {
	writer := NewYAMLFileWriter()

	tests := []struct {
		name       string
		basePath   string
		projectKey string
		issueKey   string
		expected   string
	}{
		{
			name:       "standard path",
			basePath:   "/tmp/test",
			projectKey: "PROJ",
			issueKey:   "PROJ-123",
			expected:   "/tmp/test/projects/PROJ/issues/PROJ-123.yaml",
		},
		{
			name:       "relative path",
			basePath:   "./data",
			projectKey: "MY",
			issueKey:   "MY-456",
			expected:   "data/projects/MY/issues/MY-456.yaml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := writer.GetIssueFilePath(tt.basePath, tt.projectKey, tt.issueKey)
			// Normalize path separators for cross-platform compatibility
			expected := filepath.FromSlash(tt.expected)
			if result != expected {
				t.Errorf("Expected '%s', got '%s'", expected, result)
			}
		})
	}
}

func TestYAMLFileWriter_CreateDirectoryStructure_InvalidInput(t *testing.T) {
	writer := NewYAMLFileWriter()

	tests := []struct {
		name       string
		basePath   string
		projectKey string
		shouldFail bool
	}{
		{"empty base path", "", "PROJ", true},
		{"empty project key", "/tmp/test", "", true},
		{"valid inputs", "/tmp/test", "PROJ", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := writer.CreateDirectoryStructure(tt.basePath, tt.projectKey)
			if tt.shouldFail {
				if err == nil {
					t.Error("Expected error for invalid input, got nil")
				}
				if !IsInvalidInputError(err) {
					t.Errorf("Expected invalid input error, got: %v", err)
				}
			} else if err != nil {
				t.Errorf("Expected no error for valid input, got: %v", err)
			}
		})
	}
}

func TestYAMLFileWriter_WriteIssueToYAML_InvalidInput(t *testing.T) {
	writer := NewYAMLFileWriter()

	tests := []struct {
		name  string
		issue *client.Issue
	}{
		{"nil issue", nil},
		{"empty key", &client.Issue{Key: "", Summary: "Test"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := writer.WriteIssueToYAML(tt.issue, "/tmp/test")
			if err == nil {
				t.Error("Expected error for invalid input, got nil")
			}
			if !IsInvalidInputError(err) {
				t.Errorf("Expected invalid input error, got: %v", err)
			}
		})
	}
}

func TestSchemaError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *SchemaError
		expected string
	}{
		{
			name: "error with context",
			err: &SchemaError{
				Type:    "file_error",
				Message: "could not write file",
				Context: "/tmp/test.yaml",
			},
			expected: "schema error (file_error) for /tmp/test.yaml: could not write file",
		},
		{
			name: "error without context",
			err: &SchemaError{
				Type:    "invalid_input",
				Message: "data is invalid",
			},
			expected: "schema error (invalid_input): data is invalid",
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

func TestErrorTypeCheckers(t *testing.T) {
	serializationErr := &SchemaError{Type: "serialization_error"}
	fileErr := &SchemaError{Type: "file_error"}
	inputErr := &SchemaError{Type: "invalid_input"}
	otherErr := &SchemaError{Type: "other"}

	// Test IsSerializationError
	if !IsSerializationError(serializationErr) {
		t.Error("Expected serialization error to be detected")
	}
	if IsSerializationError(fileErr) {
		t.Error("Expected file error to not be detected as serialization error")
	}

	// Test IsFileError
	if !IsFileError(fileErr) {
		t.Error("Expected file error to be detected")
	}
	if IsFileError(inputErr) {
		t.Error("Expected input error to not be detected as file error")
	}

	// Test IsInvalidInputError
	if !IsInvalidInputError(inputErr) {
		t.Error("Expected input error to be detected")
	}
	if IsInvalidInputError(otherErr) {
		t.Error("Expected other error to not be detected as input error")
	}
}

// TestToYAML_WithRelationships tests YAML serialization with JCG-012 relationship discovery
func TestToYAML_WithRelationships(t *testing.T) {
	// Create test issue with relationships using the mock helper
	issue := client.CreateTestIssueWithRelationships("TEST-123")

	yamlBytes, err := ToYAML(issue)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	yamlString := string(yamlBytes)

	// Check that relationships are included in YAML
	expectedRelationshipFields := []string{
		"relationships:",
		"epic_link: EPIC-123",
		"subtasks:",
		"- SUB-1",
		"- SUB-2",
		"issue_links:",
		"- type: Blocks",
		"direction: outward",
		"issue_key: BLOCKED-456",
		"summary: Issue that is blocked by this one",
		"- type: Clones",
		"direction: inward",
		"issue_key: ORIGINAL-789",
		"summary: Original issue that this clones",
	}

	for _, field := range expectedRelationshipFields {
		if !strings.Contains(yamlString, field) {
			t.Errorf("Expected YAML to contain relationship field '%s', but it didn't. YAML:\n%s", field, yamlString)
		}
	}

	t.Logf("✅ YAML serialization with relationships successful:\n%s", yamlString)
}
