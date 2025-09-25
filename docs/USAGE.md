# JIRA CDC Git Sync - Usage Guide

## Overview

JIRA CDC Git Sync is a command-line tool that synchronizes JIRA issues into Git repositories as YAML files. It provides a GitOps-friendly approach to tracking JIRA issues alongside your code.

## Quick Start

### 1. Setup Configuration

Copy the example configuration and fill in your JIRA details:

```bash
cp .env.example .env
```

Edit `.env` with your JIRA instance details:

```bash
# Required JIRA Configuration
JIRA_BASE_URL=https://your-company.atlassian.net
JIRA_EMAIL=your-email@company.com
JIRA_PAT=your-personal-access-token

# Optional Application Configuration
LOG_LEVEL=info
LOG_FORMAT=text
```

### 2. Get Your JIRA Personal Access Token

1. Log in to your JIRA instance
2. Go to **Profile → Personal Access Tokens**
3. Click **"Create token"**
4. Give it a name like `jira-sync-tool`
5. Copy the generated token
6. Paste it in your `.env` file as `JIRA_PAT`

### 3. Build the Tool

```bash
make build
```

### 4. Sync Your First Issue

```bash
./build/jira-sync sync --issues=PROJ-123 --repo=/path/to/your/repo
```

## Command Reference

### Basic Sync Command

```bash
jira-sync sync --issues=<ISSUE-KEY> --repo=<REPOSITORY-PATH>
```

**Required Flags:**
- `--issues, -i`: JIRA issue key(s) - single issue or comma-separated list (e.g., `PROJ-123`, `PROJ-1,PROJ-2,PROJ-3`)
- `--repo, -r`: Target Git repository path (absolute or relative)

**Global Flags:**
- `--log-level, -l`: Log level (`debug`, `info`, `warn`, `error`) - default: `info`
- `--log-format`: Log format (`text`, `json`) - default: `text`

### Examples

```bash
# Sync a single issue to a local repository
./build/jira-sync sync --issues=PROJ-123 --repo=./my-project

# Sync multiple issues at once
./build/jira-sync sync --issues=PROJ-1,PROJ-2,PROJ-3 --repo=./my-project

# Sync with debug logging
./build/jira-sync sync --issues=PROJ-123 --repo=./my-project --log-level=debug

# Sync with JSON logging for production
./build/jira-sync sync --issues=PROJ-123 --repo=./my-project --log-format=json

# Get help
./build/jira-sync --help
./build/jira-sync sync --help
```

## Batch Operations (v0.2.0)

### Sync Multiple Issues

Sync multiple issues by providing a comma-separated list:

```bash
# Sync multiple specific issues
./build/jira-sync sync --issues=PROJ-123,PROJ-456,PROJ-789 --repo=./my-project

# With custom rate limiting (slower for busy JIRA instances)
./build/jira-sync sync --issues=PROJ-1,PROJ-2,PROJ-3 --repo=./my-project --rate-limit=500ms

# With increased concurrency (faster processing)
./build/jira-sync sync --issues=PROJ-1,PROJ-2,PROJ-3 --repo=./my-project --concurrency=8
```

## Incremental Sync Operations (v0.3.0)

### State-Based Sync

Only sync issues that have changed since the last sync:

```bash
# Incremental sync (only changed issues)
./build/jira-sync sync --jql="project = PROJ" --repo=./my-project --incremental

# Force full sync (ignore state, sync all issues)
./build/jira-sync sync --issues=PROJ-1,PROJ-2 --repo=./my-project --force

# Dry run to preview what would be synced
./build/jira-sync sync --jql="Epic Link = PROJ-123" --repo=./my-project --dry-run

# Combine incremental with other flags
./build/jira-sync sync --jql="project = PROJ" --repo=./my-project --incremental --rate-limit=200ms
```

### State Management

The tool automatically tracks sync history and provides state management:

```bash
# View current sync state
./build/jira-sync status --repo=./my-project

# Reset sync state (force full sync on next run)
./build/jira-sync reset-state --repo=./my-project

# Backup current state
./build/jira-sync backup-state --repo=./my-project --file=backup.yaml
```

### JQL Query Sync

Sync issues using JIRA Query Language (JQL) for flexible targeting:

```bash
# Sync all issues in a project with specific status
./build/jira-sync sync --jql="project = PROJ AND status = 'To Do'" --repo=./my-project

# Sync all issues in an epic
./build/jira-sync sync --jql="Epic Link = PROJ-123" --repo=./my-project

# Sync issues assigned to current user
./build/jira-sync sync --jql="assignee = currentUser()" --repo=./my-project

# Sync recently updated issues
./build/jira-sync sync --jql="updated >= -7d AND project = PROJ" --repo=./my-project
```

## Smart JQL Capabilities (v0.2.0+)

### EPIC-Focused Sync

The system provides intelligent EPIC discovery and smart JQL generation:

```bash
# Simple EPIC sync (auto-generates comprehensive JQL)
./build/jira-sync sync --epic=RHOAIENG-123 --repo=./my-project

# Preview what issues would be synced before execution
./build/jira-sync sync --epic=RHOAIENG-123 --preview

# EPIC sync with custom parameters
./build/jira-sync sync --epic=RHOAIENG-123 --repo=./my-project --include-subtasks --include-linked
```

### Template-Based Queries

Use built-in templates for common sync patterns:

```bash
# Sync all EPIC issues using template
./build/jira-sync sync --template=epic-all-issues --param=epic_key:RHOAIENG-123 --repo=./my-project

# Sync only EPIC stories
./build/jira-sync sync --template=epic-stories-only --param=epic_key:RHOAIENG-123 --repo=./my-project

# Sync active issues in a project
./build/jira-sync sync --template=project-active-issues --param=project_key:RHOAIENG --repo=./my-project

# Sync current sprint issues
./build/jira-sync sync --template=my-current-sprint --repo=./my-project

# Sync recent updates
./build/jira-sync sync --template=recent-updates --param=project_key:RHOAIENG --param=days:14 --repo=./my-project
```

### Query Validation and Preview

Validate and preview JQL queries before execution:

```bash
# Validate JQL syntax
./build/jira-sync validate --jql="project = RHOAIENG AND status = 'To Do'"

# Preview query results (shows counts, breakdowns, execution time)
./build/jira-sync preview --jql="Epic Link = RHOAIENG-123"

# Preview with detailed breakdown
./build/jira-sync preview --jql="project = RHOAIENG" --detailed
```

### Saved Query Management

Save and reuse complex queries:

```bash
# Save a query for reuse
./build/jira-sync save-query --name="my-epic-sync" --jql="Epic Link = RHOAIENG-123" --description="My EPIC sync pattern"

# List saved queries
./build/jira-sync list-queries

# Use saved query
./build/jira-sync sync --saved-query="my-epic-sync" --repo=./my-project

# Export/import queries
./build/jira-sync export-queries --file=my-queries.json
./build/jira-sync import-queries --file=my-queries.json
```

### Performance Tuning

Adjust sync performance based on your JIRA instance capacity:

```bash
# Conservative (for busy/slow JIRA instances)
./build/jira-sync sync --jql="project = PROJ" --repo=./my-project --concurrency=2 --rate-limit=1s

# Default (recommended for most instances)
./build/jira-sync sync --jql="project = PROJ" --repo=./my-project --concurrency=5 --rate-limit=100ms

# Aggressive (for dedicated/fast JIRA instances)
./build/jira-sync sync --jql="project = PROJ" --repo=./my-project --concurrency=10 --rate-limit=50ms
```

### Rate Limiting Configuration

The `--rate-limit` flag controls the delay between API calls:

- **100ms** (default): Good for most JIRA instances
- **500ms-1s**: Better for busy or slow JIRA instances  
- **50ms**: Only for dedicated or very fast JIRA instances
- **2s+**: For heavily loaded instances or when being very conservative

### Concurrency Configuration

The `--concurrency` flag controls parallel workers:

- **2-3**: Conservative, good for busy JIRA instances
- **5** (default): Balanced performance for most scenarios
- **8-10**: Aggressive, only for dedicated JIRA instances

## How It Works

The sync process follows these steps:

1. **🔧 Load Configuration** - Reads `.env` file and validates settings
2. **🔗 Connect to JIRA** - Authenticates using your credentials
3. **📋 Fetch Issue** - Retrieves the specified JIRA issue
4. **📁 Prepare Repository** - Initializes Git repository if needed
5. **📝 Write YAML File** - Creates structured YAML file with issue data
6. **💾 Commit to Git** - Commits with conventional commit message

## File Structure

Issues are organized in a structured directory layout with relationship mapping (v0.2.0):

```
your-repo/
└── projects/
    └── {PROJECT-KEY}/
        ├── issues/
        │   ├── {PROJECT-KEY}-123.yaml
        │   ├── {PROJECT-KEY}-456.yaml
        │   └── ...
        └── relationships/
            ├── epic_links/
            │   └── {epic-key} -> ../../issues/{story-key}.yaml
            ├── subtasks/
            │   └── {parent-key} -> ../../issues/{subtask-key}.yaml
            └── issue_links/
                ├── blocks/
                │   └── {blocker-key} -> ../../../issues/{blocked-key}.yaml
                └── clones/
                    └── {original-key} -> ../../../issues/{clone-key}.yaml
```

**Example with Relationships:**
```
my-repo/
└── projects/
    └── RHOAIENG/
        ├── issues/
        │   ├── RHOAIENG-123.yaml    # Epic
        │   ├── RHOAIENG-456.yaml    # Story in Epic 123
        │   ├── RHOAIENG-789.yaml    # Subtask of Story 456
        │   └── RHOAIENG-999.yaml    # Bug that blocks Story 456
        └── relationships/
            ├── epic_links/
            │   └── RHOAIENG-123 -> ../../issues/RHOAIENG-456.yaml
            ├── subtasks/
            │   └── RHOAIENG-456 -> ../../issues/RHOAIENG-789.yaml
            └── issue_links/
                └── blocks/
                    └── RHOAIENG-999 -> ../../../issues/RHOAIENG-456.yaml
```

### Relationship Types

The system creates symbolic links for these JIRA relationship types:

- **Epic Links**: Stories linked to epics via "Epic Link" field
- **Subtasks**: Parent-child task relationships 
- **Issue Links**: Blocks, clones, duplicates, and other custom link types
- **Story-Epic**: Reverse epic relationships for navigation

## YAML File Format

Each issue is stored as a YAML file with the following structure:

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
issue_type: Bug
created: "2024-01-15T10:30:00Z"
updated: "2024-01-16T14:20:00Z"
```

## Git Integration

### Repository Initialization

If the target path is not a Git repository, the tool will:
- Initialize a new Git repository with `git init`
- Set up the initial structure
- Proceed with the sync

### Commit Format

The tool uses conventional commit format with JIRA issue metadata:

```
feat(PROJ): add issue PROJ-123 - Fix authentication bug

Issue Details:
- Type: Bug
- Status: In Progress
- Priority: High
- Assignee: John Doe <john.doe@company.com>
- Reporter: Jane Smith <jane.smith@company.com>
- Created: 2024-01-15T10:30:00Z
- Updated: 2024-01-16T14:20:00Z

🤖 Generated with [Claude Code](https://claude.ai/code)

Co-Authored-By: Claude <noreply@anthropic.com>
```

### Working Tree Validation

The tool validates that the Git repository has no uncommitted changes before proceeding. If there are uncommitted changes, the sync will fail with a clear error message.

## Error Handling

### Common Issues and Solutions

**Configuration Errors:**
```bash
Error: failed to load configuration: JIRA_BASE_URL is required
```
- **Solution**: Check your `.env` file and ensure all required fields are filled

**Authentication Errors:**
```bash
Error: failed to authenticate with JIRA: 401 Unauthorized
```
- **Solution**: Verify your JIRA_EMAIL and JIRA_PAT are correct

**Issue Not Found:**
```bash
Error: failed to fetch JIRA issue: 404 Not Found
```
- **Solution**: Verify the issue key exists and you have permission to view it

**Dirty Repository:**
```bash
Error: Git repository validation failed: repository has uncommitted changes
```
- **Solution**: Commit or stash your changes before running the sync

**Invalid Issue Key Format:**
```bash
Error: invalid issue key: issue key 'invalid' does not match JIRA format
```
- **Solution**: Use proper JIRA issue key format (e.g., `PROJ-123`)

## Performance

- **Single Issue Sync**: < 1 second (typical)
- **Network Dependent**: Performance varies based on JIRA instance response time
- **Local Operations**: Git operations are optimized for speed

## Security Considerations

### Credential Protection

- ✅ **Store credentials in `.env` file** (gitignored by default)
- ✅ **Use Personal Access Tokens** instead of passwords
- ❌ **Never commit `.env` file** to version control
- ❌ **Never hardcode credentials** in scripts

### Network Security

- ✅ **HTTPS only** - Tool requires HTTPS for JIRA connections
- ✅ **Token-based auth** - Uses modern authentication methods
- ✅ **Input validation** - All inputs are validated before processing

## Troubleshooting

### Debug Mode

For detailed troubleshooting, use debug logging:

```bash
./build/jira-sync sync --issues=PROJ-123 --repo=./repo --log-level=debug
```

### Verify Configuration

Test your configuration without syncing:

```bash
# This will validate config and auth but fail at issue fetching
./build/jira-sync sync --issues=INVALID-999 --repo=/tmp/test
```

### Check File Permissions

Ensure the tool has write access to your target repository:

```bash
ls -la /path/to/your/repo
```

### Validate JIRA Access

Test direct access to your JIRA instance:

```bash
curl -H "Authorization: Bearer YOUR_PAT" \
     -H "Accept: application/json" \
     "https://your-jira.atlassian.net/rest/api/2/issue/PROJ-123"
```

## Integration with CI/CD

### GitOps Workflow

```bash
# Example CI script
#!/bin/bash
set -e

# Sync critical issues (single or batch)
./build/jira-sync sync --issues=PROJ-123,PROJ-456 --repo=./docs/issues

# Commit and push
git add docs/issues/
git commit -m "docs: update JIRA issue sync"
git push origin main
```

### Environment Variables

For CI/CD environments, you can set configuration via environment variables instead of `.env` files:

```bash
export JIRA_BASE_URL=https://company.atlassian.net
export JIRA_EMAIL=automation@company.com
export JIRA_PAT=$JIRA_PAT_SECRET
export LOG_FORMAT=json

./build/jira-sync sync --issues=PROJ-123 --repo=./repo
```

## Development and Testing

### Running Tests

```bash
# Full test suite
make test

# End-to-end tests (requires .env file)
go test ./test -v -run TestEndToEnd

# Skip E2E tests
CI=true go test ./test -v
```

### Building from Source

```bash
# Build binary
make build

# Build with custom version
make build VERSION=v1.0.0

# Development build
go build -o jira-sync ./cmd/jira-sync
```

## Support

For issues, questions, or contributions:

1. Check this usage guide first
2. Review error messages carefully
3. Try debug mode for detailed logs
4. Check your JIRA permissions
5. Verify network connectivity

## Version Compatibility

- **Go Version**: 1.21+
- **JIRA API**: REST API v2 (most JIRA instances)
- **Git Version**: Any modern Git (2.0+)
- **Operating System**: Linux, macOS, Windows