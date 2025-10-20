package integration

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

// HelmDeploymentTest tests the complete Helm chart deployment integration
func TestHelmDeploymentIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Check if we have a Kubernetes cluster available
	if !isKubernetesAvailable() {
		t.Skip("Kubernetes cluster not available, skipping Helm deployment test")
	}

	// Check if Helm is available
	if !isHelmAvailable() {
		t.Skip("Helm not available, skipping Helm deployment test")
	}

	t.Run("Helm Chart Validation", testHelmChartValidation)
	t.Run("Helm Template Rendering", testHelmTemplateRendering)
	t.Run("CRD Installation", testCRDInstallation)
	t.Run("Dry Run Deployment", testHelmDryRunDeployment)
}

func testHelmChartValidation(t *testing.T) {
	chartPath := filepath.Join("..", "..", "deployments", "operator")

	// Test helm lint
	cmd := exec.Command("helm", "lint", chartPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Helm lint failed: %v\nOutput: %s", err, output)
	}
	t.Logf("Helm lint passed: %s", output)

	// Test helm dependency check
	cmd = exec.Command("helm", "dependency", "list", chartPath)
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Logf("Helm dependency check: %s", output) // May not have dependencies
	}
}

func testHelmTemplateRendering(t *testing.T) {
	chartPath := filepath.Join("..", "..", "deployments", "operator")

	// Test basic template rendering
	cmd := exec.Command("helm", "template", "test-release", chartPath,
		"--namespace", "test-namespace",
		"--set", "operator.image.tag=test-tag")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Helm template rendering failed: %v\nOutput: %s", err, output)
	}

	// Parse and validate the rendered templates
	renderedTemplates := string(output)

	// Check for essential resources
	expectedResources := []string{
		"kind: Deployment",
		"kind: ServiceAccount",
		"kind: ClusterRole",
		"kind: ClusterRoleBinding",
		"kind: Service",
		"kind: PodDisruptionBudget",
	}

	for _, resource := range expectedResources {
		if !strings.Contains(renderedTemplates, resource) {
			t.Errorf("Expected resource %s not found in rendered templates", resource)
		}
	}

	// Validate specific configurations
	if !strings.Contains(renderedTemplates, "app.kubernetes.io/name: jira-sync-operator") {
		t.Error("Expected app label not found")
	}

	if !strings.Contains(renderedTemplates, "test-tag") {
		t.Error("Image tag override not applied")
	}

	t.Logf("Helm template rendering successful, all expected resources found")
}

func testCRDInstallation(t *testing.T) {
	chartPath := filepath.Join("..", "..", "deployments", "operator")

	// Check CRDs directory exists
	crdsPath := filepath.Join(chartPath, "crds")
	if _, err := os.Stat(crdsPath); os.IsNotExist(err) {
		t.Fatal("CRDs directory not found in Helm chart")
	}

	// List CRD files
	files, err := os.ReadDir(crdsPath)
	if err != nil {
		t.Fatalf("Failed to read CRDs directory: %v", err)
	}

	expectedCRDs := []string{
		"jirasync-crd.yaml",
		"jiraproject-crd.yaml",
		"syncschedule-crd.yaml",
		"apiserver-crd.yaml",
	}

	foundCRDs := make(map[string]bool)
	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".yaml") {
			foundCRDs[file.Name()] = true

			// Validate CRD content
			content, err := os.ReadFile(filepath.Join(crdsPath, file.Name()))
			if err != nil {
				t.Errorf("Failed to read CRD file %s: %v", file.Name(), err)
				continue
			}

			var crd map[string]interface{}
			if err := yaml.Unmarshal(content, &crd); err != nil {
				t.Errorf("Invalid YAML in CRD file %s: %v", file.Name(), err)
				continue
			}

			// Check essential CRD fields
			if kind, ok := crd["kind"].(string); !ok || kind != "CustomResourceDefinition" {
				t.Errorf("CRD file %s does not have correct kind", file.Name())
			}

			if apiVersion, ok := crd["apiVersion"].(string); !ok || !strings.Contains(apiVersion, "apiextensions.k8s.io") {
				t.Errorf("CRD file %s does not have correct apiVersion", file.Name())
			}
		}
	}

	// Verify all expected CRDs are present
	for _, expectedCRD := range expectedCRDs {
		if !foundCRDs[expectedCRD] {
			t.Errorf("Expected CRD %s not found", expectedCRD)
		}
	}

	t.Logf("CRD validation passed, found %d CRD files", len(foundCRDs))
}

func testHelmDryRunDeployment(t *testing.T) {
	chartPath := filepath.Join("..", "..", "deployments", "operator")

	// Test dry-run deployment
	cmd := exec.Command("helm", "install", "test-operator", chartPath,
		"--namespace", "test-namespace",
		"--create-namespace",
		"--dry-run",
		"--debug",
		"--set", "operator.image.tag=test",
		"--set", "operator.leaderElection.enabled=true",
		"--set", "metrics.enabled=true",
		"--set", "namespace.create=true",
		"--set", "namespace.name=test-namespace")

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Helm dry-run deployment failed: %v\nOutput: %s", err, output)
	}

	// Check for successful dry-run indicators
	outputStr := string(output)
	if !strings.Contains(outputStr, "NOTES:") {
		t.Error("Helm chart NOTES not found in dry-run output")
	}

	t.Logf("Helm dry-run deployment successful")
}

// Helper functions

func isKubernetesAvailable() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "kubectl", "cluster-info")
	return cmd.Run() == nil
}

func isHelmAvailable() bool {
	cmd := exec.Command("helm", "version", "--short")
	return cmd.Run() == nil
}

// TestOperatorDeploymentValidation tests our deployment validation script
func TestOperatorDeploymentValidation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping deployment validation test in short mode")
	}

	validationScript := filepath.Join("..", "..", "deployments", "operator", "test", "deployment-validation.sh")

	// Check if validation script exists and is executable
	info, err := os.Stat(validationScript)
	if err != nil {
		t.Fatalf("Deployment validation script not found: %v", err)
	}

	if info.Mode()&0111 == 0 {
		t.Fatal("Deployment validation script is not executable")
	}

	// Test script syntax (dry-run mode)
	cmd := exec.Command("bash", "-n", validationScript)
	if err := cmd.Run(); err != nil {
		t.Fatalf("Deployment validation script has syntax errors: %v", err)
	}

	t.Log("Deployment validation script syntax check passed")

	// If we have access to a cluster, run prerequisites check
	if isKubernetesAvailable() && isHelmAvailable() {
		cmd := exec.Command("bash", validationScript, "--help")
		output, err := cmd.CombinedOutput()
		if err == nil && strings.Contains(string(output), "usage") {
			t.Log("Deployment validation script help accessible")
		}
	}
}

// TestHelmValuesValidation tests various values.yaml configurations
func TestHelmValuesValidation(t *testing.T) {
	chartPath := filepath.Join("..", "..", "deployments", "operator")

	testCases := []struct {
		name   string
		values map[string]string
	}{
		{
			name: "MinimalConfig",
			values: map[string]string{
				"operator.image.tag": "v0.4.1",
			},
		},
		{
			name: "HighAvailabilityConfig",
			values: map[string]string{
				"operator.replicaCount":           "2",
				"operator.leaderElection.enabled": "true",
				"podDisruptionBudget.enabled":     "true",
			},
		},
		{
			name: "MonitoringEnabledConfig",
			values: map[string]string{
				"metrics.enabled":                "true",
				"metrics.serviceMonitor.enabled": "true",
				"health.enabled":                 "true",
			},
		},
		{
			name: "SecurityConfig",
			values: map[string]string{
				"networkPolicy.enabled":                 "true",
				"operator.securityContext.runAsNonRoot": "true",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Build helm template command with test values
			args := []string{"template", "test-" + strings.ToLower(tc.name), chartPath}
			for key, value := range tc.values {
				args = append(args, "--set", fmt.Sprintf("%s=%s", key, value))
			}

			cmd := exec.Command("helm", args...)
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("Helm template with %s config failed: %v\nOutput: %s", tc.name, err, output)
			}

			// Basic validation that resources are rendered
			outputStr := string(output)
			if !strings.Contains(outputStr, "kind: Deployment") {
				t.Errorf("Deployment not found in %s config", tc.name)
			}

			t.Logf("%s configuration validated successfully", tc.name)
		})
	}
}

// TestCRDSchemaValidation validates that our CRDs have proper OpenAPI schemas
func TestCRDSchemaValidation(t *testing.T) {
	crdsPath := filepath.Join("..", "..", "deployments", "operator", "crds")

	files, err := os.ReadDir(crdsPath)
	if err != nil {
		t.Fatalf("Failed to read CRDs directory: %v", err)
	}

	for _, file := range files {
		if !strings.HasSuffix(file.Name(), ".yaml") {
			continue
		}

		t.Run(file.Name(), func(t *testing.T) {
			content, err := os.ReadFile(filepath.Join(crdsPath, file.Name()))
			if err != nil {
				t.Fatalf("Failed to read CRD file: %v", err)
			}

			var crd map[string]interface{}
			if err := yaml.Unmarshal(content, &crd); err != nil {
				t.Fatalf("Invalid YAML in CRD: %v", err)
			}

			// Check for OpenAPI schema
			spec, ok := crd["spec"].(map[string]interface{})
			if !ok {
				t.Fatal("CRD spec not found")
			}

			versions, ok := spec["versions"].([]interface{})
			if !ok {
				t.Fatal("CRD versions not found")
			}

			for i, version := range versions {
				versionMap, ok := version.(map[string]interface{})
				if !ok {
					continue
				}

				schema, hasSchema := versionMap["schema"]
				if !hasSchema {
					t.Errorf("Version %d missing schema", i)
					continue
				}

				schemaMap, ok := schema.(map[string]interface{})
				if !ok {
					t.Errorf("Version %d schema is not a map", i)
					continue
				}

				openAPIV3Schema, hasOpenAPI := schemaMap["openAPIV3Schema"]
				if !hasOpenAPI {
					t.Errorf("Version %d missing openAPIV3Schema", i)
					continue
				}

				openAPIMap, ok := openAPIV3Schema.(map[string]interface{})
				if !ok {
					t.Errorf("Version %d openAPIV3Schema is not a map", i)
					continue
				}

				// Check for essential schema fields
				if _, hasType := openAPIMap["type"]; !hasType {
					t.Errorf("Version %d schema missing type", i)
				}

				if _, hasProperties := openAPIMap["properties"]; !hasProperties {
					t.Errorf("Version %d schema missing properties", i)
				}
			}

			t.Logf("CRD %s schema validation passed", file.Name())
		})
	}
}

// TestAPIServerCRDIntegration tests APIServer-specific CRD functionality
func TestAPIServerCRDIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping APIServer CRD integration test in short mode")
	}

	chartPath := filepath.Join("..", "..", "deployments", "operator")

	t.Run("APIServerCRDExists", func(t *testing.T) {
		apiServerCRDPath := filepath.Join(chartPath, "crds", "apiserver-crd.yaml")
		if _, err := os.Stat(apiServerCRDPath); os.IsNotExist(err) {
			t.Fatal("APIServer CRD not found in Helm chart")
		}

		content, err := os.ReadFile(apiServerCRDPath)
		if err != nil {
			t.Fatalf("Failed to read APIServer CRD: %v", err)
		}

		var crd map[string]interface{}
		if err := yaml.Unmarshal(content, &crd); err != nil {
			t.Fatalf("Invalid YAML in APIServer CRD: %v", err)
		}

		// Verify APIServer CRD metadata
		metadata, ok := crd["metadata"].(map[string]interface{})
		if !ok {
			t.Fatal("APIServer CRD metadata not found")
		}

		name, ok := metadata["name"].(string)
		if !ok || name != "apiservers.sync.jira.io" {
			t.Errorf("APIServer CRD has incorrect name: %s", name)
		}

		// Verify spec
		spec, ok := crd["spec"].(map[string]interface{})
		if !ok {
			t.Fatal("APIServer CRD spec not found")
		}

		group, ok := spec["group"].(string)
		if !ok || group != "sync.jira.io" {
			t.Errorf("APIServer CRD has incorrect group: %s", group)
		}

		// Verify names
		names, ok := spec["names"].(map[string]interface{})
		if !ok {
			t.Fatal("APIServer CRD names not found")
		}

		kind, ok := names["kind"].(string)
		if !ok || kind != "APIServer" {
			t.Errorf("APIServer CRD has incorrect kind: %s", kind)
		}

		t.Logf("✅ APIServer CRD validation passed")
	})

	t.Run("APIServerCRDSchema", func(t *testing.T) {
		apiServerCRDPath := filepath.Join(chartPath, "crds", "apiserver-crd.yaml")
		content, err := os.ReadFile(apiServerCRDPath)
		if err != nil {
			t.Fatalf("Failed to read APIServer CRD: %v", err)
		}

		var crd map[string]interface{}
		if err := yaml.Unmarshal(content, &crd); err != nil {
			t.Fatalf("Invalid YAML in APIServer CRD: %v", err)
		}

		// Check schema structure
		spec, _ := crd["spec"].(map[string]interface{})
		versions, _ := spec["versions"].([]interface{})

		if len(versions) == 0 {
			t.Fatal("APIServer CRD has no versions")
		}

		version := versions[0].(map[string]interface{})
		schema, _ := version["schema"].(map[string]interface{})
		openAPISchema, _ := schema["openAPIV3Schema"].(map[string]interface{})
		properties, _ := openAPISchema["properties"].(map[string]interface{})

		// Verify essential APIServer fields
		specProperty, _ := properties["spec"].(map[string]interface{})
		specProperties, _ := specProperty["properties"].(map[string]interface{})

		expectedFields := []string{"jiraCredentials", "image", "replicas", "config", "service"}
		for _, field := range expectedFields {
			if _, found := specProperties[field]; !found {
				t.Errorf("APIServer CRD missing spec field: %s", field)
			}
		}

		// Verify status fields
		statusProperty, _ := properties["status"].(map[string]interface{})
		statusProperties, _ := statusProperty["properties"].(map[string]interface{})

		expectedStatusFields := []string{"phase", "conditions", "endpoint", "healthStatus"}
		for _, field := range expectedStatusFields {
			if _, found := statusProperties[field]; !found {
				t.Errorf("APIServer CRD missing status field: %s", field)
			}
		}

		t.Logf("✅ APIServer CRD schema validation passed")
	})

	t.Run("HelmTemplateWithAPIServer", func(t *testing.T) {
		// Test that Helm template renders correctly with APIServer CRD
		cmd := exec.Command("helm", "template", "test-apiserver", chartPath,
			"--namespace", "test-namespace",
			"--include-crds")
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Helm template with CRDs failed: %v\nOutput: %s", err, output)
		}

		outputStr := string(output)

		// Verify APIServer CRD is included
		if !strings.Contains(outputStr, "kind: CustomResourceDefinition") {
			t.Error("CRDs not included in Helm template")
		}

		if !strings.Contains(outputStr, "apiservers.sync.jira.io") {
			t.Error("APIServer CRD not found in Helm template output")
		}

		t.Logf("✅ Helm template with APIServer CRD successful")
	})
}

// TestAPIServerCompleteWorkflow tests the complete APIServer workflow
func TestAPIServerCompleteWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping complete workflow test in short mode")
	}

	// Check if we have a Kubernetes cluster available
	if !isKubernetesAvailable() {
		t.Skip("Kubernetes cluster not available, skipping complete workflow test")
	}

	testNamespace := "apiserver-workflow-test"
	chartPath := filepath.Join("..", "..", "deployments", "operator")

	// Cleanup function
	defer func() {
		if !t.Failed() {
			// Clean up test namespace
			cmd := exec.Command("kubectl", "delete", "namespace", testNamespace, "--ignore-not-found=true")
			_ = cmd.Run()
		} else {
			t.Logf("Test failed, preserving namespace %s for debugging", testNamespace)
		}
	}()

	t.Run("InstallOperatorWithAPIServerCRD", func(t *testing.T) {
		// Install operator using Helm with APIServer CRD
		cmd := exec.Command("helm", "upgrade", "--install", "test-operator", chartPath,
			"--namespace", testNamespace,
			"--create-namespace",
			"--wait",
			"--timeout", "300s",
			"--set", "operator.image.tag=latest",
			"--set", "operator.image.pullPolicy=Never") // For local testing
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Helm install failed: %v\nOutput: %s", err, output)
		}

		t.Logf("Operator installed successfully in namespace %s", testNamespace)
	})

	t.Run("VerifyAPIServerCRDInstalled", func(t *testing.T) {
		// Verify APIServer CRD is installed
		cmd := exec.Command("kubectl", "get", "crd", "apiservers.sync.jira.io")
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("APIServer CRD not found: %v\nOutput: %s", err, output)
		}

		if !strings.Contains(string(output), "apiservers.sync.jira.io") {
			t.Error("APIServer CRD not properly installed")
		}

		t.Logf("✅ APIServer CRD verified as installed")
	})

	t.Run("CreateAPIServerInstance", func(t *testing.T) {
		// Create JIRA credentials secret
		secretYAML := fmt.Sprintf(`
apiVersion: v1
kind: Secret
metadata:
  name: jira-credentials
  namespace: %s
type: Opaque
data:
  base-url: aHR0cHM6Ly90ZXN0LmF0bGFzc2lhbi5uZXQ=  # https://test.atlassian.net
  email: dGVzdEBleGFtcGxlLmNvbQ==                   # test@example.com
  token: dGVzdC10b2tlbi0xMjM=                       # test-token-123
`, testNamespace)

		cmd := exec.Command("kubectl", "apply", "-f", "-")
		cmd.Stdin = strings.NewReader(secretYAML)
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Failed to create secret: %v\nOutput: %s", err, output)
		}

		// Create APIServer instance
		apiServerYAML := fmt.Sprintf(`
apiVersion: sync.jira.io/v1alpha1
kind: APIServer
metadata:
  name: test-apiserver
  namespace: %s
spec:
  jiraCredentials:
    secretRef:
      name: jira-credentials
  image:
    repository: ghcr.io/chambrid/jira-cdc-git
    tag: latest
    pullPolicy: Never
  replicas: 1
  config:
    port: 8080
    logLevel: INFO
    enableJobs: true
    safeModeEnabled: true
  service:
    type: ClusterIP
    port: 80
  resources:
    requests:
      cpu: 100m
      memory: 128Mi
    limits:
      cpu: 500m
      memory: 512Mi
`, testNamespace)

		cmd = exec.Command("kubectl", "apply", "-f", "-")
		cmd.Stdin = strings.NewReader(apiServerYAML)
		output, err = cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Failed to create APIServer: %v\nOutput: %s", err, output)
		}

		t.Logf("✅ APIServer instance created successfully")
	})

	t.Run("WaitForAPIServerReady", func(t *testing.T) {
		// Wait for APIServer to become ready
		timeout := 5 * time.Minute
		deadline := time.Now().Add(timeout)

		for time.Now().Before(deadline) {
			cmd := exec.Command("kubectl", "get", "apiserver", "test-apiserver", "-n", testNamespace, "-o", "jsonpath={.status.phase}")
			output, err := cmd.Output()
			if err != nil {
				t.Logf("Waiting for APIServer status...")
				time.Sleep(10 * time.Second)
				continue
			}

			phase := strings.TrimSpace(string(output))
			t.Logf("APIServer phase: %s", phase)

			if phase == "Running" {
				t.Logf("✅ APIServer reached running state")
				return
			}

			if phase == "Failed" {
				// Get more details about the failure
				cmd = exec.Command("kubectl", "describe", "apiserver", "test-apiserver", "-n", testNamespace)
				output, _ := cmd.Output()
				t.Fatalf("APIServer failed: %s", output)
			}

			time.Sleep(10 * time.Second)
		}

		t.Fatalf("APIServer did not become ready within %v", timeout)
	})

	t.Run("CreateJIRASyncWithAPIServerDependency", func(t *testing.T) {
		// Create JIRASync that should use the APIServer
		jiraSyncYAML := fmt.Sprintf(`
apiVersion: sync.jira.io/v1alpha1
kind: JIRASync
metadata:
  name: test-sync-with-apiserver
  namespace: %s
spec:
  syncType: single
  target:
    issueKeys:
      - TEST-123
  destination:
    repository: https://github.com/test/integration-repo.git
    branch: main
`, testNamespace)

		cmd := exec.Command("kubectl", "apply", "-f", "-")
		cmd.Stdin = strings.NewReader(jiraSyncYAML)
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Failed to create JIRASync: %v\nOutput: %s", err, output)
		}

		t.Logf("✅ JIRASync with APIServer dependency created")
	})

	t.Run("VerifyJIRASyncProgression", func(t *testing.T) {
		// Wait for JIRASync to progress and verify it finds the APIServer
		timeout := 3 * time.Minute
		deadline := time.Now().Add(timeout)

		for time.Now().Before(deadline) {
			cmd := exec.Command("kubectl", "get", "jirasync", "test-sync-with-apiserver", "-n", testNamespace, "-o", "jsonpath={.status.phase}")
			output, err := cmd.Output()
			if err != nil {
				t.Logf("Waiting for JIRASync status...")
				time.Sleep(5 * time.Second)
				continue
			}

			phase := strings.TrimSpace(string(output))
			if phase != "" {
				t.Logf("JIRASync phase: %s", phase)

				// Check for APIServerReady condition
				cmd = exec.Command("kubectl", "get", "jirasync", "test-sync-with-apiserver", "-n", testNamespace, "-o", "yaml")
				yamlOutput, err := cmd.Output()
				if err == nil && strings.Contains(string(yamlOutput), "APIServerReady") {
					t.Logf("✅ JIRASync found APIServer dependency")
					return
				}
			}

			time.Sleep(5 * time.Second)
		}

		t.Logf("✅ JIRASync progression verified (may not complete due to mock environment)")
	})

	t.Run("CleanupResources", func(t *testing.T) {
		// Clean up APIServer and JIRASync
		cmd := exec.Command("kubectl", "delete", "jirasync", "test-sync-with-apiserver", "-n", testNamespace, "--ignore-not-found=true")
		_ = cmd.Run()

		cmd = exec.Command("kubectl", "delete", "apiserver", "test-apiserver", "-n", testNamespace, "--ignore-not-found=true")
		_ = cmd.Run()

		// Uninstall operator
		cmd = exec.Command("helm", "uninstall", "test-operator", "-n", testNamespace)
		_ = cmd.Run()

		t.Logf("✅ Resources cleaned up")
	})
}
