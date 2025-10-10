# API Documentation

## Overview

The JIRA CDC Git Sync API provides programmatic access to sync operations with comprehensive status tracking and Kubernetes integration. The API supports both traditional job-based operations (v0.4.0) and enhanced CRD-based operations (v0.4.1+) with real-time status management.

## API Versions

### v0.4.0 - Job-Based API
Direct job creation and management through the API server.

### v0.4.1+ - Enhanced CRD-Based API
CRD creation with Kubernetes operator management and comprehensive status tracking.

## Base URL

```
http://<api-server-host>:<port>/api/v1
```

## Authentication

The API supports multiple authentication methods:

- **Bearer Token**: `Authorization: Bearer <token>`
- **API Key**: `X-API-Key: <api-key>`
- **Basic Auth**: `Authorization: Basic <encoded-credentials>`

## Enhanced Sync Operations (v0.4.1+)

### Single Issue Sync

**Endpoint**: `POST /api/v1/sync/single/enhanced`

Creates a JIRASync CRD resource for syncing a single issue with comprehensive status tracking.

**Request Body**:
```json
{
  "issue_key": "PROJ-123",
  "repository": "https://github.com/example/repo.git",
  "options": {
    "incremental": true,
    "force": false,
    "include_links": true
  },
  "safe_mode": true,
  "async": true
}
```

**Response** (202 Accepted):
```json
{
  "status": "success",
  "message": "CRD sync operation initiated",
  "data": {
    "job_id": "sync-proj-123-20240115",
    "status": "pending",
    "crd_name": "sync-proj-123-20240115",
    "crd_namespace": "default",
    "mode": "crd",
    "conversion_info": {
      "original_request_type": "SingleSyncRequest",
      "crd_fields": {
        "syncType": "single",
        "target.issueKeys": "[\"PROJ-123\"]",
        "destination.repository": "https://github.com/example/repo.git"
      },
      "annotations": {
        "jira-sync.io/original-request": "single",
        "jira-sync.io/api-version": "v1"
      }
    }
  }
}
```

### Batch Issue Sync

**Endpoint**: `POST /api/v1/sync/batch/enhanced`

Creates a JIRASync CRD resource for syncing multiple issues.

**Request Body**:
```json
{
  "issue_keys": ["PROJ-100", "PROJ-101", "PROJ-102"],
  "repository": "git@github.com:example/batch.git",
  "options": {
    "concurrency": 2,
    "force": true
  },
  "parallelism": 2,
  "safe_mode": false,
  "async": true
}
```

### JQL Query Sync

**Endpoint**: `POST /api/v1/sync/jql/enhanced`

Creates a JIRASync CRD resource for syncing issues matching a JQL query.

**Request Body**:
```json
{
  "jql": "project = PROJ AND status = 'In Progress'",
  "repository": "https://github.com/example/jql.git",
  "options": {
    "incremental": true,
    "include_links": true
  },
  "parallelism": 3,
  "safe_mode": true,
  "async": true
}
```

## Status Tracking and Monitoring

### Get Sync Status

**Endpoint**: `GET /api/v1/sync/{job_id}/status`

Retrieves comprehensive status information for a sync operation.

**Response**:
```json
{
  "status": "success",
  "data": {
    "job_id": "sync-proj-123-20240115",
    "phase": "Running",
    "observed_generation": 1,
    "progress": {
      "percentage": 75,
      "current_operation": "sync-issues",
      "total_operations": 4,
      "completed_operations": 3,
      "estimated_completion": "2024-01-15T10:30:00Z"
    },
    "sync_state": {
      "start_time": "2024-01-15T10:00:00Z",
      "total_issues": 100,
      "processed_issues": 75,
      "successful_issues": 73,
      "failed_issues": 2,
      "last_sync_time": "2024-01-15T10:25:00Z",
      "config_hash": "abc123def456"
    },
    "conditions": [
      {
        "type": "Ready",
        "status": "False",
        "reason": "SyncInProgress",
        "message": "Sync operation in progress (75% complete)",
        "last_transition_time": "2024-01-15T10:25:00Z"
      },
      {
        "type": "Processing",
        "status": "True",
        "reason": "SyncActive",
        "message": "Processing issues 76-100",
        "last_transition_time": "2024-01-15T10:20:00Z"
      }
    ],
    "last_error": "Rate limit exceeded, retrying in 30s",
    "retry_count": 2,
    "last_status_update": "2024-01-15T10:25:30Z"
  }
}
```

### List Sync Operations

**Endpoint**: `GET /api/v1/sync`

Lists all sync operations with status information.

**Query Parameters**:
- `phase`: Filter by phase (pending, running, completed, failed)
- `limit`: Maximum number of results (default: 50)
- `continue`: Pagination token

**Response**:
```json
{
  "status": "success",
  "data": {
    "items": [
      {
        "job_id": "sync-proj-123-20240115",
        "phase": "Running",
        "progress": {
          "percentage": 75
        },
        "created": "2024-01-15T10:00:00Z",
        "last_updated": "2024-01-15T10:25:30Z"
      }
    ],
    "total": 1,
    "continue": null
  }
}
```

## Health Status Monitoring

### Health Check

**Endpoint**: `GET /api/v1/health`

Returns API server health status including operator connectivity.

**Response**:
```json
{
  "status": "healthy",
  "timestamp": "2024-01-15T10:30:00Z",
  "version": "v0.4.1",
  "components": {
    "api_server": "healthy",
    "kubernetes_operator": "healthy",
    "crd_support": "available"
  },
  "metrics": {
    "active_syncs": 5,
    "completed_syncs_24h": 150,
    "average_sync_duration": "2m30s"
  }
}
```

### System Information

**Endpoint**: `GET /api/v1/system/info`

Returns comprehensive system information.

**Response**:
```json
{
  "version": "v0.4.1",
  "commit": "abc123def456",
  "build_time": "2024-01-15T08:00:00Z",
  "go_version": "go1.21.5",
  "capabilities": {
    "crd_support": true,
    "operator_integration": true,
    "status_management": true,
    "progress_tracking": true
  },
  "kubernetes": {
    "version": "v1.28.2",
    "operator_version": "v0.4.1",
    "crd_version": "v1alpha1"
  }
}
```

## Enhanced Response Format

All enhanced API endpoints return responses in a standardized format:

```json
{
  "status": "success|error",
  "message": "Human readable message",
  "data": {
    // Response data specific to endpoint
  },
  "metadata": {
    "request_id": "req-123456",
    "timestamp": "2024-01-15T10:30:00Z",
    "api_version": "v1",
    "server_version": "v0.4.1"
  }
}
```

## Error Handling

Enhanced error responses include detailed status information:

```json
{
  "status": "error",
  "message": "Sync operation failed",
  "error": {
    "code": "SYNC_FAILED",
    "details": "JIRA authentication failed",
    "field": "credentials",
    "retry_after": 300
  },
  "metadata": {
    "request_id": "req-123456",
    "timestamp": "2024-01-15T10:30:00Z"
  }
}
```

### Common Error Codes

- `VALIDATION_ERROR`: Request validation failed
- `AUTHENTICATION_FAILED`: JIRA authentication failed
- `AUTHORIZATION_DENIED`: Insufficient permissions
- `RESOURCE_NOT_FOUND`: Requested resource not found
- `RATE_LIMIT_EXCEEDED`: API rate limit exceeded
- `SYNC_FAILED`: Sync operation failed
- `CRD_CREATION_FAILED`: CRD creation failed
- `OPERATOR_UNAVAILABLE`: Kubernetes operator unavailable

## Integration with Kubernetes

### CRD Resource Management

The enhanced API integrates with Kubernetes CRDs for:

- **Declarative Management**: Resources managed as Kubernetes objects
- **Status Reporting**: Real-time status through CRD status subresource
- **Event Emission**: Kubernetes events for status changes
- **Condition Management**: Standard Kubernetes condition patterns

### Accessing CRD Resources

After creating a sync operation through the API, you can access the underlying CRD:

```bash
# Get the CRD resource
kubectl get jirasync sync-proj-123-20240115 -o yaml

# Monitor status changes
kubectl get jirasync sync-proj-123-20240115 -w

# Check conditions
kubectl describe jirasync sync-proj-123-20240115
```

## Migration from v0.4.0 to v0.4.1+

### Backward Compatibility

The enhanced API maintains full backward compatibility:

- All v0.4.0 endpoints continue to work
- Response formats are preserved
- Existing clients require no changes

### New Features in v0.4.1+

- **Enhanced Status Tracking**: Detailed progress and condition reporting
- **CRD Integration**: Kubernetes-native resource management
- **Operator Management**: Automated lifecycle management
- **Event Monitoring**: Real-time status change notifications
- **Health Monitoring**: Comprehensive health status calculation

### Migration Benefits

- **Better Observability**: Detailed status and progress tracking
- **Kubernetes Integration**: Native Kubernetes resource management
- **Improved Reliability**: Operator-managed lifecycle and recovery
- **Enhanced Monitoring**: Built-in metrics and event emission

## Rate Limiting

The API implements intelligent rate limiting:

- **Request Rate Limiting**: Maximum requests per minute per client
- **JIRA API Protection**: Automatic JIRA rate limit handling
- **Circuit Breaker**: Automatic failure detection and recovery
- **Backoff Strategies**: Exponential backoff for failed requests

Rate limit headers are included in responses:

```
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 95
X-RateLimit-Reset: 1642238400
```

## Security

### Input Validation

All API endpoints perform comprehensive input validation:

- **Issue Key Validation**: JIRA issue key format validation
- **Repository URL Validation**: Safe repository URL checking
- **JQL Query Validation**: SQL injection prevention
- **Parameter Limits**: DOS protection through input limits

### Protocol Security

- **HTTPS Only**: All communications over TLS
- **No Credential Logging**: Sensitive data excluded from logs
- **Secret Management**: Kubernetes secrets for credential storage

## Client Libraries

### Go Client Example

```go
package main

import (
    "context"
    "github.com/chambrid/jira-cdc-git/internal/operator/apiclient"
)

func main() {
    client := apiclient.NewAPIClient("http://api-server:8080", "bearer-token")
    
    response, err := client.TriggerSingleSync(context.Background(), &apiclient.SingleSyncRequest{
        IssueKey:   "PROJ-123",
        Repository: "https://github.com/example/repo.git",
        Options: &apiclient.SyncOptions{
            Incremental: true,
        },
    })
    
    if err != nil {
        panic(err)
    }
    
    // Monitor progress
    status, err := client.GetJobStatus(context.Background(), response.JobID)
    // ...
}
```

### curl Examples

```bash
# Single issue sync
curl -X POST http://api-server:8080/api/v1/sync/single/enhanced \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer your-token" \
  -d '{
    "issue_key": "PROJ-123",
    "repository": "https://github.com/example/repo.git",
    "options": {"incremental": true},
    "async": true
  }'

# Get status
curl http://api-server:8080/api/v1/sync/sync-proj-123-20240115/status \
  -H "Authorization: Bearer your-token"

# Health check
curl http://api-server:8080/api/v1/health
```