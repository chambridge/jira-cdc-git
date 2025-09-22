package schema

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chambrid/jira-cdc-git/pkg/client"
)

func TestYAMLFileWriter_WriteIssueToYAML_IntegrationTest(t *testing.T) {
	// Create temporary directory for test
	tmpDir := t.TempDir()

	writer := NewYAMLFileWriter()

	// Create test issue based on SPIKE-001 recommendations
	issue := &client.Issue{
		Key:         "PROJ-123",
		Summary:     "Integration Test Issue",
		Description: "This is a test issue for integration testing",
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

	// Write issue to YAML file
	filePath, err := writer.WriteIssueToYAML(issue, tmpDir)
	if err != nil {
		t.Fatalf("Expected no error writing YAML, got: %v", err)
	}

	// Verify file path structure
	expectedPath := filepath.Join(tmpDir, "projects", "PROJ", "issues", "PROJ-123.yaml")
	if filePath != expectedPath {
		t.Errorf("Expected file path '%s', got '%s'", expectedPath, filePath)
	}

	// Verify file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Fatalf("Expected file to exist at %s", filePath)
	}

	// Read and verify file content
	fileContent, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read written file: %v", err)
	}

	yamlContent := string(fileContent)

	// Verify YAML structure matches SPIKE-001 recommendations
	expectedContent := []string{
		"key: PROJ-123",
		"summary: Integration Test Issue",
		"description: This is a test issue for integration testing",
		"name: In Progress",
		"category: indeterminate",
		"name: John Doe",
		"email: john.doe@company.com",
		"name: Jane Smith",
		"email: jane.smith@company.com",
		"created: \"2024-01-01T10:00:00Z\"",
		"updated: \"2024-01-02T15:30:00Z\"",
		"priority: High",
		"issuetype: Story",
	}

	for _, expected := range expectedContent {
		if !strings.Contains(yamlContent, expected) {
			t.Errorf("Expected YAML to contain '%s', but it didn't. YAML content:\n%s", expected, yamlContent)
		}
	}

	// Verify we can read the YAML back
	roundTripIssue, err := FromYAML(fileContent)
	if err != nil {
		t.Fatalf("Failed to parse written YAML: %v", err)
	}

	// Verify round-trip preserves data
	if roundTripIssue.Key != issue.Key {
		t.Errorf("Round-trip failed for Key: expected '%s', got '%s'", issue.Key, roundTripIssue.Key)
	}
	if roundTripIssue.Summary != issue.Summary {
		t.Errorf("Round-trip failed for Summary: expected '%s', got '%s'", issue.Summary, roundTripIssue.Summary)
	}
	if roundTripIssue.Status.Name != issue.Status.Name {
		t.Errorf("Round-trip failed for Status.Name: expected '%s', got '%s'", issue.Status.Name, roundTripIssue.Status.Name)
	}
	if roundTripIssue.Assignee.Email != issue.Assignee.Email {
		t.Errorf("Round-trip failed for Assignee.Email: expected '%s', got '%s'", issue.Assignee.Email, roundTripIssue.Assignee.Email)
	}
}

func TestYAMLFileWriter_CreateDirectoryStructure_IntegrationTest(t *testing.T) {
	tmpDir := t.TempDir()

	writer := NewYAMLFileWriter()

	// Create directory structure
	err := writer.CreateDirectoryStructure(tmpDir, "TESTPROJECT")
	if err != nil {
		t.Fatalf("Expected no error creating directories, got: %v", err)
	}

	// Verify directory structure exists
	expectedDirs := []string{
		filepath.Join(tmpDir, "projects"),
		filepath.Join(tmpDir, "projects", "TESTPROJECT"),
		filepath.Join(tmpDir, "projects", "TESTPROJECT", "issues"),
	}

	for _, dir := range expectedDirs {
		if info, err := os.Stat(dir); os.IsNotExist(err) {
			t.Errorf("Expected directory to exist: %s", dir)
		} else if err != nil {
			t.Errorf("Error checking directory %s: %v", dir, err)
		} else if !info.IsDir() {
			t.Errorf("Expected %s to be a directory", dir)
		}
	}

	// Check that we can write to the directory
	issuesDir := filepath.Join(tmpDir, "projects", "TESTPROJECT", "issues")
	testFile := filepath.Join(issuesDir, "test.txt")
	err = os.WriteFile(testFile, []byte("test"), 0644)
	if err != nil {
		t.Errorf("Cannot write to created directory: %v", err)
	}

	// Clean up test file
	_ = os.Remove(testFile)
}

func TestYAMLFileWriter_MultipleIssues_IntegrationTest(t *testing.T) {
	tmpDir := t.TempDir()

	writer := NewYAMLFileWriter()

	// Create multiple test issues from different projects
	issues := []*client.Issue{
		{
			Key:     "PROJ-123",
			Summary: "First issue",
			Status:  client.Status{Name: "Open"},
		},
		{
			Key:     "PROJ-456",
			Summary: "Second issue",
			Status:  client.Status{Name: "Closed"},
		},
		{
			Key:     "OTHER-789",
			Summary: "Third issue from different project",
			Status:  client.Status{Name: "In Progress"},
		},
	}

	// Write all issues
	writtenPaths := make([]string, 0, len(issues))
	for _, issue := range issues {
		filePath, err := writer.WriteIssueToYAML(issue, tmpDir)
		if err != nil {
			t.Fatalf("Failed to write issue %s: %v", issue.Key, err)
		}
		writtenPaths = append(writtenPaths, filePath)
	}

	// Verify directory structure for multiple projects
	expectedDirs := []string{
		filepath.Join(tmpDir, "projects", "PROJ", "issues"),
		filepath.Join(tmpDir, "projects", "OTHER", "issues"),
	}

	for _, dir := range expectedDirs {
		if info, err := os.Stat(dir); os.IsNotExist(err) {
			t.Errorf("Expected directory to exist: %s", dir)
		} else if err != nil {
			t.Errorf("Error checking directory %s: %v", dir, err)
		} else if !info.IsDir() {
			t.Errorf("Expected %s to be a directory", dir)
		}
	}

	// Verify all files exist and contain correct content
	for i, filePath := range writtenPaths {
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			t.Errorf("Expected file to exist: %s", filePath)
			continue
		}

		// Read and verify content
		content, err := os.ReadFile(filePath)
		if err != nil {
			t.Errorf("Failed to read file %s: %v", filePath, err)
			continue
		}

		yamlContent := string(content)
		if !strings.Contains(yamlContent, issues[i].Key) {
			t.Errorf("File %s should contain key %s", filePath, issues[i].Key)
		}
		if !strings.Contains(yamlContent, issues[i].Summary) {
			t.Errorf("File %s should contain summary %s", filePath, issues[i].Summary)
		}
	}
}

func TestYAMLFileWriter_EmptyFields_IntegrationTest(t *testing.T) {
	tmpDir := t.TempDir()

	writer := NewYAMLFileWriter()

	// Create issue with some empty fields (realistic scenario)
	issue := &client.Issue{
		Key:         "PROJ-999",
		Summary:     "Issue with empty fields",
		Description: "", // Empty description
		Status: client.Status{
			Name:     "Open",
			Category: "", // Empty category
		},
		Assignee: client.User{
			Name:  "", // No assignee
			Email: "",
		},
		Reporter: client.User{
			Name:  "Reporter Name",
			Email: "reporter@example.com",
		},
		Created:   "2024-01-01T10:00:00Z",
		Updated:   "2024-01-01T10:00:00Z",
		Priority:  "Medium",
		IssueType: "Bug",
	}

	// Write issue
	filePath, err := writer.WriteIssueToYAML(issue, tmpDir)
	if err != nil {
		t.Fatalf("Expected no error writing issue with empty fields, got: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Fatalf("Expected file to exist at %s", filePath)
	}

	// Read content and verify it handles empty fields gracefully
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read written file: %v", err)
	}

	yamlContent := string(content)

	// Verify required fields are present
	requiredFields := []string{
		"key: PROJ-999",
		"summary: Issue with empty fields",
		"priority: Medium",
		"issuetype: Bug",
	}

	for _, field := range requiredFields {
		if !strings.Contains(yamlContent, field) {
			t.Errorf("Expected YAML to contain '%s', but it didn't. YAML:\n%s", field, yamlContent)
		}
	}

	// Verify we can still parse it back
	roundTripIssue, err := FromYAML(content)
	if err != nil {
		t.Fatalf("Failed to parse YAML with empty fields: %v", err)
	}

	if roundTripIssue.Key != issue.Key {
		t.Errorf("Round-trip failed for issue with empty fields")
	}
}
