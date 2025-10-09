# System Architecture Specification

## Overview

The jira-cdc-git system implements a clean architecture pattern with clear separation of concerns, dependency injection, and interface-based design. This specification defines the overall system architecture, component relationships, and data flow.

## Architecture Principles

### 1. Clean Architecture
- **Domain Layer**: Core business logic and entities (pkg/)
- **Application Layer**: Use cases and orchestration (internal/)
- **Interface Layer**: External interfaces and adapters (cmd/)
- **Infrastructure Layer**: External services and frameworks

### 2. Dependency Injection
- All components depend on interfaces, not implementations
- Mock implementations available for all interfaces
- Testable design with clear boundaries

### 3. Single Responsibility
- Each package has a single, well-defined responsibility
- Minimal coupling between components
- High cohesion within components

## System Components

```
┌─────────────────────────────────────────────────────────────┐
│                           CLI Layer                          │
│  ┌─────────────────┐  ┌─────────────────┐  ┌──────────────┐  │
│  │   Root Command  │  │  Sync Command   │  │ Validation   │  │
│  │   (cobra.Command)│  │  (runSyncCmd)   │  │ (flags)      │  │
│  └─────────────────┘  └─────────────────┘  └──────────────┘  │
└─────────────────────────────────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────┐
│                      Application Layer                      │
│  ┌─────────────────────────────────────────────────────────┐ │
│  │              Enhanced Sync Workflow                     │ │
│  │Config → Client → JQL/EPIC → Batch → Schema → Git       │ │
│  └─────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────┐
│                       Domain Layer                          │
│ ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────┐ │
│ │  Config  │ │  Client  │ │   JQL    │ │  Schema  │ │ Git  │ │
│ │(Provider)│ │(Interface)│ │(Builder) │ │(Writer)  │ │(Repo)│ │
│ └──────────┘ └──────────┘ └──────────┘ └──────────┘ └──────┘ │
│ ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐        │ │
│ │   EPIC   │ │  Links   │ │  Batch   │ │  Sync    │        │ │
│ │(Analyzer)│ │(Manager) │ │ (Engine) │ │(Results) │        │ │
│ └──────────┘ └──────────┘ └──────────┘ └──────────┘        │ │
└─────────────────────────────────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────┐
│                    Infrastructure Layer                     │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌──────┐ │
│  │   godotenv  │  │  go-jira    │  │  yaml.v3    │  │go-git│ │
│  │  (.env)     │  │ (REST API)  │  │ (YAML)      │ │(Git) │ │
│  └─────────────┘  └─────────────┘  └─────────────┘  └──────┘ │
└─────────────────────────────────────────────────────────────┘
```

## Component Interfaces

### Configuration Component
```go
// Domain interface
type Provider interface {
    Load() (*Config, error)
    Validate(*Config) error
    LoadFromEnv() (*Config, error)
}

// Implementation
type DotEnvLoader struct {
    envLoader EnvLoader
}
```

### JIRA Client Component
```go
// Domain interface
type Client interface {
    GetIssue(issueKey string) (*Issue, error)
    SearchIssues(jql string) ([]*Issue, error)
    SearchIssuesWithPagination(jql string, startAt, maxResults int) ([]*Issue, int, error)
    Authenticate() error
}

// Implementation
type JIRAClient struct {
    client *jira.Client
    config *config.Config
}
```

### JQL Query Builder Component
```go
// Domain interface
type QueryBuilder interface {
    BuildEpicQuery(epicKey string) (*Query, error)
    BuildFromTemplate(templateName string, params map[string]string) (*Query, error)
    ValidateQuery(jql string) (*ValidationResult, error)
    OptimizeQuery(jql string) (*Query, error)
    PreviewQuery(jql string) (*PreviewResult, error)
    SaveQuery(name, description, jql string) error
    GetSavedQueries() ([]*SavedQuery, error)
    GetTemplates() []*Template
}

// Implementation
type JIRAQueryBuilder struct {
    client       Client
    epicAnalyzer epic.EpicAnalyzer
    savedQueries []*SavedQuery
    templates    []*Template
}
```

### EPIC Analyzer Component
```go
// Domain interface
type EpicAnalyzer interface {
    AnalyzeEpic(epicKey string) (*AnalysisResult, error)
    DiscoverEpicIssues(epicKey string) ([]*client.Issue, error)
    GetEpicHierarchy(epicKey string) (*EpicHierarchy, error)
    ValidateEpicCompleteness(epicKey string) (*CompletenessReport, error)
}

// Implementation
type JIRAEpicAnalyzer struct {
    client  client.Client
    options *DiscoveryOptions
    cache   map[string]*AnalysisResult
}
```

### Batch Sync Engine Component
```go
// Domain interface
type BatchSyncOrchestrator interface {
    SyncIssues(ctx context.Context, issues []string, repoPath string) (*BatchResult, error)
    SyncJQL(ctx context.Context, jql string, repoPath string) (*BatchResult, error)
}

// Implementation
type BatchSyncEngine struct {
    client       client.Client
    fileWriter   schema.FileWriter
    gitRepo      git.Repository
    linkManager  links.LinkManager
    concurrency  int
}
```

### Schema Component
```go
// Domain interface
type FileWriter interface {
    WriteIssueToYAML(issue *client.Issue, basePath string) (string, error)
    CreateDirectoryStructure(basePath, projectKey string) error
    GetIssueFilePath(basePath, projectKey, issueKey string) string
}

// Implementation
type YAMLFileWriter struct{}
```

### Git Component
```go
// Domain interface
type Repository interface {
    Initialize(repoPath string) error
    IsRepository(repoPath string) bool
    ValidateWorkingTree(repoPath string) error
    GetCurrentBranch(repoPath string) (string, error)
    CommitIssueFile(repoPath, filePath string, issue *client.Issue) error
    GetRepositoryStatus(repoPath string) (*RepositoryStatus, error)
}

// Implementation
type GitRepository struct {
    AuthorName  string
    AuthorEmail string
}
```

## Data Flow Architecture

### 1. Sync Workflow Data Flow
```
┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│   CLI Args  │───▶│   Config    │───▶│ JIRA Client │
│ issue, repo │    │ Load .env   │    │ Authenticate│
└─────────────┘    └─────────────┘    └─────────────┘
                                              │
                                              ▼
┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│Git Commit   │◀───│  YAML File  │◀───│JIRA Issue   │
│Conventional │    │  Write      │    │Fetch API    │
└─────────────┘    └─────────────┘    └─────────────┘
```

### 2. Configuration Loading Flow
```
┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│Environment  │    │   .env      │    │Validation   │
│Variables    │───▶│   File      │───▶│Rules        │───▶ Config
│JIRA_*       │    │godotenv     │    │Required     │
└─────────────┘    └─────────────┘    └─────────────┘
```

### 3. File Structure Generation
```
┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│Issue Key    │    │Project Key  │    │Directory    │
│PROJ-123     │───▶│PROJ         │───▶│Structure    │
└─────────────┘    └─────────────┘    └─────────────┘
                                              │
                                              ▼
                                    /projects/PROJ/issues/
                                         PROJ-123.yaml
```

## Error Handling Architecture

### Error Type Hierarchy
```go
// Base error interface
type Error interface {
    error
    Type() string
    Context() map[string]interface{}
}

// Component-specific errors
type ClientError interface {
    Error
    IsAuthenticationError() bool
    IsNotFoundError() bool
    IsAPIError() bool
}

type GitError interface {
    Error
    IsDirtyWorkingTreeError() bool
    IsRepositoryError() bool
}

type SchemaError interface {
    Error
    IsFileError() bool
    IsYAMLError() bool
    IsDirectoryError() bool
}

type ConfigError interface {
    Error
    IsValidationError() bool
    IsEnvFileError() bool
}
```

### Error Propagation Flow
```
Infrastructure Layer (go-jira, go-git, yaml.v3)
           │
           ▼ (wrap with context)
Domain Layer (Client, Git, Schema, Config)
           │
           ▼ (preserve type information)
Application Layer (Sync Workflow)
           │
           ▼ (format for user)
CLI Layer (Error Messages, Exit Codes)
```

## Testing Architecture

### Testing Strategy
```go
// Unit Tests: Mock all dependencies
func TestSyncCommand(t *testing.T) {
    mockConfig := &config.MockProvider{}
    mockClient := &client.MockClient{}
    mockWriter := &schema.MockFileWriter{}
    mockGit := &git.MockRepository{}
    
    // Test with mocked dependencies
}

// Integration Tests: Real implementations
func TestEndToEndWorkflow(t *testing.T) {
    realConfig := config.NewDotEnvLoader()
    realClient := client.NewClient(cfg)
    realWriter := schema.NewYAMLFileWriter()
    realGit := git.NewGitRepository(name, email)
    
    // Test with real dependencies
}
```

### Mock Architecture
```go
// All interfaces have corresponding mock implementations
type MockClient struct {
    GetIssueFunc     func(string) (*Issue, error)
    AuthenticateFunc func() error
    
    // Call tracking
    GetIssueCalls     []string
    AuthenticateCalls int
}

type MockFileWriter struct {
    WriteIssueToYAMLFunc func(*client.Issue, string) (string, error)
    
    // State tracking
    WrittenFiles map[string]*client.Issue
}
```

## Performance Architecture

### Performance Requirements by Component
```go
// Configuration: < 10ms
func (l *DotEnvLoader) Load() (*Config, error)

// JIRA Client: < 5s per API call
func (c *JIRAClient) GetIssue(key string) (*Issue, error)

// File Operations: < 100ms
func (w *YAMLFileWriter) WriteIssueToYAML(issue, path) (string, error)

// Git Operations: < 100ms per operation
func (g *GitRepository) CommitIssueFile(repo, file, issue) error
```

### Optimization Strategies
1. **HTTP Connection Reuse**: Single HTTP client instance
2. **Git Repository Caching**: Avoid repeated repository opens
3. **YAML Streaming**: Direct marshal to file for large issues
4. **Path Caching**: Cache resolved absolute paths

## Security Architecture (Enterprise-Grade v0.4.1+)

### Multi-Layer Security Model
```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           Enterprise Security Perimeter                     │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐       │
│  │   Input     │  │    RBAC     │  │Credentials  │  │ Pod/Runtime │       │
│  │ Validation  │  │ Least Priv  │  │ Protection  │  │  Security   │       │
│  │ (15+ Attack │  │ (Jobs/Pods) │  │ (K8s Secrets│  │ (Standards) │       │
│  │ Scenarios)  │  │             │  │  Only)      │  │             │       │
│  └─────────────┘  └─────────────┘  └─────────────┘  └─────────────┘       │
│                                                                             │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐       │
│  │  Protocol   │  │ CRD Schema  │  │File System  │  │ Audit/Log   │       │
│  │ Enforcement │  │ Validation  │  │ Boundaries  │  │ Security    │       │
│  │(HTTPS Only) │  │(Structural) │  │(Path Valid) │  │(No Secrets) │       │
│  └─────────────┘  └─────────────┘  └─────────────┘  └─────────────┘       │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Security Controls (JCG-028 Implementation)

#### Layer 1: Input Validation & Attack Prevention
- **15+ Attack Scenarios Protected**: SQL injection, XSS, command injection, directory traversal
- **Protocol Enforcement**: HTTPS/Git only, blocks file://, ftp://, data: URIs
- **Length Limits**: DoS protection via input size validation
- **Pattern Validation**: Regex-based input sanitization for all user inputs

#### Layer 2: RBAC & Kubernetes Security
- **Minimal Permissions**: ServiceAccount with job management only (principle of least privilege)
- **ClusterRole Scope**: Jobs (full), Pods/Events (read-only), no secret access
- **Pod Security Standards**: Non-root containers, read-only filesystems, dropped capabilities
- **Network Security**: HTTPS-only communications, network policies

#### Layer 3: Credential & Runtime Protection
- **Kubernetes Secrets**: Encrypted at rest, namespace isolation
- **No Credential Logging**: Sensitive data excluded from all logs and error messages
- **Runtime Hardening**: Security contexts, resource limits, privilege escalation prevention

#### Layer 4: Validation & Testing
- **CRD Structural Validation**: Kubernetes-native input validation at API level
- **Security Test Suite**: Automated validation of 15+ attack scenarios (all must be rejected)
- **Continuous Validation**: Security tests integrated into CI/CD pipeline

### Security Test Framework
```bash
# Enterprise security validation
kubectl apply -f crds/v1alpha1/tests/security/    # All should be REJECTED
kubectl auth can-i create jobs --as=system:serviceaccount:jira-sync-v040:jira-sync-api  # Should be "yes"
kubectl auth can-i create secrets --as=system:serviceaccount:jira-sync-v040:jira-sync-api  # Should be "no"
```

## Deployment Architecture

### Single Binary Deployment
```
jira-sync (single Go binary)
├── Embedded build metadata
├── Version information
├── All dependencies included
└── Cross-platform support (Linux, macOS, Windows)
```

### Configuration Management
```
Environment Variables (highest priority)
├── JIRA_BASE_URL
├── JIRA_EMAIL
├── JIRA_PAT
├── LOG_LEVEL
└── LOG_FORMAT

.env File (fallback)
├── Same variables as above
├── Local development
└── CI/CD environments
```

### Runtime Dependencies
```
System Requirements:
├── Git (optional - go-git provides pure Go implementation)
├── Network access to JIRA instance
├── File system write permissions
└── Go runtime (embedded in binary)

External Services:
├── JIRA REST API v2
├── Git repository (local or remote)
└── File system storage
```

## Extension Architecture

### Future Extension Points
```go
// Multiple JIRA instances
type MultiClientProvider interface {
    GetClient(baseURL string) (Client, error)
}

// Multiple file formats
type FormatWriter interface {
    WriteIssue(issue *Issue, format string, path string) error
}

// Remote Git operations
type RemoteRepository interface {
    Repository
    Push(remote, branch string) error
    Pull(remote, branch string) error
}

// Webhook integration
type WebhookHandler interface {
    HandleIssueUpdate(issue *Issue) error
    RegisterWebhook(url string) error
}
```

### Plugin Architecture (Future)
```go
type Plugin interface {
    Name() string
    Version() string
    Initialize(config map[string]interface{}) error
    ProcessIssue(issue *Issue) (*Issue, error)
}

type PluginManager interface {
    LoadPlugin(path string) (Plugin, error)
    RegisterPlugin(plugin Plugin) error
    ProcessIssueWithPlugins(issue *Issue) (*Issue, error)
}
```

## Quality Gates

### Architecture Validation
1. **Interface Compliance**: All implementations satisfy interfaces
2. **Dependency Direction**: No circular dependencies
3. **Layer Isolation**: Domain layer has no external dependencies
4. **Error Handling**: All errors properly typed and handled
5. **Testing Coverage**: >90% unit test coverage, >80% integration coverage

### Performance Gates
1. **Startup Time**: CLI startup < 100ms
2. **Sync Time**: Complete workflow < 30s
3. **Memory Usage**: Peak memory < 50MB
4. **File I/O**: All file operations < 100ms

### Security Gates
1. **Input Validation**: All inputs validated
2. **Credential Safety**: No credential logging
3. **Path Safety**: No directory traversal vulnerabilities
4. **Transport Security**: HTTPS only for external communication