package epic

import (
	"os"
	"strings"
	"testing"
	"unicode"

	"github.com/chambrid/jira-cdc-git/pkg/client"
	"github.com/chambrid/jira-cdc-git/pkg/config"
)

func TestJIRAEpicAnalyzer_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Load configuration from environment
	cfg, err := config.LoadFromCurrentDir()
	if err != nil {
		t.Skipf("Skipping integration test - could not load config: %v", err)
	}

	// Create real JIRA client
	jiraClient, err := client.NewClient(cfg)
	if err != nil {
		t.Skipf("Skipping integration test - could not create JIRA client: %v", err)
	}

	// Test authentication
	if err := jiraClient.Authenticate(); err != nil {
		t.Skipf("Skipping integration test - JIRA authentication failed: %v", err)
	}

	// Use a test EPIC key from environment or skip
	epicKey := os.Getenv("TEST_EPIC_KEY")
	if epicKey == "" {
		t.Skip("Skipping integration test - TEST_EPIC_KEY environment variable not set")
	}

	t.Logf("Running integration test with EPIC: %s", epicKey)

	// Create analyzer with different strategies
	strategies := []struct {
		name     string
		strategy EpicDiscoveryStrategy
	}{
		{"Epic Link", StrategyEpicLink},
		{"Custom Field", StrategyCustomField},
		{"Hybrid", StrategyHybrid},
	}

	for _, strategyTest := range strategies {
		t.Run(strategyTest.name, func(t *testing.T) {
			options := &DiscoveryOptions{
				Strategy:            strategyTest.strategy,
				MaxDepth:            3,
				IncludeSubtasks:     true,
				IncludeLinkedIssues: true,
				BatchSize:           100,
				UseCache:            true,
			}

			analyzer := NewJIRAEpicAnalyzer(jiraClient, options)

			// Test EPIC discovery
			t.Run("DiscoverEpicIssues", func(t *testing.T) {
				issues, err := analyzer.DiscoverEpicIssues(epicKey)
				if err != nil {
					t.Fatalf("Failed to discover EPIC issues: %v", err)
				}

				t.Logf("Discovered %d issues for EPIC %s using %s strategy",
					len(issues), epicKey, strategyTest.name)

				if len(issues) == 0 {
					t.Log("No issues found - this might be expected for some EPICs")
					return
				}

				// Validate issue structure
				for i, issue := range issues {
					if i >= 5 { // Only validate first 5 issues to avoid too much output
						break
					}

					if issue.Key == "" {
						t.Errorf("Issue %d has empty key", i)
					}

					if issue.IssueType == "" {
						t.Errorf("Issue %s has empty issue type", issue.Key)
					}

					if issue.Summary == "" {
						t.Errorf("Issue %s has empty summary", issue.Key)
					}

					t.Logf("  Issue %s: %s (%s)", issue.Key, issue.Summary, issue.IssueType)
				}
			})

			// Test full EPIC analysis
			t.Run("AnalyzeEpic", func(t *testing.T) {
				result, err := analyzer.AnalyzeEpic(epicKey)
				if err != nil {
					t.Fatalf("Failed to analyze EPIC: %v", err)
				}

				if result == nil {
					t.Fatal("Analysis result is nil")
				}

				if result.EpicKey != epicKey {
					t.Errorf("Expected epic key %s, got %s", epicKey, result.EpicKey)
				}

				t.Logf("EPIC Analysis Results:")
				t.Logf("  Summary: %s", result.EpicSummary)
				t.Logf("  Status: %s", result.EpicStatus)
				t.Logf("  Total Issues: %d", result.TotalIssues)

				// Log issue breakdown by type
				if len(result.IssuesByType) > 0 {
					t.Logf("  Issues by Type:")
					for issueType, issues := range result.IssuesByType {
						t.Logf("    %s: %d", toTitle(issueType), len(issues))
					}
				}

				// Log issue breakdown by status
				if len(result.IssuesByStatus) > 0 {
					t.Logf("  Issues by Status:")
					for status, issues := range result.IssuesByStatus {
						t.Logf("    %s: %d", status, len(issues))
					}
				}

				// Log relationship types
				if len(result.RelationshipTypes) > 0 {
					t.Logf("  Relationship Types:")
					for relType, count := range result.RelationshipTypes {
						t.Logf("    %s: %d", toTitle(strings.ReplaceAll(relType, "_", " ")), count)
					}
				}

				// Validate performance metrics
				if result.Performance == nil {
					t.Error("Performance metrics are missing")
				} else {
					t.Logf("  Performance:")
					t.Logf("    Discovery Time: %dms", result.Performance.DiscoveryTimeMs)
					t.Logf("    Analysis Time: %dms", result.Performance.AnalysisTimeMs)
					t.Logf("    API Calls: %d", result.Performance.TotalAPICallsCount)
					t.Logf("    Cache Hits: %d", result.Performance.CacheHitCount)
					t.Logf("    Cache Misses: %d", result.Performance.CacheMissCount)

					// Performance assertions
					if result.Performance.DiscoveryTimeMs < 0 {
						t.Error("Discovery time should be non-negative")
					}

					if result.Performance.AnalysisTimeMs < 0 {
						t.Error("Analysis time should be non-negative")
					}

					if result.Performance.TotalAPICallsCount < 0 {
						t.Error("API call count should be non-negative")
					}
				}

				// Validate hierarchy
				if result.Hierarchy == nil {
					t.Error("Hierarchy is missing")
				} else {
					t.Logf("  Hierarchy:")
					t.Logf("    Stories: %d", len(result.Hierarchy.Stories))
					t.Logf("    Tasks: %d", len(result.Hierarchy.Tasks))
					t.Logf("    Bugs: %d", len(result.Hierarchy.Bugs))
					t.Logf("    Direct Issues: %d", len(result.Hierarchy.DirectIssues))
					t.Logf("    Levels: %d", result.Hierarchy.Levels)

					if result.Hierarchy.EpicKey != epicKey {
						t.Errorf("Hierarchy epic key mismatch: expected %s, got %s",
							epicKey, result.Hierarchy.EpicKey)
					}
				}

				// Validate completeness report
				if result.Completeness == nil {
					t.Error("Completeness report is missing")
				} else {
					t.Logf("  Completeness:")
					t.Logf("    Expected Issues: %d", result.Completeness.TotalExpectedIssues)
					t.Logf("    Found Issues: %d", result.Completeness.TotalFoundIssues)
					t.Logf("    Completeness: %.1f%%", result.Completeness.CompletenessPercent)

					if len(result.Completeness.BrokenLinks) > 0 {
						t.Logf("    Broken Links: %d", len(result.Completeness.BrokenLinks))
						for i, link := range result.Completeness.BrokenLinks {
							if i >= 3 { // Only log first 3 broken links
								t.Logf("      ... and %d more", len(result.Completeness.BrokenLinks)-3)
								break
							}
							t.Logf("      %s", link)
						}
					}

					if len(result.Completeness.Recommendations) > 0 {
						t.Logf("    Recommendations:")
						for _, rec := range result.Completeness.Recommendations {
							t.Logf("      - %s", rec)
						}
					}

					// Completeness assertions
					if result.Completeness.CompletenessPercent < 0 || result.Completeness.CompletenessPercent > 100 {
						t.Errorf("Completeness percent should be 0-100, got %.2f",
							result.Completeness.CompletenessPercent)
					}
				}
			})

			// Test hierarchy generation
			t.Run("GetEpicHierarchy", func(t *testing.T) {
				hierarchy, err := analyzer.GetEpicHierarchy(epicKey)
				if err != nil {
					t.Fatalf("Failed to get EPIC hierarchy: %v", err)
				}

				if hierarchy == nil {
					t.Fatal("Hierarchy is nil")
				}

				if hierarchy.EpicKey != epicKey {
					t.Errorf("Expected epic key %s, got %s", epicKey, hierarchy.EpicKey)
				}

				// Test hierarchy structure consistency
				totalIssues := len(hierarchy.Stories) + len(hierarchy.Tasks) +
					len(hierarchy.Bugs) + len(hierarchy.DirectIssues)

				if totalIssues == 0 {
					t.Log("No direct issues found in hierarchy - this might be expected")
				} else {
					t.Logf("Hierarchy contains %d direct issues", totalIssues)

					// Count subtasks recursively
					subtaskCount := countSubtasks(hierarchy.Stories) +
						countSubtasks(hierarchy.Tasks) +
						countSubtasks(hierarchy.Bugs) +
						countSubtasks(hierarchy.DirectIssues)

					if subtaskCount > 0 {
						t.Logf("Hierarchy contains %d subtasks across all levels", subtaskCount)
					}
				}

				if hierarchy.Levels < 1 {
					t.Errorf("Expected at least 1 level, got %d", hierarchy.Levels)
				}
			})

			// Test completeness validation
			t.Run("ValidateEpicCompleteness", func(t *testing.T) {
				report, err := analyzer.ValidateEpicCompleteness(epicKey)
				if err != nil {
					t.Fatalf("Failed to validate EPIC completeness: %v", err)
				}

				if report == nil {
					t.Fatal("Completeness report is nil")
				}

				// Basic validation
				if report.TotalFoundIssues < 0 {
					t.Error("Total found issues should be non-negative")
				}

				if report.TotalExpectedIssues < 0 {
					t.Error("Total expected issues should be non-negative")
				}

				if report.CompletenessPercent < 0 || report.CompletenessPercent > 100 {
					t.Errorf("Completeness percent should be 0-100, got %.2f",
						report.CompletenessPercent)
				}

				t.Logf("Completeness validation: %.1f%% (%d/%d issues)",
					report.CompletenessPercent, report.TotalFoundIssues, report.TotalExpectedIssues)
			})
		})
	}
}

// Helper function to count subtasks recursively
func countSubtasks(nodes []*HierarchyNode) int {
	count := 0
	for _, node := range nodes {
		count += len(node.Subtasks)
		count += countSubtasks(node.Subtasks) // Recursive count
	}
	return count
}

// toTitle converts first character to uppercase (replacement for deprecated strings.Title)
func toTitle(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

func TestJIRAEpicAnalyzer_PerformanceWithLargeEpic(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	// This test is designed to run with a large EPIC if available
	largeEpicKey := os.Getenv("TEST_LARGE_EPIC_KEY")
	if largeEpicKey == "" {
		t.Skip("Skipping performance test - TEST_LARGE_EPIC_KEY environment variable not set")
	}

	cfg, err := config.LoadFromCurrentDir()
	if err != nil {
		t.Skipf("Skipping performance test - could not load config: %v", err)
	}

	jiraClient, err := client.NewClient(cfg)
	if err != nil {
		t.Skipf("Skipping performance test - could not create JIRA client: %v", err)
	}

	if err := jiraClient.Authenticate(); err != nil {
		t.Skipf("Skipping performance test - JIRA authentication failed: %v", err)
	}

	t.Logf("Running performance test with large EPIC: %s", largeEpicKey)

	options := &DiscoveryOptions{
		Strategy:            StrategyHybrid,
		MaxDepth:            5,
		IncludeSubtasks:     true,
		IncludeLinkedIssues: true,
		BatchSize:           100,
		UseCache:            true,
	}

	analyzer := NewJIRAEpicAnalyzer(jiraClient, options)

	result, err := analyzer.AnalyzeEpic(largeEpicKey)
	if err != nil {
		t.Fatalf("Failed to analyze large EPIC: %v", err)
	}

	t.Logf("Large EPIC analysis results:")
	t.Logf("  Total Issues: %d", result.TotalIssues)
	t.Logf("  Discovery Time: %dms", result.Performance.DiscoveryTimeMs)
	t.Logf("  Analysis Time: %dms", result.Performance.AnalysisTimeMs)
	t.Logf("  Total Time: %dms", result.Performance.DiscoveryTimeMs+result.Performance.AnalysisTimeMs)
	t.Logf("  API Calls: %d", result.Performance.TotalAPICallsCount)

	// Performance assertions for large EPICs
	totalTime := result.Performance.DiscoveryTimeMs + result.Performance.AnalysisTimeMs

	// Should handle large EPICs within reasonable time (30 seconds for 1000+ issues)
	if result.TotalIssues > 1000 && totalTime > 30000 {
		t.Errorf("Large EPIC processing took too long: %dms for %d issues",
			totalTime, result.TotalIssues)
	}

	// Should not use excessive API calls (should be less than issues + reasonable overhead)
	if result.Performance.TotalAPICallsCount > result.TotalIssues+20 {
		t.Errorf("Too many API calls: %d for %d issues",
			result.Performance.TotalAPICallsCount, result.TotalIssues)
	}

	t.Logf("Performance test completed successfully")
}
