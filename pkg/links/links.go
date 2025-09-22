package links

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/chambrid/jira-cdc-git/pkg/client"
)

// LinkManager defines the interface for symbolic link operations
// This enables dependency injection and testing with mock implementations
type LinkManager interface {
	CreateRelationshipLinks(issue *client.Issue, basePath string) error
	CreateDirectoryStructure(basePath, projectKey string) error
	ValidateLink(linkPath string) error
	CleanupBrokenLinks(basePath, projectKey string) error
	GetRelationshipPath(basePath, projectKey, relationshipType string) string
}

// SymbolicLinkManager implements LinkManager using OS symbolic links
// Based on SPIKE-004 findings: 0.06ms per link creation on macOS
type SymbolicLinkManager struct{}

// NewSymbolicLinkManager creates a new symbolic link manager
func NewSymbolicLinkManager() LinkManager {
	return &SymbolicLinkManager{}
}

// CreateRelationshipLinks creates symbolic links for all relationships in an issue
// Directory structure: /projects/{project}/relationships/{type}/{source-issue} -> ../../../issues/{target-issue}.yaml
func (m *SymbolicLinkManager) CreateRelationshipLinks(issue *client.Issue, basePath string) error {
	if issue == nil {
		return &LinkError{
			Type:    "invalid_input",
			Message: "issue cannot be nil",
		}
	}

	if issue.Key == "" {
		return &LinkError{
			Type:    "invalid_input",
			Message: "issue key cannot be empty",
		}
	}

	if issue.Relationships == nil {
		// No relationships to process, not an error
		return nil
	}

	projectKey := extractProjectKey(issue.Key)
	if projectKey == "" {
		return &LinkError{
			Type:    "invalid_input",
			Message: fmt.Sprintf("could not extract project key from issue key: %s", issue.Key),
		}
	}

	// Create relationships directory structure
	if err := m.CreateDirectoryStructure(basePath, projectKey); err != nil {
		return fmt.Errorf("failed to create relationship directory structure: %w", err)
	}

	// Create epic link
	if issue.Relationships.EpicLink != "" {
		if err := m.createEpicLink(basePath, projectKey, issue.Key, issue.Relationships.EpicLink); err != nil {
			return fmt.Errorf("failed to create epic link: %w", err)
		}
	}

	// Create parent link for subtasks
	if issue.Relationships.ParentIssue != "" {
		if err := m.createSubtaskLink(basePath, projectKey, issue.Key, issue.Relationships.ParentIssue); err != nil {
			return fmt.Errorf("failed to create subtask link: %w", err)
		}
	}

	// Create subtask links (reverse relationship)
	for _, subtaskKey := range issue.Relationships.Subtasks {
		if err := m.createParentLink(basePath, projectKey, issue.Key, subtaskKey); err != nil {
			return fmt.Errorf("failed to create parent link for subtask %s: %w", subtaskKey, err)
		}
	}

	// Create issue links
	for _, link := range issue.Relationships.IssueLinks {
		if err := m.createIssueLink(basePath, projectKey, issue.Key, link); err != nil {
			return fmt.Errorf("failed to create issue link %s: %w", link.Type, err)
		}
	}

	return nil
}

// CreateDirectoryStructure creates the relationships directory structure
// Pattern: /projects/{project-key}/relationships/{type}/
func (m *SymbolicLinkManager) CreateDirectoryStructure(basePath, projectKey string) error {
	if basePath == "" {
		return &LinkError{
			Type:    "invalid_input",
			Message: "base path cannot be empty",
		}
	}

	if projectKey == "" {
		return &LinkError{
			Type:    "invalid_input",
			Message: "project key cannot be empty",
		}
	}

	// Create all relationship type directories
	relationshipTypes := []string{"epic", "subtasks", "parent", "blocks", "clones", "documents"}

	for _, relType := range relationshipTypes {
		relPath := filepath.Join(basePath, "projects", projectKey, "relationships", relType)
		if err := os.MkdirAll(relPath, 0755); err != nil {
			return &LinkError{
				Type:    "directory_creation_error",
				Message: fmt.Sprintf("failed to create relationship directory: %s", relPath),
				Err:     err,
			}
		}
	}

	return nil
}

// ValidateLink checks if a symbolic link exists and points to a valid target
func (m *SymbolicLinkManager) ValidateLink(linkPath string) error {
	if linkPath == "" {
		return &LinkError{
			Type:    "invalid_input",
			Message: "link path cannot be empty",
		}
	}

	// Check if the link exists
	linkInfo, err := os.Lstat(linkPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &LinkError{
				Type:    "link_not_found",
				Message: fmt.Sprintf("symbolic link does not exist: %s", linkPath),
				Err:     err,
			}
		}
		return &LinkError{
			Type:    "link_access_error",
			Message: fmt.Sprintf("cannot access symbolic link: %s", linkPath),
			Err:     err,
		}
	}

	// Verify it's actually a symbolic link
	if linkInfo.Mode()&os.ModeSymlink == 0 {
		return &LinkError{
			Type:    "not_symbolic_link",
			Message: fmt.Sprintf("path is not a symbolic link: %s", linkPath),
		}
	}

	// Check if the target exists
	_, err = os.Stat(linkPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &LinkError{
				Type:    "broken_link",
				Message: fmt.Sprintf("symbolic link target does not exist: %s", linkPath),
				Err:     err,
			}
		}
		return &LinkError{
			Type:    "target_access_error",
			Message: fmt.Sprintf("cannot access symbolic link target: %s", linkPath),
			Err:     err,
		}
	}

	return nil
}

// CleanupBrokenLinks removes broken symbolic links from the relationships directory
func (m *SymbolicLinkManager) CleanupBrokenLinks(basePath, projectKey string) error {
	if basePath == "" {
		return &LinkError{
			Type:    "invalid_input",
			Message: "base path cannot be empty",
		}
	}

	if projectKey == "" {
		return &LinkError{
			Type:    "invalid_input",
			Message: "project key cannot be empty",
		}
	}

	relationshipsPath := filepath.Join(basePath, "projects", projectKey, "relationships")

	// Walk through all relationship directories
	return filepath.Walk(relationshipsPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			// Skip directories that don't exist or can't be accessed
			return nil
		}

		// Only process symbolic links
		if info.Mode()&os.ModeSymlink == 0 {
			return nil
		}

		// Check if the link is broken
		err = m.ValidateLink(path)
		if err != nil {
			if linkErr, ok := err.(*LinkError); ok && linkErr.Type == "broken_link" {
				// Remove broken link
				if removeErr := os.Remove(path); removeErr != nil {
					return &LinkError{
						Type:    "cleanup_error",
						Message: fmt.Sprintf("failed to remove broken link: %s", path),
						Err:     removeErr,
					}
				}
			}
		}

		return nil
	})
}

// GetRelationshipPath returns the directory path for a specific relationship type
func (m *SymbolicLinkManager) GetRelationshipPath(basePath, projectKey, relationshipType string) string {
	return filepath.Join(basePath, "projects", projectKey, "relationships", relationshipType)
}

// Helper functions for creating specific relationship types

func (m *SymbolicLinkManager) createEpicLink(basePath, projectKey, issueKey, epicKey string) error {
	epicDir := m.GetRelationshipPath(basePath, projectKey, "epic")
	linkPath := filepath.Join(epicDir, issueKey)
	targetPath := "../../issues/" + epicKey + ".yaml"

	return m.createSymbolicLink(linkPath, targetPath, "epic")
}

func (m *SymbolicLinkManager) createSubtaskLink(basePath, projectKey, subtaskKey, parentKey string) error {
	parentDir := m.GetRelationshipPath(basePath, projectKey, "parent")
	linkPath := filepath.Join(parentDir, subtaskKey)
	targetPath := "../../issues/" + parentKey + ".yaml"

	return m.createSymbolicLink(linkPath, targetPath, "parent")
}

func (m *SymbolicLinkManager) createParentLink(basePath, projectKey, parentKey, subtaskKey string) error {
	subtasksDir := m.GetRelationshipPath(basePath, projectKey, "subtasks")

	// Create parent-specific directory for grouping subtasks
	parentSubtasksDir := filepath.Join(subtasksDir, parentKey)
	if err := os.MkdirAll(parentSubtasksDir, 0755); err != nil {
		return &LinkError{
			Type:    "directory_creation_error",
			Message: fmt.Sprintf("failed to create parent subtasks directory: %s", parentSubtasksDir),
			Err:     err,
		}
	}

	linkPath := filepath.Join(parentSubtasksDir, subtaskKey)
	targetPath := "../../../issues/" + subtaskKey + ".yaml"

	return m.createSymbolicLink(linkPath, targetPath, "subtasks")
}

func (m *SymbolicLinkManager) createIssueLink(basePath, projectKey, sourceKey string, link client.IssueLink) error {
	// Map link types to directory names
	var dirName string
	switch strings.ToLower(link.Type) {
	case "blocks":
		dirName = "blocks"
	case "clones":
		dirName = "clones"
	case "documents":
		dirName = "documents"
	default:
		// Use the original type for unmapped relationships
		dirName = strings.ToLower(link.Type)
	}

	linkDir := m.GetRelationshipPath(basePath, projectKey, dirName)

	// Create direction-specific subdirectory
	directionDir := filepath.Join(linkDir, link.Direction)
	if err := os.MkdirAll(directionDir, 0755); err != nil {
		return &LinkError{
			Type:    "directory_creation_error",
			Message: fmt.Sprintf("failed to create direction directory: %s", directionDir),
			Err:     err,
		}
	}

	linkPath := filepath.Join(directionDir, sourceKey)
	targetPath := "../../../issues/" + link.IssueKey + ".yaml"

	return m.createSymbolicLink(linkPath, targetPath, link.Type)
}

func (m *SymbolicLinkManager) createSymbolicLink(linkPath, targetPath, linkType string) error {
	// Remove existing link if it exists
	if _, err := os.Lstat(linkPath); err == nil {
		if err := os.Remove(linkPath); err != nil {
			return &LinkError{
				Type:    "link_removal_error",
				Message: fmt.Sprintf("failed to remove existing link: %s", linkPath),
				Err:     err,
			}
		}
	}

	// Create the symbolic link
	if err := os.Symlink(targetPath, linkPath); err != nil {
		return &LinkError{
			Type:    "link_creation_error",
			Message: fmt.Sprintf("failed to create %s symbolic link: %s -> %s", linkType, linkPath, targetPath),
			Err:     err,
		}
	}

	return nil
}

// extractProjectKey extracts the project key from a full issue key
// Example: "PROJ-123" -> "PROJ"
func extractProjectKey(issueKey string) string {
	parts := strings.Split(issueKey, "-")
	if len(parts) < 2 {
		return ""
	}
	return parts[0]
}
