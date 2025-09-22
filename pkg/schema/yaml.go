package schema

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/chambrid/jira-cdc-git/pkg/client"
	"gopkg.in/yaml.v3"
)

// FileWriter defines the interface for file writing operations
// This enables dependency injection and testing with mock implementations
type FileWriter interface {
	WriteIssueToYAML(issue *client.Issue, basePath string) (string, error)
	CreateDirectoryStructure(basePath, projectKey string) error
	GetIssueFilePath(basePath, projectKey, issueKey string) string
}

// YAMLFileWriter implements FileWriter for YAML file operations
type YAMLFileWriter struct{}

// NewYAMLFileWriter creates a new YAML file writer
func NewYAMLFileWriter() FileWriter {
	return &YAMLFileWriter{}
}

// WriteIssueToYAML writes a JIRA issue to a YAML file in the correct directory structure
// Directory structure: /projects/{project-key}/issues/{issue-key}.yaml
// Based on SPIKE-001 recommendations and JCG-004 requirements
func (w *YAMLFileWriter) WriteIssueToYAML(issue *client.Issue, basePath string) (string, error) {
	if issue == nil {
		return "", &SchemaError{
			Type:    "invalid_input",
			Message: "issue cannot be nil",
		}
	}

	if issue.Key == "" {
		return "", &SchemaError{
			Type:    "invalid_input",
			Message: "issue key cannot be empty",
		}
	}

	// Extract project key from issue key (e.g., "PROJ-123" -> "PROJ")
	projectKey := extractProjectKey(issue.Key)
	if projectKey == "" {
		return "", &SchemaError{
			Type:    "invalid_input",
			Message: fmt.Sprintf("could not extract project key from issue key: %s", issue.Key),
		}
	}

	// Create directory structure
	if err := w.CreateDirectoryStructure(basePath, projectKey); err != nil {
		return "", fmt.Errorf("failed to create directory structure: %w", err)
	}

	// Get file path
	filePath := w.GetIssueFilePath(basePath, projectKey, issue.Key)

	// Convert issue to YAML
	yamlData, err := yaml.Marshal(issue)
	if err != nil {
		return "", &SchemaError{
			Type:    "serialization_error",
			Message: "failed to marshal issue to YAML",
			Err:     err,
		}
	}

	// Write YAML to file
	if err := os.WriteFile(filePath, yamlData, 0644); err != nil {
		return "", &SchemaError{
			Type:    "file_error",
			Message: fmt.Sprintf("failed to write YAML file: %s", filePath),
			Err:     err,
		}
	}

	return filePath, nil
}

// CreateDirectoryStructure creates the required directory structure
// Pattern: /projects/{project-key}/issues/
func (w *YAMLFileWriter) CreateDirectoryStructure(basePath, projectKey string) error {
	if basePath == "" {
		return &SchemaError{
			Type:    "invalid_input",
			Message: "base path cannot be empty",
		}
	}

	if projectKey == "" {
		return &SchemaError{
			Type:    "invalid_input",
			Message: "project key cannot be empty",
		}
	}

	// Construct the full path
	issuesDir := filepath.Join(basePath, "projects", projectKey, "issues")

	// Create directories with proper permissions
	if err := os.MkdirAll(issuesDir, 0755); err != nil {
		return &SchemaError{
			Type:    "file_error",
			Message: fmt.Sprintf("failed to create directory: %s", issuesDir),
			Err:     err,
		}
	}

	return nil
}

// GetIssueFilePath returns the full file path for an issue YAML file
// Pattern: /projects/{project-key}/issues/{issue-key}.yaml
func (w *YAMLFileWriter) GetIssueFilePath(basePath, projectKey, issueKey string) string {
	return filepath.Join(basePath, "projects", projectKey, "issues", issueKey+".yaml")
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

// ToYAML converts an issue to YAML bytes for direct use
func ToYAML(issue *client.Issue) ([]byte, error) {
	if issue == nil {
		return nil, &SchemaError{
			Type:    "invalid_input",
			Message: "issue cannot be nil",
		}
	}

	yamlData, err := yaml.Marshal(issue)
	if err != nil {
		return nil, &SchemaError{
			Type:    "serialization_error",
			Message: "failed to marshal issue to YAML",
			Err:     err,
		}
	}

	return yamlData, nil
}

// FromYAML converts YAML bytes to an Issue struct
func FromYAML(yamlData []byte) (*client.Issue, error) {
	if len(yamlData) == 0 {
		return nil, &SchemaError{
			Type:    "invalid_input",
			Message: "YAML data cannot be empty",
		}
	}

	var issue client.Issue
	if err := yaml.Unmarshal(yamlData, &issue); err != nil {
		return nil, &SchemaError{
			Type:    "deserialization_error",
			Message: "failed to unmarshal YAML to issue",
			Err:     err,
		}
	}

	return &issue, nil
}
