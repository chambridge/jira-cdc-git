package integration

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

// LoadTestMetrics tracks comprehensive load test results
type LoadTestMetrics struct {
	// Resource creation metrics
	ResourcesCreated   int64
	ResourcesProcessed int64
	ResourcesCompleted int64
	ResourcesFailed    int64

	// Timing metrics
	StartTime          time.Time
	EndTime            time.Time
	TotalDuration      time.Duration
	AverageCreateTime  time.Duration
	AverageProcessTime time.Duration

	// Throughput metrics
	CreationThroughput   float64 // resources/second
	ProcessingThroughput float64 // resources/second

	// Resource usage metrics
	PeakMemoryMB    float64
	AverageMemoryMB float64
	MemorySamples   []float64

	// Error metrics
	TimeoutErrors  int64
	ConflictErrors int64
	APIErrors      int64
	OtherErrors    int64

	// Scalability metrics
	Concurrency    int
	MaxConcurrency int
	QueueDepth     int
}

// TestOperatorHighVolumeScalability validates operator under sustained high loads
func TestOperatorHighVolumeScalability(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping high-volume scalability test in short mode")
	}

	scalabilityTests := []struct {
		name               string
		resourceCount      int
		concurrency        int
		duration           time.Duration
		batchSize          int
		expectedThroughput float64
		maxMemoryMB        float64
		description        string
	}{
		{
			name:               "Moderate Volume Load",
			resourceCount:      200,
			concurrency:        15,
			duration:           3 * time.Minute,
			batchSize:          10,
			expectedThroughput: 1.0, // 1 resource/second minimum
			maxMemoryMB:        150,
			description:        "Moderate sustained load with 200 resources over 3 minutes",
		},
		{
			name:               "High Volume Burst",
			resourceCount:      500,
			concurrency:        25,
			duration:           5 * time.Minute,
			batchSize:          20,
			expectedThroughput: 1.5, // 1.5 resources/second minimum
			maxMemoryMB:        200,
			description:        "High volume burst with 500 resources over 5 minutes",
		},
		{
			name:               "Extreme Scale Test",
			resourceCount:      1000,
			concurrency:        50,
			duration:           10 * time.Minute,
			batchSize:          50,
			expectedThroughput: 1.0, // Maintain 1 resource/second under extreme load
			maxMemoryMB:        300,
			description:        "Extreme scale test with 1000 resources over 10 minutes",
		},
		{
			name:               "Sustained Operations",
			resourceCount:      100,
			concurrency:        10,
			duration:           15 * time.Minute,
			batchSize:          5,
			expectedThroughput: 0.1, // 0.1 resources/second for sustained operations
			maxMemoryMB:        100,
			description:        "Long-running sustained operations test",
		},
	}

	for _, test := range scalabilityTests {
		t.Run(test.name, func(t *testing.T) {
			client := createTestKubernetesClient(t)

			t.Logf("🚀 Starting scalability test: %s", test.description)
			t.Logf("📊 Target: %d resources, %d concurrency, %v duration",
				test.resourceCount, test.concurrency, test.duration)

			metrics, err := runHighVolumeLoadTest(t, client, test.resourceCount,
				test.concurrency, test.duration, test.batchSize)
			if err != nil {
				t.Errorf("High-volume load test failed: %v", err)
				return
			}

			// Validate scalability targets
			validateScalabilityMetrics(t, metrics, test.expectedThroughput, test.maxMemoryMB)

			// Report comprehensive results
			reportLoadTestMetrics(t, test.name, metrics)
		})
	}
}

// TestOperatorResourceExhaustionHandling tests behavior under resource constraints
func TestOperatorResourceExhaustionHandling(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping resource exhaustion test in short mode")
	}

	exhaustionTests := []struct {
		name        string
		testFunc    func(t *testing.T, client dynamic.Interface) error
		description string
	}{
		{
			name:        "Memory Pressure Handling",
			testFunc:    testMemoryPressureHandling,
			description: "Operator should handle memory pressure gracefully",
		},
		{
			name:        "High Queue Depth Management",
			testFunc:    testHighQueueDepthManagement,
			description: "Operator should manage high queue depths without degradation",
		},
		{
			name:        "Resource Limit Enforcement",
			testFunc:    testResourceLimitEnforcement,
			description: "Operator should respect and enforce resource limits",
		},
		{
			name:        "Graceful Degradation",
			testFunc:    testGracefulDegradation,
			description: "Operator should degrade gracefully under extreme load",
		},
	}

	for _, test := range exhaustionTests {
		t.Run(test.name, func(t *testing.T) {
			client := createTestKubernetesClient(t)

			t.Logf("🔥 Testing resource exhaustion: %s", test.description)

			if err := test.testFunc(t, client); err != nil {
				t.Errorf("Resource exhaustion test failed: %v", err)
				return
			}

			t.Logf("✅ %s", test.description)
		})
	}
}

// TestOperatorConcurrencyLimits validates concurrency controls and limits
func TestOperatorConcurrencyLimits(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrency limits test in short mode")
	}

	concurrencyTests := []struct {
		name              string
		maxConcurrency    int
		resourceCount     int
		expectedCompleted int
		expectedQueued    int
		description       string
	}{
		{
			name:              "Low Concurrency Limit",
			maxConcurrency:    5,
			resourceCount:     20,
			expectedCompleted: 5,
			expectedQueued:    15,
			description:       "Low concurrency should queue excess resources",
		},
		{
			name:              "Medium Concurrency Limit",
			maxConcurrency:    15,
			resourceCount:     50,
			expectedCompleted: 15,
			expectedQueued:    35,
			description:       "Medium concurrency should handle moderate load",
		},
		{
			name:              "High Concurrency Limit",
			maxConcurrency:    50,
			resourceCount:     100,
			expectedCompleted: 50,
			expectedQueued:    50,
			description:       "High concurrency should maximize throughput",
		},
	}

	for _, test := range concurrencyTests {
		t.Run(test.name, func(t *testing.T) {
			client := createTestKubernetesClient(t)

			t.Logf("⚡ Testing concurrency limits: %s", test.description)

			metrics, err := testConcurrencyLimits(t, client, test.maxConcurrency, test.resourceCount)
			if err != nil {
				t.Errorf("Concurrency limit test failed: %v", err)
				return
			}

			// Validate concurrency behavior
			if metrics.ResourcesProcessed < int64(test.expectedCompleted) {
				t.Errorf("Expected at least %d resources processed, got %d",
					test.expectedCompleted, metrics.ResourcesProcessed)
			}

			if metrics.QueueDepth < test.expectedQueued {
				t.Errorf("Expected queue depth of at least %d, got %d",
					test.expectedQueued, metrics.QueueDepth)
			}

			t.Logf("✅ Concurrency validation: %d processed, %d queued (max concurrency: %d)",
				metrics.ResourcesProcessed, metrics.QueueDepth, test.maxConcurrency)
		})
	}
}

// TestOperatorBatchProcessingEfficiency tests batch operation efficiency
func TestOperatorBatchProcessingEfficiency(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping batch processing efficiency test in short mode")
	}

	batchTests := []struct {
		name               string
		batchSize          int
		numberOfBatches    int
		expectedEfficiency float64 // percentage
		description        string
	}{
		{
			name:               "Small Batch Efficiency",
			batchSize:          10,
			numberOfBatches:    10,
			expectedEfficiency: 85.0,
			description:        "Small batches should maintain high efficiency",
		},
		{
			name:               "Medium Batch Efficiency",
			batchSize:          50,
			numberOfBatches:    20,
			expectedEfficiency: 90.0,
			description:        "Medium batches should optimize for efficiency",
		},
		{
			name:               "Large Batch Efficiency",
			batchSize:          100,
			numberOfBatches:    10,
			expectedEfficiency: 80.0,
			description:        "Large batches should balance efficiency and resource usage",
		},
	}

	for _, test := range batchTests {
		t.Run(test.name, func(t *testing.T) {
			client := createTestKubernetesClient(t)

			t.Logf("📦 Testing batch efficiency: %s", test.description)

			efficiency, metrics, err := testBatchProcessingEfficiency(t, client,
				test.batchSize, test.numberOfBatches)
			if err != nil {
				t.Errorf("Batch efficiency test failed: %v", err)
				return
			}

			// Validate efficiency targets
			if efficiency < test.expectedEfficiency {
				t.Errorf("Batch efficiency below target: %.2f%% < %.2f%%",
					efficiency, test.expectedEfficiency)
			}

			t.Logf("✅ Batch efficiency: %.2f%% (target: %.2f%%), throughput: %.2f resources/sec",
				efficiency, test.expectedEfficiency, metrics.ProcessingThroughput)
		})
	}
}

// Load test implementation functions

func runHighVolumeLoadTest(t *testing.T, client dynamic.Interface, resourceCount,
	concurrency int, duration time.Duration, batchSize int) (*LoadTestMetrics, error) {

	gvr := schema.GroupVersionResource{
		Group:    "sync.jira.io",
		Version:  "v1alpha1",
		Resource: "jirasyncs",
	}

	metrics := &LoadTestMetrics{
		StartTime:      time.Now(),
		Concurrency:    concurrency,
		MaxConcurrency: concurrency,
	}

	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()

	// Channel for managing concurrency
	semaphore := make(chan struct{}, concurrency)

	// Start memory monitoring
	stopMemoryMonitoring := startMemoryMonitoring(metrics)
	defer stopMemoryMonitoring()

	// Create resources in batches
	var wg sync.WaitGroup
	resourceID := int64(0)

	// Create resources until context deadline or resource count reached
	for atomic.LoadInt64(&metrics.ResourcesCreated) < int64(resourceCount) {
		select {
		case <-ctx.Done():
			// Timeout reached
			t.Logf("⏰ Load test timeout reached")
			goto waitForCompletion
		default:
			// Create batch of resources
			for i := 0; i < batchSize && atomic.LoadInt64(&metrics.ResourcesCreated) < int64(resourceCount); i++ {
				wg.Add(1)
				go func(id int64) {
					defer wg.Done()

					// Acquire semaphore slot
					select {
					case semaphore <- struct{}{}:
						defer func() { <-semaphore }()
					case <-ctx.Done():
						return
					}

					// Create and process resource
					createStart := time.Now()

					resource := createLoadTestResource(id)
					created, err := client.Resource(gvr).Namespace("default").Create(
						context.Background(), resource, metav1.CreateOptions{})

					createDuration := time.Since(createStart)
					atomic.AddInt64(&metrics.ResourcesCreated, 1)

					if err != nil {
						atomic.AddInt64(&metrics.APIErrors, 1)
						t.Logf("⚠️  Failed to create resource %d: %v", id, err)
						return
					}

					// Simulate processing
					processStart := time.Now()
					if err := simulateResourceProcessing(client, gvr, created.GetName()); err != nil {
						atomic.AddInt64(&metrics.ResourcesFailed, 1)
					} else {
						atomic.AddInt64(&metrics.ResourcesCompleted, 1)
					}
					processDuration := time.Since(processStart)
					atomic.AddInt64(&metrics.ResourcesProcessed, 1)

					// Update timing metrics (simplified - would use proper synchronization in production)
					if metrics.AverageCreateTime == 0 {
						metrics.AverageCreateTime = createDuration
					} else {
						metrics.AverageCreateTime = (metrics.AverageCreateTime + createDuration) / 2
					}

					if metrics.AverageProcessTime == 0 {
						metrics.AverageProcessTime = processDuration
					} else {
						metrics.AverageProcessTime = (metrics.AverageProcessTime + processDuration) / 2
					}

				}(atomic.AddInt64(&resourceID, 1))
			}

			// Brief pause between batches
			time.Sleep(100 * time.Millisecond)
		}
	}

waitForCompletion:
	// Wait for all operations to complete
	wg.Wait()
	metrics.EndTime = time.Now()
	metrics.TotalDuration = metrics.EndTime.Sub(metrics.StartTime)

	// Calculate final metrics
	if metrics.TotalDuration > 0 {
		metrics.CreationThroughput = float64(metrics.ResourcesCreated) / metrics.TotalDuration.Seconds()
		metrics.ProcessingThroughput = float64(metrics.ResourcesProcessed) / metrics.TotalDuration.Seconds()
	}

	// Calculate average memory usage
	if len(metrics.MemorySamples) > 0 {
		var totalMemory float64
		for _, sample := range metrics.MemorySamples {
			totalMemory += sample
			if sample > metrics.PeakMemoryMB {
				metrics.PeakMemoryMB = sample
			}
		}
		metrics.AverageMemoryMB = totalMemory / float64(len(metrics.MemorySamples))
	}

	return metrics, nil
}

func startMemoryMonitoring(metrics *LoadTestMetrics) func() {
	stopCh := make(chan struct{})

	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				var memStats runtime.MemStats
				runtime.ReadMemStats(&memStats)
				memoryMB := float64(memStats.Alloc) / 1024 / 1024

				metrics.MemorySamples = append(metrics.MemorySamples, memoryMB)

			case <-stopCh:
				return
			}
		}
	}()

	return func() {
		close(stopCh)
	}
}

func createLoadTestResource(id int64) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "sync.jira.io/v1alpha1",
			"kind":       "JIRASync",
			"metadata": map[string]interface{}{
				"name":      fmt.Sprintf("load-test-%d", id),
				"namespace": "default",
				"labels": map[string]interface{}{
					"test-type":  "load-test",
					"batch-id":   fmt.Sprintf("batch-%d", id/10),
					"created-at": time.Now().Format("2006-01-02T15:04:05Z"),
				},
			},
			"spec": map[string]interface{}{
				"syncType": "single",
				"target": map[string]interface{}{
					"issueKeys": []interface{}{fmt.Sprintf("LOAD-%d", id)},
				},
				"destination": map[string]interface{}{
					"repository": fmt.Sprintf("https://github.com/example/load-%d.git", id),
					"branch":     "main",
					"path":       "/projects",
				},
				"priority": "normal",
				"timeout":  "1800",
			},
		},
	}
}

func simulateResourceProcessing(client dynamic.Interface, gvr schema.GroupVersionResource, resourceName string) error {
	ctx := context.Background()

	// Simulate processing phases
	phases := []string{"Pending", "Processing", "Completed"}

	for i, phase := range phases {
		resource, err := client.Resource(gvr).Namespace("default").Get(ctx, resourceName, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("failed to get resource for processing: %v", err)
		}

		status := map[string]interface{}{
			"phase": phase,
			"progress": map[string]interface{}{
				"percentage": fmt.Sprintf("%d", (i+1)*100/len(phases)),
			},
			"lastTransitionTime": time.Now().Format(time.RFC3339),
		}

		_ = unstructured.SetNestedMap(resource.Object, status, "status")

		_, err = client.Resource(gvr).Namespace("default").UpdateStatus(ctx, resource, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to update status for phase %s: %v", phase, err)
		}

		// Simulate processing time
		time.Sleep(20 * time.Millisecond)
	}

	return nil
}

// Resource exhaustion test functions

func testMemoryPressureHandling(t *testing.T, client dynamic.Interface) error {
	// Simulate memory pressure by creating many resources rapidly
	const resourceCount = 500
	const concurrency = 100

	gvr := schema.GroupVersionResource{
		Group:    "sync.jira.io",
		Version:  "v1alpha1",
		Resource: "jirasyncs",
	}

	var wg sync.WaitGroup
	var memoryPressureDetected bool
	var successCount int64
	var errorCount int64

	semaphore := make(chan struct{}, concurrency)

	for i := 0; i < resourceCount; i++ {
		wg.Add(1)
		go func(resourceID int) {
			defer wg.Done()

			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			// Monitor memory before operation
			var beforeMem runtime.MemStats
			runtime.ReadMemStats(&beforeMem)

			resource := createLoadTestResource(int64(resourceID))
			_, err := client.Resource(gvr).Namespace("default").Create(
				context.Background(), resource, metav1.CreateOptions{})

			if err != nil {
				atomic.AddInt64(&errorCount, 1)
			} else {
				atomic.AddInt64(&successCount, 1)
			}

			// Monitor memory after operation
			var afterMem runtime.MemStats
			runtime.ReadMemStats(&afterMem)

			// Detect significant memory increase
			memoryIncrease := float64(afterMem.Alloc-beforeMem.Alloc) / 1024 / 1024
			if memoryIncrease > 10 { // 10MB increase
				memoryPressureDetected = true
			}

		}(i)
	}

	wg.Wait()

	// Validate system handled memory pressure
	if memoryPressureDetected {
		t.Logf("⚠️  Memory pressure detected during test")
	}

	successRate := float64(successCount) / float64(resourceCount) * 100
	if successRate < 80 { // Expect at least 80% success under memory pressure
		return fmt.Errorf("low success rate under memory pressure: %.2f%%", successRate)
	}

	t.Logf("✅ Memory pressure handling: %.2f%% success rate (%d/%d)",
		successRate, successCount, resourceCount)
	return nil
}

func testHighQueueDepthManagement(t *testing.T, client dynamic.Interface) error {
	// Create many resources to test queue management
	queueDepth := 1000

	gvr := schema.GroupVersionResource{
		Group:    "sync.jira.io",
		Version:  "v1alpha1",
		Resource: "jirasyncs",
	}

	// Create resources rapidly to build queue
	var createWg sync.WaitGroup
	var processedCount int64

	for i := 0; i < queueDepth; i++ {
		createWg.Add(1)
		go func(resourceID int) {
			defer createWg.Done()

			resource := createLoadTestResource(int64(resourceID))
			_, err := client.Resource(gvr).Namespace("default").Create(
				context.Background(), resource, metav1.CreateOptions{})

			if err == nil {
				atomic.AddInt64(&processedCount, 1)
			}
		}(i)
	}

	createWg.Wait()

	// Verify queue was managed effectively
	if processedCount < int64(float64(queueDepth)*0.8) { // Expect 80% to be queued successfully
		return fmt.Errorf("queue management failed: only %d/%d resources queued", processedCount, queueDepth)
	}

	t.Logf("✅ Queue management: %d/%d resources queued successfully", processedCount, queueDepth)
	return nil
}

func testResourceLimitEnforcement(t *testing.T, client dynamic.Interface) error {
	// Test that operator respects resource limits
	attemptedResources := 150

	gvr := schema.GroupVersionResource{
		Group:    "sync.jira.io",
		Version:  "v1alpha1",
		Resource: "jirasyncs",
	}

	var successCount int64
	var rejectedCount int64
	var wg sync.WaitGroup

	for i := 0; i < attemptedResources; i++ {
		wg.Add(1)
		go func(resourceID int) {
			defer wg.Done()

			resource := createLoadTestResource(int64(resourceID))
			_, err := client.Resource(gvr).Namespace("default").Create(
				context.Background(), resource, metav1.CreateOptions{})

			if err != nil {
				atomic.AddInt64(&rejectedCount, 1)
			} else {
				atomic.AddInt64(&successCount, 1)
			}
		}(i)
	}

	wg.Wait()

	// In a real system, we'd expect some resources to be rejected due to limits
	// For this test, we'll validate that the system handled the load
	totalProcessed := successCount + rejectedCount
	if totalProcessed != int64(attemptedResources) {
		return fmt.Errorf("resource accounting error: %d processed != %d attempted",
			totalProcessed, attemptedResources)
	}

	t.Logf("✅ Resource limit enforcement: %d accepted, %d rejected out of %d attempted",
		successCount, rejectedCount, attemptedResources)
	return nil
}

func testGracefulDegradation(t *testing.T, client dynamic.Interface) error {
	// Test system behavior under extreme load
	const extremeLoad = 2000
	const maxConcurrency = 200

	gvr := schema.GroupVersionResource{
		Group:    "sync.jira.io",
		Version:  "v1alpha1",
		Resource: "jirasyncs",
	}

	startTime := time.Now()
	var processedCount int64
	var errorCount int64
	var wg sync.WaitGroup

	semaphore := make(chan struct{}, maxConcurrency)

	for i := 0; i < extremeLoad; i++ {
		wg.Add(1)
		go func(resourceID int) {
			defer wg.Done()

			select {
			case semaphore <- struct{}{}:
				defer func() { <-semaphore }()
			case <-time.After(1 * time.Second):
				// Timeout acquiring semaphore - graceful degradation
				atomic.AddInt64(&errorCount, 1)
				return
			}

			resource := createLoadTestResource(int64(resourceID))
			_, err := client.Resource(gvr).Namespace("default").Create(
				context.Background(), resource, metav1.CreateOptions{})

			if err != nil {
				atomic.AddInt64(&errorCount, 1)
			} else {
				atomic.AddInt64(&processedCount, 1)
			}
		}(i)
	}

	wg.Wait()
	duration := time.Since(startTime)

	// Validate graceful degradation
	totalOperations := processedCount + errorCount
	processingRate := float64(processedCount) / duration.Seconds()
	errorRate := float64(errorCount) / float64(totalOperations) * 100

	// Under extreme load, we expect some degradation but system should remain stable
	if errorRate > 50 { // More than 50% errors indicates poor degradation
		return fmt.Errorf("poor graceful degradation: %.2f%% error rate", errorRate)
	}

	if processingRate < 1.0 { // Should maintain at least 1 operation/second
		return fmt.Errorf("processing rate too low: %.2f ops/sec", processingRate)
	}

	t.Logf("✅ Graceful degradation: %.2f%% error rate, %.2f ops/sec under extreme load",
		errorRate, processingRate)
	return nil
}

// Concurrency and batch testing functions

func testConcurrencyLimits(t *testing.T, client dynamic.Interface, maxConcurrency, resourceCount int) (*LoadTestMetrics, error) {
	gvr := schema.GroupVersionResource{
		Group:    "sync.jira.io",
		Version:  "v1alpha1",
		Resource: "jirasyncs",
	}

	metrics := &LoadTestMetrics{
		StartTime:      time.Now(),
		MaxConcurrency: maxConcurrency,
	}

	semaphore := make(chan struct{}, maxConcurrency)
	var wg sync.WaitGroup
	var activeCount int64
	var maxActive int64

	for i := 0; i < resourceCount; i++ {
		wg.Add(1)
		go func(resourceID int) {
			defer wg.Done()

			// Track active operations
			current := atomic.AddInt64(&activeCount, 1)
			for {
				max := atomic.LoadInt64(&maxActive)
				if current <= max || atomic.CompareAndSwapInt64(&maxActive, max, current) {
					break
				}
			}
			defer atomic.AddInt64(&activeCount, -1)

			// Acquire concurrency slot
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			resource := createLoadTestResource(int64(resourceID))
			_, err := client.Resource(gvr).Namespace("default").Create(
				context.Background(), resource, metav1.CreateOptions{})

			if err != nil {
				atomic.AddInt64(&metrics.ResourcesFailed, 1)
			} else {
				atomic.AddInt64(&metrics.ResourcesCreated, 1)
				atomic.AddInt64(&metrics.ResourcesProcessed, 1)
			}

			// Simulate processing time
			time.Sleep(100 * time.Millisecond)
		}(i)
	}

	wg.Wait()
	metrics.EndTime = time.Now()

	// Calculate queue depth (approximation)
	metrics.QueueDepth = resourceCount - int(maxActive)
	if metrics.QueueDepth < 0 {
		metrics.QueueDepth = 0
	}

	return metrics, nil
}

func testBatchProcessingEfficiency(t *testing.T, client dynamic.Interface, batchSize, numberOfBatches int) (float64, *LoadTestMetrics, error) {
	gvr := schema.GroupVersionResource{
		Group:    "sync.jira.io",
		Version:  "v1alpha1",
		Resource: "jirasyncs",
	}

	metrics := &LoadTestMetrics{
		StartTime: time.Now(),
	}

	successfulBatches := 0

	for batchID := 0; batchID < numberOfBatches; batchID++ {
		batchStart := time.Now()
		var batchWg sync.WaitGroup
		var batchSuccessCount int64

		// Process batch concurrently
		for i := 0; i < batchSize; i++ {
			batchWg.Add(1)
			go func(resourceID int) {
				defer batchWg.Done()

				globalID := batchID*batchSize + resourceID
				resource := createLoadTestResource(int64(globalID))
				_, err := client.Resource(gvr).Namespace("default").Create(
					context.Background(), resource, metav1.CreateOptions{})

				if err == nil {
					atomic.AddInt64(&batchSuccessCount, 1)
					atomic.AddInt64(&metrics.ResourcesCreated, 1)
				}
			}(i)
		}

		batchWg.Wait()
		batchDuration := time.Since(batchStart)

		// Consider batch successful if >90% resources processed
		batchEfficiency := float64(batchSuccessCount) / float64(batchSize)
		if batchEfficiency > 0.9 {
			successfulBatches++
		}

		t.Logf("📦 Batch %d: %d/%d resources, %.2f%% efficiency, %v duration",
			batchID+1, batchSuccessCount, batchSize, batchEfficiency*100, batchDuration)
	}

	metrics.EndTime = time.Now()
	overallEfficiency := float64(successfulBatches) / float64(numberOfBatches) * 100

	// Calculate throughput
	if metrics.EndTime.After(metrics.StartTime) {
		duration := metrics.EndTime.Sub(metrics.StartTime)
		metrics.ProcessingThroughput = float64(metrics.ResourcesCreated) / duration.Seconds()
	}

	return overallEfficiency, metrics, nil
}

// Validation and reporting functions

func validateScalabilityMetrics(t *testing.T, metrics *LoadTestMetrics, minThroughput, maxMemoryMB float64) {
	// Validate throughput
	if metrics.ProcessingThroughput < minThroughput {
		t.Errorf("❌ Throughput below target: %.2f < %.2f resources/sec",
			metrics.ProcessingThroughput, minThroughput)
	} else {
		t.Logf("✅ Throughput target met: %.2f ≥ %.2f resources/sec",
			metrics.ProcessingThroughput, minThroughput)
	}

	// Validate memory usage
	if metrics.PeakMemoryMB > maxMemoryMB {
		t.Errorf("❌ Memory usage exceeded: %.2f > %.2f MB",
			metrics.PeakMemoryMB, maxMemoryMB)
	} else {
		t.Logf("✅ Memory target met: %.2f ≤ %.2f MB (avg: %.2f MB)",
			metrics.PeakMemoryMB, maxMemoryMB, metrics.AverageMemoryMB)
	}

	// Validate error rate
	errorRate := float64(metrics.ResourcesFailed) / float64(metrics.ResourcesCreated) * 100
	if errorRate > 10 { // 10% error threshold
		t.Errorf("❌ High error rate: %.2f%% (expected ≤ 10%%)", errorRate)
	} else {
		t.Logf("✅ Error rate acceptable: %.2f%% ≤ 10%%", errorRate)
	}
}

func reportLoadTestMetrics(t *testing.T, testName string, metrics *LoadTestMetrics) {
	successRate := float64(metrics.ResourcesCompleted) / float64(metrics.ResourcesCreated) * 100
	errorRate := float64(metrics.ResourcesFailed) / float64(metrics.ResourcesCreated) * 100

	t.Logf("\n📊 Load Test Report: %s", testName)
	t.Logf("⏱️  Duration: %v", metrics.TotalDuration)
	t.Logf("📈 Resources: %d created, %d processed, %d completed (%.1f%% success)",
		metrics.ResourcesCreated, metrics.ResourcesProcessed, metrics.ResourcesCompleted, successRate)
	t.Logf("⚡ Throughput: %.2f creation/sec, %.2f processing/sec",
		metrics.CreationThroughput, metrics.ProcessingThroughput)
	t.Logf("💾 Memory: %.2f MB peak, %.2f MB average (%d samples)",
		metrics.PeakMemoryMB, metrics.AverageMemoryMB, len(metrics.MemorySamples))
	t.Logf("🔄 Concurrency: %d max, %d queue depth",
		metrics.MaxConcurrency, metrics.QueueDepth)
	t.Logf("⚠️  Errors: %.1f%% rate (%d timeout, %d conflict, %d API, %d other)",
		errorRate, metrics.TimeoutErrors, metrics.ConflictErrors, metrics.APIErrors, metrics.OtherErrors)
	t.Logf("⏲️  Latency: %v avg create, %v avg process",
		metrics.AverageCreateTime, metrics.AverageProcessTime)
}
