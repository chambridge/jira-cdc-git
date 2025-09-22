package schema

// IssueFile represents the YAML file structure for storing JIRA issues
// Based on SPIKE-001 findings and requirements
type IssueFile struct {
	Key         string `yaml:"key"`
	Summary     string `yaml:"summary"`
	Description string `yaml:"description"`
	Status      Status `yaml:"status"`
	Assignee    User   `yaml:"assignee"`
	Reporter    User   `yaml:"reporter"`
	Created     string `yaml:"created"`
	Updated     string `yaml:"updated"`
	Priority    string `yaml:"priority"`
	IssueType   string `yaml:"issuetype"`
}

// Status represents issue status information
type Status struct {
	Name     string `yaml:"name"`
	Category string `yaml:"category,omitempty"`
}

// User represents user information
type User struct {
	Name  string `yaml:"name"`
	Email string `yaml:"email,omitempty"`
}

// FileProcessor defines the interface for YAML file operations
type FileProcessor interface {
	// Write saves an issue to a YAML file at the specified path
	Write(issue *IssueFile, filePath string) error

	// Read loads an issue from a YAML file
	Read(filePath string) (*IssueFile, error)

	// GenerateFilePath creates the standard file path for an issue
	// Format: {repoPath}/projects/{projectKey}/issues/{issueKey}.yaml
	GenerateFilePath(repoPath, issueKey string) string
}
