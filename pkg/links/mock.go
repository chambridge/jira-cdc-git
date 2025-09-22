package links

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/chambrid/jira-cdc-git/pkg/client"
)

// MockLinkManager implements LinkManager for testing purposes
type MockLinkManager struct {
	// Function fields allow test customization
	CreateRelationshipLinksFunc  func(*client.Issue, string) error
	CreateDirectoryStructureFunc func(string, string) error
	ValidateLinkFunc             func(string) error
	CleanupBrokenLinksFunc       func(string, string) error
	GetRelationshipPathFunc      func(string, string, string) string

	// State tracking for verification in tests
	CreatedLinks       map[string]string // linkPath -> targetPath
	CreatedDirectories []string
	ValidatedLinks     []string
	CleanedUpProjects  []string
	CallCount          map[string]int
}

// NewMockLinkManager creates a new mock link manager
func NewMockLinkManager() *MockLinkManager {
	return &MockLinkManager{
		CreatedLinks:       make(map[string]string),
		CreatedDirectories: make([]string, 0),
		ValidatedLinks:     make([]string, 0),
		CleanedUpProjects:  make([]string, 0),
		CallCount:          make(map[string]int),
	}
}

func (m *MockLinkManager) CreateRelationshipLinks(issue *client.Issue, basePath string) error {
	m.CallCount["CreateRelationshipLinks"]++

	if m.CreateRelationshipLinksFunc != nil {
		return m.CreateRelationshipLinksFunc(issue, basePath)
	}

	// Default mock behavior: simulate link creation
	if issue == nil {
		return NewInvalidInputError("issue cannot be nil")
	}

	if issue.Key == "" {
		return NewInvalidInputError("issue key cannot be empty")
	}

	if issue.Relationships == nil {
		return nil // No relationships to process
	}

	projectKey := extractProjectKey(issue.Key)

	// Mock epic link creation
	if issue.Relationships.EpicLink != "" {
		linkPath := filepath.Join(basePath, "projects", projectKey, "relationships", "epic", issue.Key)
		targetPath := "../../issues/" + issue.Relationships.EpicLink + ".yaml"
		m.CreatedLinks[linkPath] = targetPath
	}

	// Mock parent link creation (for subtasks)
	if issue.Relationships.ParentIssue != "" {
		linkPath := filepath.Join(basePath, "projects", projectKey, "relationships", "parent", issue.Key)
		targetPath := "../../issues/" + issue.Relationships.ParentIssue + ".yaml"
		m.CreatedLinks[linkPath] = targetPath
	}

	// Mock subtask links creation
	for _, subtaskKey := range issue.Relationships.Subtasks {
		linkPath := filepath.Join(basePath, "projects", projectKey, "relationships", "subtasks", issue.Key, subtaskKey)
		targetPath := "../../../issues/" + subtaskKey + ".yaml"
		m.CreatedLinks[linkPath] = targetPath
	}

	// Mock issue links creation
	for _, link := range issue.Relationships.IssueLinks {
		dirName := strings.ToLower(link.Type)
		linkPath := filepath.Join(basePath, "projects", projectKey, "relationships", dirName, link.Direction, issue.Key)
		targetPath := "../../../issues/" + link.IssueKey + ".yaml"
		m.CreatedLinks[linkPath] = targetPath
	}

	return nil
}

func (m *MockLinkManager) CreateDirectoryStructure(basePath, projectKey string) error {
	m.CallCount["CreateDirectoryStructure"]++

	if m.CreateDirectoryStructureFunc != nil {
		return m.CreateDirectoryStructureFunc(basePath, projectKey)
	}

	// Default mock behavior: track directory creation
	if basePath == "" {
		return NewInvalidInputError("base path cannot be empty")
	}

	if projectKey == "" {
		return NewInvalidInputError("project key cannot be empty")
	}

	relationshipTypes := []string{"epic", "subtasks", "parent", "blocks", "clones", "documents"}
	for _, relType := range relationshipTypes {
		dirPath := filepath.Join(basePath, "projects", projectKey, "relationships", relType)
		m.CreatedDirectories = append(m.CreatedDirectories, dirPath)
	}

	return nil
}

func (m *MockLinkManager) ValidateLink(linkPath string) error {
	m.CallCount["ValidateLink"]++
	m.ValidatedLinks = append(m.ValidatedLinks, linkPath)

	if m.ValidateLinkFunc != nil {
		return m.ValidateLinkFunc(linkPath)
	}

	// Default mock behavior: assume all links are valid
	if linkPath == "" {
		return NewInvalidInputError("link path cannot be empty")
	}

	return nil
}

func (m *MockLinkManager) CleanupBrokenLinks(basePath, projectKey string) error {
	m.CallCount["CleanupBrokenLinks"]++
	m.CleanedUpProjects = append(m.CleanedUpProjects, projectKey)

	if m.CleanupBrokenLinksFunc != nil {
		return m.CleanupBrokenLinksFunc(basePath, projectKey)
	}

	// Default mock behavior: assume no broken links to clean up
	if basePath == "" {
		return NewInvalidInputError("base path cannot be empty")
	}

	if projectKey == "" {
		return NewInvalidInputError("project key cannot be empty")
	}

	return nil
}

func (m *MockLinkManager) GetRelationshipPath(basePath, projectKey, relationshipType string) string {
	m.CallCount["GetRelationshipPath"]++

	if m.GetRelationshipPathFunc != nil {
		return m.GetRelationshipPathFunc(basePath, projectKey, relationshipType)
	}

	// Default mock behavior: return expected path
	return filepath.Join(basePath, "projects", projectKey, "relationships", relationshipType)
}

// Helper methods for test verification

// GetCreatedLinksCount returns the number of links created
func (m *MockLinkManager) GetCreatedLinksCount() int {
	return len(m.CreatedLinks)
}

// GetCreatedDirectoriesCount returns the number of directories created
func (m *MockLinkManager) GetCreatedDirectoriesCount() int {
	return len(m.CreatedDirectories)
}

// HasCreatedLink checks if a specific link was created
func (m *MockLinkManager) HasCreatedLink(linkPath, targetPath string) bool {
	target, exists := m.CreatedLinks[linkPath]
	return exists && target == targetPath
}

// HasCreatedDirectory checks if a specific directory was created
func (m *MockLinkManager) HasCreatedDirectory(dirPath string) bool {
	for _, created := range m.CreatedDirectories {
		if created == dirPath {
			return true
		}
	}
	return false
}

// GetCallCount returns the number of calls to a specific method
func (m *MockLinkManager) GetCallCount(method string) int {
	return m.CallCount[method]
}

// Reset clears all tracked state (useful between tests)
func (m *MockLinkManager) Reset() {
	m.CreatedLinks = make(map[string]string)
	m.CreatedDirectories = make([]string, 0)
	m.ValidatedLinks = make([]string, 0)
	m.CleanedUpProjects = make([]string, 0)
	m.CallCount = make(map[string]int)
}

// Test helper functions for creating test issues with relationships

// CreateTestIssueWithEpicLink creates a test issue with an epic relationship
func CreateTestIssueWithEpicLink(issueKey, epicKey string) *client.Issue {
	return &client.Issue{
		Key:     issueKey,
		Summary: "Test issue with epic link",
		Relationships: &client.Relationships{
			EpicLink: epicKey,
		},
	}
}

// CreateTestIssueWithSubtasks creates a test issue with subtask relationships
func CreateTestIssueWithSubtasks(issueKey string, subtaskKeys []string) *client.Issue {
	return &client.Issue{
		Key:     issueKey,
		Summary: "Test issue with subtasks",
		Relationships: &client.Relationships{
			Subtasks: subtaskKeys,
		},
	}
}

// CreateTestSubtaskIssue creates a test subtask issue with parent relationship
func CreateTestSubtaskIssue(subtaskKey, parentKey string) *client.Issue {
	return &client.Issue{
		Key:     subtaskKey,
		Summary: "Test subtask issue",
		Relationships: &client.Relationships{
			ParentIssue: parentKey,
		},
	}
}

// CreateTestIssueWithLinks creates a test issue with issue links
func CreateTestIssueWithLinks(issueKey string, links []client.IssueLink) *client.Issue {
	return &client.Issue{
		Key:     issueKey,
		Summary: "Test issue with links",
		Relationships: &client.Relationships{
			IssueLinks: links,
		},
	}
}

// CreateTestIssueLink creates a test issue link
func CreateTestIssueLink(linkType, direction, targetKey string) client.IssueLink {
	return client.IssueLink{
		Type:      linkType,
		Direction: direction,
		IssueKey:  targetKey,
		Summary:   fmt.Sprintf("Target issue %s", targetKey),
	}
}
