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

## How It Works

The sync process follows these steps:

1. **🔧 Load Configuration** - Reads `.env` file and validates settings
2. **🔗 Connect to JIRA** - Authenticates using your credentials
3. **📋 Fetch Issue** - Retrieves the specified JIRA issue
4. **📁 Prepare Repository** - Initializes Git repository if needed
5. **📝 Write YAML File** - Creates structured YAML file with issue data
6. **💾 Commit to Git** - Commits with conventional commit message

## File Structure

Issues are organized in a structured directory layout:

```
your-repo/
└── projects/
    └── {PROJECT-KEY}/
        └── issues/
            ├── {PROJECT-KEY}-123.yaml
            ├── {PROJECT-KEY}-456.yaml
            └── ...
```

**Example:**
```
my-repo/
└── projects/
    └── RHOAIENG/
        └── issues/
            ├── RHOAIENG-123.yaml
            ├── RHOAIENG-456.yaml
            └── RHOAIENG-789.yaml
```

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