# Status Management Guide

## Overview

This guide provides practical examples and usage patterns for the comprehensive status management capabilities introduced in v0.4.1. The status management system provides real-time progress tracking, condition monitoring, and health status calculation for all sync operations.

## Quick Start Examples

### Basic Status Monitoring

```bash
# Create a sync operation
kubectl apply -f - <<EOF
apiVersion: sync.jira.io/v1alpha1
kind: JIRASync
metadata:
  name: example-sync
spec:
  syncType: "single"
  target:
    issueKeys: ["PROJ-123"]
  destination:
    repository: "https://github.com/example/repo.git"
    branch: "main"
EOF

# Monitor status in real-time
kubectl get jirasync example-sync -w

# Get detailed status
kubectl describe jirasync example-sync
```

### Progress Tracking

```bash
# Check completion percentage
kubectl get jirasync example-sync -o jsonpath='{.status.progress.percentage}%'

# Monitor processed vs total issues
kubectl get jirasync example-sync -o jsonpath='{.status.syncState.processedIssues}/{.status.syncState.totalIssues}'

# Check current operation
kubectl get jirasync example-sync -o jsonpath='{.status.progress.currentOperation}'

# Estimated completion time
kubectl get jirasync example-sync -o jsonpath='{.status.progress.estimatedCompletion}'
```

## Status Field Examples

### Complete Status Structure

```yaml
# Example of a JIRASync resource with full status information
apiVersion: sync.jira.io/v1alpha1
kind: JIRASync
metadata:
  name: project-sync-example
  namespace: default
spec:
  syncType: "jql"
  target:
    jqlQuery: "project = PROJ AND status = 'In Progress'"
  destination:
    repository: "https://github.com/company/issues.git"
    branch: "main"
status:
  # Resource lifecycle phase
  phase: "Running"
  observedGeneration: 1
  
  # Detailed progress information
  progress:
    percentage: 65                    # 65% complete
    currentOperation: "sync-issues"   # Currently syncing issues
    totalOperations: 4                # 4 total operations in workflow
    completedOperations: 2            # 2 operations completed
    estimatedCompletion: "2024-01-15T11:45:00Z"
    
  # Sync operation state
  syncState:
    startTime: "2024-01-15T11:00:00Z"
    totalIssues: 150                  # 150 issues found by JQL
    processedIssues: 98               # 98 issues processed so far
    successfulIssues: 96              # 96 successful syncs
    failedIssues: 2                   # 2 failed syncs
    lastSyncTime: "2024-01-15T11:30:00Z"
    configHash: "7f9a8b2c3d4e5f6a"   # Configuration fingerprint
    
  # Error tracking
  lastError: "Rate limit exceeded for 2 issues, retrying"
  retryCount: 1                       # First retry attempt
  
  # Kubernetes conditions
  conditions:
  - type: "Ready"
    status: "False"
    reason: "SyncInProgress"
    message: "Sync operation 65% complete (98/150 issues)"
    lastTransitionTime: "2024-01-15T11:30:00Z"
  - type: "Processing"
    status: "True"
    reason: "SyncActive"
    message: "Processing issues 99-150, handling rate limits"
    lastTransitionTime: "2024-01-15T11:20:00Z"
  - type: "Progressing"
    status: "True"
    reason: "ProgressMade"
    message: "Processed 98 of 150 issues, 52 remaining"
    lastTransitionTime: "2024-01-15T11:30:00Z"
    
  # Timestamps
  lastStatusUpdate: "2024-01-15T11:30:15Z"
```

## Monitoring Scripts and Automation

### Progress Monitoring Script

```bash
#!/bin/bash
# monitor-sync.sh - Monitor sync progress with status updates

SYNC_NAME="$1"
if [ -z "$SYNC_NAME" ]; then
    echo "Usage: $0 <sync-name>"
    exit 1
fi

echo "Monitoring sync: $SYNC_NAME"
echo "=================="

while true; do
    # Get current status
    PHASE=$(kubectl get jirasync "$SYNC_NAME" -o jsonpath='{.status.phase}' 2>/dev/null)
    PERCENTAGE=$(kubectl get jirasync "$SYNC_NAME" -o jsonpath='{.status.progress.percentage}' 2>/dev/null)
    PROCESSED=$(kubectl get jirasync "$SYNC_NAME" -o jsonpath='{.status.syncState.processedIssues}' 2>/dev/null)
    TOTAL=$(kubectl get jirasync "$SYNC_NAME" -o jsonpath='{.status.syncState.totalIssues}' 2>/dev/null)
    OPERATION=$(kubectl get jirasync "$SYNC_NAME" -o jsonpath='{.status.progress.currentOperation}' 2>/dev/null)
    ERROR=$(kubectl get jirasync "$SYNC_NAME" -o jsonpath='{.status.lastError}' 2>/dev/null)
    
    # Display status
    echo "$(date '+%H:%M:%S') - Phase: $PHASE | Progress: $PERCENTAGE% | Issues: $PROCESSED/$TOTAL | Operation: $OPERATION"
    
    if [ -n "$ERROR" ]; then
        echo "  ⚠️  Error: $ERROR"
    fi
    
    # Check if complete
    if [ "$PHASE" = "Completed" ] || [ "$PHASE" = "Failed" ]; then
        echo "Sync finished with phase: $PHASE"
        break
    fi
    
    sleep 10
done
```

### Health Check Script

```bash
#!/bin/bash
# health-check.sh - Check health status of all sync operations

echo "JIRA Sync Health Check"
echo "====================="

# Get all JIRASync resources
SYNCS=$(kubectl get jirasync -o jsonpath='{.items[*].metadata.name}')

if [ -z "$SYNCS" ]; then
    echo "No sync operations found"
    exit 0
fi

for sync in $SYNCS; do
    echo "Checking: $sync"
    
    # Get status information
    PHASE=$(kubectl get jirasync "$sync" -o jsonpath='{.status.phase}')
    RETRY_COUNT=$(kubectl get jirasync "$sync" -o jsonpath='{.status.retryCount}')
    LAST_ERROR=$(kubectl get jirasync "$sync" -o jsonpath='{.status.lastError}')
    
    # Calculate health status
    HEALTH="Unknown"
    if [ -n "$PHASE" ]; then
        case "$PHASE" in
            "Completed")
                HEALTH="✅ Healthy"
                ;;
            "Running")
                if [ -n "$RETRY_COUNT" ] && [ "$RETRY_COUNT" -gt 3 ]; then
                    HEALTH="⚠️  Degraded (High retry count: $RETRY_COUNT)"
                else
                    HEALTH="✅ Healthy"
                fi
                ;;
            "Failed")
                HEALTH="❌ Unhealthy"
                ;;
            "Pending")
                HEALTH="🔄 Starting"
                ;;
        esac
    fi
    
    echo "  Status: $HEALTH"
    if [ -n "$LAST_ERROR" ]; then
        echo "  Last Error: $LAST_ERROR"
    fi
    echo ""
done
```

### Condition Monitoring

```bash
#!/bin/bash
# watch-conditions.sh - Monitor condition changes

SYNC_NAME="$1"

echo "Monitoring conditions for: $SYNC_NAME"
echo "====================================="

kubectl get jirasync "$SYNC_NAME" -o jsonpath='{.status.conditions[*]}' | jq -r '
  "Type: " + .type +
  " | Status: " + .status +
  " | Reason: " + .reason +
  " | Message: " + .message +
  " | Updated: " + .lastTransitionTime
' 2>/dev/null

echo ""
echo "Watching for changes..."
kubectl get jirasync "$SYNC_NAME" -w -o jsonpath='{.status.conditions[*]}' | while read -r line; do
    if [ -n "$line" ]; then
        echo "$(date '+%H:%M:%S') - Condition updated"
        echo "$line" | jq -r '
          "  " + .type + ": " + .status + " (" + .reason + ") - " + .message
        ' 2>/dev/null
    fi
done
```

## Dashboard Examples

### Custom Columns for kubectl

```bash
# Create a custom columns view for sync status
kubectl get jirasync -o custom-columns=\
NAME:.metadata.name,\
PHASE:.status.phase,\
PROGRESS:.status.progress.percentage,\
ISSUES:.status.syncState.processedIssues/.status.syncState.totalIssues,\
OPERATION:.status.progress.currentOperation,\
ERRORS:.status.retryCount

# Watch with custom columns
kubectl get jirasync -w -o custom-columns=\
NAME:.metadata.name,\
PHASE:.status.phase,\
PROGRESS:.status.progress.percentage,\
LAST_UPDATE:.status.lastStatusUpdate
```

### Status Summary Script

```bash
#!/bin/bash
# status-summary.sh - Generate status summary report

echo "JIRA Sync Status Summary"
echo "========================"
echo "Generated at: $(date)"
echo ""

# Count by phase
echo "Operations by Phase:"
kubectl get jirasync -o jsonpath='{.items[*].status.phase}' | tr ' ' '\n' | sort | uniq -c | while read count phase; do
    echo "  $phase: $count"
done

echo ""

# Active operations
echo "Active Operations:"
kubectl get jirasync -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.status.phase}{"\t"}{.status.progress.percentage}{"\n"}{end}' | \
grep -E "(Running|Pending)" | while IFS=$'\t' read name phase progress; do
    echo "  $name: $phase ($progress%)"
done

echo ""

# Recent errors
echo "Recent Errors:"
kubectl get jirasync -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.status.lastError}{"\n"}{end}' | \
grep -v $'\t$' | while IFS=$'\t' read name error; do
    echo "  $name: $error"
done
```

## Prometheus Metrics Integration

### Metrics Collection

The status manager automatically exposes Prometheus metrics:

```yaml
# Example Prometheus scrape configuration
- job_name: 'jira-sync-operator'
  static_configs:
  - targets: ['jira-sync-operator:8080']
  metrics_path: /metrics
  scrape_interval: 30s
```

### Key Metrics

- `jirasync_status_updates_total`: Total status updates
- `jirasync_progress_percentage`: Current progress percentage
- `jirasync_retry_count`: Current retry count
- `jirasync_processing_duration_seconds`: Processing duration
- `jirasync_condition_transitions_total`: Condition state transitions

### Grafana Dashboard Query Examples

```promql
# Progress tracking
jirasync_progress_percentage{job="jira-sync-operator"}

# Error rate
rate(jirasync_status_updates_total{status="error"}[5m])

# Average processing time
avg(jirasync_processing_duration_seconds) by (sync_type)

# Health status distribution
count by (phase) (jirasync_phase_info)
```

## API Integration Examples

### Using curl for Status Monitoring

```bash
# Get status via API
curl -H "Authorization: Bearer $TOKEN" \
  http://api-server:8080/api/v1/sync/example-sync/status | jq

# Monitor progress
while true; do
  PROGRESS=$(curl -s -H "Authorization: Bearer $TOKEN" \
    http://api-server:8080/api/v1/sync/example-sync/status | \
    jq -r '.data.progress.percentage')
  echo "Progress: $PROGRESS%"
  sleep 10
done
```

### Go Client Example

```go
package main

import (
    "context"
    "fmt"
    "time"
    
    "github.com/chambrid/jira-cdc-git/internal/operator/apiclient"
)

func monitorSyncProgress(client *apiclient.Client, jobID string) {
    ctx := context.Background()
    
    for {
        status, err := client.GetJobStatus(ctx, jobID)
        if err != nil {
            fmt.Printf("Error getting status: %v\n", err)
            return
        }
        
        fmt.Printf("Phase: %s, Progress: %d%%, Issues: %d/%d\n",
            status.Phase,
            status.Progress.Percentage,
            status.SyncState.ProcessedIssues,
            status.SyncState.TotalIssues)
            
        if status.Phase == "Completed" || status.Phase == "Failed" {
            fmt.Printf("Sync finished with phase: %s\n", status.Phase)
            break
        }
        
        time.Sleep(10 * time.Second)
    }
}
```

## Troubleshooting Workflows

### Status-Based Troubleshooting

```bash
#!/bin/bash
# troubleshoot-sync.sh - Automated troubleshooting

SYNC_NAME="$1"

echo "Troubleshooting sync: $SYNC_NAME"
echo "================================"

# Check basic status
PHASE=$(kubectl get jirasync "$SYNC_NAME" -o jsonpath='{.status.phase}')
echo "Current phase: $PHASE"

case "$PHASE" in
    "Pending")
        echo "Checking for common Pending issues..."
        
        # Check conditions
        echo "Conditions:"
        kubectl get jirasync "$SYNC_NAME" -o jsonpath='{.status.conditions[*]}' | jq -r '.message' 2>/dev/null
        
        # Check operator logs
        echo "Recent operator logs:"
        kubectl logs -l app=jira-sync-operator --tail=20 | grep "$SYNC_NAME"
        ;;
        
    "Running")
        echo "Checking progress and performance..."
        
        # Check progress
        PERCENTAGE=$(kubectl get jirasync "$SYNC_NAME" -o jsonpath='{.status.progress.percentage}')
        PROCESSED=$(kubectl get jirasync "$SYNC_NAME" -o jsonpath='{.status.syncState.processedIssues}')
        TOTAL=$(kubectl get jirasync "$SYNC_NAME" -o jsonpath='{.status.syncState.totalIssues}')
        
        echo "Progress: $PERCENTAGE% ($PROCESSED/$TOTAL issues)"
        
        # Check for stalled progress
        LAST_UPDATE=$(kubectl get jirasync "$SYNC_NAME" -o jsonpath='{.status.lastStatusUpdate}')
        echo "Last status update: $LAST_UPDATE"
        
        # Check retry count
        RETRY_COUNT=$(kubectl get jirasync "$SYNC_NAME" -o jsonpath='{.status.retryCount}')
        if [ -n "$RETRY_COUNT" ] && [ "$RETRY_COUNT" -gt 0 ]; then
            echo "⚠️  High retry count: $RETRY_COUNT"
            echo "Last error: $(kubectl get jirasync "$SYNC_NAME" -o jsonpath='{.status.lastError}')"
        fi
        ;;
        
    "Failed")
        echo "Analyzing failure..."
        
        # Get error information
        ERROR=$(kubectl get jirasync "$SYNC_NAME" -o jsonpath='{.status.lastError}')
        RETRY_COUNT=$(kubectl get jirasync "$SYNC_NAME" -o jsonpath='{.status.retryCount}')
        
        echo "Last error: $ERROR"
        echo "Retry count: $RETRY_COUNT"
        
        # Check failed condition
        kubectl get jirasync "$SYNC_NAME" -o jsonpath='{.status.conditions[?(@.type=="Failed")]}' | jq 2>/dev/null
        ;;
esac

echo ""
echo "Recent events:"
kubectl get events --field-selector involvedObject.name="$SYNC_NAME" --sort-by='.lastTimestamp' | tail -5
```

## Best Practices

### Status Monitoring

1. **Regular Health Checks**: Monitor condition status and retry counts
2. **Progress Tracking**: Use percentage and estimated completion for planning
3. **Error Handling**: Set up alerts for high retry counts or failed conditions
4. **Resource Monitoring**: Track total vs processed issues for performance insights

### Automated Monitoring

1. **Prometheus Integration**: Use metrics for dashboards and alerting
2. **Event Monitoring**: Watch Kubernetes events for status changes
3. **Log Correlation**: Combine status data with operator logs for debugging
4. **Custom Dashboards**: Create operation-specific monitoring views

### Performance Optimization

1. **Batch Size Tuning**: Monitor processing rates to optimize batch sizes
2. **Rate Limit Handling**: Use retry count metrics to tune rate limiting
3. **Concurrency Adjustment**: Use processing duration to optimize parallelism
4. **Resource Scaling**: Scale operator replicas based on active sync count

## Integration Examples

### CI/CD Pipeline Integration

```yaml
# GitHub Actions example
name: Sync JIRA Issues
on:
  schedule:
    - cron: '0 */6 * * *'  # Every 6 hours

jobs:
  sync:
    runs-on: ubuntu-latest
    steps:
    - name: Trigger Sync
      run: |
        kubectl apply -f - <<EOF
        apiVersion: sync.jira.io/v1alpha1
        kind: JIRASync
        metadata:
          name: scheduled-sync-$(date +%Y%m%d-%H%M%S)
        spec:
          syncType: "jql"
          target:
            jqlQuery: "project = PROJ AND updated >= -6h"
          destination:
            repository: "https://github.com/company/issues.git"
        EOF
        
    - name: Wait for Completion
      run: |
        SYNC_NAME="scheduled-sync-$(date +%Y%m%d-%H%M%S)"
        while true; do
          PHASE=$(kubectl get jirasync "$SYNC_NAME" -o jsonpath='{.status.phase}')
          if [ "$PHASE" = "Completed" ]; then
            echo "Sync completed successfully"
            break
          elif [ "$PHASE" = "Failed" ]; then
            echo "Sync failed"
            kubectl describe jirasync "$SYNC_NAME"
            exit 1
          fi
          sleep 30
        done
```

### Monitoring Integration

```yaml
# Prometheus AlertManager rules
groups:
- name: jira-sync
  rules:
  - alert: SyncStalled
    expr: |
      (time() - jirasync_last_status_update_timestamp) > 600
      and on(sync_name) jirasync_phase_info{phase="Running"}
    for: 5m
    labels:
      severity: warning
    annotations:
      summary: "JIRA sync {{ $labels.sync_name }} appears stalled"
      
  - alert: HighRetryCount
    expr: jirasync_retry_count > 5
    for: 2m
    labels:
      severity: warning
    annotations:
      summary: "JIRA sync {{ $labels.sync_name }} has high retry count"
```

This comprehensive guide provides practical examples and patterns for effectively using the enhanced status management capabilities in JIRA CDC Git Sync v0.4.1+.