# Security Configuration and Best Practices

## Overview

The JIRA CDC Git Sync system implements enterprise-grade security features designed for production Kubernetes environments. This document provides comprehensive guidance on security configuration, threat mitigation, and validation procedures.

## Security Architecture

### Defense in Depth Strategy

The system employs multiple layers of security protection:

1. **Network Security**: HTTPS-only connections, protocol restrictions
2. **Access Control**: RBAC with minimal required permissions
3. **Input Validation**: Comprehensive sanitization and injection prevention
4. **Credential Management**: Kubernetes-native secret handling
5. **Runtime Security**: Pod security standards and non-root execution

### Threat Model

The system protects against these threat categories:

| Threat Category | Attack Vectors | Mitigation |
|----------------|----------------|------------|
| **Injection Attacks** | SQL injection, XSS, command injection | Input validation, sanitization |
| **Path Traversal** | Directory traversal, null byte injection | Path validation, allowlist |
| **Protocol Abuse** | FTP, file://, data: URIs | Protocol allowlist (HTTPS/Git only) |
| **DoS Attacks** | Oversized inputs, resource exhaustion | Length limits, rate limiting |
| **Credential Theft** | Log exposure, environment leaks | Secure secret management |
| **Privilege Escalation** | Excessive RBAC permissions | Minimal permission model |

## RBAC Configuration

### Quick Start

```bash
# 1. Apply RBAC configuration
kubectl apply -f deployments/api-server/rbac.yaml

# 2. Verify ServiceAccount
kubectl get serviceaccount jira-sync-api -n jira-sync-v040

# 3. Verify permissions
kubectl describe clusterrole jira-sync-api
kubectl get clusterrolebinding jira-sync-api
```

### Permission Matrix

The operator uses **minimal required permissions** following the principle of least privilege:

```yaml
# ServiceAccount
apiVersion: v1
kind: ServiceAccount
metadata:
  name: jira-sync-api
  namespace: jira-sync-v040

# ClusterRole with minimal permissions
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: jira-sync-api
rules:
# Job management - full lifecycle required
- apiGroups: ["batch"]
  resources: ["jobs"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
# Pod monitoring - read-only for status checking
- apiGroups: [""]
  resources: ["pods"]
  verbs: ["get", "list", "watch"]
# Pod logs - read-only for debugging
- apiGroups: [""]
  resources: ["pods/log"]
  verbs: ["get", "list"]
# Events - read-only for audit trail
- apiGroups: [""]
  resources: ["events"]
  verbs: ["get", "list", "watch"]
```

### RBAC Validation

```bash
# Test job creation permissions
kubectl auth can-i create jobs --as=system:serviceaccount:jira-sync-v040:jira-sync-api

# Test pod reading permissions  
kubectl auth can-i get pods --as=system:serviceaccount:jira-sync-v040:jira-sync-api

# Verify no excessive permissions
kubectl auth can-i create secrets --as=system:serviceaccount:jira-sync-v040:jira-sync-api  # Should be "no"
```

## Credential Management

### Secure JIRA Credentials

```bash
# Create credentials with secure token
kubectl create secret generic jira-credentials \
  --from-literal=base-url=https://your-company.atlassian.net \
  --from-literal=email=your-email@company.com \
  --from-literal=pat=your-personal-access-token \
  --namespace=jira-sync-v040

# Verify secret (credentials will be masked)
kubectl describe secret jira-credentials -n jira-sync-v040
```

### API Server Integration Credentials

```bash
# For enhanced API integration (optional)
kubectl create secret generic api-credentials \
  --from-literal=token=your-api-server-token \
  --namespace=jira-sync-v040
```

### Credential Security Best Practices

- ✅ **Use Personal Access Tokens**: Never use passwords
- ✅ **Rotate Regularly**: Implement token rotation schedule
- ✅ **Namespace Isolation**: Credentials isolated per deployment
- ✅ **No Logging**: Sensitive data excluded from logs
- ❌ **Never Commit**: Never commit credentials to version control
- ❌ **No Environment Variables**: Avoid environment-based credentials in production

## Input Validation and Security Testing

### Comprehensive Attack Prevention

The system validates all inputs against 15+ attack scenarios:

#### Injection Attack Prevention
- **SQL Injection**: JQL query sanitization and parameterization
- **XSS Prevention**: HTML/JavaScript injection blocking
- **Command Injection**: Shell metacharacter filtering
- **LDAP Injection**: Directory service query protection

#### Path Security
- **Directory Traversal**: `../` sequence detection and blocking
- **Null Byte Injection**: Null character filtering
- **Symbolic Link Attacks**: Path resolution validation
- **Unicode Normalization**: UTF-8 encoding validation

#### Protocol and Network Security
- **Protocol Allowlist**: HTTPS and Git protocols only
- **URL Validation**: Malicious URL pattern detection
- **File URI Blocking**: Local file access prevention
- **FTP/Data URI Blocking**: Unsafe protocol rejection

### Security Test Suite

Location: `crds/v1alpha1/tests/security/jirasync-security-tests.yaml`

```bash
# Apply security test cases (all should fail for security)
kubectl apply -f crds/v1alpha1/tests/security/jirasync-security-tests.yaml

# Monitor test results - all should be rejected
kubectl get jirasyncs | grep security-test

# View specific rejection reasons
kubectl describe jirasync security-test-local-file
kubectl describe jirasync security-test-sql-injection
kubectl describe jirasync security-test-command-injection
```

#### Test Categories

1. **Local File Access Tests** (`security-test-local-file`)
2. **JavaScript Injection Tests** (`security-test-javascript`)
3. **Directory Traversal Tests** (`security-test-traversal`)
4. **SQL Injection Tests** (`security-test-sql-injection`)
5. **Command Injection Tests** (`security-test-command-injection`)
6. **Null Byte Injection Tests** (`security-test-null-byte`)
7. **Control Character Tests** (`security-test-control-chars`)
8. **Unicode Attack Tests** (`security-test-unicode`)
9. **Protocol Abuse Tests** (`security-test-ftp`, `security-test-data-uri`)
10. **DoS Attack Tests** (`security-test-long-string`)
11. **Label Injection Tests** (`security-test-label-injection`)
12. **Issue Key Injection Tests** (`security-test-issue-injection`)
13. **Schedule Injection Tests** (`security-test-schedule-injection`)
14. **HTTP Downgrade Tests** (`security-test-http-url`)

## Runtime Security

### Pod Security Standards

Deploy with security-hardened configuration:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: jira-sync-operator
  namespace: jira-sync-v040
spec:
  template:
    spec:
      serviceAccountName: jira-sync-api
      # Pod-level security context
      securityContext:
        runAsNonRoot: true
        runAsUser: 1000
        runAsGroup: 1000
        fsGroup: 2000
        seccompProfile:
          type: RuntimeDefault
      containers:
      - name: operator
        image: jira-sync-operator:latest
        # Container-level security context
        securityContext:
          allowPrivilegeEscalation: false
          readOnlyRootFilesystem: true
          runAsNonRoot: true
          capabilities:
            drop:
            - ALL
        # Resource limits
        resources:
          limits:
            memory: "128Mi"
            cpu: "100m"
          requests:
            memory: "64Mi"
            cpu: "50m"
```

### Network Policies

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: jira-sync-operator-netpol
  namespace: jira-sync-v040
spec:
  podSelector:
    matchLabels:
      app: jira-sync-operator
  policyTypes:
  - Egress
  egress:
  # Allow HTTPS to JIRA (port 443)
  - ports:
    - protocol: TCP
      port: 443
  # Allow DNS resolution
  - ports:
    - protocol: UDP
      port: 53
  # Allow API server access (if using API integration)
  - ports:
    - protocol: TCP
      port: 8080
    to:
    - podSelector:
        matchLabels:
          app: jira-sync-api
```

## Security Validation

### Pre-Deployment Checklist

- [ ] **RBAC Applied**: `kubectl get clusterrole jira-sync-api`
- [ ] **ServiceAccount Created**: `kubectl get sa jira-sync-api -n jira-sync-v040`
- [ ] **Secrets Configured**: `kubectl get secret jira-credentials -n jira-sync-v040`
- [ ] **Security Tests Pass**: All security test cases should be rejected
- [ ] **Pod Security Context**: Non-root user, read-only filesystem
- [ ] **Network Policies**: Egress restrictions configured
- [ ] **Resource Limits**: CPU/memory limits set

### Security Monitoring

```bash
# Monitor for RBAC violations
kubectl get events --field-selector reason=Forbidden -n jira-sync-v040

# Check for privilege escalation attempts
kubectl logs -l app=jira-sync-operator | grep -i "forbidden\|unauthorized"

# Validate network traffic (if using network monitoring)
kubectl logs -l app=jira-sync-operator | grep -E "(http:|ftp:|file:)"  # Should be empty

# Monitor resource usage
kubectl top pods -l app=jira-sync-operator -n jira-sync-v040
```

### Incident Response

#### Security Alert Response

1. **Immediate Actions**:
   ```bash
   # Stop the operator
   kubectl scale deployment jira-sync-operator --replicas=0 -n jira-sync-v040
   
   # Review recent logs
   kubectl logs deployment/jira-sync-operator -n jira-sync-v040 --since=1h
   
   # Check for suspicious activity
   kubectl get events -n jira-sync-v040 --sort-by=.lastTimestamp
   ```

2. **Investigation**:
   ```bash
   # Audit RBAC permissions
   kubectl describe clusterrolebinding jira-sync-api
   
   # Check secret access patterns
   kubectl get events --field-selector involvedObject.name=jira-credentials -n jira-sync-v040
   
   # Review JIRASync resources for malicious patterns
   kubectl get jirasyncs -o yaml | grep -E "(javascript:|data:|file:|ftp:)"
   ```

3. **Recovery**:
   ```bash
   # Rotate credentials
   kubectl delete secret jira-credentials -n jira-sync-v040
   # Recreate with new credentials
   
   # Update RBAC if needed
   kubectl apply -f deployments/api-server/rbac.yaml
   
   # Restart with security validation
   kubectl scale deployment jira-sync-operator --replicas=1 -n jira-sync-v040
   ```

## Compliance and Auditing

### Security Audit Checklist

- [ ] **Access Controls**: RBAC follows least privilege principle
- [ ] **Data Protection**: Credentials encrypted at rest and in transit
- [ ] **Input Validation**: All user inputs validated and sanitized
- [ ] **Logging**: Security events logged without exposing secrets
- [ ] **Network Security**: HTTPS-only, protocol restrictions enforced
- [ ] **Runtime Security**: Non-root containers, read-only filesystems
- [ ] **Monitoring**: Security violations detected and alerted

### Compliance Evidence

```bash
# Generate compliance report
echo "=== RBAC Compliance ==="
kubectl describe clusterrole jira-sync-api

echo "=== Security Context Compliance ==="
kubectl get deployment jira-sync-operator -n jira-sync-v040 -o jsonpath='{.spec.template.spec.securityContext}'

echo "=== Network Policy Compliance ==="
kubectl get networkpolicy jira-sync-operator-netpol -n jira-sync-v040 -o yaml

echo "=== Secret Management Compliance ==="
kubectl describe secret jira-credentials -n jira-sync-v040
```

## Security Updates

### Version-Specific Security Features

- **v0.4.1+**: Complete RBAC implementation with minimal permissions
- **v0.4.1+**: Comprehensive input validation (15+ attack scenarios)
- **v0.4.1+**: Security test suite with automated validation
- **v0.4.1+**: Pod security standards compliance

### Security Roadmap

- **Future**: Network policy automation
- **Future**: Admission controller integration
- **Future**: Enhanced audit logging
- **Future**: Security scanning integration

## Support and Contacts

For security-related issues:

1. **General Security Questions**: Review this documentation
2. **Security Vulnerabilities**: Follow responsible disclosure process
3. **Configuration Issues**: Check troubleshooting sections in [docs/OPERATOR.md](OPERATOR.md)
4. **Emergency Response**: Follow incident response procedures above

---

**Last Updated**: 2024-10-08  
**Security Review**: v0.4.1 security implementation (JCG-028)  
**Next Review**: Planned for v0.5.0 release