# JIRA Sync Operator Upgrade Guide

This document provides comprehensive upgrade procedures for the JIRA Sync Operator.

## Overview

The JIRA Sync Operator follows semantic versioning and provides safe upgrade paths between versions. This guide covers:

- Pre-upgrade validation
- Backup and recovery procedures
- Step-by-step upgrade instructions
- Post-upgrade verification
- Rollback procedures

## Pre-Upgrade Checklist

### 1. Version Compatibility

Check the compatibility matrix before upgrading:

| From Version | To Version | Upgrade Path | Notes |
|--------------|------------|--------------|--------|
| v0.4.0       | v0.4.1     | Direct       | Initial operator release |
| v0.4.1       | v0.4.2+    | Direct       | CRD schema compatible |

### 2. Environment Validation

```bash
# Verify current operator status
kubectl get deployment jira-sync-operator -n jira-sync-system

# Check CRD versions
kubectl get crd jirasyncs.sync.jira.io -o yaml | grep version

# Verify API server integration
kubectl get svc jira-sync-api -n jira-sync-v040

# Check resource quotas
kubectl describe resourcequota -n jira-sync-system
```

### 3. Backup Current State

```bash
# Create backup directory
mkdir -p ./operator-backup/$(date +%Y%m%d-%H%M%S)
cd ./operator-backup/$(date +%Y%m%d-%H%M%S)

# Backup CRD definitions
kubectl get crd jirasyncs.sync.jira.io -o yaml > jirasync-crd-backup.yaml
kubectl get crd jiraprojects.sync.jira.io -o yaml > jiraproject-crd-backup.yaml
kubectl get crd syncschedules.sync.jira.io -o yaml > syncschedule-crd-backup.yaml

# Backup all JIRASync resources
kubectl get jirasync --all-namespaces -o yaml > jirasync-resources-backup.yaml

# Backup operator deployment
kubectl get deployment jira-sync-operator -n jira-sync-system -o yaml > operator-deployment-backup.yaml

# Backup RBAC
kubectl get clusterrole jira-sync-operator-manager -o yaml > operator-clusterrole-backup.yaml
kubectl get clusterrolebinding jira-sync-operator-manager -o yaml > operator-clusterrolebinding-backup.yaml

# Backup configuration
kubectl get configmap -n jira-sync-system -o yaml > operator-configmaps-backup.yaml
kubectl get secret -n jira-sync-system -o yaml > operator-secrets-backup.yaml
```

## Upgrade Procedures

### Method 1: Helm Upgrade (Recommended)

#### Standard Upgrade

```bash
# Update Helm repository
helm repo update

# Review changes
helm diff upgrade jira-sync-operator ./deployments/operator \
  --namespace jira-sync-system \
  --values ./deployments/operator/values.yaml

# Perform upgrade
helm upgrade jira-sync-operator ./deployments/operator \
  --namespace jira-sync-system \
  --values ./deployments/operator/values.yaml \
  --timeout 10m \
  --wait

# Verify upgrade
helm status jira-sync-operator -n jira-sync-system
```

#### Upgrade with Custom Values

```bash
# Create custom values for upgrade
cat > upgrade-values.yaml << EOF
operator:
  image:
    tag: "v0.4.2"
  
  leaderElection:
    enabled: true
  
  resources:
    requests:
      memory: "128Mi"
    limits:
      memory: "512Mi"

apiServer:
  host: "jira-sync-api.jira-sync-v040.svc.cluster.local:8080"
EOF

# Upgrade with custom values
helm upgrade jira-sync-operator ./deployments/operator \
  --namespace jira-sync-system \
  --values ./deployments/operator/values.yaml \
  --values upgrade-values.yaml \
  --timeout 10m \
  --wait
```

### Method 2: Manual Kubernetes Upgrade

```bash
# Apply updated CRDs (if changed)
kubectl apply -f ./crds/v1alpha1/

# Update operator deployment
kubectl set image deployment/jira-sync-operator \
  operator=localhost/jira-sync-operator:v0.4.2 \
  -n jira-sync-system

# Wait for rollout completion
kubectl rollout status deployment/jira-sync-operator -n jira-sync-system --timeout=300s
```

## Version-Specific Upgrade Notes

### Upgrading to v0.4.1

- **New Features**: Initial operator release with CRD support
- **Breaking Changes**: None (first operator version)
- **Migration**: Automatic migration from v0.4.0 API-only deployments

### Upgrading to v0.4.2 (Future)

- **New Features**: Real-time change detection
- **Breaking Changes**: None expected
- **Migration**: CRD schema additions only

## Post-Upgrade Verification

### 1. Operator Health Check

```bash
# Check operator pod status
kubectl get pods -n jira-sync-system -l app.kubernetes.io/name=jira-sync-operator

# Verify operator logs
kubectl logs -n jira-sync-system -l app.kubernetes.io/name=jira-sync-operator --tail=50

# Check health endpoints
kubectl port-forward -n jira-sync-system svc/jira-sync-operator-health 8081:8081 &
curl http://localhost:8081/healthz
curl http://localhost:8081/readyz
```

### 2. CRD Validation

```bash
# Verify CRD versions
kubectl get crd jirasyncs.sync.jira.io -o jsonpath='{.spec.versions[*].name}'

# Test CRD functionality
kubectl apply -f - << EOF
apiVersion: sync.jira.io/v1alpha1
kind: JIRASync
metadata:
  name: upgrade-test
  namespace: default
spec:
  jiraConfig:
    baseURL: "https://test.atlassian.net"
    credentialsSecret: "test-secret"
  syncConfig:
    mode: "Single"
    issueKeys: ["TEST-1"]
  gitConfig:
    repository: "/tmp/test-repo"
EOF

# Verify resource creation
kubectl get jirasync upgrade-test -o yaml

# Clean up test resource
kubectl delete jirasync upgrade-test
```

### 3. Integration Testing

```bash
# Test operator-API server integration
kubectl apply -f - << EOF
apiVersion: sync.jira.io/v1alpha1
kind: JIRASync
metadata:
  name: integration-test
  namespace: default
spec:
  jiraConfig:
    baseURL: "https://demo.atlassian.net"
    credentialsSecret: "jira-credentials"
  syncConfig:
    mode: "JQL"
    jqlQuery: "project = DEMO AND created >= -1d"
  gitConfig:
    repository: "/tmp/demo-repo"
EOF

# Monitor sync progress
kubectl get jirasync integration-test -w

# Check sync status
kubectl get jirasync integration-test -o jsonpath='{.status.phase}'

# Clean up
kubectl delete jirasync integration-test
```

## Rollback Procedures

### Emergency Rollback with Helm

```bash
# List releases
helm history jira-sync-operator -n jira-sync-system

# Rollback to previous version
helm rollback jira-sync-operator <revision> -n jira-sync-system --timeout 10m

# Verify rollback
helm status jira-sync-operator -n jira-sync-system
kubectl get pods -n jira-sync-system -l app.kubernetes.io/name=jira-sync-operator
```

### Manual Rollback

```bash
# Restore from backup
kubectl apply -f ./operator-backup/<timestamp>/operator-deployment-backup.yaml

# Wait for rollout
kubectl rollout status deployment/jira-sync-operator -n jira-sync-system

# Restore CRDs if needed
kubectl apply -f ./operator-backup/<timestamp>/jirasync-crd-backup.yaml
kubectl apply -f ./operator-backup/<timestamp>/jiraproject-crd-backup.yaml
kubectl apply -f ./operator-backup/<timestamp>/syncschedule-crd-backup.yaml
```

### Resource Recovery

```bash
# Restore JIRASync resources
kubectl apply -f ./operator-backup/<timestamp>/jirasync-resources-backup.yaml

# Restore configuration
kubectl apply -f ./operator-backup/<timestamp>/operator-configmaps-backup.yaml
```

## Troubleshooting

### Common Upgrade Issues

#### CRD Schema Conflicts

```bash
# Check for schema validation errors
kubectl describe crd jirasyncs.sync.jira.io

# Force CRD update if safe
kubectl replace -f ./crds/v1alpha1/jirasync-crd.yaml
```

#### Operator Pod Crashes

```bash
# Check operator logs
kubectl logs -n jira-sync-system -l app.kubernetes.io/name=jira-sync-operator --previous

# Verify RBAC permissions
kubectl auth can-i create jobs --as=system:serviceaccount:jira-sync-system:jira-sync-operator
```

#### API Server Integration Failures

```bash
# Test API server connectivity
kubectl exec -n jira-sync-system -l app.kubernetes.io/name=jira-sync-operator -- \
  curl -f http://jira-sync-api.jira-sync-v040.svc.cluster.local:8080/api/v1/health

# Check network policies
kubectl get networkpolicy -n jira-sync-system
```

### Recovery Procedures

#### Complete Reinstallation

```bash
# Remove operator
helm uninstall jira-sync-operator -n jira-sync-system

# Remove CRDs (if --keep-crds=false)
kubectl delete crd jirasyncs.sync.jira.io jiraprojects.sync.jira.io syncschedules.sync.jira.io

# Clean reinstall
helm install jira-sync-operator ./deployments/operator \
  --namespace jira-sync-system \
  --create-namespace \
  --values ./deployments/operator/values.yaml \
  --timeout 10m \
  --wait

# Restore resources
kubectl apply -f ./operator-backup/<timestamp>/jirasync-resources-backup.yaml
```

## Best Practices

### Planning Upgrades

1. **Test in Non-Production**: Always test upgrades in development/staging first
2. **Maintenance Windows**: Schedule upgrades during low-activity periods
3. **Staged Rollouts**: Consider blue-green or canary deployment strategies
4. **Monitoring**: Ensure comprehensive monitoring during upgrades

### Backup Strategy

1. **Regular Backups**: Automated daily backups of CRDs and resources
2. **Version Control**: Store operator configuration in Git
3. **Documentation**: Maintain upgrade logs and decisions

### Communication

1. **Stakeholder Notification**: Inform users of planned upgrades
2. **Status Updates**: Provide real-time upgrade status
3. **Post-Upgrade Reports**: Document changes and impacts

## Support

For upgrade issues or questions:

1. Check operator logs: `kubectl logs -n jira-sync-system -l app.kubernetes.io/name=jira-sync-operator`
2. Review this upgrade guide
3. Consult the [troubleshooting guide](./TROUBLESHOOTING.md)
4. Open a support ticket with backup information