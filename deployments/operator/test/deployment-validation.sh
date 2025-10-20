#!/bin/bash

# JIRA Sync Operator Deployment Validation Script
# This script validates a fresh operator deployment and runs smoke tests

set -euo pipefail

# Configuration
NAMESPACE="${NAMESPACE:-jira-sync-system}"
TIMEOUT="${TIMEOUT:-300}"
HELM_RELEASE="${HELM_RELEASE:-jira-sync-operator}"
OPERATOR_IMAGE="${OPERATOR_IMAGE:-localhost/jira-sync-operator:latest}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Logging functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Validation functions
check_prerequisites() {
    log_info "Checking prerequisites..."
    
    # Check kubectl
    if ! command -v kubectl &> /dev/null; then
        log_error "kubectl is not installed or not in PATH"
        exit 1
    fi
    
    # Check helm
    if ! command -v helm &> /dev/null; then
        log_error "helm is not installed or not in PATH"
        exit 1
    fi
    
    # Check Kubernetes connection
    if ! kubectl cluster-info &> /dev/null; then
        log_error "Cannot connect to Kubernetes cluster"
        exit 1
    fi
    
    log_success "Prerequisites check passed"
}

check_namespace() {
    log_info "Checking namespace: $NAMESPACE"
    
    if ! kubectl get namespace "$NAMESPACE" &> /dev/null; then
        log_warning "Namespace $NAMESPACE does not exist. Creating..."
        kubectl create namespace "$NAMESPACE"
    fi
    
    log_success "Namespace $NAMESPACE is ready"
}

check_crds() {
    log_info "Validating Custom Resource Definitions..."
    
    local crds=("jirasyncs.sync.jira.io" "jiraprojects.sync.jira.io" "syncschedules.sync.jira.io")
    
    for crd in "${crds[@]}"; do
        log_info "Checking CRD: $crd"
        
        if kubectl get crd "$crd" &> /dev/null; then
            log_success "CRD $crd found"
            
            # Check CRD version
            local version=$(kubectl get crd "$crd" -o jsonpath='{.spec.versions[0].name}')
            log_info "CRD $crd version: $version"
            
            # Validate CRD schema
            if kubectl get crd "$crd" -o jsonpath='{.spec.versions[0].schema}' | jq . &> /dev/null; then
                log_success "CRD $crd schema is valid JSON"
            else
                log_error "CRD $crd schema is invalid"
                return 1
            fi
        else
            log_error "CRD $crd not found"
            return 1
        fi
    done
    
    log_success "All CRDs validated successfully"
}

check_rbac() {
    log_info "Validating RBAC configuration..."
    
    # Check ServiceAccount
    local sa_name="$HELM_RELEASE"
    if kubectl get serviceaccount "$sa_name" -n "$NAMESPACE" &> /dev/null; then
        log_success "ServiceAccount $sa_name found"
    else
        log_error "ServiceAccount $sa_name not found"
        return 1
    fi
    
    # Check ClusterRole
    local cr_name="$HELM_RELEASE-manager"
    if kubectl get clusterrole "$cr_name" &> /dev/null; then
        log_success "ClusterRole $cr_name found"
        
        # Verify key permissions
        local permissions=("jirasyncs" "jobs" "events" "leases")
        for perm in "${permissions[@]}"; do
            if kubectl get clusterrole "$cr_name" -o yaml | grep -q "$perm"; then
                log_success "Permission for $perm found"
            else
                log_warning "Permission for $perm not found"
            fi
        done
    else
        log_error "ClusterRole $cr_name not found"
        return 1
    fi
    
    # Check ClusterRoleBinding
    local crb_name="$HELM_RELEASE-manager"
    if kubectl get clusterrolebinding "$crb_name" &> /dev/null; then
        log_success "ClusterRoleBinding $crb_name found"
    else
        log_error "ClusterRoleBinding $crb_name not found"
        return 1
    fi
    
    log_success "RBAC configuration validated successfully"
}

check_deployment() {
    log_info "Validating operator deployment..."
    
    local deployment_name="$HELM_RELEASE"
    
    # Check deployment exists
    if ! kubectl get deployment "$deployment_name" -n "$NAMESPACE" &> /dev/null; then
        log_error "Deployment $deployment_name not found"
        return 1
    fi
    
    log_success "Deployment $deployment_name found"
    
    # Wait for deployment to be ready
    log_info "Waiting for deployment to be ready (timeout: ${TIMEOUT}s)..."
    if kubectl wait --for=condition=available --timeout="${TIMEOUT}s" deployment/"$deployment_name" -n "$NAMESPACE"; then
        log_success "Deployment is ready"
    else
        log_error "Deployment failed to become ready within ${TIMEOUT}s"
        kubectl get deployment "$deployment_name" -n "$NAMESPACE" -o yaml
        kubectl describe deployment "$deployment_name" -n "$NAMESPACE"
        return 1
    fi
    
    # Check pod status
    local pod_name=$(kubectl get pods -n "$NAMESPACE" -l "app.kubernetes.io/name=$HELM_RELEASE" -o jsonpath='{.items[0].metadata.name}')
    if [[ -n "$pod_name" ]]; then
        log_info "Operator pod: $pod_name"
        
        # Check pod is running
        local pod_phase=$(kubectl get pod "$pod_name" -n "$NAMESPACE" -o jsonpath='{.status.phase}')
        if [[ "$pod_phase" == "Running" ]]; then
            log_success "Operator pod is running"
        else
            log_error "Operator pod is not running (phase: $pod_phase)"
            kubectl describe pod "$pod_name" -n "$NAMESPACE"
            return 1
        fi
    else
        log_error "No operator pod found"
        return 1
    fi
    
    log_success "Deployment validation passed"
}

check_services() {
    log_info "Validating services..."
    
    # Check metrics service
    local metrics_service="$HELM_RELEASE-metrics"
    if kubectl get service "$metrics_service" -n "$NAMESPACE" &> /dev/null; then
        log_success "Metrics service $metrics_service found"
    else
        log_warning "Metrics service $metrics_service not found (may be disabled)"
    fi
    
    # Check health service
    local health_service="$HELM_RELEASE-health"
    if kubectl get service "$health_service" -n "$NAMESPACE" &> /dev/null; then
        log_success "Health service $health_service found"
    else
        log_warning "Health service $health_service not found (may be disabled)"
    fi
    
    log_success "Services validation passed"
}

check_health_endpoints() {
    log_info "Validating health endpoints..."
    
    local pod_name=$(kubectl get pods -n "$NAMESPACE" -l "app.kubernetes.io/name=$HELM_RELEASE" -o jsonpath='{.items[0].metadata.name}')
    
    if [[ -n "$pod_name" ]]; then
        # Test health endpoint
        log_info "Testing health endpoint..."
        if kubectl exec -n "$NAMESPACE" "$pod_name" -- curl -f http://localhost:8081/healthz &> /dev/null; then
            log_success "Health endpoint is responding"
        else
            log_error "Health endpoint is not responding"
            return 1
        fi
        
        # Test readiness endpoint
        log_info "Testing readiness endpoint..."
        if kubectl exec -n "$NAMESPACE" "$pod_name" -- curl -f http://localhost:8081/readyz &> /dev/null; then
            log_success "Readiness endpoint is responding"
        else
            log_error "Readiness endpoint is not responding"
            return 1
        fi
        
        # Test metrics endpoint
        log_info "Testing metrics endpoint..."
        if kubectl exec -n "$NAMESPACE" "$pod_name" -- curl -f http://localhost:8080/metrics | grep -q "go_info" &> /dev/null; then
            log_success "Metrics endpoint is responding"
        else
            log_warning "Metrics endpoint is not responding (may be disabled)"
        fi
    else
        log_error "No operator pod found for health check"
        return 1
    fi
    
    log_success "Health endpoints validation passed"
}

run_smoke_tests() {
    log_info "Running smoke tests..."
    
    # Test 1: Create a simple JIRASync resource
    log_info "Test 1: Creating test JIRASync resource..."
    
    cat <<EOF | kubectl apply -f -
apiVersion: sync.jira.io/v1alpha1
kind: JIRASync
metadata:
  name: deployment-validation-test
  namespace: default
spec:
  jiraConfig:
    baseURL: "https://test.atlassian.net"
    credentialsSecret: "test-credentials"
  syncConfig:
    mode: "Single"
    issueKeys: ["TEST-1"]
  gitConfig:
    repository: "/tmp/test-repo"
EOF
    
    # Wait for resource to be processed
    sleep 5
    
    # Check if resource was created and has status
    if kubectl get jirasync deployment-validation-test -o jsonpath='{.metadata.name}' &> /dev/null; then
        log_success "Test JIRASync resource created successfully"
        
        # Check if operator updated the status
        local status_phase=$(kubectl get jirasync deployment-validation-test -o jsonpath='{.status.phase}' 2>/dev/null || echo "")
        if [[ -n "$status_phase" ]]; then
            log_success "Operator processed the resource (status: $status_phase)"
        else
            log_warning "Operator has not yet processed the resource"
        fi
    else
        log_error "Failed to create test JIRASync resource"
        return 1
    fi
    
    # Clean up test resource
    kubectl delete jirasync deployment-validation-test &> /dev/null || true
    log_info "Test resource cleaned up"
    
    # Test 2: Check operator logs for errors
    log_info "Test 2: Checking operator logs for errors..."
    
    local pod_name=$(kubectl get pods -n "$NAMESPACE" -l "app.kubernetes.io/name=$HELM_RELEASE" -o jsonpath='{.items[0].metadata.name}')
    local recent_logs=$(kubectl logs -n "$NAMESPACE" "$pod_name" --tail=50 2>/dev/null || echo "")
    
    if echo "$recent_logs" | grep -i error | grep -v "context canceled" &> /dev/null; then
        log_warning "Found error messages in operator logs:"
        echo "$recent_logs" | grep -i error | head -5
    else
        log_success "No critical errors found in operator logs"
    fi
    
    # Test 3: Leader election (if enabled)
    log_info "Test 3: Checking leader election..."
    
    local lease_name="jirasync.sync.jira.io"
    if kubectl get lease "$lease_name" -n "$NAMESPACE" &> /dev/null; then
        local holder=$(kubectl get lease "$lease_name" -n "$NAMESPACE" -o jsonpath='{.spec.holderIdentity}')
        log_success "Leader election active (holder: $holder)"
    else
        log_info "Leader election not found (may be disabled)"
    fi
    
    log_success "Smoke tests completed successfully"
}

check_integration() {
    log_info "Checking API server integration..."
    
    # Test API server connectivity from operator pod
    local pod_name=$(kubectl get pods -n "$NAMESPACE" -l "app.kubernetes.io/name=$HELM_RELEASE" -o jsonpath='{.items[0].metadata.name}')
    
    if [[ -n "$pod_name" ]]; then
        log_info "Testing API server connectivity..."
        
        # Get API server host from operator environment
        local api_host=$(kubectl get pod "$pod_name" -n "$NAMESPACE" -o jsonpath='{.spec.containers[0].args}' | grep -o 'api-server-host=[^[:space:]]*' | cut -d= -f2 || echo "")
        
        if [[ -n "$api_host" ]]; then
            log_info "API server host: $api_host"
            
            # Test connectivity (timeout after 10 seconds)
            if kubectl exec -n "$NAMESPACE" "$pod_name" -- timeout 10 curl -f "$api_host/api/v1/health" &> /dev/null; then
                log_success "API server is reachable from operator"
            else
                log_warning "API server is not reachable from operator (this may be expected in test environments)"
            fi
        else
            log_warning "API server host not configured"
        fi
    fi
    
    log_success "Integration check completed"
}

generate_report() {
    log_info "Generating validation report..."
    
    local report_file="/tmp/operator-validation-report-$(date +%Y%m%d-%H%M%S).txt"
    
    cat > "$report_file" <<EOF
JIRA Sync Operator Deployment Validation Report
Generated: $(date)
Namespace: $NAMESPACE
Helm Release: $HELM_RELEASE
Operator Image: $OPERATOR_IMAGE

DEPLOYMENT STATUS:
$(kubectl get deployment "$HELM_RELEASE" -n "$NAMESPACE" -o wide 2>/dev/null || echo "Deployment not found")

POD STATUS:
$(kubectl get pods -n "$NAMESPACE" -l "app.kubernetes.io/name=$HELM_RELEASE" -o wide 2>/dev/null || echo "No pods found")

CRD STATUS:
$(kubectl get crd | grep sync.jira.io 2>/dev/null || echo "No CRDs found")

RECENT OPERATOR LOGS:
$(kubectl logs -n "$NAMESPACE" -l "app.kubernetes.io/name=$HELM_RELEASE" --tail=20 2>/dev/null || echo "No logs available")

HELM STATUS:
$(helm status "$HELM_RELEASE" -n "$NAMESPACE" 2>/dev/null || echo "Helm release not found")
EOF
    
    log_success "Validation report generated: $report_file"
    echo "$report_file"
}

# Main execution
main() {
    log_info "Starting JIRA Sync Operator deployment validation..."
    log_info "Namespace: $NAMESPACE"
    log_info "Timeout: ${TIMEOUT}s"
    log_info "Helm Release: $HELM_RELEASE"
    
    # Run validation steps
    check_prerequisites
    check_namespace
    check_crds
    check_rbac
    check_deployment
    check_services
    check_health_endpoints
    run_smoke_tests
    check_integration
    
    # Generate report
    local report_file=$(generate_report)
    
    log_success "✅ All validation checks passed successfully!"
    log_info "Full report available at: $report_file"
    
    return 0
}

# Error handling
trap 'log_error "Validation failed at line $LINENO. Exit code: $?"' ERR

# Run main function if script is executed directly
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi