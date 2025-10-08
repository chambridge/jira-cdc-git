# JIRA CDC Git Sync

A Kubernetes-native system for synchronizing JIRA data into Git repositories with real-time change data capture (CDC) capabilities.

## Overview

This system provides automated synchronization of JIRA issues and their relationships into structured Git repositories. Each JIRA issue becomes a file, with relationships maintained through symbolic links, enabling version-controlled tracking of project data and enabling GitOps workflows for project management.

## Key Features

- **Real-time Sync**: Continuous synchronization of JIRA data changes
- **Git-based Storage**: Issues stored as files with relationships as symbolic links
- **Kubernetes Native**: Designed for deployment on Kubernetes with operator support
- **Flexible Sync Targets**: Support for projects, issue lists, and JQL queries
- **Rate Limiting**: Built-in JIRA API rate limiting and performance optimization
- **Web Interface**: Simple UI for managing sync tasks and monitoring
- **Job-based Processing**: Kubernetes Jobs handle sync operations for scalability

## Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   JIRA API      │    │  Sync Backend   │    │  Git Repository │
│                 │◄──►│                 │◄──►│                 │
│ - Issues        │    │ - Rate Limiting │    │ - Issue Files   │
│ - Projects      │    │ - Job Scheduler │    │ - Symbolic Links│
│ - Changes       │    │ - API Server    │    │ - Version History│
└─────────────────┘    └─────────────────┘    └─────────────────┘
                               ▲
                               │
                       ┌─────────────────┐
                       │  Web Interface  │
                       │                 │
                       │ - Sync Tasks    │
                       │ - Monitoring    │
                       │ - Configuration │
                       └─────────────────┘
```

## Data Structure

### Issue Files
Each JIRA issue is stored as a structured file:
```
/projects/{project-key}/issues/{issue-key}.yaml
```

### Relationships
Issue relationships are maintained via symbolic links:
```
/projects/{project-key}/relationships/
├── blocks/
│   └── {issue-key} -> ../../issues/{blocked-issue-key}.yaml
├── subtasks/
│   └── {parent-key}/
│       └── {subtask-key} -> ../../../issues/{subtask-key}.yaml
└── epic/
    └── {epic-key}/
        └── {story-key} -> ../../../issues/{story-key}.yaml
```

## Sync Operations

### Supported Sync Types
- **Project Sync**: All issues within a JIRA project
- **Issue List Sync**: Specific list of issue keys
- **JQL Query Sync**: All issues matching a JQL query
- **Incremental Sync**: Only changed issues since last sync

### Performance Considerations
- Configurable batch sizes for API requests
- Intelligent rate limiting with backoff strategies
- Parallel processing for independent operations
- Git optimization for large repositories

## Development Philosophy

This project follows a fast iterative delivery approach:
- **Always Working Code**: Every commit maintains a working system
- **Test-Driven Development**: Comprehensive test coverage at all levels
- **Incremental Deliverables**: Each component delivers standalone value
- **Documentation First**: Requirements and architecture documented before implementation

## Architecture Design

### Microservices-Ready Foundation
The project is architected for evolution from CLI tool to full Kubernetes-native microservices:

1. **Microservices Ready**:
   - `cmd/jira-sync/` (CLI) → `cmd/api-server/`, `cmd/operator/` ✅ IMPLEMENTED
   - Each command becomes a separate containerized service
   - Shared business logic enables consistent behavior across services

2. **Shared Libraries**:
   - `pkg/` components will be reused across all services
   - Clean interfaces enable different implementations per service
   - Dependency injection ready for testing and service composition

3. **Container/Kubernetes Native**:
   - Makefile includes Docker and K8s targets
   - Configuration via environment variables
   - Structured logging ready for observability platforms

### Project Structure
```
cmd/                    # Application entry points
├── jira-sync/         # CLI application
├── api-server/        # REST API server (v0.4.0)
└── operator/          # ✅ Kubernetes operator (v0.4.1)

pkg/                   # Public, reusable components
├── client/           # JIRA client interfaces and implementations  
├── git/              # Git operations
├── schema/           # YAML schema and data models
└── config/           # Configuration management

internal/             # Private application code
├── cli/              # CLI-specific logic
├── sync/             # Core sync business logic
├── filesystem/       # File operations
└── operator/         # ✅ Operator implementation (v0.4.1)
    ├── controllers/  # ✅ JIRASync controller reconciliation
    └── types/        # ✅ CRD type definitions

crds/                 # ✅ Custom Resource Definitions (v0.4.1)
└── v1alpha1/         # ✅ JIRASync, JIRAProject, SyncSchedule CRDs
```

## Current Release: v0.4.1 (Partial - Operator Implementation)

**Status**: 🚀 IN PROGRESS - Kubernetes Operator and CRD Management  
- ✅ **JCG-025**: Custom Resource Definitions (CRDs) - COMPLETED
- ✅ **JCG-026**: Operator Controller Core Logic - COMPLETED  
- ✅ **JCG-027**: API Server Integration - COMPLETED

### Technology Stack
- **Language**: Go 1.24+
- **Authentication**: JIRA Personal Access Token (PAT) with email
- **Git Operations**: Local repository operations with conventional commits
- **Configuration**: Environment variables via .env file
- **Interface**: Advanced CLI tool with profile management, batch operations, JQL support, and incremental sync
- **State Management**: File-based sync state tracking with YAML/JSON persistence
- **Profile System**: Template-based configuration management with export/import capabilities
- **JQL Integration**: Smart query building with template system and EPIC analysis
- **Kubernetes**: controller-runtime v0.19.1 for operator functionality
- **CRDs**: v1alpha1 API with JIRASync, JIRAProject, SyncSchedule resources
- **Operator**: Production-ready reconciliation with finalizers and retry logic
- **API Integration**: Operator integrates with v0.4.0 API server with circuit breaker patterns
- **Architecture**: Clean interface-based design with implemented Kubernetes operator
- **Testing**: Comprehensive end-to-end testing with performance benchmarking and always-working code validation

### Quick Start (v0.3.0)

1. **Prerequisites**
   - Go 1.24+ installed
   - Git repository initialized
   - JIRA Personal Access Token

2. **Configuration**
   Create `.env` file:
   ```bash
   JIRA_BASE_URL=https://your-domain.atlassian.net
   JIRA_EMAIL=your-email@company.com
   JIRA_PAT=your-personal-access-token
   ```

3. **Build and Usage**
   ```bash
   # Build the CLI tool
   make build
   
   # Sync single JIRA issue to local Git repository
   ./build/jira-sync sync --issues=PROJ-123 --repo=/path/to/repo
   
   # Sync multiple issues with custom rate limiting
   ./build/jira-sync sync --issues=PROJ-1,PROJ-2,PROJ-3 --repo=/path/to/repo --rate-limit=200ms
   
   # Sync issues using JQL query
   ./build/jira-sync sync --jql="project = PROJ AND status = 'To Do'" --repo=/path/to/repo
   
   # Incremental sync (only changed issues since last sync)
   ./build/jira-sync sync --jql="Epic Link = PROJ-123" --repo=/path/to/repo --incremental
   
   # Force full sync (ignore state, sync all issues)
   ./build/jira-sync sync --issues=PROJ-1,PROJ-2 --repo=/path/to/repo --force
   
   # Dry run to preview changes without syncing
   ./build/jira-sync sync --jql="project = PROJ" --repo=/path/to/repo --dry-run
   
   # Create and use profiles for common workflows
   ./build/jira-sync profile create --template=epic-all-issues --name=my-epic --epic_key=PROJ-123 --repository=/path/to/repo
   ./build/jira-sync sync --profile=my-epic --incremental
   
   # Batch sync with custom concurrency
   ./build/jira-sync sync --jql="Epic Link = PROJ-123" --repo=/path/to/repo --concurrency=8
   ```

### Current Capabilities (v0.3.0)
- **Batch Operations**: Sync multiple issues via comma-separated lists or JQL queries
- **Relationship Mapping**: Symbolic links for epic/story, subtasks, and blocks/clones relationships
- **Incremental Sync**: Only sync changed issues since last sync with state management
- **State Tracking**: Persistent sync history and timestamps with YAML/JSON storage
- **Force & Dry Run**: Options for full sync override and preview-only mode
- **Rate Limiting**: Configurable API throttling with --rate-limit flag (e.g., 100ms, 1s, 2s)
- **Parallel Processing**: Configurable concurrency with --concurrency flag (1-10 workers)
- **Progress Reporting**: Real-time feedback for batch operations with sync statistics
- **Enhanced CLI**: Comprehensive help text with usage examples and performance guidelines
- **Local Git Integration**: Conventional commits with proper metadata and issue relationships
- **EPIC Analysis**: Intelligent EPIC discovery and hierarchy mapping with 85%+ test coverage
- **Smart JQL Building**: Template-based query generation with 5 built-in patterns
- **Query Preview**: Show issue counts and execution time before sync operations
- **Profile Management**: Save and reuse sync configurations with templates (epic-all-issues, epic-stories-only, project-active-issues, my-current-sprint, recent-updates)
- **Profile Export/Import**: Share team sync configurations via YAML files
- **State Recovery**: Robust handling of interrupted syncs with validation and backup
- **Comprehensive Testing**: 400+ tests with comprehensive end-to-end workflow validation, performance benchmarking, and thread-safe concurrency testing

### Profile and JQL Capabilities
- **Profile Templates**: 5 built-in templates for common patterns (epic-all-issues, epic-stories-only, project-active-issues, my-current-sprint, recent-updates)
- **Profile Management**: Create, update, delete, list, show, export, and import sync profiles
- **Template Variables**: Dynamic profile creation with variable substitution (epic_key, repository, etc.)
- **Usage Tracking**: Automatic recording of profile usage statistics
- **Team Sharing**: Export/import profiles for team collaboration
- **EPIC Query Building**: Intelligent EPIC discovery and JQL generation
- **Query Validation**: Syntax checking with intelligent suggestions
- **Query Optimization**: Performance improvements for large datasets
- **Query Preview**: Fast preview showing issue counts, project breakdown, and execution time

### Upcoming Releases
- **v0.4.0**: API server and Kubernetes job scheduling infrastructure
- **v0.4.1**: Kubernetes operator and CRD management
- **v0.4.2**: Real-time change detection and webhook integration
- **v0.5.0**: Web interface and management dashboard

## Documentation

### Project Documentation Structure

The project documentation is organized into several directories:

| Directory | Purpose | Contents |
|-----------|---------|----------|
| **[docs/](docs/)** | User and operational documentation | [USAGE.md](docs/USAGE.md), [PROJECT_STRUCTURE.md](docs/PROJECT_STRUCTURE.md), [DEVELOPMENT.md](docs/DEVELOPMENT.md) |
| **[specs/](specs/)** | Technical specifications and interface documentation | Interface specs, system architecture, implementation requirements |
| **[requirements/](requirements/)** | Product requirements and feature specifications | Release requirements, acceptance criteria, feature definitions |
| **[releases/](releases/)** | Version-specific implementation tracking | Task lists, completion status, version documentation |

### Key Documentation Files

#### User Documentation
- **[docs/USAGE.md](docs/USAGE.md)**: Complete usage guide with setup, commands, examples, and troubleshooting
- **[docs/PROJECT_STRUCTURE.md](docs/PROJECT_STRUCTURE.md)**: Project organization and architecture overview
- **[docs/DEVELOPMENT.md](docs/DEVELOPMENT.md)**: Development setup and workflow guide

#### Technical Specifications
- **[specs/SYSTEM_ARCHITECTURE.md](specs/SYSTEM_ARCHITECTURE.md)**: Overall system architecture and design patterns
- **[specs/CLIENT_INTERFACE.md](specs/CLIENT_INTERFACE.md)**: JIRA API client interface and implementation requirements
- **[specs/SCHEMA_INTERFACE.md](specs/SCHEMA_INTERFACE.md)**: File operations and YAML schema specification
- **[specs/GIT_INTERFACE.md](specs/GIT_INTERFACE.md)**: Git operations and repository management specification
- **[specs/CONFIG_INTERFACE.md](specs/CONFIG_INTERFACE.md)**: Configuration management and validation specification
- **[specs/CLI_INTERFACE.md](specs/CLI_INTERFACE.md)**: Command-line interface specification

#### Requirements and Planning
- **[requirements/README.md](requirements/README.md)**: Complete product requirements by release version
- **[releases/v0.1.0/tasks.md](releases/v0.1.0/tasks.md)**: Implementation tasks and completion status for v0.1.0

### Documentation for Developers

For developers working on the project:

1. **Start with**: [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md) for development environment setup
2. **Architecture**: [specs/SYSTEM_ARCHITECTURE.md](specs/SYSTEM_ARCHITECTURE.md) for system design
3. **Interfaces**: [specs/](specs/) directory for detailed interface specifications
4. **Requirements**: [requirements/README.md](requirements/README.md) for feature requirements
5. **Implementation**: [releases/v0.1.0/tasks.md](releases/v0.1.0/tasks.md) for current implementation status

### Documentation for Users

For users of the tool:

1. **Quick Start**: See the "Quick Start (v0.1.0)" section above
2. **Complete Guide**: [docs/USAGE.md](docs/USAGE.md) for comprehensive usage documentation
3. **Configuration**: [docs/USAGE.md#setup-configuration](docs/USAGE.md#setup-configuration) for detailed setup instructions
4. **Troubleshooting**: [docs/USAGE.md#troubleshooting](docs/USAGE.md#troubleshooting) for common issues and solutions

## Contributing

This project uses conventional commits with DCO sign-off requirements. See [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md) for detailed development workflow and contributing guidelines.

### Development Workflow

1. Review [specs/SYSTEM_ARCHITECTURE.md](specs/SYSTEM_ARCHITECTURE.md) for architecture overview
2. Follow [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md) for setup and workflow
3. Check [releases/v0.1.0/tasks.md](releases/v0.1.0/tasks.md) for current implementation status
4. Reference [specs/](specs/) for interface specifications when implementing features

## License

Apache 2.0 - See LICENSE file for details.