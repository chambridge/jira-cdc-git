# Kubernetes Operator Usage Guide

## Overview

The JIRA CDC Git Sync Kubernetes operator provides declarative management of JIRA sync operations through Custom Resource Definitions (CRDs). The operator enables GitOps-style workflows for JIRA issue synchronization.

## Prerequisites

- Kubernetes cluster (v1.24+)
- kubectl configured for your cluster
- JIRA credentials (base URL, email, Personal Access Token)
- Optional: v0.4.0 API server for enhanced sync operations

## API Server Integration (v0.4.1)

The operator supports integration with the v0.4.0 API server for enhanced sync operations:

### Benefits
- **Enhanced Monitoring**: Centralized job tracking and metrics
- **Circuit Breaker**: Automatic error handling and retry mechanisms  
- **Multiple Auth**: Bearer token, API key, and Basic authentication
- **Health Checks**: Automatic API server availability monitoring

### Configuration
Enable API integration by setting environment variables:

```bash
# API server integration environment variables
API_SERVER_URL=http://api-server:8080
API_AUTH_TYPE=Bearer  # Bearer, APIKey, or Basic
API_AUTH_TOKEN=your-api-token
```

### Deployment with API Integration
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: jira-sync-operator
spec:
  template:
    spec:
      containers:
      - name: operator
        image: jira-sync-operator:latest
        env:
        - name: API_SERVER_URL
          value: "http://api-server:8080"
        - name: API_AUTH_TYPE
          value: "Bearer"
        - name: API_AUTH_TOKEN
          valueFrom:
            secretKeyRef:
              name: api-credentials
              key: token
```

## Quick Start

### 1. Build the Operator

```bash
make build-operator
```

### 2. Install CRDs

```bash
kubectl apply -f crds/v1alpha1/
```

### 3. Create JIRA Credentials Secret

```bash
kubectl create secret generic jira-credentials \
  --from-literal=base-url=https://your-company.atlassian.net \
  --from-literal=email=your-email@company.com \
  --from-literal=pat=your-personal-access-token
```

### 4. Deploy the Operator

```bash
kubectl apply -f - <<EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: jira-sync-operator
spec:
  replicas: 1
  selector:
    matchLabels:
      app: jira-sync-operator
  template:
    metadata:
      labels:
        app: jira-sync-operator
    spec:
      containers:
      - name: operator
        image: jira-sync-operator:latest
        command: ["./build/operator"]
        env:
        - name: LEADER_ELECTION
          value: "true"
EOF
```

## Usage Examples

### Single Issue Sync

```yaml
apiVersion: sync.jira.io/v1alpha1
kind: JIRASync
metadata:
  name: single-issue-sync
spec:
  syncType: "single"
  target:
    issueKeys: ["PROJ-123"]
  destination:
    repository: "https://github.com/company/jira-issues.git"
    branch: "main"
```

### JQL Query Sync

```yaml
apiVersion: sync.jira.io/v1alpha1
kind: JIRASync
metadata:
  name: epic-sync
spec:
  syncType: "jql"
  target:
    jqlQuery: "Epic Link = PROJ-456"
  destination:
    repository: "https://github.com/company/jira-issues.git"
    branch: "main"
  retryPolicy:
    maxRetries: 3
    backoffMultiplier: 2.0
    initialDelay: 5
```

### Incremental Project Sync

```yaml
apiVersion: sync.jira.io/v1alpha1
kind: JIRASync
metadata:
  name: project-sync
spec:
  syncType: "incremental"
  target:
    projectKey: "PROJ"
  destination:
    repository: "https://github.com/company/jira-issues.git"
    branch: "main"
```

## Resource Status

Monitor sync operations:

```bash
# List all sync resources
kubectl get jirasyncs

# Get detailed status
kubectl describe jirasync single-issue-sync

# Watch status changes
kubectl get jirasyncs -w
```

## Resource Phases

- **Pending**: Sync initialized, job creation pending
- **Running**: Kubernetes job executing sync operation  
- **Completed**: Sync finished successfully
- **Failed**: Sync encountered an error

## Troubleshooting

### Check Operator Logs

```bash
kubectl logs -l app=jira-sync-operator
```

### Check Job Status

```bash
kubectl get jobs -l app=jira-sync
kubectl logs job/sync-job-name
```

### Validate CRDs

```bash
kubectl get crds | grep sync.jira.io
kubectl describe crd jirasyncs.sync.jira.io
```

### Common Issues

**Status stays in Pending**:
- Check operator logs for errors
- Verify JIRA credentials secret exists
- Ensure required permissions are granted

**Job failures**:
- Check job logs for specific error details
- Verify JIRA connectivity and permissions
- Validate target repository access

**Resource not found**:
- Ensure CRDs are installed: `kubectl apply -f crds/v1alpha1/`
- Check API versions match your Kubernetes cluster

**API Integration Issues**:
- Check operator logs for API client errors
- Verify API server URL and credentials are correct
- Test API server connectivity: `curl $API_SERVER_URL/health`
- Validate API authentication token permissions

## Testing

Run operator-specific tests:

```bash
make test-operator
```

## Security Configuration

### RBAC Deployment

The operator requires specific RBAC permissions for secure operation. Deploy the complete RBAC configuration before starting the operator:

```bash
# Apply RBAC configuration (ServiceAccount, ClusterRole, ClusterRoleBinding)
kubectl apply -f deployments/api-server/rbac.yaml

# Verify ServiceAccount creation
kubectl get serviceaccount jira-sync-api -n jira-sync-v040

# Verify ClusterRole permissions
kubectl describe clusterrole jira-sync-api

# Verify ClusterRoleBinding
kubectl get clusterrolebinding jira-sync-api
```

### RBAC Permissions Overview

The operator uses **minimal required permissions** following the principle of least privilege:

| Resource | Permissions | Purpose |
|----------|-------------|---------|
| **Jobs** (batch/v1) | `get`, `list`, `watch`, `create`, `update`, `patch`, `delete` | Full job lifecycle management |
| **Pods** (v1) | `get`, `list`, `watch` | Monitor job execution status |
| **Pod Logs** (v1) | `get`, `list` | Debug and troubleshooting |
| **Events** (v1) | `get`, `list`, `watch` | Audit trail and monitoring |

### Security Validation Tests

Validate security protections by running the security test suite:

```bash
# Apply security test cases (all should fail for security reasons)
kubectl apply -f crds/v1alpha1/tests/security/jirasync-security-tests.yaml

# Monitor test results - all should be rejected
kubectl get jirasyncs | grep security-test

# View rejection reasons
kubectl describe jirasync security-test-local-file
```

### Secure Credential Management

Create and manage JIRA credentials securely:

```bash
# Create credentials secret with secure token
kubectl create secret generic jira-credentials \
  --from-literal=base-url=https://your-company.atlassian.net \
  --from-literal=email=your-email@company.com \
  --from-literal=pat=your-personal-access-token \
  --namespace=jira-sync-v040

# Verify secret creation (credentials will be masked)
kubectl describe secret jira-credentials -n jira-sync-v040

# Test credential access from operator pod
kubectl get secret jira-credentials -n jira-sync-v040 -o yaml
```

### Security-Enhanced Deployment

Deploy the operator with security best practices:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: jira-sync-operator
  namespace: jira-sync-v040
spec:
  replicas: 1
  selector:
    matchLabels:
      app: jira-sync-operator
  template:
    metadata:
      labels:
        app: jira-sync-operator
    spec:
      serviceAccountName: jira-sync-api  # Use dedicated ServiceAccount
      securityContext:
        runAsNonRoot: true
        runAsUser: 1000
        fsGroup: 2000
      containers:
      - name: operator
        image: jira-sync-operator:latest
        securityContext:
          allowPrivilegeEscalation: false
          readOnlyRootFilesystem: true
          capabilities:
            drop:
            - ALL
        env:
        - name: LEADER_ELECTION
          value: "true"
        # API integration credentials (if used)
        - name: API_SERVER_URL
          value: "http://api-server:8080"
        - name: API_AUTH_TOKEN
          valueFrom:
            secretKeyRef:
              name: api-credentials
              key: token
```

### Security Troubleshooting

Common security-related issues and solutions:

**RBAC Permission Denied**:
```bash
# Check if RBAC is applied
kubectl get clusterrole jira-sync-api
kubectl get clusterrolebinding jira-sync-api

# Review operator logs for permission errors
kubectl logs -l app=jira-sync-operator | grep -i "forbidden\|unauthorized"
```

**Security Test Failures**:
```bash
# Expected: Security tests should fail (be rejected)
# If security tests succeed, check input validation implementation

# View test resource status
kubectl get jirasyncs -o wide | grep security-test
```

**Credential Issues**:
```bash
# Check secret exists and has correct keys
kubectl get secret jira-credentials -n jira-sync-v040 -o jsonpath='{.data}' | base64 -d

# Test JIRA connectivity from operator pod
kubectl exec -it deploy/jira-sync-operator -- curl -H "Authorization: Bearer $JIRA_PAT" \
  "https://your-company.atlassian.net/rest/api/2/myself"
```

For comprehensive security documentation, see [docs/SECURITY.md](docs/SECURITY.md).

## Configuration

The operator supports these environment variables:
- `LEADER_ELECTION`: Enable leader election (default: false)
- `METRICS_BIND_ADDRESS`: Metrics server address (default: :8080)
- `HEALTH_PROBE_BIND_ADDRESS`: Health probe address (default: :8081)

## Performance

- **Reconciliation**: <100ms for simple resource updates
- **Scale**: Manages 100+ sync resources efficiently
- **Resource Usage**: <50m CPU, <128Mi memory
- **Job Creation**: <200ms to trigger sync operations

## Next Steps

- Configure RBAC permissions for production deployment
- Set up monitoring and alerting for operator health
- Integrate with existing GitOps workflows
- Review security considerations for credential management