# CLI Interface Specification

## Overview

The CLI interface provides the user-facing command-line interface for the jira-cdc-git application. Built with Cobra framework, it implements the sync command with proper validation, error handling, and user feedback.

## Command Structure

### Root Command
```
jira-sync - JIRA CDC Git Sync Tool

A tool for synchronizing JIRA issues to Git repositories as YAML files.
Provides GitOps-friendly approach to tracking JIRA issues alongside code.
```

### Sync Command
```
jira-sync sync --issues=<ISSUE-KEYS> --repo=<REPOSITORY-PATH>
```

## Flag Specifications

### Required Flags
```go
type SyncFlags struct {
    Issues string // JIRA issue key(s) - single or comma-separated (e.g., "PROJ-123" or "PROJ-1,PROJ-2")
    Repo   string // Git repository path (absolute or relative)
}
```

### Global Flags
```go
type GlobalFlags struct {
    LogLevel  string // Log level: debug, info, warn, error
    LogFormat string // Log format: text, json
}
```

### Flag Definitions
```go
func init() {
    // Sync operation flags (mutually exclusive)
    syncCmd.Flags().StringVarP(&flags.Issues, "issues", "i", "", "JIRA issue key(s) - single issue or comma-separated list")
    syncCmd.Flags().StringVarP(&flags.JQL, "jql", "j", "", "JQL query to find issues to sync")
    
    // Required flags
    syncCmd.Flags().StringVarP(&flags.Repo, "repo", "r", "", "Git repository path (required)")
    
    // Performance tuning flags (v0.2.0)
    syncCmd.Flags().IntVarP(&flags.Concurrency, "concurrency", "c", 5, "Parallel workers for batch processing (1-10, default: 5)")
    syncCmd.Flags().StringVar(&flags.RateLimit, "rate-limit", "", "API call delay override (e.g., 100ms, 1s, 2s)")
    
    // Mark required flags
    syncCmd.MarkFlagRequired("repo")
    
    // Global flags
    rootCmd.PersistentFlags().StringVarP(&globalFlags.LogLevel, "log-level", "l", "info", 
        "Log level (debug, info, warn, error)")
    rootCmd.PersistentFlags().StringVar(&globalFlags.LogFormat, "log-format", "text", 
        "Log format (text, json)")
}
```

## Input Validation

### Issue List Validation
```go
func validateIssueList(issues string) ([]string, error) {
    if issues == "" {
        return nil, &CLIError{
            Type:    "ValidationError",
            Field:   "issues",
            Message: "issues list is required",
        }
    }
    
    // Parse comma-separated issue keys
    rawIssues := strings.Split(issues, ",")
    var validIssues []string
    seen := make(map[string]bool)
    
    for _, issue := range rawIssues {
        trimmed := strings.TrimSpace(issue)
        if trimmed == "" {
            continue
        }
        
        // Skip duplicates
        if seen[trimmed] {
            continue
        }
        seen[trimmed] = true
        
        // Validate individual issue key
        if err := validateIssueKey(trimmed); err != nil {
            return nil, err
        }
        
        validIssues = append(validIssues, trimmed)
    }
    
    if len(validIssues) == 0 {
        return nil, &CLIError{
            Type:    "ValidationError",
            Field:   "issues",
            Message: "no valid issues found in list",
        }
    }
    
    return validIssues, nil
}

func validateIssueKey(issueKey string) error {
    // JIRA issue key format: PROJECT-NUMBER
    issueKeyRegex := regexp.MustCompile(`^[A-Z][A-Z0-9]*(-[A-Z0-9]+)*-\\d+$`)
    if !issueKeyRegex.MatchString(issueKey) {
        return &CLIError{
            Type:    "ValidationError", 
            Field:   "issues",
            Message: fmt.Sprintf("invalid issue key '%s': must match format PROJECT-123", issueKey),
        }
    }
    
    return nil
}
```

### Repository Path Validation
```go
func validateRepositoryPath(repoPath string) error {
    if repoPath == "" {
        return &CLIError{
            Type:    "ValidationError",
            Field:   "repo", 
            Message: "repository path is required",
        }
    }
    
    // Convert to absolute path
    absPath, err := filepath.Abs(repoPath)
    if err != nil {
        return &CLIError{
            Type:    "ValidationError",
            Field:   "repo",
            Message: fmt.Sprintf("invalid repository path '%s': %v", repoPath, err),
        }
    }
    
    // Check if parent directory exists
    parentDir := filepath.Dir(absPath)
    if _, err := os.Stat(parentDir); os.IsNotExist(err) {
        return &CLIError{
            Type:    "ValidationError",
            Field:   "repo",
            Message: fmt.Sprintf("parent directory does not exist: %s", parentDir),
        }
    }
    
    return nil
}
```

### Rate Limit Validation (v0.2.0)
```go
func validateRateLimit(rateLimitStr string) (time.Duration, error) {
    if rateLimitStr == "" {
        return 0, nil // Use config default
    }
    
    duration, err := time.ParseDuration(rateLimitStr)
    if err != nil {
        return 0, &CLIError{
            Type:    "ValidationError",
            Field:   "rate-limit",
            Message: fmt.Sprintf("invalid duration format '%s': %v (expected: 100ms, 1s, 2s, etc.)", rateLimitStr, err),
        }
    }
    
    if duration < 0 {
        return 0, &CLIError{
            Type:    "ValidationError",
            Field:   "rate-limit", 
            Message: fmt.Sprintf("rate limit delay must be non-negative, got %v", duration),
        }
    }
    
    return duration, nil
}
```

### Mutual Exclusivity Validation (v0.2.0)
```go
func validateMutualExclusivity(issues, jql string) error {
    if issues != "" && jql != "" {
        return &CLIError{
            Type:    "ValidationError",
            Field:   "sync-mode",
            Message: "cannot specify both --issues and --jql flags",
        }
    }
    
    if issues == "" && jql == "" {
        return &CLIError{
            Type:    "ValidationError", 
            Field:   "sync-mode",
            Message: "must specify either --issues or --jql flag",
        }
    }
    
    return nil
}
```

## Command Implementation

### Sync Command Structure
```go
var syncCmd = &cobra.Command{
    Use:   "sync",
    Short: "Sync JIRA issue(s) to Git repository",
    Long: `Sync fetches one or more JIRA issues and creates corresponding YAML files 
in the specified Git repository with conventional commit formatting.

Each issue will be stored as: {repo}/projects/{PROJECT}/issues/{ISSUE-KEY}.yaml`,
    Example: `  # Single issue sync
  jira-sync sync --issues=PROJ-123 --repo=./my-repo
  
  # Multiple issues sync with rate limiting
  jira-sync sync --issues=PROJ-1,PROJ-2,PROJ-3 --repo=./my-repo --rate-limit=200ms
  
  # JQL query sync with custom concurrency
  jira-sync sync --jql="project = PROJ AND status = 'To Do'" --repo=./my-repo --concurrency=8
  
  # Conservative sync for busy JIRA instances
  jira-sync sync --jql="Epic Link = PROJ-123" --repo=./my-repo --concurrency=2 --rate-limit=1s
  
  # Debug mode with shortcuts
  jira-sync sync -i TEAM-456 -r /path/to/repository -l debug`,
    RunE: runSyncCommand,
}
```

### Command Execution Flow
```go
func runSyncCommand(cmd *cobra.Command, args []string) error {
    // 1. Validate input flags
    if err := validateFlags(); err != nil {
        return err
    }
    
    // 2. Load configuration
    fmt.Println("🔧 Loading configuration...")
    configLoader := config.NewDotEnvLoader()
    cfg, err := configLoader.Load()
    if err != nil {
        return &CLIError{
            Type:    "ConfigurationError",
            Message: fmt.Sprintf("failed to load configuration: %v", err),
        }
    }
    
    // 3. Initialize JIRA client
    fmt.Println("🔗 Connecting to JIRA...")
    jiraClient, err := client.NewClient(cfg)
    if err != nil {
        return &CLIError{
            Type:    "ClientError",
            Message: fmt.Sprintf("failed to create JIRA client: %v", err),
        }
    }
    
    // 4. Authenticate with JIRA
    if err := jiraClient.Authenticate(); err != nil {
        return &CLIError{
            Type:    "AuthenticationError", 
            Message: fmt.Sprintf("failed to authenticate with JIRA: %v", err),
        }
    }
    
    // 5. Parse and validate issue list
    issues, err := validateIssueList(flags.Issues)
    if err != nil {
        return err
    }
    
    // 6. Process each issue
    for i, issueKey := range issues {
        fmt.Printf("📋 [%d/%d] Fetching issue %s...\n", i+1, len(issues), issueKey)
        issue, err := jiraClient.GetIssue(issueKey)
        if err != nil {
            return &CLIError{
                Type:    "IssueError",
                Message: fmt.Sprintf("failed to fetch issue %s: %v", issueKey, err),
            }
        }
    
    // 6. Initialize Git repository
    fmt.Printf("📁 Preparing repository %s...\n", flags.Repo)
    gitRepo := git.NewGitRepository("JIRA Sync Tool", "jira-sync@automated.local")
    
    if err := gitRepo.Initialize(flags.Repo); err != nil {
        return &CLIError{
            Type:    "GitError",
            Message: fmt.Sprintf("failed to initialize repository: %v", err),
        }
    }
    
    // 7. Validate working tree
    if err := gitRepo.ValidateWorkingTree(flags.Repo); err != nil {
        return &CLIError{
            Type:    "GitError",
            Message: fmt.Sprintf("repository validation failed: %v", err),
        }
    }
    
    // 8. Write YAML file
    fmt.Printf("📝 Writing YAML file for %s...\n", issue.Key)
    fileWriter := schema.NewYAMLFileWriter()
    yamlPath, err := fileWriter.WriteIssueToYAML(issue, flags.Repo)
    if err != nil {
        return &CLIError{
            Type:    "FileError",
            Message: fmt.Sprintf("failed to write YAML file: %v", err),
        }
    }
    
    // 9. Commit to Git
    fmt.Println("💾 Committing to Git...")
    if err := gitRepo.CommitIssueFile(flags.Repo, yamlPath, issue); err != nil {
        return &CLIError{
            Type:    "GitError", 
            Message: fmt.Sprintf("failed to commit file: %v", err),
        }
    }
    
    // 10. Success feedback
    fmt.Printf("✅ Successfully synced %s to %s\n", issue.Key, yamlPath)
    fmt.Printf("📋 Issue: %s - %s\n", issue.Key, issue.Summary)
    
    return nil
}
```

## Error Handling

### Error Types
```go
type CLIError struct {
    Type    string // Error category
    Field   string // Field causing error (for validation)
    Message string // User-friendly error message
}

func (e *CLIError) Error() string {
    if e.Field != "" {
        return fmt.Sprintf("%s for field '%s': %s", e.Type, e.Field, e.Message)
    }
    return fmt.Sprintf("%s: %s", e.Type, e.Message)
}
```

### Error Categories
- **ValidationError**: Invalid command-line arguments
- **ConfigurationError**: Configuration loading or validation failures
- **ClientError**: JIRA client creation failures
- **AuthenticationError**: JIRA authentication failures
- **IssueError**: Issue fetching failures
- **GitError**: Git operation failures
- **FileError**: File writing failures

### User-Friendly Error Messages
```go
func formatError(err error) string {
    switch e := err.(type) {
    case *CLIError:
        return fmt.Sprintf("Error: %s", e.Message)
    case *client.AuthenticationError:
        return "Error: Failed to authenticate with JIRA. Please check your credentials in .env file."
    case *client.NotFoundError:
        return fmt.Sprintf("Error: Issue '%s' not found. Please verify the issue key exists and you have permission to view it.", e.IssueKey)
    case *git.DirtyWorkingTreeError:
        return "Error: Git repository has uncommitted changes. Please commit or stash your changes before syncing."
    default:
        return fmt.Sprintf("Error: %v", err)
    }
}
```

## Help and Usage

### Command Help
```
Usage:
  jira-sync sync --issues=<ISSUE-KEYS> --repo=<REPOSITORY-PATH> [flags]

Flags:
  -h, --help            help for sync
  -i, --issues string   JIRA issue key(s) - single issue or comma-separated list (required)
  -r, --repo string     Git repository path (required)

Global Flags:
  -l, --log-level string    Log level (debug, info, warn, error) (default "info")
      --log-format string   Log format (text, json) (default "text")

Examples:
  jira-sync sync --issues=PROJ-123 --repo=./my-repo
  jira-sync sync --issues=PROJ-1,PROJ-2,PROJ-3 --repo=./my-repo
  jira-sync sync -i TEAM-456 -r /path/to/repository
  jira-sync sync --issues=RHOAIENG-789 --repo=. --log-level=debug
```

### Version Information
```go
var rootCmd = &cobra.Command{
    Use:   "jira-sync",
    Short: "JIRA CDC Git Sync Tool",
    Long: `A tool for synchronizing JIRA issues to Git repositories as YAML files.
Provides GitOps-friendly approach to tracking JIRA issues alongside code.`,
    Version: buildVersion,
}
```

## Testing Requirements

### Unit Tests
- Test flag parsing and validation
- Test error handling scenarios
- Test command execution flow with mocks
- Test help text generation

### Integration Tests
- Test complete CLI workflow with real components
- Test various flag combinations
- Test error scenarios with proper exit codes
- Test output formatting

### CLI Testing Framework
```go
func executeCommand(args ...string) (string, error) {
    cmd := getRootCommand()
    cmd.SetArgs(args)
    
    buf := new(bytes.Buffer)
    cmd.SetOut(buf)
    cmd.SetErr(buf)
    
    err := cmd.Execute()
    return buf.String(), err
}

func TestSyncCommandValidation(t *testing.T) {
    tests := []struct {
        name        string
        args        []string
        expectError bool
        errorType   string
    }{
        {
            name:        "missing issues flag",
            args:        []string{"sync", "--repo=./test"},
            expectError: true,
            errorType:   "ValidationError",
        },
        {
            name:        "invalid issue key format",
            args:        []string{"sync", "--issues=invalid", "--repo=./test"},
            expectError: true,
            errorType:   "ValidationError",
        },
        {
            name:        "valid single issue",
            args:        []string{"sync", "--issues=PROJ-123", "--repo=./test"},
            expectError: false,
        },
        {
            name:        "valid multiple issues",
            args:        []string{"sync", "--issues=PROJ-1,PROJ-2", "--repo=./test"},
            expectError: false,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            _, err := executeCommand(tt.args...)
            
            if tt.expectError {
                require.Error(t, err)
                if tt.errorType != "" {
                    cliErr, ok := err.(*CLIError)
                    require.True(t, ok)
                    assert.Equal(t, tt.errorType, cliErr.Type)
                }
            } else {
                require.NoError(t, err)
            }
        })
    }
}
```

## Performance Requirements

- **Command Startup**: < 100ms
- **Flag Parsing**: < 10ms
- **Validation**: < 50ms
- **Complete Workflow**: < 30 seconds (per requirements)
- **Memory Usage**: < 10MB during execution

## Security Requirements

1. **Input Validation**: All user inputs must be validated
2. **Path Traversal Prevention**: Repository paths must be validated
3. **Credential Protection**: No credential logging in output
4. **Error Message Safety**: Sensitive information not exposed in errors
5. **Command Injection Prevention**: All paths properly escaped

## Usage Examples

### Basic Usage
```bash
# Sync single issue to current directory
jira-sync sync --issues=PROJ-123 --repo=.

# Sync multiple issues at once
jira-sync sync --issues=PROJ-1,PROJ-2,PROJ-3 --repo=.

# Sync with specific repository path
jira-sync sync --issues=RHOAIENG-456 --repo=/path/to/my-repo

# Sync with debug logging
jira-sync sync --issues=TEAM-789 --repo=./repo --log-level=debug

# Sync with JSON logging for automation
jira-sync sync --issues=PROJ-999 --repo=./repo --log-format=json
```

### Help Commands
```bash
# Get general help
jira-sync --help

# Get sync command help
jira-sync sync --help

# Get version information
jira-sync --version
```

## Exit Codes

- **0**: Success
- **1**: General error (validation, configuration, etc.)
- **2**: Authentication error
- **3**: Issue not found error
- **4**: Git repository error
- **5**: File operation error