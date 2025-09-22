package links

import (
	"path/filepath"
	"testing"

	"github.com/chambrid/jira-cdc-git/pkg/client"
)

func TestNewMockLinkManager(t *testing.T) {
	mock := NewMockLinkManager()
	if mock == nil {
		t.Fatal("NewMockLinkManager returned nil")
	}

	// Verify it implements the interface
	var _ LinkManager = mock

	// Verify initial state
	if mock.GetCreatedLinksCount() != 0 {
		t.Error("Expected initial links count to be 0")
	}

	if mock.GetCreatedDirectoriesCount() != 0 {
		t.Error("Expected initial directories count to be 0")
	}
}

func TestMockLinkManager_CreateRelationshipLinks_Default(t *testing.T) {
	mock := NewMockLinkManager()

	// Test with epic link
	issue := CreateTestIssueWithEpicLink("PROJ-123", "PROJ-100")

	err := mock.CreateRelationshipLinks(issue, "/tmp/test")
	if err != nil {
		t.Fatalf("CreateRelationshipLinks failed: %v", err)
	}

	// Verify epic link was tracked
	expectedLink := filepath.Join("/tmp/test", "projects", "PROJ", "relationships", "epic", "PROJ-123")
	expectedTarget := "../../issues/PROJ-100.yaml"

	if !mock.HasCreatedLink(expectedLink, expectedTarget) {
		t.Errorf("Expected epic link not found: %s -> %s", expectedLink, expectedTarget)
	}

	// Verify call count
	if mock.GetCallCount("CreateRelationshipLinks") != 1 {
		t.Errorf("Expected 1 call, got %d", mock.GetCallCount("CreateRelationshipLinks"))
	}
}

func TestMockLinkManager_CreateRelationshipLinks_Subtasks(t *testing.T) {
	mock := NewMockLinkManager()

	// Test with subtasks
	subtasks := []string{"PROJ-124", "PROJ-125"}
	issue := CreateTestIssueWithSubtasks("PROJ-123", subtasks)

	err := mock.CreateRelationshipLinks(issue, "/tmp/test")
	if err != nil {
		t.Fatalf("CreateRelationshipLinks failed: %v", err)
	}

	// Verify subtask links were tracked
	for _, subtaskKey := range subtasks {
		expectedLink := filepath.Join("/tmp/test", "projects", "PROJ", "relationships", "subtasks", "PROJ-123", subtaskKey)
		expectedTarget := "../../../issues/" + subtaskKey + ".yaml"

		if !mock.HasCreatedLink(expectedLink, expectedTarget) {
			t.Errorf("Expected subtask link not found: %s -> %s", expectedLink, expectedTarget)
		}
	}

	// Verify total links count
	if mock.GetCreatedLinksCount() != len(subtasks) {
		t.Errorf("Expected %d links, got %d", len(subtasks), mock.GetCreatedLinksCount())
	}
}

func TestMockLinkManager_CreateRelationshipLinks_IssueLinks(t *testing.T) {
	mock := NewMockLinkManager()

	// Test with issue links
	links := []client.IssueLink{
		CreateTestIssueLink("blocks", "outward", "PROJ-200"),
		CreateTestIssueLink("clones", "inward", "PROJ-201"),
	}
	issue := CreateTestIssueWithLinks("PROJ-123", links)

	err := mock.CreateRelationshipLinks(issue, "/tmp/test")
	if err != nil {
		t.Fatalf("CreateRelationshipLinks failed: %v", err)
	}

	// Verify issue links were tracked
	for _, link := range links {
		expectedLink := filepath.Join("/tmp/test", "projects", "PROJ", "relationships", link.Type, link.Direction, "PROJ-123")
		expectedTarget := "../../../issues/" + link.IssueKey + ".yaml"

		if !mock.HasCreatedLink(expectedLink, expectedTarget) {
			t.Errorf("Expected issue link not found: %s -> %s", expectedLink, expectedTarget)
		}
	}

	// Verify total links count
	if mock.GetCreatedLinksCount() != len(links) {
		t.Errorf("Expected %d links, got %d", len(links), mock.GetCreatedLinksCount())
	}
}

func TestMockLinkManager_CreateRelationshipLinks_NilIssue(t *testing.T) {
	mock := NewMockLinkManager()

	err := mock.CreateRelationshipLinks(nil, "/tmp/test")
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

func TestMockLinkManager_CreateDirectoryStructure_Default(t *testing.T) {
	mock := NewMockLinkManager()

	err := mock.CreateDirectoryStructure("/tmp/test", "PROJ")
	if err != nil {
		t.Fatalf("CreateDirectoryStructure failed: %v", err)
	}

	// Verify all expected directories were tracked
	expectedDirs := []string{"epic", "subtasks", "parent", "blocks", "clones", "documents"}
	for _, relType := range expectedDirs {
		expectedDir := filepath.Join("/tmp/test", "projects", "PROJ", "relationships", relType)
		if !mock.HasCreatedDirectory(expectedDir) {
			t.Errorf("Expected directory not found: %s", expectedDir)
		}
	}

	// Verify total directories count
	if mock.GetCreatedDirectoriesCount() != len(expectedDirs) {
		t.Errorf("Expected %d directories, got %d", len(expectedDirs), mock.GetCreatedDirectoriesCount())
	}

	// Verify call count
	if mock.GetCallCount("CreateDirectoryStructure") != 1 {
		t.Errorf("Expected 1 call, got %d", mock.GetCallCount("CreateDirectoryStructure"))
	}
}

func TestMockLinkManager_ValidateLink_Default(t *testing.T) {
	mock := NewMockLinkManager()

	err := mock.ValidateLink("/path/to/link")
	if err != nil {
		t.Fatalf("ValidateLink failed: %v", err)
	}

	// Verify link was tracked
	if len(mock.ValidatedLinks) != 1 {
		t.Errorf("Expected 1 validated link, got %d", len(mock.ValidatedLinks))
	}

	if mock.ValidatedLinks[0] != "/path/to/link" {
		t.Errorf("Expected validated link '/path/to/link', got '%s'", mock.ValidatedLinks[0])
	}

	// Verify call count
	if mock.GetCallCount("ValidateLink") != 1 {
		t.Errorf("Expected 1 call, got %d", mock.GetCallCount("ValidateLink"))
	}
}

func TestMockLinkManager_CleanupBrokenLinks_Default(t *testing.T) {
	mock := NewMockLinkManager()

	err := mock.CleanupBrokenLinks("/tmp/test", "PROJ")
	if err != nil {
		t.Fatalf("CleanupBrokenLinks failed: %v", err)
	}

	// Verify project was tracked
	if len(mock.CleanedUpProjects) != 1 {
		t.Errorf("Expected 1 cleaned up project, got %d", len(mock.CleanedUpProjects))
	}

	if mock.CleanedUpProjects[0] != "PROJ" {
		t.Errorf("Expected cleaned up project 'PROJ', got '%s'", mock.CleanedUpProjects[0])
	}

	// Verify call count
	if mock.GetCallCount("CleanupBrokenLinks") != 1 {
		t.Errorf("Expected 1 call, got %d", mock.GetCallCount("CleanupBrokenLinks"))
	}
}

func TestMockLinkManager_GetRelationshipPath_Default(t *testing.T) {
	mock := NewMockLinkManager()

	path := mock.GetRelationshipPath("/base", "PROJ", "epic")
	expected := filepath.Join("/base", "projects", "PROJ", "relationships", "epic")

	if path != expected {
		t.Errorf("GetRelationshipPath returned '%s', expected '%s'", path, expected)
	}

	// Verify call count
	if mock.GetCallCount("GetRelationshipPath") != 1 {
		t.Errorf("Expected 1 call, got %d", mock.GetCallCount("GetRelationshipPath"))
	}
}

func TestMockLinkManager_CustomFunctions(t *testing.T) {
	mock := NewMockLinkManager()

	// Set custom function
	mock.CreateRelationshipLinksFunc = func(issue *client.Issue, basePath string) error {
		return NewInvalidInputError("custom error")
	}

	issue := CreateTestIssueWithEpicLink("PROJ-123", "PROJ-100")
	err := mock.CreateRelationshipLinks(issue, "/tmp/test")

	if err == nil {
		t.Fatal("Expected custom error")
	}

	linkErr, ok := err.(*LinkError)
	if !ok {
		t.Fatalf("Expected LinkError, got %T", err)
	}

	if linkErr.Message != "custom error" {
		t.Errorf("Expected 'custom error', got '%s'", linkErr.Message)
	}
}

func TestMockLinkManager_Reset(t *testing.T) {
	mock := NewMockLinkManager()

	// Add some state
	issue := CreateTestIssueWithEpicLink("PROJ-123", "PROJ-100")
	_ = mock.CreateRelationshipLinks(issue, "/tmp/test")
	_ = mock.CreateDirectoryStructure("/tmp/test", "PROJ")
	_ = mock.ValidateLink("/path/to/link")
	_ = mock.CleanupBrokenLinks("/tmp/test", "PROJ")

	// Verify state exists
	if mock.GetCreatedLinksCount() == 0 {
		t.Error("Expected some links before reset")
	}
	if mock.GetCreatedDirectoriesCount() == 0 {
		t.Error("Expected some directories before reset")
	}
	if len(mock.ValidatedLinks) == 0 {
		t.Error("Expected some validated links before reset")
	}
	if len(mock.CleanedUpProjects) == 0 {
		t.Error("Expected some cleaned up projects before reset")
	}

	// Reset
	mock.Reset()

	// Verify state is cleared
	if mock.GetCreatedLinksCount() != 0 {
		t.Error("Expected no links after reset")
	}
	if mock.GetCreatedDirectoriesCount() != 0 {
		t.Error("Expected no directories after reset")
	}
	if len(mock.ValidatedLinks) != 0 {
		t.Error("Expected no validated links after reset")
	}
	if len(mock.CleanedUpProjects) != 0 {
		t.Error("Expected no cleaned up projects after reset")
	}
	if mock.GetCallCount("CreateRelationshipLinks") != 0 {
		t.Error("Expected no call counts after reset")
	}
}

// Test helper functions

func TestCreateTestIssueWithEpicLink(t *testing.T) {
	issue := CreateTestIssueWithEpicLink("PROJ-123", "PROJ-100")

	if issue.Key != "PROJ-123" {
		t.Errorf("Expected key 'PROJ-123', got '%s'", issue.Key)
	}

	if issue.Relationships == nil {
		t.Fatal("Expected relationships to be set")
	}

	if issue.Relationships.EpicLink != "PROJ-100" {
		t.Errorf("Expected epic link 'PROJ-100', got '%s'", issue.Relationships.EpicLink)
	}
}

func TestCreateTestIssueWithSubtasks(t *testing.T) {
	subtasks := []string{"PROJ-124", "PROJ-125"}
	issue := CreateTestIssueWithSubtasks("PROJ-123", subtasks)

	if issue.Key != "PROJ-123" {
		t.Errorf("Expected key 'PROJ-123', got '%s'", issue.Key)
	}

	if issue.Relationships == nil {
		t.Fatal("Expected relationships to be set")
	}

	if len(issue.Relationships.Subtasks) != len(subtasks) {
		t.Errorf("Expected %d subtasks, got %d", len(subtasks), len(issue.Relationships.Subtasks))
	}

	for i, expected := range subtasks {
		if issue.Relationships.Subtasks[i] != expected {
			t.Errorf("Expected subtask '%s', got '%s'", expected, issue.Relationships.Subtasks[i])
		}
	}
}

func TestCreateTestSubtaskIssue(t *testing.T) {
	issue := CreateTestSubtaskIssue("PROJ-124", "PROJ-123")

	if issue.Key != "PROJ-124" {
		t.Errorf("Expected key 'PROJ-124', got '%s'", issue.Key)
	}

	if issue.Relationships == nil {
		t.Fatal("Expected relationships to be set")
	}

	if issue.Relationships.ParentIssue != "PROJ-123" {
		t.Errorf("Expected parent issue 'PROJ-123', got '%s'", issue.Relationships.ParentIssue)
	}
}

func TestCreateTestIssueWithLinks(t *testing.T) {
	links := []client.IssueLink{
		CreateTestIssueLink("blocks", "outward", "PROJ-200"),
	}
	issue := CreateTestIssueWithLinks("PROJ-123", links)

	if issue.Key != "PROJ-123" {
		t.Errorf("Expected key 'PROJ-123', got '%s'", issue.Key)
	}

	if issue.Relationships == nil {
		t.Fatal("Expected relationships to be set")
	}

	if len(issue.Relationships.IssueLinks) != len(links) {
		t.Errorf("Expected %d issue links, got %d", len(links), len(issue.Relationships.IssueLinks))
	}

	link := issue.Relationships.IssueLinks[0]
	if link.Type != "blocks" {
		t.Errorf("Expected link type 'blocks', got '%s'", link.Type)
	}
	if link.Direction != "outward" {
		t.Errorf("Expected direction 'outward', got '%s'", link.Direction)
	}
	if link.IssueKey != "PROJ-200" {
		t.Errorf("Expected issue key 'PROJ-200', got '%s'", link.IssueKey)
	}
}

func TestCreateTestIssueLink(t *testing.T) {
	link := CreateTestIssueLink("blocks", "outward", "PROJ-200")

	if link.Type != "blocks" {
		t.Errorf("Expected type 'blocks', got '%s'", link.Type)
	}
	if link.Direction != "outward" {
		t.Errorf("Expected direction 'outward', got '%s'", link.Direction)
	}
	if link.IssueKey != "PROJ-200" {
		t.Errorf("Expected issue key 'PROJ-200', got '%s'", link.IssueKey)
	}
	if link.Summary != "Target issue PROJ-200" {
		t.Errorf("Expected summary 'Target issue PROJ-200', got '%s'", link.Summary)
	}
}
