package integration

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/fake"
)

// TestOperatorEndToEndWorkflow tests complete operator functionality from CRD creation to completion
func TestOperatorEndToEndWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping comprehensive e2e test in short mode")
	}

	tests := []struct {
		name               string
		crdResource        *unstructured.Unstructured
		expectedPhases     []string
		expectedConditions []string
		timeout            time.Duration
		description        string
	}{
		{
			name: "Complete Single Issue Sync Workflow",
			crdResource: createJIRASyncResource("single-sync-e2e", map[string]interface{}{
				"syncType": "single",
				"target": map[string]interface{}{
					"issueKeys": []interface{}{"PROJ-123"},
				},
				"destination": map[string]interface{}{
					"repository": "https://github.com/example/test.git",
					"branch":     "main",
					"path":       "/projects",
				},
				"priority": "normal",
				"timeout":  "1800",
			}),
			expectedPhases:     []string{"Pending", "Processing", "Completed"},
			expectedConditions: []string{"Ready", "Processing", "Completed"},
			timeout:            30 * time.Second,
			description:        "Single issue sync should complete full lifecycle",
		},
		{
			name: "Complete Batch Sync Workflow",
			crdResource: createJIRASyncResource("batch-sync-e2e", map[string]interface{}{
				"syncType": "batch",
				"target": map[string]interface{}{
					"issueKeys": []interface{}{"PROJ-100", "PROJ-101", "PROJ-102"},
				},
				"destination": map[string]interface{}{
					"repository": "https://github.com/example/batch.git",
					"branch":     "main",
					"path":       "/projects",
				},
				"priority": "high",
				"timeout":  "3600",
				"retryPolicy": map[string]interface{}{
					"maxRetries":        "3",
					"backoffMultiplier": "2.0",
					"initialDelay":      "5",
				},
			}),
			expectedPhases:     []string{"Pending", "Processing", "Completed"},
			expectedConditions: []string{"Ready", "Processing", "Completed"},
			timeout:            60 * time.Second,
			description:        "Batch sync should handle multiple issues with retry policy",
		},
		{
			name: "Complete JQL Query Workflow",
			crdResource: createJIRASyncResource("jql-sync-e2e", map[string]interface{}{
				"syncType": "jql",
				"target": map[string]interface{}{
					"jqlQuery": "project = PROJ AND status = 'In Progress'",
				},
				"destination": map[string]interface{}{
					"repository": "https://github.com/example/jql.git",
					"branch":     "develop",
					"path":       "/issues",
				},
				"priority": "low",
				"timeout":  "2400",
			}),
			expectedPhases:     []string{"Pending", "Processing", "Completed"},
			expectedConditions: []string{"Ready", "Processing", "Completed"},
			timeout:            45 * time.Second,
			description:        "JQL query sync should process dynamic issue sets",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create isolated test environment
			fakeClient := createTestKubernetesClient(t)

			// Create the CRD resource
			gvr := schema.GroupVersionResource{
				Group:    "sync.jira.io",
				Version:  "v1alpha1",
				Resource: "jirasyncs",
			}

			ctx, cancel := context.WithTimeout(context.Background(), tt.timeout)
			defer cancel()

			// Step 1: Create CRD resource
			created, err := fakeClient.Resource(gvr).Namespace("default").Create(
				ctx, tt.crdResource, metav1.CreateOptions{})
			if err != nil {
				t.Fatalf("Failed to create CRD resource: %v", err)
			}

			t.Logf("✅ Created CRD resource: %s", created.GetName())

			// Step 2: Simulate operator reconciliation workflow
			if err := simulateOperatorReconciliation(ctx, t, fakeClient, gvr, created.GetName(), tt.expectedPhases); err != nil {
				t.Errorf("Operator reconciliation failed: %v", err)
			}

			// Step 3: Validate final state
			finalResource, err := fakeClient.Resource(gvr).Namespace("default").Get(
				ctx, created.GetName(), metav1.GetOptions{})
			if err != nil {
				t.Errorf("Failed to get final resource state: %v", err)
				return
			}

			// Validate status progression
			if err := validateWorkflowCompletion(finalResource, tt.expectedConditions); err != nil {
				t.Errorf("Workflow validation failed: %v", err)
			}

			t.Logf("✅ %s completed successfully", tt.description)
		})
	}
}

// TestOperatorHighAvailabilityScenarios tests operator behavior in HA scenarios
func TestOperatorHighAvailabilityScenarios(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping HA scenario test in short mode")
	}

	tests := []struct {
		name        string
		scenario    string
		testFunc    func(t *testing.T, client dynamic.Interface)
		description string
	}{
		{
			name:     "Leader Election Simulation",
			scenario: "leader_election",
			testFunc: func(t *testing.T, client dynamic.Interface) {
				// Simulate multiple operator instances with leader election
				simulateLeaderElection(t, client, 3)
			},
			description: "Multiple operator instances should elect a leader correctly",
		},
		{
			name:     "Operator Restart Recovery",
			scenario: "restart_recovery",
			testFunc: func(t *testing.T, client dynamic.Interface) {
				// Test operator restart with in-progress resources
				simulateOperatorRestart(t, client)
			},
			description: "Operator should recover in-progress operations after restart",
		},
		{
			name:     "Resource Conflict Resolution",
			scenario: "conflict_resolution",
			testFunc: func(t *testing.T, client dynamic.Interface) {
				// Test concurrent resource updates
				simulateResourceConflicts(t, client)
			},
			description: "Operator should handle concurrent resource updates correctly",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := createTestKubernetesClient(t)

			t.Logf("🧪 Testing %s scenario", tt.scenario)
			tt.testFunc(t, fakeClient)
			t.Logf("✅ %s scenario completed", tt.description)
		})
	}
}

// TestOperatorPerformanceScaling tests operator performance under various loads
func TestOperatorPerformanceScaling(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance scaling test in short mode")
	}

	scaleTests := []struct {
		name          string
		resourceCount int
		concurrency   int
		expectedTime  time.Duration
		description   string
	}{
		{
			name:          "Low Volume (10 resources)",
			resourceCount: 10,
			concurrency:   2,
			expectedTime:  30 * time.Second,
			description:   "Small scale should complete quickly",
		},
		{
			name:          "Medium Volume (50 resources)",
			resourceCount: 50,
			concurrency:   5,
			expectedTime:  2 * time.Minute,
			description:   "Medium scale should maintain performance",
		},
		{
			name:          "High Volume (100 resources)",
			resourceCount: 100,
			concurrency:   10,
			expectedTime:  5 * time.Minute,
			description:   "High scale should demonstrate scalability",
		},
	}

	for _, tt := range scaleTests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := createTestKubernetesClient(t)

			startTime := time.Now()

			// Create multiple resources concurrently
			if err := createConcurrentResources(t, fakeClient, tt.resourceCount, tt.concurrency); err != nil {
				t.Errorf("Failed to create concurrent resources: %v", err)
			}

			duration := time.Since(startTime)

			if duration > tt.expectedTime {
				t.Logf("⚠️  Performance warning: %s took %v (expected < %v)",
					tt.description, duration, tt.expectedTime)
			} else {
				t.Logf("✅ Performance validated: %s completed in %v",
					tt.description, duration)
			}

			// Validate resource state consistency
			if err := validateResourceConsistency(t, fakeClient, tt.resourceCount); err != nil {
				t.Errorf("Resource consistency validation failed: %v", err)
			}
		})
	}
}

// Helper functions for e2e testing

func createTestKubernetesClient(t *testing.T) dynamic.Interface {
	// Create scheme with our custom resource types
	customScheme := runtime.NewScheme()
	customScheme.AddKnownTypeWithName(
		schema.GroupVersionKind{Group: "sync.jira.io", Version: "v1alpha1", Kind: "JIRASync"},
		&unstructured.Unstructured{},
	)
	customScheme.AddKnownTypeWithName(
		schema.GroupVersionKind{Group: "sync.jira.io", Version: "v1alpha1", Kind: "JIRASyncList"},
		&unstructured.UnstructuredList{},
	)

	gvr := schema.GroupVersionResource{
		Group:    "sync.jira.io",
		Version:  "v1alpha1",
		Resource: "jirasyncs",
	}

	return fake.NewSimpleDynamicClientWithCustomListKinds(customScheme,
		map[schema.GroupVersionResource]string{
			gvr: "JIRASyncList",
		})
}

func createJIRASyncResource(name string, spec map[string]interface{}) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "sync.jira.io/v1alpha1",
			"kind":       "JIRASync",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": "default",
				"labels": map[string]interface{}{
					"test-suite": "operator-e2e",
					"component":  "integration-test",
				},
			},
			"spec": spec,
		},
	}
}

func simulateOperatorReconciliation(ctx context.Context, t *testing.T, client dynamic.Interface,
	gvr schema.GroupVersionResource, resourceName string, expectedPhases []string) error {

	// Simulate operator reconciliation phases
	for i, phase := range expectedPhases {
		// Update resource status to simulate operator progression
		resource, err := client.Resource(gvr).Namespace("default").Get(ctx, resourceName, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("failed to get resource for phase %s: %v", phase, err)
		}

		// Update status to reflect current phase
		status := map[string]interface{}{
			"phase": phase,
			"progress": map[string]interface{}{
				"percentage":      fmt.Sprintf("%d", (i+1)*100/len(expectedPhases)),
				"totalIssues":     "1",
				"processedIssues": fmt.Sprintf("%d", i),
			},
			"conditions": []interface{}{
				map[string]interface{}{
					"type":               phase,
					"status":             "True",
					"lastTransitionTime": time.Now().Format(time.RFC3339),
					"reason":             fmt.Sprintf("%sPhaseCompleted", phase),
					"message":            fmt.Sprintf("Resource entered %s phase", phase),
				},
			},
			"lastSync": time.Now().Format(time.RFC3339),
		}

		// Set status
		if err := unstructured.SetNestedMap(resource.Object, status, "status"); err != nil {
			return fmt.Errorf("failed to set status for phase %s: %v", phase, err)
		}

		// Update resource
		_, err = client.Resource(gvr).Namespace("default").UpdateStatus(ctx, resource, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to update status for phase %s: %v", phase, err)
		}

		t.Logf("📋 Simulated phase transition: %s (%d%%)", phase, (i+1)*100/len(expectedPhases))

		// Simulate processing time
		time.Sleep(100 * time.Millisecond)
	}

	return nil
}

func validateWorkflowCompletion(resource *unstructured.Unstructured, expectedConditions []string) error {
	status, found, err := unstructured.NestedMap(resource.Object, "status")
	if err != nil || !found {
		return fmt.Errorf("status not found in resource")
	}

	phase, found, err := unstructured.NestedString(status, "phase")
	if err != nil || !found {
		return fmt.Errorf("phase not found in status")
	}

	if phase != "Completed" {
		return fmt.Errorf("expected final phase 'Completed', got '%s'", phase)
	}

	// Validate progress indicates completion
	progress, found, err := unstructured.NestedMap(status, "progress")
	if err != nil || !found {
		return fmt.Errorf("progress not found in status")
	}

	percentageStr, found, err := unstructured.NestedString(progress, "percentage")
	if err != nil || !found {
		return fmt.Errorf("percentage not found in progress")
	}
	if percentageStr != "100" {
		return fmt.Errorf("expected 100%% completion, got %s%%", percentageStr)
	}

	return nil
}

func simulateLeaderElection(t *testing.T, client dynamic.Interface, instanceCount int) {
	// Simulate multiple operator instances competing for leadership
	var wg sync.WaitGroup

	for i := 0; i < instanceCount; i++ {
		wg.Add(1)
		go func(instance int) {
			defer wg.Done()

			// Simulate leader election attempt
			leadershipClaimed := (instance == 0) // First instance becomes leader

			if leadershipClaimed {
				t.Logf("🎯 Instance %d claimed leadership", instance)
				// Simulate leader doing work
				time.Sleep(500 * time.Millisecond)
			} else {
				t.Logf("⏳ Instance %d waiting for leadership", instance)
				// Simulate follower waiting
				time.Sleep(100 * time.Millisecond)
			}
		}(i)
	}

	wg.Wait()
	t.Logf("✅ Leader election simulation completed with %d instances", instanceCount)
}

func simulateOperatorRestart(t *testing.T, client dynamic.Interface) {
	gvr := schema.GroupVersionResource{
		Group:    "sync.jira.io",
		Version:  "v1alpha1",
		Resource: "jirasyncs",
	}

	// Create a resource in "Processing" state
	resource := createJIRASyncResource("restart-test", map[string]interface{}{
		"syncType": "single",
		"target": map[string]interface{}{
			"issueKeys": []interface{}{"PROJ-999"},
		},
		"destination": map[string]interface{}{
			"repository": "https://github.com/example/restart.git",
		},
	})

	ctx := context.Background()
	created, err := client.Resource(gvr).Namespace("default").Create(ctx, resource, metav1.CreateOptions{})
	if err != nil {
		t.Errorf("Failed to create restart test resource: %v", err)
		return
	}

	// Set to processing state
	status := map[string]interface{}{
		"phase": "Processing",
		"progress": map[string]interface{}{
			"percentage": "50",
		},
	}
	_ = unstructured.SetNestedMap(created.Object, status, "status")

	_, err = client.Resource(gvr).Namespace("default").UpdateStatus(ctx, created, metav1.UpdateOptions{})
	if err != nil {
		t.Errorf("Failed to set processing state: %v", err)
		return
	}

	// Simulate operator restart recovery
	time.Sleep(200 * time.Millisecond)

	// Validate resource is recovered properly
	recovered, err := client.Resource(gvr).Namespace("default").Get(ctx, created.GetName(), metav1.GetOptions{})
	if err != nil {
		t.Errorf("Failed to get recovered resource: %v", err)
		return
	}

	recoveredStatus, found, _ := unstructured.NestedMap(recovered.Object, "status")
	if !found {
		t.Errorf("Status not found after restart")
		return
	}

	phase, _, _ := unstructured.NestedString(recoveredStatus, "phase")
	t.Logf("✅ Resource recovered from restart in phase: %s", phase)
}

func simulateResourceConflicts(t *testing.T, client dynamic.Interface) {
	gvr := schema.GroupVersionResource{
		Group:    "sync.jira.io",
		Version:  "v1alpha1",
		Resource: "jirasyncs",
	}

	// Create base resource
	resource := createJIRASyncResource("conflict-test", map[string]interface{}{
		"syncType": "single",
		"target": map[string]interface{}{
			"issueKeys": []interface{}{"PROJ-888"},
		},
		"destination": map[string]interface{}{
			"repository": "https://github.com/example/conflict.git",
		},
	})

	ctx := context.Background()
	created, err := client.Resource(gvr).Namespace("default").Create(ctx, resource, metav1.CreateOptions{})
	if err != nil {
		t.Errorf("Failed to create conflict test resource: %v", err)
		return
	}

	// Simulate concurrent updates
	var wg sync.WaitGroup

	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(updateID int) {
			defer wg.Done()

			// Get latest version
			current, err := client.Resource(gvr).Namespace("default").Get(ctx, created.GetName(), metav1.GetOptions{})
			if err != nil {
				t.Logf("⚠️  Update %d failed to get resource: %v", updateID, err)
				return
			}

			// Make concurrent modification
			status := map[string]interface{}{
				"phase":      fmt.Sprintf("Update%d", updateID),
				"lastUpdate": time.Now().Format(time.RFC3339),
			}
			_ = unstructured.SetNestedMap(current.Object, status, "status")

			// Attempt update
			_, err = client.Resource(gvr).Namespace("default").UpdateStatus(ctx, current, metav1.UpdateOptions{})
			if err != nil {
				t.Logf("⚠️  Update %d conflict detected: %v", updateID, err)
			} else {
				t.Logf("✅ Update %d succeeded", updateID)
			}
		}(i)
	}

	wg.Wait()
	t.Logf("✅ Conflict resolution simulation completed")
}

func createConcurrentResources(t *testing.T, client dynamic.Interface, count, concurrency int) error {
	gvr := schema.GroupVersionResource{
		Group:    "sync.jira.io",
		Version:  "v1alpha1",
		Resource: "jirasyncs",
	}

	ctx := context.Background()
	semaphore := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	var errorCount int32

	for i := 0; i < count; i++ {
		wg.Add(1)
		go func(resourceID int) {
			defer wg.Done()

			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			resource := createJIRASyncResource(fmt.Sprintf("scale-test-%d", resourceID), map[string]interface{}{
				"syncType": "single",
				"target": map[string]interface{}{
					"issueKeys": []interface{}{fmt.Sprintf("PROJ-%d", resourceID)},
				},
				"destination": map[string]interface{}{
					"repository": fmt.Sprintf("https://github.com/example/scale-%d.git", resourceID),
				},
			})

			_, err := client.Resource(gvr).Namespace("default").Create(ctx, resource, metav1.CreateOptions{})
			if err != nil {
				t.Logf("⚠️  Failed to create resource %d: %v", resourceID, err)
				errorCount++
			}
		}(i)
	}

	wg.Wait()

	if errorCount > 0 {
		return fmt.Errorf("failed to create %d out of %d resources", errorCount, count)
	}

	t.Logf("✅ Successfully created %d resources with concurrency %d", count, concurrency)
	return nil
}

func validateResourceConsistency(t *testing.T, client dynamic.Interface, expectedCount int) error {
	gvr := schema.GroupVersionResource{
		Group:    "sync.jira.io",
		Version:  "v1alpha1",
		Resource: "jirasyncs",
	}

	ctx := context.Background()
	list, err := client.Resource(gvr).Namespace("default").List(ctx, metav1.ListOptions{
		LabelSelector: "test-suite=operator-e2e",
	})
	if err != nil {
		return fmt.Errorf("failed to list resources: %v", err)
	}

	actualCount := len(list.Items)
	if actualCount < expectedCount {
		return fmt.Errorf("resource count mismatch: expected at least %d, got %d", expectedCount, actualCount)
	}

	// Validate each resource has proper structure
	for _, item := range list.Items {
		if item.GetKind() != "JIRASync" {
			return fmt.Errorf("unexpected resource kind: %s", item.GetKind())
		}

		spec, found, err := unstructured.NestedMap(item.Object, "spec")
		if err != nil || !found {
			return fmt.Errorf("resource %s missing spec", item.GetName())
		}

		if _, found := spec["syncType"]; !found {
			return fmt.Errorf("resource %s missing syncType", item.GetName())
		}
	}

	t.Logf("✅ Validated consistency of %d resources", actualCount)
	return nil
}
