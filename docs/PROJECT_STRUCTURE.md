# Project Structure Design

## Current v0.2.0 Structure
```
.
├── cmd/                    # Application entry points
│   └── jira-sync/         # CLI application (v0.2.0)
│       └── main.go
├── pkg/                   # Public API (reusable components)
│   ├── client/           # JIRA client interfaces and implementations
│   ├── git/              # Git operations
│   ├── schema/           # YAML schema and data models
│   ├── config/           # Configuration management
│   ├── jql/              # JQL query building and templates (v0.2.0+)
│   ├── epic/             # EPIC analysis and discovery (v0.2.0+)
│   └── links/            # Symbolic link management (v0.2.0+)
├── internal/             # Private application code
│   ├── cli/              # CLI-specific logic
│   ├── sync/             # Core sync business logic with batch operations
│   └── filesystem/       # File operations
├── deployments/          # Kubernetes manifests (future)
│   ├── cli/              # CLI container deployment
│   ├── api/              # API server deployment (v0.3.0+)
│   └── operator/         # Operator manifests (v0.4.0+)
├── build/                # Build artifacts and scripts
├── docs/                 # Additional documentation
├── specs/                # Technical interface specifications
├── requirements/         # Product requirements by version
├── releases/             # Version-specific implementation tracking
├── test/                 # Integration tests
└── scripts/              # Development and deployment scripts
```

## Future Evolution (v0.3.0+)
```
cmd/
├── jira-sync/            # CLI tool with EPIC workflows
├── api-server/           # REST API server (v0.4.0)
├── worker/               # Kubernetes Job worker (v0.4.0)
└── operator/             # Kubernetes operator (v0.4.0)

pkg/
├── api/                  # Shared API definitions (v0.4.0)
├── client/               # JIRA client with search capabilities
├── git/                  # Git operations
├── schema/               # Data models
├── config/               # Configuration
├── jql/                  # JQL query building system
├── epic/                 # EPIC analysis and discovery
├── links/                # Symbolic link management
├── sync/                 # Sync profiles and state management (v0.3.0)
└── metrics/              # Observability (v0.4.0)

internal/
├── cli/                  # Enhanced CLI with EPIC commands
├── server/               # API server logic (v0.4.0)
├── worker/               # Worker service logic (v0.4.0)
├── operator/             # Operator logic (v0.4.0)
└── sync/                 # Batch sync engine with incremental capabilities
```

## Design Principles

### Separation of Concerns
- **cmd/**: Application entry points only
- **pkg/**: Reusable, public interfaces
- **internal/**: Private implementation details

### Future Kubernetes Readiness
- Each cmd/ will become a separate container
- pkg/ components are shared libraries
- Deployments structure ready for Helm charts

### Dependency Injection
- Interfaces in pkg/ for major components
- Concrete implementations in internal/
- Easy mocking for tests

### Observability Ready
- Structured logging throughout
- Metrics collection hooks
- Tracing capability built-in