# JIRA Sync Operator Operations Guide

This guide provides comprehensive operational procedures for the JIRA Sync Operator.

## Quick Reference

### Essential Commands

```bash
# Check operator status
kubectl get deployment jira-sync-operator -n jira-sync-system

# View operator logs
kubectl logs -n jira-sync-system -l app.kubernetes.io/name=jira-sync-operator -f

# Check CRD resources
kubectl get jirasync --all-namespaces

# Health check
kubectl port-forward -n jira-sync-system svc/jira-sync-operator-health 8081:8081
curl http://localhost:8081/healthz
```

## Installation

### Prerequisites

- Kubernetes cluster 1.19+
- Helm 3.0+
- kubectl configured
- v0.4.0 API server deployed

### Fresh Installation

```bash
# Add CRDs (if not using Helm CRD management)
kubectl apply -f ./crds/

# Install operator
helm install jira-sync-operator ./deployments/operator \
  --namespace jira-sync-system \
  --create-namespace \
  --values ./deployments/operator/values.yaml \
  --wait

# Verify installation
./deployments/operator/test/deployment-validation.sh
```

### Configuration

#### Basic Configuration (values.yaml)

```yaml
operator:
  image:
    repository: localhost/jira-sync-operator
    tag: "v0.4.1"
  
  leaderElection:
    enabled: true
  
  resources:
    requests:
      cpu: 50m
      memory: 64Mi
    limits:
      cpu: 200m
      memory: 256Mi

apiServer:
  host: "jira-sync-api.jira-sync-v040.svc.cluster.local:8080"

metrics:
  enabled: true
  serviceMonitor:
    enabled: false  # Set to true if using Prometheus Operator
```

## Daily Operations

### Monitoring Operator Health

#### Health Checks

```bash
# Check deployment status
kubectl get deployment jira-sync-operator -n jira-sync-system

# Check pod status
kubectl get pods -n jira-sync-system -l app.kubernetes.io/name=jira-sync-operator

# Check health endpoints
kubectl port-forward -n jira-sync-system svc/jira-sync-operator-health 8081:8081 &
curl http://localhost:8081/healthz
curl http://localhost:8081/readyz
```

#### Logs Monitoring

```bash
# Real-time logs
kubectl logs -n jira-sync-system -l app.kubernetes.io/name=jira-sync-operator -f

# Recent logs with timestamps
kubectl logs -n jira-sync-system -l app.kubernetes.io/name=jira-sync-operator --tail=100 --timestamps

# Search for errors
kubectl logs -n jira-sync-system -l app.kubernetes.io/name=jira-sync-operator | grep -i error

# Export logs for analysis
kubectl logs -n jira-sync-system -l app.kubernetes.io/name=jira-sync-operator > operator-logs-$(date +%Y%m%d).log
```

### Managing JIRASync Resources

#### Create JIRASync Resource

```bash
cat <<EOF | kubectl apply -f -
apiVersion: sync.jira.io/v1alpha1
kind: JIRASync
metadata:
  name: example-sync
  namespace: default
spec:
  jiraConfig:
    baseURL: "https://your-domain.atlassian.net"
    credentialsSecret: "jira-credentials"
  syncConfig:
    mode: "JQL"
    jqlQuery: "project = PROJ AND status = 'To Do'"
  gitConfig:
    repository: "/tmp/sync-repo"
EOF
```

#### Monitor Sync Status

```bash
# Get all syncs
kubectl get jirasync --all-namespaces

# Watch sync progress
kubectl get jirasync example-sync -w

# Get detailed status
kubectl describe jirasync example-sync

# Check sync conditions
kubectl get jirasync example-sync -o jsonpath='{.status.conditions[*]}'
```

#### Common Sync Operations

```bash
# Restart a stuck sync (delete and recreate)
kubectl delete jirasync example-sync
kubectl apply -f example-sync.yaml

# Force sync by updating annotation
kubectl annotate jirasync example-sync sync.jira.io/force-sync="$(date)"

# Check sync logs in API server
kubectl logs -n jira-sync-v040 -l app=jira-sync-api | grep example-sync
```

## Maintenance Procedures

### Regular Maintenance

#### Weekly Tasks

```bash
# Check resource usage
kubectl top pods -n jira-sync-system

# Review error logs
kubectl logs -n jira-sync-system -l app.kubernetes.io/name=jira-sync-operator --since=7d | grep -i error

# Verify CRD health
kubectl get crd | grep sync.jira.io

# Check for stuck resources
kubectl get jirasync --all-namespaces | grep -v Completed
```

#### Monthly Tasks

```bash
# Update operator image (if new version available)
helm upgrade jira-sync-operator ./deployments/operator \
  --namespace jira-sync-system \
  --set operator.image.tag=v0.4.2

# Cleanup completed sync resources older than 30 days
kubectl get jirasync --all-namespaces -o json | \
  jq -r '.items[] | select(.status.phase == "Completed" and (.metadata.creationTimestamp | strptime("%Y-%m-%dT%H:%M:%SZ") | mktime) < (now - 2592000)) | "\(.metadata.namespace) \(.metadata.name)"' | \
  while read namespace name; do kubectl delete jirasync "$name" -n "$namespace"; done
```

### Scaling Operations

#### Horizontal Scaling

```bash
# Scale operator replicas
kubectl scale deployment jira-sync-operator --replicas=2 -n jira-sync-system

# Enable leader election for HA
helm upgrade jira-sync-operator ./deployments/operator \
  --namespace jira-sync-system \
  --set operator.leaderElection.enabled=true \
  --set operator.replicaCount=2
```

#### Resource Adjustment

```bash
# Increase resources for high load
helm upgrade jira-sync-operator ./deployments/operator \
  --namespace jira-sync-system \
  --set operator.resources.requests.cpu=100m \
  --set operator.resources.requests.memory=128Mi \
  --set operator.resources.limits.cpu=500m \
  --set operator.resources.limits.memory=512Mi
```

## Troubleshooting

### Common Issues

#### Operator Not Starting

**Symptoms**: Pod in CrashLoopBackOff or Pending state

```bash
# Check pod events
kubectl describe pod -n jira-sync-system -l app.kubernetes.io/name=jira-sync-operator

# Check resource constraints
kubectl describe node | grep -A 5 "Allocated resources"

# Verify RBAC permissions
kubectl auth can-i create jobs --as=system:serviceaccount:jira-sync-system:jira-sync-operator

# Check image availability
kubectl get pod -n jira-sync-system -l app.kubernetes.io/name=jira-sync-operator -o jsonpath='{.items[0].status.containerStatuses[0].state}'
```

**Resolution**:
1. Verify image exists and is accessible
2. Check resource requests vs node capacity
3. Validate RBAC configuration
4. Review pod logs for specific errors

#### JIRASync Resources Not Processing

**Symptoms**: Resources stuck in "Pending" or "Processing" phase

```bash
# Check operator logs for errors
kubectl logs -n jira-sync-system -l app.kubernetes.io/name=jira-sync-operator | grep -A 5 -B 5 error

# Verify API server connectivity
kubectl exec -n jira-sync-system -l app.kubernetes.io/name=jira-sync-operator -- \
  curl -f http://jira-sync-api.jira-sync-v040.svc.cluster.local:8080/api/v1/health

# Check resource validation
kubectl get jirasync problem-sync -o yaml | kubectl apply --dry-run=server -f -
```

**Resolution**:
1. Verify API server is running and accessible
2. Check JIRA credentials in secret
3. Validate JQL query syntax
4. Review resource status conditions

#### High Memory Usage

**Symptoms**: Operator pod consuming excessive memory

```bash
# Check memory usage
kubectl top pod -n jira-sync-system

# Check for memory leaks in logs
kubectl logs -n jira-sync-system -l app.kubernetes.io/name=jira-sync-operator | grep -i "memory\|oom"

# Get heap profile (if pprof enabled)
kubectl port-forward -n jira-sync-system svc/jira-sync-operator-metrics 8080:8080 &
curl http://localhost:8080/debug/pprof/heap > heap.prof
```

**Resolution**:
1. Increase memory limits temporarily
2. Check for resource leaks
3. Reduce concurrent sync operations
4. Update to latest operator version

### Diagnostic Commands

#### Operator State Inspection

```bash
# Get operator configuration
kubectl get deployment jira-sync-operator -n jira-sync-system -o yaml

# Check environment variables
kubectl exec -n jira-sync-system -l app.kubernetes.io/name=jira-sync-operator -- env | grep -E "JIRA|API|LEADER"

# Verify controller registration
kubectl logs -n jira-sync-system -l app.kubernetes.io/name=jira-sync-operator | grep "controller.*starting"
```

#### Network Diagnostics

```bash
# Test API server connectivity
kubectl exec -n jira-sync-system -l app.kubernetes.io/name=jira-sync-operator -- \
  nslookup jira-sync-api.jira-sync-v040.svc.cluster.local

# Check network policies
kubectl get networkpolicy -n jira-sync-system

# Test external JIRA connectivity
kubectl run -it --rm debug --image=busybox --restart=Never -- \
  nslookup your-domain.atlassian.net
```

## Security Operations

### RBAC Management

#### Verify Permissions

```bash
# Check operator permissions
kubectl auth can-i create jirasync --as=system:serviceaccount:jira-sync-system:jira-sync-operator
kubectl auth can-i create jobs --as=system:serviceaccount:jira-sync-system:jira-sync-operator
kubectl auth can-i get secrets --as=system:serviceaccount:jira-sync-system:jira-sync-operator

# Audit RBAC
kubectl get clusterrole jira-sync-operator-manager -o yaml
```

#### Credential Management

```bash
# Rotate JIRA credentials
kubectl create secret generic jira-credentials-new \
  --from-literal=base-url=https://your-domain.atlassian.net \
  --from-literal=email=your-email@company.com \
  --from-literal=token=new-token \
  -n jira-sync-system

# Update deployment to use new secret
kubectl patch deployment jira-sync-operator -n jira-sync-system -p \
  '{"spec":{"template":{"spec":{"containers":[{"name":"operator","env":[{"name":"JIRA_CREDENTIALS_SECRET","value":"jira-credentials-new"}]}]}}}}'

# Delete old secret
kubectl delete secret jira-credentials -n jira-sync-system
```

### Security Monitoring

```bash
# Check for privilege escalation
kubectl logs -n jira-sync-system -l app.kubernetes.io/name=jira-sync-operator | grep -i "privilege\|root\|sudo"

# Monitor resource access
kubectl get events -n jira-sync-system --field-selector reason=FailedMount

# Audit network access
kubectl get networkpolicy -n jira-sync-system -o yaml
```

## Performance Tuning

### Resource Optimization

```bash
# Monitor resource usage over time
kubectl top pod -n jira-sync-system --containers

# Adjust based on usage patterns
helm upgrade jira-sync-operator ./deployments/operator \
  --namespace jira-sync-system \
  --set operator.resources.requests.cpu=75m \
  --set operator.resources.limits.memory=384Mi
```

### Concurrency Tuning

```bash
# Adjust controller worker threads (if supported)
helm upgrade jira-sync-operator ./deployments/operator \
  --namespace jira-sync-system \
  --set operator.env.CONTROLLER_MAX_CONCURRENT_RECONCILES=5
```

## Disaster Recovery

### Backup Procedures

```bash
# Backup CRDs and resources
mkdir -p backup/$(date +%Y%m%d)
kubectl get crd jirasyncs.sync.jira.io -o yaml > backup/$(date +%Y%m%d)/jirasync-crd.yaml
kubectl get jirasync --all-namespaces -o yaml > backup/$(date +%Y%m%d)/jirasync-resources.yaml

# Backup operator configuration
helm get values jira-sync-operator -n jira-sync-system > backup/$(date +%Y%m%d)/operator-values.yaml
kubectl get secret jira-credentials -n jira-sync-system -o yaml > backup/$(date +%Y%m%d)/credentials.yaml
```

### Recovery Procedures

```bash
# Restore from backup
kubectl apply -f backup/20231010/jirasync-crd.yaml
kubectl apply -f backup/20231010/jirasync-resources.yaml

# Reinstall operator
helm install jira-sync-operator ./deployments/operator \
  --namespace jira-sync-system \
  --create-namespace \
  --values backup/20231010/operator-values.yaml
```

## Support and Escalation

### Collecting Support Information

```bash
# Generate support bundle
./deployments/operator/test/deployment-validation.sh > support-info.txt

# Additional diagnostics
kubectl cluster-info >> support-info.txt
kubectl get events -n jira-sync-system --sort-by='.firstTimestamp' >> support-info.txt
kubectl top nodes >> support-info.txt
```

### Contact Information

- **Documentation**: [Project README](../../README.md)
- **Issues**: [GitHub Issues](https://github.com/chambrid/jira-cdc-git/issues)
- **Emergency**: Follow your organization's incident response procedures