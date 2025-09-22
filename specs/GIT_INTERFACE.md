# Git Operations Interface Specification

## Overview

The Repository interface defines the contract for Git operations in the jira-cdc-git system. This specification uses the `go-git` library for type-safe Git operations and implements conventional commit formatting.

## Interface Definition

```go
type Repository interface {
    // Initialize creates a new Git repository if one doesn't exist
    Initialize(repoPath string) error
    
    // IsRepository checks if the given path is a Git repository
    IsRepository(repoPath string) bool
    
    // ValidateWorkingTree ensures the repository has no uncommitted changes
    ValidateWorkingTree(repoPath string) error
    
    // GetCurrentBranch returns the current branch name
    GetCurrentBranch(repoPath string) (string, error)
    
    // CommitIssueFile adds and commits a YAML issue file with conventional commit message
    CommitIssueFile(repoPath, filePath string, issue *client.Issue) error
    
    // GetRepositoryStatus returns the current status of the repository
    GetRepositoryStatus(repoPath string) (*RepositoryStatus, error)
}
```

## Data Types

### RepositoryStatus Structure

```go
type RepositoryStatus struct {
    IsClean        bool   // No uncommitted changes
    CurrentBranch  string // Name of current branch
    UntrackedFiles int    // Number of untracked files
    ModifiedFiles  int    // Number of modified files
    StagedFiles    int    // Number of staged files
}
```

### GitRepository Implementation

```go
type GitRepository struct {
    AuthorName  string
    AuthorEmail string
}
```

## Implementation Requirements

### Library Choice: go-git
**Rationale for `github.com/go-git/go-git/v5`:**
- Type-safe API operations vs command-line string parsing
- Better error handling with specific error types
- Improved performance (direct API vs process spawning)
- Pure Go implementation, no external git dependency
- Enhanced testability with mocking capabilities

### Repository Operations

#### Initialize Repository
```go
func (g *GitRepository) Initialize(repoPath string) error {
    if g.IsRepository(repoPath) {
        return nil // Already a repository
    }
    
    _, err := git.PlainInit(repoPath, false)
    if err != nil {
        return &GitError{
            Type:      "RepositoryInit",
            Operation: "PlainInit", 
            RepoPath:  repoPath,
            Err:       err,
        }
    }
    
    return nil
}
```

#### Validate Working Tree
```go
func (g *GitRepository) ValidateWorkingTree(repoPath string) error {
    repo, err := git.PlainOpen(repoPath)
    if err != nil {
        return &GitError{
            Type:      "RepositoryOpen",
            Operation: "PlainOpen",
            RepoPath:  repoPath,
            Err:       err,
        }
    }
    
    worktree, err := repo.Worktree()
    if err != nil {
        return &GitError{
            Type:      "WorktreeAccess",
            Operation: "Worktree",
            RepoPath:  repoPath,
            Err:       err,
        }
    }
    
    status, err := worktree.Status()
    if err != nil {
        return &GitError{
            Type:      "StatusCheck",
            Operation: "Status",
            RepoPath:  repoPath,
            Err:       err,
        }
    }
    
    if !status.IsClean() {
        return &DirtyWorkingTreeError{
            RepoPath: repoPath,
            Status:   status,
        }
    }
    
    return nil
}
```

## Conventional Commit Specification

### Commit Message Format
```
{type}({scope}): {description}

{body}

🤖 Generated with [Claude Code](https://claude.ai/code)

Co-Authored-By: Claude <noreply@anthropic.com>
```

### Implementation Example
```go
func (g *GitRepository) CommitIssueFile(repoPath, filePath string, issue *client.Issue) error {
    repo, err := git.PlainOpen(repoPath)
    if err != nil {
        return &GitError{
            Type:      "RepositoryOpen",
            Operation: "PlainOpen",
            RepoPath:  repoPath,
            Err:       err,
        }
    }
    
    worktree, err := repo.Worktree()
    if err != nil {
        return &GitError{
            Type:      "WorktreeAccess", 
            Operation: "Worktree",
            RepoPath:  repoPath,
            Err:       err,
        }
    }
    
    // Add file to staging area
    relPath, err := filepath.Rel(repoPath, filePath)
    if err != nil {
        return &GitError{
            Type:      "PathResolution",
            Operation: "Rel",
            RepoPath:  repoPath,
            FilePath:  filePath,
            Err:       err,
        }
    }
    
    _, err = worktree.Add(relPath)
    if err != nil {
        return &GitError{
            Type:      "FileAdd",
            Operation: "Add",
            RepoPath:  repoPath,
            FilePath:  relPath,
            Err:       err,
        }
    }
    
    // Create commit message
    projectKey := extractProjectKey(issue.Key)
    commitType := getCommitType(issue.IssueType)
    
    message := fmt.Sprintf("%s(%s): add issue %s - %s", 
        commitType, projectKey, issue.Key, issue.Summary)
    
    body := fmt.Sprintf(`Issue Details:
- Type: %s
- Status: %s
- Priority: %s
- Assignee: %s <%s>
- Reporter: %s <%s>
- Created: %s
- Updated: %s

🤖 Generated with [Claude Code](https://claude.ai/code)

Co-Authored-By: Claude <noreply@anthropic.com>`,
        issue.IssueType,
        issue.Status.Name,
        issue.Priority,
        issue.Assignee.Name, issue.Assignee.Email,
        issue.Reporter.Name, issue.Reporter.Email,
        issue.Created,
        issue.Updated)
    
    fullMessage := message + "\n\n" + body
    
    // Create commit
    commit, err := worktree.Commit(fullMessage, &git.CommitOptions{
        Author: &object.Signature{
            Name:  g.AuthorName,
            Email: g.AuthorEmail,
            When:  time.Now(),
        },
    })
    if err != nil {
        return &GitError{
            Type:      "CommitCreation",
            Operation: "Commit",
            RepoPath:  repoPath,
            Err:       err,
        }
    }
    
    return nil
}
```

### Commit Type Mapping
```go
func getCommitType(issueType string) string {
    switch strings.ToLower(issueType) {
    case "bug":
        return "fix"
    case "task", "story":
        return "feat" 
    case "epic":
        return "feat"
    case "improvement", "enhancement":
        return "feat"
    case "sub-task":
        return "feat"
    default:
        return "feat"
    }
}
```

## Error Handling

### Error Types
```go
type GitError struct {
    Type      string // Error category
    Operation string // Git operation that failed
    RepoPath  string // Repository path
    FilePath  string // File path (optional)
    Err       error  // Underlying error
}

type DirtyWorkingTreeError struct {
    RepoPath string
    Status   git.Status
}
```

### Error Categories
- **RepositoryInit**: Failed to initialize repository
- **RepositoryOpen**: Failed to open existing repository
- **WorktreeAccess**: Failed to access repository worktree
- **StatusCheck**: Failed to check repository status
- **FileAdd**: Failed to add file to staging area
- **CommitCreation**: Failed to create commit
- **PathResolution**: Failed to resolve file paths

## Testing Requirements

### Unit Tests
- Test repository initialization
- Test working tree validation
- Test commit message generation
- Test error handling scenarios

### Integration Tests
- Test with real Git repositories
- Test file operations and commits
- Test branch detection and status
- Test dirty working tree detection

### Mock Implementation
```go
type MockRepository struct {
    InitializeFunc         func(string) error
    IsRepositoryFunc       func(string) bool
    ValidateWorkingTreeFunc func(string) error
    CommitIssueFileFunc    func(string, string, *client.Issue) error
    
    // State tracking
    Repositories map[string]bool
    Commits     []MockCommit
}

type MockCommit struct {
    RepoPath string
    FilePath string
    Message  string
    Issue    *client.Issue
}
```

## Usage Examples

```go
// Create Git repository manager
gitRepo := git.NewGitRepository("John Doe", "john.doe@company.com")

// Initialize repository if needed
if err := gitRepo.Initialize("/path/to/repo"); err != nil {
    return fmt.Errorf("failed to initialize repository: %w", err)
}

// Validate clean working tree
if err := gitRepo.ValidateWorkingTree("/path/to/repo"); err != nil {
    return fmt.Errorf("repository has uncommitted changes: %w", err)
}

// Commit issue file
if err := gitRepo.CommitIssueFile("/path/to/repo", "/path/to/file.yaml", issue); err != nil {
    return fmt.Errorf("failed to commit file: %w", err)
}
```

## Validation Requirements

1. Repository path must exist and be accessible
2. Working tree must be clean before operations
3. File paths must be within repository bounds
4. Commit messages must follow conventional format
5. Author information must be provided

## Performance Requirements

- **Repository Init**: < 100ms
- **Status Check**: < 50ms  
- **File Add**: < 10ms per file
- **Commit Creation**: < 100ms
- **Working Tree Validation**: < 50ms

## Security Requirements

- File paths must be validated (no directory traversal)
- Repository operations must be within specified boundaries
- No sensitive information in commit messages
- Proper file permissions on created repositories (0755)