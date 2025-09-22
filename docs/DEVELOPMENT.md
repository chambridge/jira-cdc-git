# Development Guide

## Development Workflow

### Available Make Targets

```bash
# Development workflow
make dev          # Full development workflow (deps, lint, test, build, run)
make quick        # Quick build and test
make deps         # Install dependencies
make fmt          # Format code
make lint         # Run linters
make test         # Run all tests with coverage
make build        # Build binary

# Testing
make test-unit           # Run unit tests only
make test-integration    # Run integration tests
make test-coverage       # Generate coverage report
make test-watch          # Watch for changes and run tests

# Building
make build-all           # Build for all platforms
make build-linux         # Build for Linux
make build-darwin        # Build for macOS
make build-windows       # Build for Windows

# CI/CD
make ci-pipeline         # Run full CI pipeline
make validate-all        # Run all validation checks
make pre-commit          # Run pre-commit checks
```

### Architecture Evolution Path

The project is designed to evolve from a CLI tool to a full microservices architecture:

#### Current State (v0.1.0)
- Single CLI application (`cmd/jira-sync/`)
- Shared libraries in `pkg/`
- Clean interfaces for future dependency injection

#### Future Evolution (v0.3.0+)
```
cmd/
├── jira-sync/            # CLI tool (admin interface)
├── api-server/           # REST API server
├── worker/               # Kubernetes Job worker
└── operator/             # Kubernetes operator

deployments/
├── cli/                  # CLI container
├── api/                  # API server deployment
├── worker/               # Worker job templates
└── operator/             # Operator CRDs and deployment
```

### Development Best Practices

1. **Always Working Code**: Every commit should maintain a working system
2. **Test-Driven Development**: Write tests before implementation
3. **Interface-First Design**: Define interfaces before implementations
4. **Dependency Injection**: Use interfaces for major components
5. **Clean Architecture**: Separate concerns between layers

### Testing Strategy

- **Unit Tests**: Test individual components in isolation
- **Integration Tests**: Test component interactions
- **Mock Interfaces**: Use interfaces for easy mocking
- **Coverage**: Maintain >90% test coverage

### Configuration Management

- Development: Use `.env` files
- Production: Environment variables
- Container: ENV vars in Dockerfile
- Kubernetes: ConfigMaps and Secrets