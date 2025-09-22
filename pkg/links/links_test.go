package links

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/chambrid/jira-cdc-git/pkg/client"
)

func TestNewSymbolicLinkManager(t *testing.T) {
	manager := NewSymbolicLinkManager()
	if manager == nil {
		t.Fatal("NewSymbolicLinkManager returned nil")
	}

	// Verify it implements the interface
	var _ = manager
}

func TestCreateRelationshipLinks_NilIssue(t *testing.T) {
	manager := NewSymbolicLinkManager()

	err := manager.CreateRelationshipLinks(nil, "/tmp/test")
	if err == nil {
		t.Fatal("Expected error for nil issue")
	}

	linkErr, ok := err.(*LinkError)
	if !ok {
		t.Fatalf("Expected LinkError, got %T", err)
	}

	if linkErr.Type != "invalid_input" {
		t.Errorf("Expected error type 'invalid_input', got '%s'", linkErr.Type)
	}
}

func TestCreateRelationshipLinks_EmptyIssueKey(t *testing.T) {
	manager := NewSymbolicLinkManager()
	issue := &client.Issue{Key: ""}

	err := manager.CreateRelationshipLinks(issue, "/tmp/test")
	if err == nil {
		t.Fatal("Expected error for empty issue key")
	}

	linkErr, ok := err.(*LinkError)
	if !ok {
		t.Fatalf("Expected LinkError, got %T", err)
	}

	if linkErr.Type != "invalid_input" {
		t.Errorf("Expected error type 'invalid_input', got '%s'", linkErr.Type)
	}
}

func TestCreateRelationshipLinks_NoRelationships(t *testing.T) {
	manager := NewSymbolicLinkManager()
	issue := &client.Issue{
		Key:           "PROJ-123",
		Relationships: nil,
	}

	err := manager.CreateRelationshipLinks(issue, "/tmp/test")
	if err != nil {
		t.Fatalf("Expected no error for issue without relationships, got: %v", err)
	}
}

func TestCreateDirectoryStructure_Success(t *testing.T) {
	// Create temporary directory for testing
	tempDir := t.TempDir()

	manager := NewSymbolicLinkManager()
	err := manager.CreateDirectoryStructure(tempDir, "PROJ")
	if err != nil {
		t.Fatalf("CreateDirectoryStructure failed: %v", err)
	}

	// Verify all relationship directories were created
	expectedDirs := []string{"epic", "subtasks", "parent", "blocks", "clones", "documents"}
	for _, relType := range expectedDirs {
		dirPath := filepath.Join(tempDir, "projects", "PROJ", "relationships", relType)
		if _, err := os.Stat(dirPath); os.IsNotExist(err) {
			t.Errorf("Expected directory not created: %s", dirPath)
		}
	}
}

func TestCreateDirectoryStructure_EmptyBasePath(t *testing.T) {
	manager := NewSymbolicLinkManager()

	err := manager.CreateDirectoryStructure("", "PROJ")
	if err == nil {
		t.Fatal("Expected error for empty base path")
	}

	linkErr, ok := err.(*LinkError)
	if !ok {
		t.Fatalf("Expected LinkError, got %T", err)
	}

	if linkErr.Type != "invalid_input" {
		t.Errorf("Expected error type 'invalid_input', got '%s'", linkErr.Type)
	}
}

func TestCreateDirectoryStructure_EmptyProjectKey(t *testing.T) {
	manager := NewSymbolicLinkManager()

	err := manager.CreateDirectoryStructure("/tmp/test", "")
	if err == nil {
		t.Fatal("Expected error for empty project key")
	}

	linkErr, ok := err.(*LinkError)
	if !ok {
		t.Fatalf("Expected LinkError, got %T", err)
	}

	if linkErr.Type != "invalid_input" {
		t.Errorf("Expected error type 'invalid_input', got '%s'", linkErr.Type)
	}
}

func TestValidateLink_EmptyPath(t *testing.T) {
	manager := NewSymbolicLinkManager()

	err := manager.ValidateLink("")
	if err == nil {
		t.Fatal("Expected error for empty link path")
	}

	linkErr, ok := err.(*LinkError)
	if !ok {
		t.Fatalf("Expected LinkError, got %T", err)
	}

	if linkErr.Type != "invalid_input" {
		t.Errorf("Expected error type 'invalid_input', got '%s'", linkErr.Type)
	}
}

func TestValidateLink_NonExistentLink(t *testing.T) {
	manager := NewSymbolicLinkManager()

	err := manager.ValidateLink("/nonexistent/link")
	if err == nil {
		t.Fatal("Expected error for non-existent link")
	}

	linkErr, ok := err.(*LinkError)
	if !ok {
		t.Fatalf("Expected LinkError, got %T", err)
	}

	if linkErr.Type != "link_not_found" {
		t.Errorf("Expected error type 'link_not_found', got '%s'", linkErr.Type)
	}
}

func TestGetRelationshipPath(t *testing.T) {
	manager := NewSymbolicLinkManager()

	path := manager.GetRelationshipPath("/base", "PROJ", "epic")
	expected := filepath.Join("/base", "projects", "PROJ", "relationships", "epic")

	if path != expected {
		t.Errorf("GetRelationshipPath returned '%s', expected '%s'", path, expected)
	}
}

func TestCleanupBrokenLinks_EmptyBasePath(t *testing.T) {
	manager := NewSymbolicLinkManager()

	err := manager.CleanupBrokenLinks("", "PROJ")
	if err == nil {
		t.Fatal("Expected error for empty base path")
	}

	linkErr, ok := err.(*LinkError)
	if !ok {
		t.Fatalf("Expected LinkError, got %T", err)
	}

	if linkErr.Type != "invalid_input" {
		t.Errorf("Expected error type 'invalid_input', got '%s'", linkErr.Type)
	}
}

func TestCleanupBrokenLinks_EmptyProjectKey(t *testing.T) {
	manager := NewSymbolicLinkManager()

	err := manager.CleanupBrokenLinks("/tmp/test", "")
	if err == nil {
		t.Fatal("Expected error for empty project key")
	}

	linkErr, ok := err.(*LinkError)
	if !ok {
		t.Fatalf("Expected LinkError, got %T", err)
	}

	if linkErr.Type != "invalid_input" {
		t.Errorf("Expected error type 'invalid_input', got '%s'", linkErr.Type)
	}
}

func TestExtractProjectKey(t *testing.T) {
	tests := []struct {
		issueKey    string
		expectedKey string
	}{
		{"PROJ-123", "PROJ"},
		{"RHOAIENG-29356", "RHOAIENG"},
		{"A-1", "A"},
		{"COMPLEX-PROJECT-456", "COMPLEX"},
		{"", ""},
		{"INVALID", ""},
	}

	for _, test := range tests {
		result := extractProjectKey(test.issueKey)
		if result != test.expectedKey {
			t.Errorf("extractProjectKey('%s') = '%s', expected '%s'",
				test.issueKey, result, test.expectedKey)
		}
	}
}

// Integration tests using real filesystem operations

func TestCreateRelationshipLinks_EpicLink_Integration(t *testing.T) {
	tempDir := t.TempDir()

	// Create target issue file first
	issuesDir := filepath.Join(tempDir, "projects", "PROJ", "issues")
	if err := os.MkdirAll(issuesDir, 0755); err != nil {
		t.Fatalf("Failed to create issues directory: %v", err)
	}

	targetFile := filepath.Join(issuesDir, "PROJ-100.yaml")
	if err := os.WriteFile(targetFile, []byte("key: PROJ-100"), 0644); err != nil {
		t.Fatalf("Failed to create target file: %v", err)
	}

	// Create issue with epic link
	issue := &client.Issue{
		Key: "PROJ-123",
		Relationships: &client.Relationships{
			EpicLink: "PROJ-100",
		},
	}

	manager := NewSymbolicLinkManager()
	err := manager.CreateRelationshipLinks(issue, tempDir)
	if err != nil {
		t.Fatalf("CreateRelationshipLinks failed: %v", err)
	}

	// Verify epic link was created
	linkPath := filepath.Join(tempDir, "projects", "PROJ", "relationships", "epic", "PROJ-123")
	linkInfo, err := os.Lstat(linkPath)
	if err != nil {
		t.Fatalf("Epic link not created: %v", err)
	}

	if linkInfo.Mode()&os.ModeSymlink == 0 {
		t.Error("Created path is not a symbolic link")
	}

	// Verify link target
	target, err := os.Readlink(linkPath)
	if err != nil {
		t.Fatalf("Failed to read link target: %v", err)
	}

	expectedTarget := "../../issues/PROJ-100.yaml"
	if target != expectedTarget {
		t.Errorf("Link target is '%s', expected '%s'", target, expectedTarget)
	}

	// Verify link validation
	err = manager.ValidateLink(linkPath)
	if err != nil {
		t.Errorf("Link validation failed: %v", err)
	}
}

func TestCreateRelationshipLinks_Subtasks_Integration(t *testing.T) {
	tempDir := t.TempDir()

	// Create target issue files
	issuesDir := filepath.Join(tempDir, "projects", "PROJ", "issues")
	if err := os.MkdirAll(issuesDir, 0755); err != nil {
		t.Fatalf("Failed to create issues directory: %v", err)
	}

	subtaskKeys := []string{"PROJ-124", "PROJ-125"}
	for _, key := range subtaskKeys {
		targetFile := filepath.Join(issuesDir, key+".yaml")
		if err := os.WriteFile(targetFile, []byte("key: "+key), 0644); err != nil {
			t.Fatalf("Failed to create target file: %v", err)
		}
	}

	// Create issue with subtasks
	issue := &client.Issue{
		Key: "PROJ-123",
		Relationships: &client.Relationships{
			Subtasks: subtaskKeys,
		},
	}

	manager := NewSymbolicLinkManager()
	err := manager.CreateRelationshipLinks(issue, tempDir)
	if err != nil {
		t.Fatalf("CreateRelationshipLinks failed: %v", err)
	}

	// Verify subtask links were created
	for _, subtaskKey := range subtaskKeys {
		linkPath := filepath.Join(tempDir, "projects", "PROJ", "relationships", "subtasks", "PROJ-123", subtaskKey)
		linkInfo, err := os.Lstat(linkPath)
		if err != nil {
			t.Fatalf("Subtask link not created for %s: %v", subtaskKey, err)
		}

		if linkInfo.Mode()&os.ModeSymlink == 0 {
			t.Errorf("Created path is not a symbolic link: %s", linkPath)
		}

		// Verify link target
		target, err := os.Readlink(linkPath)
		if err != nil {
			t.Fatalf("Failed to read link target: %v", err)
		}

		expectedTarget := "../../../issues/" + subtaskKey + ".yaml"
		if target != expectedTarget {
			t.Errorf("Link target is '%s', expected '%s'", target, expectedTarget)
		}
	}
}

func TestCreateRelationshipLinks_IssueLinks_Integration(t *testing.T) {
	tempDir := t.TempDir()

	// Create target issue file
	issuesDir := filepath.Join(tempDir, "projects", "PROJ", "issues")
	if err := os.MkdirAll(issuesDir, 0755); err != nil {
		t.Fatalf("Failed to create issues directory: %v", err)
	}

	targetFile := filepath.Join(issuesDir, "PROJ-200.yaml")
	if err := os.WriteFile(targetFile, []byte("key: PROJ-200"), 0644); err != nil {
		t.Fatalf("Failed to create target file: %v", err)
	}

	// Create issue with issue links
	issue := &client.Issue{
		Key: "PROJ-123",
		Relationships: &client.Relationships{
			IssueLinks: []client.IssueLink{
				{
					Type:      "blocks",
					Direction: "outward",
					IssueKey:  "PROJ-200",
					Summary:   "Target issue",
				},
			},
		},
	}

	manager := NewSymbolicLinkManager()
	err := manager.CreateRelationshipLinks(issue, tempDir)
	if err != nil {
		t.Fatalf("CreateRelationshipLinks failed: %v", err)
	}

	// Verify issue link was created
	linkPath := filepath.Join(tempDir, "projects", "PROJ", "relationships", "blocks", "outward", "PROJ-123")
	linkInfo, err := os.Lstat(linkPath)
	if err != nil {
		t.Fatalf("Issue link not created: %v", err)
	}

	if linkInfo.Mode()&os.ModeSymlink == 0 {
		t.Error("Created path is not a symbolic link")
	}

	// Verify link target
	target, err := os.Readlink(linkPath)
	if err != nil {
		t.Fatalf("Failed to read link target: %v", err)
	}

	expectedTarget := "../../../issues/PROJ-200.yaml"
	if target != expectedTarget {
		t.Errorf("Link target is '%s', expected '%s'", target, expectedTarget)
	}
}

func TestCleanupBrokenLinks_Integration(t *testing.T) {
	tempDir := t.TempDir()

	// Create directory structure
	manager := NewSymbolicLinkManager()
	err := manager.CreateDirectoryStructure(tempDir, "PROJ")
	if err != nil {
		t.Fatalf("Failed to create directory structure: %v", err)
	}

	// Create a broken symbolic link
	epicDir := filepath.Join(tempDir, "projects", "PROJ", "relationships", "epic")
	brokenLink := filepath.Join(epicDir, "PROJ-123")
	nonExistentTarget := "../../../issues/PROJ-NONEXISTENT.yaml"

	err = os.Symlink(nonExistentTarget, brokenLink)
	if err != nil {
		t.Fatalf("Failed to create broken link: %v", err)
	}

	// Verify the link is broken
	err = manager.ValidateLink(brokenLink)
	if err == nil {
		t.Fatal("Expected validation to fail for broken link")
	}

	// Cleanup broken links
	err = manager.CleanupBrokenLinks(tempDir, "PROJ")
	if err != nil {
		t.Fatalf("CleanupBrokenLinks failed: %v", err)
	}

	// Verify broken link was removed
	_, err = os.Lstat(brokenLink)
	if !os.IsNotExist(err) {
		t.Error("Broken link was not removed")
	}
}
