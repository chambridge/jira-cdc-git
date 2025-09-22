# Schema and File Operations Interface Specification

## Overview

The FileWriter interface defines the contract for YAML file operations and directory structure management in the jira-cdc-git system. This specification ensures consistent file organization and YAML serialization.

## Interface Definition

```go
type FileWriter interface {
    WriteIssueToYAML(issue *client.Issue, basePath string) (string, error)
    CreateDirectoryStructure(basePath, projectKey string) error
    GetIssueFilePath(basePath, projectKey, issueKey string) string
}
```

## Directory Structure Specification

### Fixed Directory Pattern
```
{basePath}/projects/{project-key}/issues/{issue-key}.yaml
```

### Examples
```
./repo/projects/RHOAIENG/issues/RHOAIENG-29356.yaml
./repo/projects/PROJ/issues/PROJ-123.yaml
```

### Project Key Extraction
Project key is extracted from issue key by splitting on `-` and taking the first part:
- `RHOAIENG-29356` → Project: `RHOAIENG`
- `PROJ-123` → Project: `PROJ`

## YAML Schema Specification

### Complete YAML Structure
```yaml
key: PROJ-123
summary: Fix authentication bug in login service
description: |
  Detailed description of the issue...
status:
  name: In Progress
  category: In Progress
priority: High
assignee:
  name: John Doe
  email: john.doe@company.com
reporter:
  name: Jane Smith
  email: jane.smith@company.com
issuetype: Bug
created: "2024-01-15T10:30:00Z"
updated: "2024-01-16T14:20:00Z"
```

### Field Mapping
- **Scalar Fields**: `key`, `summary`, `description`, `priority`, `issuetype`
- **Time Fields**: `created`, `updated` (ISO 8601 string format)
- **Nested Objects**: `status`, `assignee`, `reporter`

### YAML Serialization Rules
1. Use human-readable field names (matching JIRA API)
2. Preserve nested structure for complex objects
3. Use block scalar (`|`) for multi-line descriptions
4. Omit empty optional fields
5. Maintain consistent indentation (2 spaces)

## Implementation Requirements

### Directory Operations
```go
func (w *YAMLFileWriter) CreateDirectoryStructure(basePath, projectKey string) error {
    projectPath := filepath.Join(basePath, "projects", projectKey, "issues")
    return os.MkdirAll(projectPath, 0755)
}
```

### File Path Generation
```go
func (w *YAMLFileWriter) GetIssueFilePath(basePath, projectKey, issueKey string) string {
    return filepath.Join(basePath, "projects", projectKey, "issues", issueKey+".yaml")
}
```

### YAML Writing
```go
func (w *YAMLFileWriter) WriteIssueToYAML(issue *client.Issue, basePath string) (string, error) {
    // Extract project key from issue key
    projectKey := extractProjectKey(issue.Key)
    
    // Create directory structure
    if err := w.CreateDirectoryStructure(basePath, projectKey); err != nil {
        return "", fmt.Errorf("failed to create directory structure: %w", err)
    }
    
    // Generate file path
    filePath := w.GetIssueFilePath(basePath, projectKey, issue.Key)
    
    // Serialize to YAML
    yamlData, err := yaml.Marshal(issue)
    if err != nil {
        return "", fmt.Errorf("failed to marshal issue to YAML: %w", err)
    }
    
    // Write file
    if err := os.WriteFile(filePath, yamlData, 0644); err != nil {
        return "", fmt.Errorf("failed to write YAML file: %w", err)
    }
    
    return filePath, nil
}
```

## Error Handling

### Error Types
- **DirectoryCreationError**: Failed to create directory structure
- **YAMLSerializationError**: Failed to serialize issue to YAML
- **FileWriteError**: Failed to write YAML file to disk
- **PathGenerationError**: Invalid project key or issue key format

### Error Context
All errors should include:
- Operation being performed
- File path involved
- Underlying system error
- Issue key context

## Testing Requirements

### Unit Tests
- Test directory structure creation
- Test YAML serialization accuracy
- Test file path generation
- Test error handling scenarios

### Integration Tests
- Test with real filesystem operations
- Test with various project keys and issue keys
- Test file permissions and access rights
- Validate YAML round-trip (write → read → parse)

## Implementation Example

```go
type YAMLFileWriter struct{}

func NewYAMLFileWriter() FileWriter {
    return &YAMLFileWriter{}
}

func (w *YAMLFileWriter) WriteIssueToYAML(issue *client.Issue, basePath string) (string, error) {
    projectKey := extractProjectKey(issue.Key)
    
    if err := w.CreateDirectoryStructure(basePath, projectKey); err != nil {
        return "", &SchemaError{
            Type:      "DirectoryCreation",
            Operation: "CreateDirectoryStructure",
            FilePath:  basePath,
            IssueKey:  issue.Key,
            Err:       err,
        }
    }
    
    filePath := w.GetIssueFilePath(basePath, projectKey, issue.Key)
    
    yamlData, err := yaml.Marshal(issue)
    if err != nil {
        return "", &SchemaError{
            Type:      "YAMLSerialization",
            Operation: "Marshal",
            FilePath:  filePath,
            IssueKey:  issue.Key,
            Err:       err,
        }
    }
    
    if err := os.WriteFile(filePath, yamlData, 0644); err != nil {
        return "", &SchemaError{
            Type:      "FileWrite",
            Operation: "WriteFile",
            FilePath:  filePath,
            IssueKey:  issue.Key,
            Err:       err,
        }
    }
    
    return filePath, nil
}
```

## Usage Examples

```go
// Create file writer
writer := schema.NewYAMLFileWriter()

// Write issue to YAML
filePath, err := writer.WriteIssueToYAML(issue, "/path/to/repo")
if err != nil {
    return fmt.Errorf("failed to write YAML: %w", err)
}

// Result: /path/to/repo/projects/PROJ/issues/PROJ-123.yaml
fmt.Printf("Issue written to: %s\n", filePath)
```

## Validation Requirements

1. Issue key must be valid JIRA format
2. Base path must be accessible and writable
3. Generated file path must be within base path (security)
4. YAML output must be valid and parseable
5. File permissions must be 0644 (readable by others)

## Performance Requirements

- **File Write**: < 100ms per file
- **Directory Creation**: < 50ms per directory structure
- **YAML Serialization**: < 10ms per issue
- **Memory Usage**: < 1MB per issue

## Mock Implementation

```go
type MockFileWriter struct {
    WriteIssueToYAMLFunc func(*client.Issue, string) (string, error)
    Files                map[string][]byte // Track written files
}

func (m *MockFileWriter) WriteIssueToYAML(issue *client.Issue, basePath string) (string, error) {
    if m.WriteIssueToYAMLFunc != nil {
        return m.WriteIssueToYAMLFunc(issue, basePath)
    }
    
    // Default mock behavior
    projectKey := extractProjectKey(issue.Key)
    filePath := filepath.Join(basePath, "projects", projectKey, "issues", issue.Key+".yaml")
    
    yamlData, _ := yaml.Marshal(issue)
    m.Files[filePath] = yamlData
    
    return filePath, nil
}
```