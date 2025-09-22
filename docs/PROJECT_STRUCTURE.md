# Project Structure Design

## Current v0.1.0 Structure
```
.
├── cmd/                    # Application entry points
│   └── jira-sync/         # CLI application (v0.1.0)
│       └── main.go
├── pkg/                   # Public API (future reusable components)
│   ├── client/           # JIRA client interfaces and implementations
│   ├── git/              # Git operations
│   ├── schema/           # YAML schema and data models
│   └── config/           # Configuration management
├── internal/             # Private application code
│   ├── cli/              # CLI-specific logic
│   ├── sync/             # Core sync business logic
│   └── filesystem/       # File operations
├── deployments/          # Kubernetes manifests (future)
│   ├── cli/              # CLI container deployment
│   ├── api/              # API server deployment (v0.3.0)
│   └── operator/         # Operator manifests (v0.4.0)
├── build/                # Build artifacts and scripts
├── docs/                 # Additional documentation
├── test/                 # Integration tests
└── scripts/              # Development and deployment scripts
```

## Future Evolution (v0.3.0+)
```
cmd/
├── jira-sync/            # CLI tool
├── api-server/           # REST API server
├── worker/               # Kubernetes Job worker
└── operator/             # Kubernetes operator

pkg/
├── api/                  # Shared API definitions
├── client/               # JIRA client
├── git/                  # Git operations
├── schema/               # Data models
├── config/               # Configuration
└── metrics/              # Observability

internal/
├── cli/                  # CLI logic
├── server/               # API server logic
├── worker/               # Worker service logic
├── operator/             # Operator logic
└── sync/                 # Shared sync business logic
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