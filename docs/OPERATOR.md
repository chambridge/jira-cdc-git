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

## Resource Status and Monitoring

The operator provides comprehensive status reporting for all sync operations with real-time progress tracking and detailed condition management.

### Status Fields

Each JIRASync resource reports detailed status information:

```yaml
status:
  phase: "Running"                    # Current operation phase
  observedGeneration: 1               # Last observed resource generation
  
  # Detailed progress tracking
  progress:
    percentage: 75                    # Completion percentage (0-100)
    currentOperation: "sync-issues"   # Current operation being performed
    totalOperations: 4                # Total operations in sync workflow
    completedOperations: 3            # Number of completed operations
    estimatedCompletion: "2024-01-15T10:30:00Z"  # Estimated completion time
    
  # Sync operation state
  syncState:
    startTime: "2024-01-15T10:00:00Z" # When sync operation started
    totalIssues: 100                  # Total issues to process
    processedIssues: 75               # Issues processed so far
    successfulIssues: 73              # Successfully synced issues
    failedIssues: 2                   # Failed issue syncs
    lastSyncTime: "2024-01-15T10:25:00Z"  # Last successful sync operation
    configHash: "abc123def456"        # Configuration hash for change detection
    
  # Error information (if any)
  lastError: "Rate limit exceeded, retrying in 30s"
  retryCount: 2                       # Current retry attempt
  
  # Kubernetes standard conditions
  conditions:
  - type: "Ready"
    status: "False"
    reason: "SyncInProgress"
    message: "Sync operation in progress (75% complete)"
    lastTransitionTime: "2024-01-15T10:25:00Z"
  - type: "Processing"
    status: "True"
    reason: "SyncActive"
    message: "Processing issues 76-100"
    lastTransitionTime: "2024-01-15T10:20:00Z"
    
  # Timestamps
  lastStatusUpdate: "2024-01-15T10:25:30Z"
```

### Monitoring Commands

Monitor sync operations with detailed status information:

```bash
# List all sync resources with basic status
kubectl get jirasyncs

# Get detailed status with progress information
kubectl describe jirasync single-issue-sync

# Watch status changes in real-time
kubectl get jirasyncs -w

# Get status as YAML for detailed inspection
kubectl get jirasync single-issue-sync -o yaml

# Monitor progress with custom output
kubectl get jirasyncs -o custom-columns=NAME:.metadata.name,PHASE:.status.phase,PROGRESS:.status.progress.percentage,ISSUES:.status.syncState.processedIssues/.status.syncState.totalIssues

# Watch for specific condition changes
kubectl get jirasyncs -w -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.status.conditions[?(@.type=="Ready")].status}{"\n"}{end}'
```

### Health Status Monitoring

The operator calculates health status based on conditions and sync state:

- **Healthy**: Sync operations completing successfully
- **Degraded**: High retry count or intermittent failures  
- **Unhealthy**: Persistent failures or critical errors
- **Unknown**: Insufficient data to determine health

```bash
# Check health status via conditions
kubectl get jirasync -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.status.conditions[?(@.type=="Ready")].status}{"\n"}{end}'
```

## Resource Phases

The operator manages resources through comprehensive lifecycle phases:

- **Pending**: Sync initialized, validation and job creation pending
- **Running**: Kubernetes job actively executing sync operation  
- **Completed**: Sync finished successfully, all issues processed
- **Failed**: Sync encountered unrecoverable error, requires intervention

### Phase Transitions

Resources transition between phases based on operation progress:

```
Pending → Running → Completed
    ↓        ↓
  Failed ← Failed
```

### Condition Types

The operator sets standard Kubernetes conditions:

- **Ready**: Resource is ready and functioning correctly
- **Processing**: Active sync operation in progress
- **Failed**: Sync operation has failed
- **Progressing**: Long-running operation making progress
- **Degraded**: Operation experiencing issues but continuing

## Troubleshooting

### Enhanced Diagnostics with Status Management

The operator provides comprehensive diagnostics through status reporting and observability features.

#### Check Resource Status

```bash
# Get comprehensive status for a specific resource
kubectl describe jirasync <resource-name>

# Check current phase and progress
kubectl get jirasync <resource-name> -o jsonpath='{.status.phase}'
kubectl get jirasync <resource-name> -o jsonpath='{.status.progress.percentage}'

# View detailed sync state
kubectl get jirasync <resource-name> -o jsonpath='{.status.syncState}' | jq

# Check for error conditions
kubectl get jirasync <resource-name> -o jsonpath='{.status.conditions[?(@.type=="Failed")]}' | jq

# Monitor retry attempts
kubectl get jirasync <resource-name> -o jsonpath='{.status.retryCount}'
```

#### Status-Based Troubleshooting

**Resource stuck in Pending**:
1. Check operator logs: `kubectl logs -l app=jira-sync-operator`
2. Verify conditions: `kubectl get jirasync <name> -o jsonpath='{.status.conditions}' | jq`
3. Check validation errors in status message
4. Verify JIRA credentials secret exists

**Processing with No Progress**:
1. Check current operation: `kubectl get jirasync <name> -o jsonpath='{.status.progress.currentOperation}'`
2. Monitor processed vs total issues: `kubectl get jirasync <name> -o jsonpath='{.status.syncState.processedIssues}/{.status.syncState.totalIssues}'`
3. Check for rate limiting: Look for retry messages in `lastError`
4. Verify API server connectivity if using API integration

**High Retry Count**:
1. Check last error: `kubectl get jirasync <name> -o jsonpath='{.status.lastError}'`
2. Review conditions for degraded status
3. Verify network connectivity and credentials
4. Check resource quotas and limits

### Check Operator Logs

```bash
# Get operator logs with status manager context
kubectl logs -l app=jira-sync-operator | grep -E "(status|condition|progress)"

# Follow logs in real-time
kubectl logs -f -l app=jira-sync-operator

# Get logs from specific operator replica
kubectl logs deployment/jira-sync-operator
```

### Check Job Status

```bash
kubectl get jobs -l app=jira-sync
kubectl logs job/sync-job-name

# Check job status through JIRASync resource
kubectl get jirasync <name> -o jsonpath='{.status.syncState.startTime}'
```

### Validate CRDs

```bash
kubectl get crds | grep sync.jira.io
kubectl describe crd jirasyncs.sync.jira.io

# Verify status subresource is enabled
kubectl get crd jirasyncs.sync.jira.io -o jsonpath='{.spec.versions[0].subresources}'
```

### Performance Monitoring

Monitor operator performance using the new status management metrics:

```bash
# Check reconciliation performance
kubectl logs -l app=jira-sync-operator | grep "reconciliation completed"

# Monitor status update frequency
kubectl get events --field-selector involvedObject.kind=JIRASync

# Check for status validation errors
kubectl logs -l app=jira-sync-operator | grep "status validation"
```

### Common Issues

**Status stays in Pending**:
- Check conditions: `kubectl get jirasync <name> -o jsonpath='{.status.conditions[?(@.type=="Ready")]}'`
- Verify operator logs for validation errors
- Ensure JIRA credentials secret exists
- Check RBAC permissions

**Sync Progress Stalled**:
- Monitor progress percentage: `kubectl get jirasync <name> -o jsonpath='{.status.progress.percentage}'`
- Check current operation: `kubectl get jirasync <name> -o jsonpath='{.status.progress.currentOperation}'`
- Review rate limiting in `lastError` field
- Verify API server health (if using API integration)

**Inconsistent Status**:
- Check for status validation warnings in operator logs
- Verify `observedGeneration` matches resource generation
- Review condition transition times for anomalies

**Resource not found**:
- Ensure CRDs are installed: `kubectl apply -f crds/v1alpha1/`
- Check API versions match your Kubernetes cluster
- Verify status subresource is properly configured

**API Integration Issues**:
- Check API client errors in operator logs
- Monitor health status: `curl $API_SERVER_URL/health`
- Verify authentication credentials
- Check circuit breaker status in logs

### Event Monitoring

The operator emits Kubernetes events for status changes:

```bash
# Monitor all operator events
kubectl get events --field-selector involvedObject.kind=JIRASync

# Watch events in real-time
kubectl get events --field-selector involvedObject.kind=JIRASync -w

# Check for specific event types
kubectl get events --field-selector involvedObject.kind=JIRASync,reason=StatusUpdated
```

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