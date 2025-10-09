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
3. **Security by Design**: Implement input validation, RBAC, and attack prevention from the start
4. **Interface-First Design**: Define interfaces before implementations
5. **Dependency Injection**: Use interfaces for major components
6. **Clean Architecture**: Separate concerns between layers

### Security Development Practices

- **Input Validation**: Validate all user inputs against injection attacks (SQL, XSS, command, path traversal)
- **Credential Protection**: Never commit secrets, use Kubernetes secrets, exclude sensitive data from logs
- **RBAC Implementation**: Follow principle of least privilege for all Kubernetes resources
- **Security Testing**: Validate security test cases in `crds/v1alpha1/tests/security/` regularly
- **Protocol Enforcement**: HTTPS-only, block unsafe schemes (file://, ftp://, data:)
- **Reference Documentation**: See [docs/SECURITY.md](SECURITY.md) for comprehensive security guidelines

### Testing Strategy

- **Unit Tests**: Test individual components in isolation
- **Integration Tests**: Test component interactions with comprehensive v0.3.0 end-to-end workflows
- **Performance Tests**: Benchmarking for large datasets (50-1000 issues) with memory usage validation
- **Mock Interfaces**: Use interfaces for easy mocking with thread-safe MockClient implementation
- **Race Detection**: Comprehensive concurrency testing with `make test-race`
- **Coverage**: Maintain >90% test coverage with 400+ tests across all components
- **Always Working Code**: Both `make build` and `make test` must pass reliably

### Test Execution Targets

- **`make test`**: Run all tests without race detection (recommended for development)
- **`make test-race`**: Run core functionality tests with race detection (skips performance tests)
- **`make test-coverage`**: Generate detailed coverage reports

### Configuration Management

- Development: Use `.env` files
- Production: Environment variables
- Container: ENV vars in Dockerfile
- Kubernetes: ConfigMaps and Secrets