package schema

import (
	"path/filepath"

	"github.com/chambrid/jira-cdc-git/pkg/client"
)

// MockFileWriter implements FileWriter for testing
type MockFileWriter struct {
	// WrittenFiles tracks what files would be written
	WrittenFiles map[string][]byte

	// CreatedDirectories tracks what directories would be created
	CreatedDirectories []string

	// WriteError simulates write failures when set
	WriteError error

	// DirectoryError simulates directory creation failures when set
	DirectoryError error

	// WriteIssueCallCount tracks how many times WriteIssueToYAML was called
	WriteIssueCallCount int

	// LastWrittenIssue tracks the last issue that was written
	LastWrittenIssue *client.Issue
}

// NewMockFileWriter creates a new mock file writer for testing
func NewMockFileWriter() *MockFileWriter {
	return &MockFileWriter{
		WrittenFiles:       make(map[string][]byte),
		CreatedDirectories: make([]string, 0),
	}
}

// WriteIssueToYAML simulates writing an issue to YAML
func (m *MockFileWriter) WriteIssueToYAML(issue *client.Issue, basePath string) (string, error) {
	m.WriteIssueCallCount++
	m.LastWrittenIssue = issue

	// Simulate write error if configured
	if m.WriteError != nil {
		return "", m.WriteError
	}

	if issue == nil || issue.Key == "" {
		return "", &SchemaError{
			Type:    "invalid_input",
			Message: "invalid issue data",
		}
	}

	// Extract project key and construct file path
	projectKey := extractProjectKey(issue.Key)
	if projectKey == "" {
		return "", &SchemaError{
			Type:    "invalid_input",
			Message: "could not extract project key",
		}
	}

	filePath := m.GetIssueFilePath(basePath, projectKey, issue.Key)

	// Simulate YAML marshaling (we'll store the issue for verification)
	yamlData, err := ToYAML(issue)
	if err != nil {
		return "", err
	}

	// Store the "written" file
	m.WrittenFiles[filePath] = yamlData

	return filePath, nil
}

// CreateDirectoryStructure simulates creating directory structure
func (m *MockFileWriter) CreateDirectoryStructure(basePath, projectKey string) error {
	// Simulate directory error if configured
	if m.DirectoryError != nil {
		return m.DirectoryError
	}

	if basePath == "" || projectKey == "" {
		return &SchemaError{
			Type:    "invalid_input",
			Message: "invalid path parameters",
		}
	}

	// Track the directory that would be created
	issuesDir := filepath.Join(basePath, "projects", projectKey, "issues")
	m.CreatedDirectories = append(m.CreatedDirectories, issuesDir)

	return nil
}

// GetIssueFilePath returns the expected file path (same as real implementation)
func (m *MockFileWriter) GetIssueFilePath(basePath, projectKey, issueKey string) string {
	return filepath.Join(basePath, "projects", projectKey, "issues", issueKey+".yaml")
}

// SetWriteError configures the mock to return a write error
func (m *MockFileWriter) SetWriteError(err error) {
	m.WriteError = err
}

// SetDirectoryError configures the mock to return a directory creation error
func (m *MockFileWriter) SetDirectoryError(err error) {
	m.DirectoryError = err
}

// GetWrittenFileContent returns the content that was "written" to a file
func (m *MockFileWriter) GetWrittenFileContent(filePath string) ([]byte, bool) {
	content, exists := m.WrittenFiles[filePath]
	return content, exists
}

// GetCreatedDirectories returns all directories that would be created
func (m *MockFileWriter) GetCreatedDirectories() []string {
	return m.CreatedDirectories
}

// Reset clears all mock state for clean test setup
func (m *MockFileWriter) Reset() {
	m.WrittenFiles = make(map[string][]byte)
	m.CreatedDirectories = make([]string, 0)
	m.WriteError = nil
	m.DirectoryError = nil
	m.WriteIssueCallCount = 0
	m.LastWrittenIssue = nil
}

// VerifyIssueWritten checks if a specific issue was written to the expected path
func (m *MockFileWriter) VerifyIssueWritten(basePath, issueKey string) (bool, string) {
	projectKey := extractProjectKey(issueKey)
	expectedPath := m.GetIssueFilePath(basePath, projectKey, issueKey)
	_, exists := m.WrittenFiles[expectedPath]
	return exists, expectedPath
}
