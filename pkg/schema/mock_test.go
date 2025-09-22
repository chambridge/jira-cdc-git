package schema

import (
	"path/filepath"
	"testing"

	"github.com/chambrid/jira-cdc-git/pkg/client"
)

func TestMockFileWriter_WriteIssueToYAML_Success(t *testing.T) {
	mock := NewMockFileWriter()

	// Create test issue
	issue := &client.Issue{
		Key:     "PROJ-123",
		Summary: "Test Issue",
		Status: client.Status{
			Name: "In Progress",
		},
	}

	// Write issue
	filePath, err := mock.WriteIssueToYAML(issue, "/tmp/test")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify file path
	expectedPath := "/tmp/test/projects/PROJ/issues/PROJ-123.yaml"
	expectedPath = filepath.FromSlash(expectedPath)
	if filePath != expectedPath {
		t.Errorf("Expected file path '%s', got '%s'", expectedPath, filePath)
	}

	// Verify call tracking
	if mock.WriteIssueCallCount != 1 {
		t.Errorf("Expected call count 1, got %d", mock.WriteIssueCallCount)
	}

	if mock.LastWrittenIssue != issue {
		t.Error("Expected last written issue to match the input issue")
	}

	// Verify file content was stored
	content, exists := mock.GetWrittenFileContent(filePath)
	if !exists {
		t.Error("Expected file content to be stored")
	}

	if len(content) == 0 {
		t.Error("Expected non-empty file content")
	}

	// Verify issue was recorded as written
	written, path := mock.VerifyIssueWritten("/tmp/test", "PROJ-123")
	if !written {
		t.Error("Expected issue to be recorded as written")
	}
	if path != expectedPath {
		t.Errorf("Expected verified path '%s', got '%s'", expectedPath, path)
	}
}

func TestMockFileWriter_WriteIssueToYAML_Error(t *testing.T) {
	mock := NewMockFileWriter()

	// Test with nil issue
	_, err := mock.WriteIssueToYAML(nil, "/tmp/test")
	if err == nil {
		t.Fatal("Expected error for nil issue, got nil")
	}

	// Test with empty key
	issue := &client.Issue{Key: "", Summary: "Test"}
	_, err = mock.WriteIssueToYAML(issue, "/tmp/test")
	if err == nil {
		t.Fatal("Expected error for empty key, got nil")
	}

	// Test configured write error
	validIssue := &client.Issue{Key: "PROJ-123", Summary: "Test"}
	mockErr := &SchemaError{Type: "test_error", Message: "mock error"}
	mock.SetWriteError(mockErr)

	_, err = mock.WriteIssueToYAML(validIssue, "/tmp/test")
	if err != mockErr {
		t.Errorf("Expected configured error, got: %v", err)
	}
}

func TestMockFileWriter_CreateDirectoryStructure_Success(t *testing.T) {
	mock := NewMockFileWriter()

	err := mock.CreateDirectoryStructure("/tmp/test", "PROJ")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify directory was tracked
	dirs := mock.GetCreatedDirectories()
	if len(dirs) != 1 {
		t.Errorf("Expected 1 directory, got %d", len(dirs))
	}

	expectedDir := "/tmp/test/projects/PROJ/issues"
	expectedDir = filepath.FromSlash(expectedDir)
	if dirs[0] != expectedDir {
		t.Errorf("Expected directory '%s', got '%s'", expectedDir, dirs[0])
	}
}

func TestMockFileWriter_CreateDirectoryStructure_Error(t *testing.T) {
	mock := NewMockFileWriter()

	// Test invalid inputs
	err := mock.CreateDirectoryStructure("", "PROJ")
	if err == nil {
		t.Error("Expected error for empty base path, got nil")
	}

	err = mock.CreateDirectoryStructure("/tmp/test", "")
	if err == nil {
		t.Error("Expected error for empty project key, got nil")
	}

	// Test configured directory error
	mockErr := &SchemaError{Type: "test_error", Message: "mock directory error"}
	mock.SetDirectoryError(mockErr)

	err = mock.CreateDirectoryStructure("/tmp/test", "PROJ")
	if err != mockErr {
		t.Errorf("Expected configured error, got: %v", err)
	}
}

func TestMockFileWriter_GetIssueFilePath(t *testing.T) {
	mock := NewMockFileWriter()

	path := mock.GetIssueFilePath("/tmp/test", "PROJ", "PROJ-123")
	expected := "/tmp/test/projects/PROJ/issues/PROJ-123.yaml"
	expected = filepath.FromSlash(expected)

	if path != expected {
		t.Errorf("Expected path '%s', got '%s'", expected, path)
	}
}

func TestMockFileWriter_Reset(t *testing.T) {
	mock := NewMockFileWriter()

	// Set up some state
	issue := &client.Issue{Key: "PROJ-123", Summary: "Test"}
	_, _ = mock.WriteIssueToYAML(issue, "/tmp/test")
	_ = mock.CreateDirectoryStructure("/tmp/test", "PROJ")
	mock.SetWriteError(&SchemaError{Type: "test", Message: "test"})
	mock.SetDirectoryError(&SchemaError{Type: "test", Message: "test"})

	// Verify state is set
	if len(mock.WrittenFiles) == 0 {
		t.Error("Expected written files to be tracked")
	}
	if len(mock.CreatedDirectories) == 0 {
		t.Error("Expected created directories to be tracked")
	}
	if mock.WriteError == nil {
		t.Error("Expected write error to be set")
	}
	if mock.DirectoryError == nil {
		t.Error("Expected directory error to be set")
	}
	if mock.WriteIssueCallCount == 0 {
		t.Error("Expected call count to be incremented")
	}
	if mock.LastWrittenIssue == nil {
		t.Error("Expected last written issue to be set")
	}

	// Reset and verify clean state
	mock.Reset()

	if len(mock.WrittenFiles) != 0 {
		t.Error("Expected written files to be cleared")
	}
	if len(mock.CreatedDirectories) != 0 {
		t.Error("Expected created directories to be cleared")
	}
	if mock.WriteError != nil {
		t.Error("Expected write error to be cleared")
	}
	if mock.DirectoryError != nil {
		t.Error("Expected directory error to be cleared")
	}
	if mock.WriteIssueCallCount != 0 {
		t.Error("Expected call count to be reset")
	}
	if mock.LastWrittenIssue != nil {
		t.Error("Expected last written issue to be cleared")
	}
}

func TestMockFileWriter_VerifyIssueWritten(t *testing.T) {
	mock := NewMockFileWriter()

	// Test issue not written
	written, path := mock.VerifyIssueWritten("/tmp/test", "PROJ-123")
	if written {
		t.Error("Expected issue to not be written initially")
	}

	expectedPath := "/tmp/test/projects/PROJ/issues/PROJ-123.yaml"
	expectedPath = filepath.FromSlash(expectedPath)
	if path != expectedPath {
		t.Errorf("Expected path '%s', got '%s'", expectedPath, path)
	}

	// Write issue and verify
	issue := &client.Issue{Key: "PROJ-123", Summary: "Test"}
	_, _ = mock.WriteIssueToYAML(issue, "/tmp/test")

	written, _ = mock.VerifyIssueWritten("/tmp/test", "PROJ-123")
	if !written {
		t.Error("Expected issue to be written after WriteIssueToYAML")
	}
}
