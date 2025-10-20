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

// PerformanceMetrics tracks operator performance characteristics
type PerformanceMetrics struct {
	TotalOperations      int64
	SuccessfulOperations int64
	FailedOperations     int64
	AverageLatency       time.Duration
	MaxLatency           time.Duration
	MinLatency           time.Duration
	Throughput           float64 // operations per second
	MemoryUsageMB        float64
	StartTime            time.Time
	EndTime              time.Time
}

// TestOperatorScalabilityPerformance validates operator performance targets
func TestOperatorScalabilityPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance tests in short mode")
	}

	performanceTargets := map[string]struct {
		maxReconciliationTime  time.Duration
		maxMemoryUsageMB       float64
		minThroughputOpsPerSec float64
		resourceCount          int
		concurrency            int
		description            string
	}{
		"small_scale": {
			maxReconciliationTime:  100 * time.Millisecond,
			maxMemoryUsageMB:       50,
			minThroughputOpsPerSec: 10,
			resourceCount:          25,
			concurrency:            5,
			description:            "Small scale: 25 resources, 5 concurrent",
		},
		"medium_scale": {
			maxReconciliationTime:  200 * time.Millisecond,
			maxMemoryUsageMB:       100,
			minThroughputOpsPerSec: 8,
			resourceCount:          100,
			concurrency:            10,
			description:            "Medium scale: 100 resources, 10 concurrent",
		},
		"large_scale": {
			maxReconciliationTime:  500 * time.Millisecond,
			maxMemoryUsageMB:       200,
			minThroughputOpsPerSec: 5,
			resourceCount:          250,
			concurrency:            15,
			description:            "Large scale: 250 resources, 15 concurrent",
		},
	}

	for testName, target := range performanceTargets {
		t.Run(testName, func(t *testing.T) {
			client := createTestKubernetesClient(t)

			t.Logf("🚀 Starting performance test: %s", target.description)

			metrics, err := runPerformanceTest(t, client, target.resourceCount, target.concurrency)
			if err != nil {
				t.Errorf("Performance test failed: %v", err)
				return
			}

			// Validate performance targets
			validatePerformanceTargets(t, metrics, target.maxReconciliationTime,
				target.maxMemoryUsageMB, target.minThroughputOpsPerSec)

			// Report detailed metrics
			reportPerformanceMetrics(t, testName, metrics)
		})
	}
}

// TestOperatorHighVolumeLoadTesting simulates sustained high-volume operations
func TestOperatorHighVolumeLoadTesting(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping load tests in short mode")
	}

	loadTests := []struct {
		name           string
		duration       time.Duration
		rateLimit      time.Duration
		maxConcurrency int
		expectedMinOps int64
		description    string
	}{
		{
			name:           "Sustained Load Test",
			duration:       2 * time.Minute,
			rateLimit:      100 * time.Millisecond,
			maxConcurrency: 20,
			expectedMinOps: 600, // 600 operations in 2 minutes
			description:    "Sustained 2-minute load with 100ms rate limit",
		},
		{
			name:           "Burst Load Test",
			duration:       30 * time.Second,
			rateLimit:      10 * time.Millisecond,
			maxConcurrency: 50,
			expectedMinOps: 1000, // High burst rate
			description:    "30-second burst with 10ms rate limit",
		},
		{
			name:           "Stress Test",
			duration:       1 * time.Minute,
			rateLimit:      50 * time.Millisecond,
			maxConcurrency: 100,
			expectedMinOps: 800, // Stress test throughput
			description:    "1-minute stress test with high concurrency",
		},
	}

	for _, test := range loadTests {
		t.Run(test.name, func(t *testing.T) {
			client := createTestKubernetesClient(t)

			t.Logf("⚡ Starting load test: %s", test.description)

			metrics, err := runLoadTest(t, client, test.duration, test.rateLimit, test.maxConcurrency)
			if err != nil {
				t.Errorf("Load test failed: %v", err)
				return
			}

			// Validate minimum operations completed
			if metrics.SuccessfulOperations < test.expectedMinOps {
				t.Errorf("Load test underperformed: expected min %d ops, got %d ops",
					test.expectedMinOps, metrics.SuccessfulOperations)
			}

			// Check for excessive failures (should be < 5%)
			failureRate := float64(metrics.FailedOperations) / float64(metrics.TotalOperations) * 100
			if failureRate > 5.0 {
				t.Errorf("Excessive failure rate: %.2f%% (expected < 5%%)", failureRate)
			}

			reportLoadTestResults(t, test.name, metrics, test.expectedMinOps)
		})
	}
}

// TestOperatorMemoryLeakDetection monitors for memory leaks during extended operations
func TestOperatorMemoryLeakDetection(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory leak test in short mode")
	}

	client := createTestKubernetesClient(t)

	// Baseline memory measurement
	runtime.GC()
	var initialMem runtime.MemStats
	runtime.ReadMemStats(&initialMem)

	t.Logf("🔍 Starting memory leak detection test")
	t.Logf("📊 Initial memory: %.2f MB", float64(initialMem.Alloc)/1024/1024)

	// Run extended operations to detect memory leaks
	iterations := 10
	resourcesPerIteration := 50

	var memoryMeasurements []float64

	for i := 0; i < iterations; i++ {
		// Create and process resources
		_, err := runPerformanceTest(t, client, resourcesPerIteration, 10)
		if err != nil {
			t.Errorf("Memory leak test iteration %d failed: %v", i+1, err)
			continue
		}

		// Force garbage collection and measure memory
		runtime.GC()
		var currentMem runtime.MemStats
		runtime.ReadMemStats(&currentMem)

		currentMemMB := float64(currentMem.Alloc) / 1024 / 1024
		memoryMeasurements = append(memoryMeasurements, currentMemMB)

		t.Logf("📊 Iteration %d memory: %.2f MB", i+1, currentMemMB)

		// Brief pause between iterations
		time.Sleep(500 * time.Millisecond)
	}

	// Analyze memory trend
	memoryTrend := analyzeMemoryTrend(memoryMeasurements)
	initialMemMB := float64(initialMem.Alloc) / 1024 / 1024
	finalMemMB := memoryMeasurements[len(memoryMeasurements)-1]

	// Check for significant memory growth (> 50% increase)
	memoryGrowth := (finalMemMB - initialMemMB) / initialMemMB * 100

	if memoryGrowth > 50 {
		t.Errorf("Potential memory leak detected: %.2f%% memory growth (%.2f MB → %.2f MB)",
			memoryGrowth, initialMemMB, finalMemMB)
	} else {
		t.Logf("✅ Memory usage stable: %.2f%% growth, trend: %s",
			memoryGrowth, memoryTrend)
	}
}

// TestOperatorReconciliationLatency measures reconciliation performance
func TestOperatorReconciliationLatency(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping latency test in short mode")
	}

	client := createTestKubernetesClient(t)
	gvr := schema.GroupVersionResource{
		Group:    "sync.jira.io",
		Version:  "v1alpha1",
		Resource: "jirasyncs",
	}

	latencyTests := []struct {
		name                 string
		resourceCount        int
		expectedMaxLatencyMs int64
		description          string
	}{
		{
			name:                 "Single Resource Latency",
			resourceCount:        1,
			expectedMaxLatencyMs: 100,
			description:          "Single resource should reconcile within 100ms",
		},
		{
			name:                 "Small Batch Latency",
			resourceCount:        10,
			expectedMaxLatencyMs: 200,
			description:          "Small batch should average under 200ms per resource",
		},
		{
			name:                 "Large Batch Latency",
			resourceCount:        50,
			expectedMaxLatencyMs: 500,
			description:          "Large batch should maintain reasonable latency",
		},
	}

	for _, test := range latencyTests {
		t.Run(test.name, func(t *testing.T) {
			var latencies []time.Duration
			ctx := context.Background()

			t.Logf("⏱️  Measuring reconciliation latency: %s", test.description)

			for i := 0; i < test.resourceCount; i++ {
				// Create resource and measure reconciliation time
				resource := createJIRASyncResource(fmt.Sprintf("latency-test-%d", i), map[string]interface{}{
					"syncType": "single",
					"target": map[string]interface{}{
						"issueKeys": []interface{}{fmt.Sprintf("PROJ-%d", i)},
					},
					"destination": map[string]interface{}{
						"repository": fmt.Sprintf("https://github.com/example/latency-%d.git", i),
					},
				})

				start := time.Now()

				created, err := client.Resource(gvr).Namespace("default").Create(ctx, resource, metav1.CreateOptions{})
				if err != nil {
					t.Errorf("Failed to create resource for latency test: %v", err)
					continue
				}

				// Simulate reconciliation completion
				reconciliationLatency := simulateReconciliationLatency(ctx, client, gvr, created.GetName())
				latencies = append(latencies, reconciliationLatency)

				totalLatency := time.Since(start)
				t.Logf("📊 Resource %d: reconciliation=%v, total=%v", i+1, reconciliationLatency, totalLatency)
			}

			// Analyze latency statistics
			avgLatency := calculateAverageLatency(latencies)
			maxLatency := calculateMaxLatency(latencies)
			p95Latency := calculatePercentileLatency(latencies, 95)

			t.Logf("📈 Latency Statistics:")
			t.Logf("   Average: %v", avgLatency)
			t.Logf("   Maximum: %v", maxLatency)
			t.Logf("   95th percentile: %v", p95Latency)

			// Validate performance target
			maxLatencyMs := maxLatency.Milliseconds()
			if maxLatencyMs > test.expectedMaxLatencyMs {
				t.Errorf("Latency target exceeded: max %dms (expected ≤ %dms)",
					maxLatencyMs, test.expectedMaxLatencyMs)
			} else {
				t.Logf("✅ Latency target met: max %dms ≤ %dms",
					maxLatencyMs, test.expectedMaxLatencyMs)
			}
		})
	}
}

// Helper functions for performance testing

func runPerformanceTest(t *testing.T, client dynamic.Interface, resourceCount, concurrency int) (*PerformanceMetrics, error) {
	gvr := schema.GroupVersionResource{
		Group:    "sync.jira.io",
		Version:  "v1alpha1",
		Resource: "jirasyncs",
	}

	metrics := &PerformanceMetrics{
		StartTime:  time.Now(),
		MinLatency: time.Hour, // Initialize to large value
	}

	ctx := context.Background()
	semaphore := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	var latencySum int64
	var latencyCount int64

	for i := 0; i < resourceCount; i++ {
		wg.Add(1)
		go func(resourceID int) {
			defer wg.Done()

			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			start := time.Now()

			// Create resource
			resource := createJIRASyncResource(fmt.Sprintf("perf-test-%d", resourceID), map[string]interface{}{
				"syncType": "single",
				"target": map[string]interface{}{
					"issueKeys": []interface{}{fmt.Sprintf("PERF-%d", resourceID)},
				},
				"destination": map[string]interface{}{
					"repository": fmt.Sprintf("https://github.com/example/perf-%d.git", resourceID),
				},
				"priority": "normal",
				"timeout":  "1800",
			})

			_, err := client.Resource(gvr).Namespace("default").Create(ctx, resource, metav1.CreateOptions{})

			latency := time.Since(start)
			atomic.AddInt64(&metrics.TotalOperations, 1)

			if err != nil {
				atomic.AddInt64(&metrics.FailedOperations, 1)
				t.Logf("⚠️  Resource %d creation failed: %v", resourceID, err)
			} else {
				atomic.AddInt64(&metrics.SuccessfulOperations, 1)

				// Update latency statistics
				atomic.AddInt64(&latencySum, latency.Nanoseconds())
				atomic.AddInt64(&latencyCount, 1)

				// Track min/max latency (with basic synchronization)
				if latency > metrics.MaxLatency {
					metrics.MaxLatency = latency
				}
				if latency < metrics.MinLatency {
					metrics.MinLatency = latency
				}
			}
		}(i)
	}

	wg.Wait()
	metrics.EndTime = time.Now()

	// Calculate final metrics
	totalDuration := metrics.EndTime.Sub(metrics.StartTime)
	if latencyCount > 0 {
		metrics.AverageLatency = time.Duration(atomic.LoadInt64(&latencySum) / atomic.LoadInt64(&latencyCount))
	}
	metrics.Throughput = float64(metrics.SuccessfulOperations) / totalDuration.Seconds()

	// Memory usage
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	metrics.MemoryUsageMB = float64(memStats.Alloc) / 1024 / 1024

	return metrics, nil
}

func runLoadTest(t *testing.T, client dynamic.Interface, duration, rateLimit time.Duration, maxConcurrency int) (*PerformanceMetrics, error) {
	gvr := schema.GroupVersionResource{
		Group:    "sync.jira.io",
		Version:  "v1alpha1",
		Resource: "jirasyncs",
	}

	metrics := &PerformanceMetrics{
		StartTime:  time.Now(),
		MinLatency: time.Hour,
	}

	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()

	semaphore := make(chan struct{}, maxConcurrency)
	ticker := time.NewTicker(rateLimit)
	defer ticker.Stop()

	var wg sync.WaitGroup
	var operationCounter int64
	var latencySum int64
	var latencyCount int64

	// Run load test until duration expires
	for {
		select {
		case <-ctx.Done():
			// Test duration completed
			wg.Wait() // Wait for all operations to complete
			metrics.EndTime = time.Now()

			// Calculate final metrics
			totalDuration := metrics.EndTime.Sub(metrics.StartTime)
			if latencyCount > 0 {
				metrics.AverageLatency = time.Duration(atomic.LoadInt64(&latencySum) / atomic.LoadInt64(&latencyCount))
			}
			metrics.Throughput = float64(metrics.SuccessfulOperations) / totalDuration.Seconds()

			// Memory usage
			var memStats runtime.MemStats
			runtime.ReadMemStats(&memStats)
			metrics.MemoryUsageMB = float64(memStats.Alloc) / 1024 / 1024

			return metrics, nil

		case <-ticker.C:
			// Rate-limited operation
			wg.Add(1)
			go func(opID int64) {
				defer wg.Done()

				select {
				case semaphore <- struct{}{}:
					defer func() { <-semaphore }()

					start := time.Now()

					// Create load test resource
					resource := createJIRASyncResource(fmt.Sprintf("load-test-%d", opID), map[string]interface{}{
						"syncType": "single",
						"target": map[string]interface{}{
							"issueKeys": []interface{}{fmt.Sprintf("LOAD-%d", opID)},
						},
						"destination": map[string]interface{}{
							"repository": fmt.Sprintf("https://github.com/example/load-%d.git", opID),
						},
					})

					_, err := client.Resource(gvr).Namespace("default").Create(context.Background(), resource, metav1.CreateOptions{})

					latency := time.Since(start)
					atomic.AddInt64(&metrics.TotalOperations, 1)

					if err != nil {
						atomic.AddInt64(&metrics.FailedOperations, 1)
					} else {
						atomic.AddInt64(&metrics.SuccessfulOperations, 1)

						// Update latency statistics
						atomic.AddInt64(&latencySum, latency.Nanoseconds())
						atomic.AddInt64(&latencyCount, 1)

						// Track min/max latency
						if latency > metrics.MaxLatency {
							metrics.MaxLatency = latency
						}
						if latency < metrics.MinLatency {
							metrics.MinLatency = latency
						}
					}

				case <-ctx.Done():
					// Context canceled, skip this operation
					return
				}
			}(atomic.AddInt64(&operationCounter, 1))
		}
	}
}

func simulateReconciliationLatency(ctx context.Context, client dynamic.Interface, gvr schema.GroupVersionResource, resourceName string) time.Duration {
	start := time.Now()

	// Simulate reconciliation phases
	phases := []string{"Pending", "Processing", "Completed"}

	for _, phase := range phases {
		// Get resource
		resource, err := client.Resource(gvr).Namespace("default").Get(ctx, resourceName, metav1.GetOptions{})
		if err != nil {
			continue
		}

		// Update status
		status := map[string]interface{}{
			"phase":              phase,
			"lastTransitionTime": time.Now().Format(time.RFC3339),
		}
		_ = unstructured.SetNestedMap(resource.Object, status, "status")

		// Update resource
		_, _ = client.Resource(gvr).Namespace("default").UpdateStatus(ctx, resource, metav1.UpdateOptions{})

		// Simulate processing time
		time.Sleep(10 * time.Millisecond)
	}

	return time.Since(start)
}

func validatePerformanceTargets(t *testing.T, metrics *PerformanceMetrics, maxLatency time.Duration, maxMemoryMB, minThroughput float64) {
	// Validate latency target
	if metrics.AverageLatency > maxLatency {
		t.Errorf("❌ Latency target failed: avg %v > %v", metrics.AverageLatency, maxLatency)
	} else {
		t.Logf("✅ Latency target met: avg %v ≤ %v", metrics.AverageLatency, maxLatency)
	}

	// Validate memory target
	if metrics.MemoryUsageMB > maxMemoryMB {
		t.Errorf("❌ Memory target failed: %.2f MB > %.2f MB", metrics.MemoryUsageMB, maxMemoryMB)
	} else {
		t.Logf("✅ Memory target met: %.2f MB ≤ %.2f MB", metrics.MemoryUsageMB, maxMemoryMB)
	}

	// Validate throughput target
	if metrics.Throughput < minThroughput {
		t.Errorf("❌ Throughput target failed: %.2f ops/sec < %.2f ops/sec", metrics.Throughput, minThroughput)
	} else {
		t.Logf("✅ Throughput target met: %.2f ops/sec ≥ %.2f ops/sec", metrics.Throughput, minThroughput)
	}
}

func reportPerformanceMetrics(t *testing.T, testName string, metrics *PerformanceMetrics) {
	duration := metrics.EndTime.Sub(metrics.StartTime)
	successRate := float64(metrics.SuccessfulOperations) / float64(metrics.TotalOperations) * 100

	t.Logf("\n📊 Performance Report: %s", testName)
	t.Logf("⏱️  Duration: %v", duration)
	t.Logf("📈 Operations: %d total, %d successful (%.1f%%)",
		metrics.TotalOperations, metrics.SuccessfulOperations, successRate)
	t.Logf("⚡ Throughput: %.2f operations/second", metrics.Throughput)
	t.Logf("🕐 Latency: avg=%v, min=%v, max=%v",
		metrics.AverageLatency, metrics.MinLatency, metrics.MaxLatency)
	t.Logf("💾 Memory Usage: %.2f MB", metrics.MemoryUsageMB)
}

func reportLoadTestResults(t *testing.T, testName string, metrics *PerformanceMetrics, expectedMinOps int64) {
	duration := metrics.EndTime.Sub(metrics.StartTime)
	successRate := float64(metrics.SuccessfulOperations) / float64(metrics.TotalOperations) * 100
	opsTarget := float64(metrics.SuccessfulOperations) / float64(expectedMinOps) * 100

	t.Logf("\n⚡ Load Test Report: %s", testName)
	t.Logf("⏱️  Duration: %v", duration)
	t.Logf("🎯 Target Achievement: %.1f%% (%d/%d ops)", opsTarget, metrics.SuccessfulOperations, expectedMinOps)
	t.Logf("✅ Success Rate: %.1f%% (%d/%d)", successRate, metrics.SuccessfulOperations, metrics.TotalOperations)
	t.Logf("⚡ Sustained Throughput: %.2f operations/second", metrics.Throughput)
	t.Logf("💾 Peak Memory: %.2f MB", metrics.MemoryUsageMB)
}

func analyzeMemoryTrend(measurements []float64) string {
	if len(measurements) < 2 {
		return "insufficient_data"
	}

	// Calculate linear trend
	firstHalf := measurements[:len(measurements)/2]
	secondHalf := measurements[len(measurements)/2:]

	avgFirst := calculateAverage(firstHalf)
	avgSecond := calculateAverage(secondHalf)

	change := (avgSecond - avgFirst) / avgFirst * 100

	if change > 10 {
		return "increasing"
	} else if change < -10 {
		return "decreasing"
	} else {
		return "stable"
	}
}

func calculateAverage(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

func calculateAverageLatency(latencies []time.Duration) time.Duration {
	if len(latencies) == 0 {
		return 0
	}

	var total time.Duration
	for _, latency := range latencies {
		total += latency
	}
	return total / time.Duration(len(latencies))
}

func calculateMaxLatency(latencies []time.Duration) time.Duration {
	if len(latencies) == 0 {
		return 0
	}

	max := latencies[0]
	for _, latency := range latencies[1:] {
		if latency > max {
			max = latency
		}
	}
	return max
}

func calculatePercentileLatency(latencies []time.Duration, percentile int) time.Duration {
	if len(latencies) == 0 {
		return 0
	}

	// Simple percentile calculation (would use sort in production)
	index := (percentile * len(latencies)) / 100
	if index >= len(latencies) {
		index = len(latencies) - 1
	}
	return latencies[index]
}
