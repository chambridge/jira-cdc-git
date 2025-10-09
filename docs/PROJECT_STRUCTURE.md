# Project Structure Design

## Current v0.4.1 Structure
```
.
├── cmd/                    # Application entry points
│   ├── jira-sync/         # CLI application
│   │   └── main.go
│   ├── api-server/        # REST API server (v0.4.0)
│   │   └── main.go
│   └── operator/          # ✅ Kubernetes operator (v0.4.1)
│       └── main.go
├── pkg/                   # Public API (reusable components)
│   ├── client/           # JIRA client interfaces and implementations
│   ├── git/              # Git operations
│   ├── schema/           # YAML schema and data models
│   ├── config/           # Configuration management
│   ├── jql/              # JQL query building and templates
│   ├── epic/             # EPIC analysis and discovery
│   ├── links/            # Symbolic link management
│   ├── profile/          # Profile management system
│   └── state/            # Sync state management
├── internal/             # Private application code
│   ├── cli/              # CLI-specific logic
│   ├── sync/             # Core sync business logic with batch operations
│   ├── filesystem/       # File operations
│   ├── api/              # API server implementation (v0.4.0)
│   └── operator/         # ✅ Operator implementation (v0.4.1)
│       ├── controllers/  # ✅ JIRASync controller reconciliation
│       └── types/        # ✅ CRD type definitions
├── crds/                 # ✅ Custom Resource Definitions (v0.4.1)
│   └── v1alpha1/         # ✅ JIRASync, JIRAProject, SyncSchedule CRDs
│       └── tests/security/ # ✅ Comprehensive security test cases (15+ attack scenarios)
├── deployments/          # ✅ Kubernetes manifests with enterprise security (v0.4.1)
│   ├── api-server/       # ✅ API server deployment with RBAC and security hardening
│   │   ├── rbac.yaml     # ✅ ServiceAccount, ClusterRole, ClusterRoleBinding
│   │   ├── deployment.yaml # ✅ Security-hardened pod deployment
│   │   └── ...           # ✅ ConfigMaps, Services, Secrets
│   ├── jobs/             # ✅ Kubernetes job templates
│   └── kind-config.yaml  # ✅ Kind cluster configuration for testing
├── build/                # Build artifacts and scripts
├── docs/                 # Additional documentation
│   ├── OPERATOR.md       # ✅ Operator usage guide (v0.4.1)
│   ├── SECURITY.md       # ✅ Comprehensive security documentation (v0.4.1)
│   ├── USAGE.md          # ✅ Complete usage guide
│   └── DEVELOPMENT.md    # ✅ Development workflow and best practices
├── specs/                # Technical interface specifications
├── requirements/         # Product requirements by version
├── releases/             # Version-specific implementation tracking
├── test/                 # Integration tests and performance benchmarks
└── scripts/              # Development and deployment scripts
```

## Remaining Future Evolution (v0.4.2+)
```
cmd/
├── jira-sync/            # ✅ CLI tool with EPIC workflows
├── api-server/           # ✅ REST API server (v0.4.0)
├── worker/               # Kubernetes Job worker (v0.4.2+)
└── operator/             # ✅ Kubernetes operator (v0.4.1)

pkg/
├── api/                  # Shared API definitions (v0.4.2+)
├── client/               # ✅ JIRA client with search capabilities
├── git/                  # ✅ Git operations
├── schema/               # ✅ Data models
├── config/               # ✅ Configuration
├── jql/                  # ✅ JQL query building system
├── epic/                 # ✅ EPIC analysis and discovery
├── links/                # ✅ Symbolic link management
├── sync/                 # ✅ Sync profiles and state management
├── profile/              # ✅ Profile management system
├── state/                # ✅ State management
└── metrics/              # Observability (v0.4.2+)

internal/
├── cli/                  # ✅ Enhanced CLI with EPIC commands
├── server/               # API server logic (v0.4.2+)
├── worker/               # Worker service logic (v0.4.2+)
├── operator/             # ✅ Operator logic (v0.4.1)
└── sync/                 # ✅ Batch sync engine with incremental capabilities
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