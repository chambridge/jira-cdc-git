# JIRA Client Interface Specification

## Overview

The Client interface defines the contract for JIRA API operations in the jira-cdc-git system. This specification is based on SPIKE-001 findings and implements Bearer token authentication for Red Hat JIRA instances.

## Interface Definition

```go
type Client interface {
    GetIssue(issueKey string) (*Issue, error)
    Authenticate() error
}
```

## Data Types

### Issue Structure

```go
type Issue struct {
    Key         string `json:"key" yaml:"key"`
    Summary     string `json:"summary" yaml:"summary"`
    Description string `json:"description" yaml:"description"`
    Status      Status `json:"status" yaml:"status"`
    Assignee    User   `json:"assignee" yaml:"assignee"`
    Reporter    User   `json:"reporter" yaml:"reporter"`
    Created     string `json:"created" yaml:"created"`
    Updated     string `json:"updated" yaml:"updated"`
    Priority    string `json:"priority" yaml:"priority"`
    IssueType   string `json:"issuetype" yaml:"issuetype"`
}
```

### Status Structure

```go
type Status struct {
    Name     string `json:"name" yaml:"name"`
    Category string `json:"category,omitempty" yaml:"category,omitempty"`
}
```

### User Structure

```go
type User struct {
    Name  string `json:"name" yaml:"name"`
    Email string `json:"email,omitempty" yaml:"email,omitempty"`
}
```

## Implementation Requirements

### Authentication
- **Method**: Bearer token authentication using Personal Access Token (PAT)
- **Transport**: Custom HTTP transport with Authorization header
- **Library**: `andygrunwald/go-jira v1.17.0`

### Error Handling
- **Authentication Errors**: Return `AuthenticationError` for 401 responses
- **Authorization Errors**: Return `AuthorizationError` for 403 responses
- **Not Found Errors**: Return `NotFoundError` for 404 responses
- **API Errors**: Return `APIError` with details for other failures

### Field Mapping
Based on SPIKE-001 findings, the Issue struct maps directly to JIRA REST API v2 response format:
- Time fields (`created`, `updated`) converted to ISO 8601 string format
- User fields include both display name and email address
- Status includes both name and category for workflow state

## Implementation Example

```go
type JIRAClient struct {
    client *jira.Client
    config *config.Config
}

func NewClient(cfg *config.Config) (Client, error) {
    // Create HTTP transport with Bearer token
    transport := &bearerTokenTransport{
        Token: cfg.JIRAPAT,
        Base:  http.DefaultTransport,
    }
    
    httpClient := &http.Client{Transport: transport}
    
    jiraClient, err := jira.NewClient(httpClient, cfg.JIRABaseURL)
    if err != nil {
        return nil, fmt.Errorf("failed to create JIRA client: %w", err)
    }
    
    return &JIRAClient{
        client: jiraClient,
        config: cfg,
    }, nil
}
```

## Testing Requirements

### Unit Tests
- Mock implementation using `MockClient`
- Test authentication success and failure scenarios
- Test issue retrieval with valid and invalid keys
- Test error handling for all error types

### Integration Tests
- Test with real JIRA instance (skipped in CI)
- Validate field mapping accuracy
- Performance testing (sub-second response times)

## Usage Examples

```go
// Create client
client, err := client.NewClient(config)
if err != nil {
    return fmt.Errorf("failed to create client: %w", err)
}

// Authenticate
if err := client.Authenticate(); err != nil {
    return fmt.Errorf("authentication failed: %w", err)
}

// Fetch issue
issue, err := client.GetIssue("PROJ-123")
if err != nil {
    return fmt.Errorf("failed to fetch issue: %w", err)
}
```

## Validation Requirements

1. Issue key must match JIRA format: `[A-Z]+-[0-9]+`
2. All required fields must be present in response
3. Time fields must be valid ISO 8601 format
4. User fields must include at minimum display name

## Performance Requirements

- **Response Time**: < 5 seconds per API call
- **Timeout**: 30 seconds for HTTP requests
- **Rate Limiting**: Respect JIRA instance rate limits
- **Connection Reuse**: HTTP client should reuse connections

## Security Requirements

- Bearer tokens must be stored securely (environment variables)
- All connections must use HTTPS
- Credentials must never be logged
- Input validation for all user-provided data