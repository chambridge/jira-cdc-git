# CRD Operations Guide - v1alpha1

This document provides comprehensive operational procedures for managing JIRASync Custom Resource Definitions in production Kubernetes environments.

## Prerequisites

### Kubernetes Requirements
- Kubernetes v1.19+ (for OpenAPI v3 schema support)
- kubectl with cluster-admin privileges
- Cluster with CustomResourceDefinition (CRD) API enabled

### Security Requirements
- RBAC properly configured for CRD management
- Service accounts with minimal required permissions
- Network policies for operator and webhook communications

## Installation Procedures

### 1. Pre-Installation Validation

```bash
# Verify Kubernetes cluster connectivity
kubectl cluster-info

# Check CRD API availability
kubectl api-versions | grep apiextensions.k8s.io

# Validate cluster-admin permissions
kubectl auth can-i create customresourcedefinitions
```

### 2. CRD Installation

```bash
# Change to CRD directory
cd crds/v1alpha1

# Install all CRDs
kubectl apply -f jirasync-crd.yaml
kubectl apply -f jiraproject-crd.yaml  
kubectl apply -f syncschedule-crd.yaml

# Verify installation
kubectl get crds | grep sync.jira.io
```

### 3. Post-Installation Validation

```bash
# Run comprehensive validation test suite
chmod +x tests/validate-crds.sh
./tests/validate-crds.sh

# Check CRD status
kubectl describe crd jirasync.sync.jira.io
kubectl describe crd jiraprojects.sync.jira.io
kubectl describe crd syncschedules.sync.jira.io
```

## Upgrade Procedures

### 1. Pre-Upgrade Checklist

```bash
# Backup existing CRD definitions
kubectl get crd jirasync.sync.jira.io -o yaml > backup-jirasync-crd.yaml
kubectl get crd jiraprojects.sync.jira.io -o yaml > backup-jiraproject-crd.yaml
kubectl get crd syncschedules.sync.jira.io -o yaml > backup-syncschedule-crd.yaml

# Backup existing custom resources
kubectl get jirasync -A -o yaml > backup-jirasync-resources.yaml
kubectl get jiraproject -A -o yaml > backup-jiraproject-resources.yaml
kubectl get syncschedule -A -o yaml > backup-syncschedule-resources.yaml

# Check for active resources
kubectl get jirasync -A --no-headers | wc -l
kubectl get jiraproject -A --no-headers | wc -l
kubectl get syncschedule -A --no-headers | wc -l
```

### 2. Upgrade Execution

```bash
# Apply updated CRDs (preserves existing resources)
kubectl apply -f jirasync-crd.yaml
kubectl apply -f jiraproject-crd.yaml
kubectl apply -f syncschedule-crd.yaml

# Verify schema updates without affecting resources
kubectl get jirasync -A
kubectl get jiraproject -A
kubectl get syncschedule -A

# Run validation suite with existing resources
./tests/validate-crds.sh
```

### 3. Rollback Procedure (if needed)

```bash
# Restore previous CRD versions
kubectl apply -f backup-jirasync-crd.yaml
kubectl apply -f backup-jiraproject-crd.yaml
kubectl apply -f backup-syncschedule-crd.yaml

# Verify rollback success
kubectl describe crd jirasync.sync.jira.io | grep "Version:"
```

## Validation and Testing

### 1. Schema Validation

```bash
# Test valid configurations
kubectl apply --dry-run=server -f tests/valid/jirasync-valid-examples.yaml

# Test invalid configurations (should fail)
kubectl apply --dry-run=server -f tests/invalid/jirasync-invalid-examples.yaml

# Test security controls (should fail)
kubectl apply --dry-run=server -f tests/security/jirasync-security-tests.yaml
```

### 2. Resource Validation

```bash
# Create test resource
kubectl apply -f - <<EOF
apiVersion: sync.jira.io/v1alpha1
kind: JIRASync
metadata:
  name: validation-test
  namespace: default
spec:
  syncType: single
  target:
    issueKeys: ["TEST-123"]
  destination:
    repository: "https://github.com/example/test.git"
EOF

# Verify resource creation
kubectl get jirasync validation-test -o yaml

# Clean up test resource
kubectl delete jirasync validation-test
```

### 3. Automated Validation

```bash
# Schedule regular validation (example cron job)
kubectl apply -f - <<EOF
apiVersion: batch/v1
kind: CronJob
metadata:
  name: crd-validation
  namespace: kube-system
spec:
  schedule: "0 2 * * *"  # Daily at 2 AM
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - name: validator
            image: bitnami/kubectl:latest
            command: ["/bin/sh"]
            args: ["-c", "cd /tests && ./validate-crds.sh"]
            volumeMounts:
            - name: tests
              mountPath: /tests
          volumes:
          - name: tests
            configMap:
              name: crd-tests
          restartPolicy: OnFailure
EOF
```

## Monitoring and Troubleshooting

### 1. Health Monitoring

```bash
# Monitor CRD status
kubectl get crds -l app.kubernetes.io/name=jira-sync-operator

# Check for validation errors
kubectl get events --field-selector reason=FailedValidation

# Monitor resource creation/updates
kubectl get events --field-selector involvedObject.apiVersion=sync.jira.io/v1alpha1
```

### 2. Common Issues and Solutions

#### CRD Installation Failures
```bash
# Check cluster permissions
kubectl auth can-i create customresourcedefinitions

# Verify API server version
kubectl version --short

# Check for conflicting CRDs
kubectl get crds | grep -E "(jirasync|jiraproject|syncschedule)"
```

#### Resource Validation Failures
```bash
# Get detailed validation errors
kubectl apply -f resource.yaml --dry-run=server -o yaml

# Check schema constraints
kubectl describe crd jirasync.sync.jira.io | grep -A 50 "Schema:"

# Validate against security patterns
grep -E "(file://|javascript:|ftp://)" resource.yaml
```

#### Performance Issues
```bash
# Check CRD resource usage
kubectl top nodes
kubectl get crds --show-managed-fields

# Monitor API server logs
kubectl logs -n kube-system -l component=kube-apiserver | grep -i crd
```

### 3. Debugging Tools

```bash
# Enable verbose kubectl output
kubectl apply -f resource.yaml -v=8

# Inspect CRD schema details
kubectl get crd jirasync.sync.jira.io -o jsonpath='{.spec.versions[0].schema}' | jq .

# Check admission controller logs
kubectl logs -n kube-system -l app=webhook-controller
```

## Security Considerations

### 1. Access Control

```bash
# Create service account with minimal permissions
kubectl create serviceaccount jirasync-operator
kubectl create clusterrole jirasync-reader --verb=get,list,watch --resource=jirasync,jiraprojects,syncschedules
kubectl create clusterrolebinding jirasync-operator --clusterrole=jirasync-reader --serviceaccount=default:jirasync-operator

# Verify permissions
kubectl auth can-i get jirasync --as=system:serviceaccount:default:jirasync-operator
```

### 2. Network Security

```bash
# Apply network policies (example)
kubectl apply -f - <<EOF
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: jirasync-operator
spec:
  podSelector:
    matchLabels:
      app: jirasync-operator
  policyTypes:
  - Ingress
  - Egress
  ingress:
  - from:
    - namespaceSelector:
        matchLabels:
          name: kube-system
  egress:
  - to: []
    ports:
    - protocol: TCP
      port: 443  # HTTPS only
EOF
```

### 3. Validation Monitoring

```bash
# Monitor for security violations
kubectl get events --field-selector reason=Forbidden,type=Warning

# Check for malicious patterns in resources
kubectl get jirasync -A -o yaml | grep -E "(file://|javascript:|ftp://|\\.\\./)"

# Audit CRD changes
kubectl get events --field-selector involvedObject.kind=CustomResourceDefinition
```

## Backup and Recovery

### 1. Backup Procedures

```bash
# Create backup script
cat > backup-crds.sh << 'EOF'
#!/bin/bash
BACKUP_DIR="/backup/$(date +%Y%m%d-%H%M%S)"
mkdir -p "$BACKUP_DIR"

# Backup CRD definitions
kubectl get crd jirasync.sync.jira.io -o yaml > "$BACKUP_DIR/jirasync-crd.yaml"
kubectl get crd jiraprojects.sync.jira.io -o yaml > "$BACKUP_DIR/jiraproject-crd.yaml"
kubectl get crd syncschedules.sync.jira.io -o yaml > "$BACKUP_DIR/syncschedule-crd.yaml"

# Backup custom resources
kubectl get jirasync -A -o yaml > "$BACKUP_DIR/jirasync-resources.yaml"
kubectl get jiraproject -A -o yaml > "$BACKUP_DIR/jiraproject-resources.yaml"
kubectl get syncschedule -A -o yaml > "$BACKUP_DIR/syncschedule-resources.yaml"

echo "Backup completed: $BACKUP_DIR"
EOF

chmod +x backup-crds.sh
```

### 2. Recovery Procedures

```bash
# Restore CRDs from backup
kubectl apply -f /backup/20231201-120000/jirasync-crd.yaml
kubectl apply -f /backup/20231201-120000/jiraproject-crd.yaml
kubectl apply -f /backup/20231201-120000/syncschedule-crd.yaml

# Restore custom resources
kubectl apply -f /backup/20231201-120000/jirasync-resources.yaml
kubectl apply -f /backup/20231201-120000/jiraproject-resources.yaml
kubectl apply -f /backup/20231201-120000/syncschedule-resources.yaml

# Verify recovery
./tests/validate-crds.sh
```

## Performance Tuning

### 1. Resource Limits

```yaml
# Set appropriate limits in CRD schemas
maxItems: 100          # Limit array sizes
maxLength: 500         # Limit string lengths
maxProperties: 20      # Limit object properties
```

### 2. API Server Optimization

```bash
# Monitor API server metrics
kubectl top nodes
kubectl get --raw /metrics | grep apiserver_request

# Tune etcd performance
kubectl get --raw /debug/pprof/heap > heap.pprof
```

### 3. Validation Performance

```bash
# Use client-side validation when possible
kubectl apply --dry-run=client -f resource.yaml

# Batch resource operations
kubectl apply -f directory/ --recursive

# Monitor validation latency
time kubectl apply --dry-run=server -f tests/valid/
```

## Compliance and Auditing

### 1. Audit Logging

```bash
# Enable CRD audit logging
kubectl patch configmap audit-policy -n kube-system --patch='
data:
  audit-policy.yaml: |
    rules:
    - level: Metadata
      resources:
      - group: sync.jira.io
        resources: ["*"]
'

# Query audit logs
kubectl logs -n kube-system -l component=kube-apiserver | grep "sync.jira.io"
```

### 2. Compliance Checks

```bash
# Check for security compliance
./tests/validate-crds.sh | grep -i security

# Verify RBAC compliance
kubectl auth can-i list secrets --as=system:serviceaccount:default:jirasync-operator

# Check resource quotas
kubectl describe resourcequota -A | grep -A 5 sync.jira.io
```

## Version Management

### 1. API Version Strategy

- **v1alpha1**: Current development version
- **v1beta1**: Planned stable API (target: v0.5.0)
- **v1**: Production stable API (target: v1.0.0)

### 2. Migration Planning

```bash
# Plan v1alpha1 -> v1beta1 migration
kubectl get jirasync -A -o json | jq '.items[].apiVersion'

# Prepare conversion webhook (future)
# kubectl apply -f conversion-webhook.yaml
```

## Support and Escalation

### 1. Log Collection

```bash
# Collect diagnostics
kubectl get crds -o yaml > crds-status.yaml
kubectl get events --sort-by=.metadata.creationTimestamp > events.log
kubectl logs -n kube-system -l component=kube-apiserver --tail=1000 > apiserver.log
```

### 2. Issue Reporting

Include in support requests:
- Kubernetes version: `kubectl version`
- CRD versions: `kubectl get crds -l app.kubernetes.io/name=jira-sync-operator`
- Resource examples: Sanitized YAML of failing resources
- Validation output: `./tests/validate-crds.sh` results
- Error logs: API server and controller logs

---

**Document Version**: v1.0.0  
**Last Updated**: 2024-01-XX  
**Maintained By**: JIRASync Engineering Team  
**Review Cycle**: Quarterly