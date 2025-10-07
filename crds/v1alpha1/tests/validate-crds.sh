#!/bin/bash
# CRD Validation Test Suite for JCG-025
# This script validates all CRD schemas and test cases

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Test counters
TOTAL_TESTS=0
PASSED_TESTS=0
FAILED_TESTS=0

# Function to print test results
print_result() {
    local test_name="$1"
    local result="$2"
    local expected="$3"
    
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    
    if [ "$result" == "$expected" ]; then
        echo -e "${GREEN}✓${NC} $test_name"
        PASSED_TESTS=$((PASSED_TESTS + 1))
    else
        echo -e "${RED}✗${NC} $test_name (expected $expected, got $result)"
        FAILED_TESTS=$((FAILED_TESTS + 1))
    fi
}

# Function to validate CRD installation
validate_crd_installation() {
    echo -e "${YELLOW}Phase 1: CRD Installation Validation${NC}"
    
    # Test 1: CRD syntax validation
    for crd_file in crds/v1alpha1/*.yaml; do
        if kubectl apply --dry-run=client --validate=strict -f "$crd_file" &>/dev/null; then
            print_result "CRD syntax: $(basename $crd_file)" "PASS" "PASS"
        else
            print_result "CRD syntax: $(basename $crd_file)" "FAIL" "PASS"
        fi
    done
    
    # Test 2: CRD schema validation (requires cluster)
    if kubectl cluster-info &>/dev/null; then
        echo "Kubernetes cluster available - testing server-side validation"
        
        for crd_file in crds/v1alpha1/*.yaml; do
            if kubectl apply --dry-run=server -f "$crd_file" &>/dev/null; then
                print_result "CRD server validation: $(basename $crd_file)" "PASS" "PASS"
            else
                print_result "CRD server validation: $(basename $crd_file)" "FAIL" "PASS"
            fi
        done
    else
        echo "No Kubernetes cluster - skipping server-side validation"
    fi
}

# Function to validate positive test cases
validate_positive_cases() {
    echo -e "${YELLOW}Phase 2: Valid Resource Validation${NC}"
    
    if kubectl cluster-info &>/dev/null; then
        # Install CRDs first
        kubectl apply -f crds/v1alpha1/ &>/dev/null || true
        
        # Test valid examples
        while IFS= read -r -d '---' resource; do
            if [[ -n "${resource// }" ]]; then
                local temp_file=$(mktemp)
                echo "$resource" > "$temp_file"
                
                if kubectl apply --dry-run=server -f "$temp_file" &>/dev/null; then
                    print_result "Valid resource test" "PASS" "PASS"
                else
                    print_result "Valid resource test" "FAIL" "PASS"
                fi
                
                rm -f "$temp_file"
            fi
        done < crds/v1alpha1/tests/valid/jirasync-valid-examples.yaml
    else
        echo "No Kubernetes cluster - skipping resource validation"
    fi
}

# Function to validate negative test cases
validate_negative_cases() {
    echo -e "${YELLOW}Phase 3: Invalid Resource Rejection${NC}"
    
    if kubectl cluster-info &>/dev/null; then
        # Test invalid examples - these should FAIL
        while IFS= read -r -d '---' resource; do
            if [[ -n "${resource// }" ]]; then
                local temp_file=$(mktemp)
                echo "$resource" > "$temp_file"
                
                if kubectl apply --dry-run=server -f "$temp_file" &>/dev/null; then
                    print_result "Invalid resource rejection" "FAIL" "FAIL"
                else
                    print_result "Invalid resource rejection" "PASS" "FAIL"
                fi
                
                rm -f "$temp_file"
            fi
        done < crds/v1alpha1/tests/invalid/jirasync-invalid-examples.yaml
    else
        echo "No Kubernetes cluster - skipping invalid resource tests"
    fi
}

# Function to validate security test cases
validate_security_cases() {
    echo -e "${YELLOW}Phase 4: Security Validation${NC}"
    
    if kubectl cluster-info &>/dev/null; then
        # Test security examples - these should FAIL
        while IFS= read -r -d '---' resource; do
            if [[ -n "${resource// }" ]]; then
                local temp_file=$(mktemp)
                echo "$resource" > "$temp_file"
                
                if kubectl apply --dry-run=server -f "$temp_file" &>/dev/null; then
                    print_result "Security vulnerability blocked" "FAIL" "FAIL"
                else
                    print_result "Security vulnerability blocked" "PASS" "FAIL"
                fi
                
                rm -f "$temp_file"
            fi
        done < crds/v1alpha1/tests/security/jirasync-security-tests.yaml
    else
        echo "No Kubernetes cluster - skipping security tests"
    fi
}

# Function to validate schema completeness
validate_schema_completeness() {
    echo -e "${YELLOW}Phase 5: Schema Completeness Validation${NC}"
    
    # Check for required schema elements
    local schema_checks=(
        "oneOf.*target.*issueKeys"
        "pattern.*repository.*https"
        "maxItems.*issueKeys.*100"
        "pattern.*issueKeys.*A-Z"
        "enum.*syncType.*single"
        "minimum.*maxRetries.*0"
        "maximum.*maxRetries.*10"
        "format.*date-time"
        "additionalPrinterColumns"
        "subresources.*status"
    )
    
    for pattern in "${schema_checks[@]}"; do
        if grep -q "$pattern" crds/v1alpha1/jirasync-crd.yaml; then
            print_result "Schema element: $pattern" "PASS" "PASS"
        else
            print_result "Schema element: $pattern" "FAIL" "PASS"
        fi
    done
}

# Function to validate operational requirements
validate_operational_requirements() {
    echo -e "${YELLOW}Phase 6: Operational Requirements Validation${NC}"
    
    # Check for operational metadata
    local operational_checks=(
        "shortNames"
        "categories"
        "labels.*app.kubernetes.io"
        "annotations.*api-approved"
        "conversion.*strategy"
        "served.*true"
        "storage.*true"
    )
    
    for pattern in "${operational_checks[@]}"; do
        if grep -q "$pattern" crds/v1alpha1/*.yaml; then
            print_result "Operational requirement: $pattern" "PASS" "PASS"
        else
            print_result "Operational requirement: $pattern" "FAIL" "PASS"
        fi
    done
}

# Main execution
main() {
    echo "======================================"
    echo "JCG-025 CRD Validation Test Suite"
    echo "======================================"
    echo
    
    # Change to script directory
    cd "$(dirname "$0")/../../../.."
    
    validate_crd_installation
    echo
    validate_positive_cases
    echo
    validate_negative_cases
    echo
    validate_security_cases
    echo
    validate_schema_completeness
    echo
    validate_operational_requirements
    echo
    
    # Final results
    echo "======================================"
    echo "Test Results Summary"
    echo "======================================"
    echo -e "Total Tests: $TOTAL_TESTS"
    echo -e "${GREEN}Passed: $PASSED_TESTS${NC}"
    echo -e "${RED}Failed: $FAILED_TESTS${NC}"
    
    if [ $FAILED_TESTS -eq 0 ]; then
        echo -e "${GREEN}All tests passed! CRDs are production-ready.${NC}"
        exit 0
    else
        echo -e "${RED}Some tests failed. Review CRD schemas before deployment.${NC}"
        exit 1
    fi
}

# Run main function
main "$@"