package links

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/chambrid/jira-cdc-git/pkg/client"
)

// TestSymbolicLinkIntegration_FullWorkflow tests the complete symbolic link workflow
// with real filesystem operations - creation, validation, navigation, and cleanup
func TestSymbolicLinkIntegration_FullWorkflow(t *testing.T) {
	tempDir := t.TempDir()

	// Create issue files first for valid targets
	issuesDir := filepath.Join(tempDir, "projects", "PROJ", "issues")
	if err := os.MkdirAll(issuesDir, 0755); err != nil {
		t.Fatalf("Failed to create issues directory: %v", err)
	}

	// Create target issue files
	targetIssues := []string{"PROJ-100", "PROJ-124", "PROJ-125", "PROJ-200"}
	for _, issueKey := range targetIssues {
		targetFile := filepath.Join(issuesDir, issueKey+".yaml")
		content := "key: " + issueKey + "\nsummary: Test issue " + issueKey
		if err := os.WriteFile(targetFile, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create target file %s: %v", issueKey, err)
		}
	}

	// Create complex issue with all relationship types
	issue := &client.Issue{
		Key:     "PROJ-123",
		Summary: "Complex issue with all relationships",
		Relationships: &client.Relationships{
			EpicLink: "PROJ-100",
			Subtasks: []string{"PROJ-124", "PROJ-125"},
			IssueLinks: []client.IssueLink{
				{
					Type:      "blocks",
					Direction: "outward",
					IssueKey:  "PROJ-200",
					Summary:   "Blocked issue",
				},
			},
		},
	}

	manager := NewSymbolicLinkManager()

	// Step 1: Create all relationship links
	err := manager.CreateRelationshipLinks(issue, tempDir)
	if err != nil {
		t.Fatalf("Failed to create relationship links: %v", err)
	}

	// Step 2: Validate all created links
	expectedLinks := []struct {
		path   string
		target string
	}{
		{
			path:   filepath.Join(tempDir, "projects", "PROJ", "relationships", "epic", "PROJ-123"),
			target: "../../issues/PROJ-100.yaml",
		},
		{
			path:   filepath.Join(tempDir, "projects", "PROJ", "relationships", "subtasks", "PROJ-123", "PROJ-124"),
			target: "../../../issues/PROJ-124.yaml",
		},
		{
			path:   filepath.Join(tempDir, "projects", "PROJ", "relationships", "subtasks", "PROJ-123", "PROJ-125"),
			target: "../../../issues/PROJ-125.yaml",
		},
		{
			path:   filepath.Join(tempDir, "projects", "PROJ", "relationships", "blocks", "outward", "PROJ-123"),
			target: "../../../issues/PROJ-200.yaml",
		},
	}

	for _, link := range expectedLinks {
		t.Run("validate_link_"+filepath.Base(link.path), func(t *testing.T) {
			// Verify link exists and is a symbolic link
			linkInfo, err := os.Lstat(link.path)
			if err != nil {
				t.Fatalf("Link not created: %s, error: %v", link.path, err)
			}

			if linkInfo.Mode()&os.ModeSymlink == 0 {
				t.Errorf("Path is not a symbolic link: %s", link.path)
			}

			// Verify link target
			target, err := os.Readlink(link.path)
			if err != nil {
				t.Fatalf("Failed to read link target: %v", err)
			}

			if target != link.target {
				t.Errorf("Link target is '%s', expected '%s'", target, link.target)
			}

			// Verify link validation passes
			err = manager.ValidateLink(link.path)
			if err != nil {
				t.Errorf("Link validation failed: %v", err)
			}

			// Verify target file is accessible through the link
			_, err = os.Stat(link.path)
			if err != nil {
				t.Errorf("Cannot access target through link: %v", err)
			}
		})
	}

	// Step 3: Test navigation - read content through symbolic links
	for _, link := range expectedLinks {
		content, err := os.ReadFile(link.path)
		if err != nil {
			t.Errorf("Failed to read content through link %s: %v", link.path, err)
		}

		if len(content) == 0 {
			t.Errorf("Empty content read through link: %s", link.path)
		}
	}

	// Step 4: Test cleanup of broken links
	// Create a broken link
	brokenLinkPath := filepath.Join(tempDir, "projects", "PROJ", "relationships", "epic", "PROJ-BROKEN")
	brokenTarget := "../../../issues/PROJ-NONEXISTENT.yaml"

	err = os.Symlink(brokenTarget, brokenLinkPath)
	if err != nil {
		t.Fatalf("Failed to create broken link: %v", err)
	}

	// Verify the link is broken
	err = manager.ValidateLink(brokenLinkPath)
	if err == nil {
		t.Error("Expected validation to fail for broken link")
	}

	linkErr, ok := err.(*LinkError)
	if !ok || linkErr.Type != "broken_link" {
		t.Errorf("Expected broken_link error, got: %v", err)
	}

	// Cleanup broken links
	err = manager.CleanupBrokenLinks(tempDir, "PROJ")
	if err != nil {
		t.Fatalf("CleanupBrokenLinks failed: %v", err)
	}

	// Verify broken link was removed
	_, err = os.Lstat(brokenLinkPath)
	if !os.IsNotExist(err) {
		t.Error("Broken link was not removed")
	}

	// Verify valid links still exist
	for _, link := range expectedLinks {
		_, err = os.Lstat(link.path)
		if err != nil {
			t.Errorf("Valid link was incorrectly removed: %s", link.path)
		}
	}
}

// TestSymbolicLinkIntegration_CrossPlatformSupport tests platform-specific behavior
func TestSymbolicLinkIntegration_CrossPlatformSupport(t *testing.T) {
	tempDir := t.TempDir()

	// Create a simple test case
	issuesDir := filepath.Join(tempDir, "projects", "TEST", "issues")
	if err := os.MkdirAll(issuesDir, 0755); err != nil {
		t.Fatalf("Failed to create issues directory: %v", err)
	}

	targetFile := filepath.Join(issuesDir, "TEST-100.yaml")
	if err := os.WriteFile(targetFile, []byte("key: TEST-100"), 0644); err != nil {
		t.Fatalf("Failed to create target file: %v", err)
	}

	issue := &client.Issue{
		Key: "TEST-123",
		Relationships: &client.Relationships{
			EpicLink: "TEST-100",
		},
	}

	manager := NewSymbolicLinkManager()
	err := manager.CreateRelationshipLinks(issue, tempDir)
	if err != nil {
		// On Windows without developer mode, symbolic links might fail
		// This is expected behavior and should be handled gracefully
		t.Logf("Symbolic link creation failed (may be expected on Windows): %v", err)
		return
	}

	// If we get here, symbolic links are supported
	linkPath := filepath.Join(tempDir, "projects", "TEST", "relationships", "epic", "TEST-123")

	// Test that the link works correctly on this platform
	err = manager.ValidateLink(linkPath)
	if err != nil {
		t.Errorf("Link validation failed on this platform: %v", err)
	}

	// Test reading through the link
	content, err := os.ReadFile(linkPath)
	if err != nil {
		t.Errorf("Failed to read through symbolic link: %v", err)
	}

	if string(content) != "key: TEST-100" {
		t.Errorf("Incorrect content read through link: %s", string(content))
	}
}

// TestSymbolicLinkIntegration_PerformanceValidation validates SPIKE-004 performance findings
func TestSymbolicLinkIntegration_PerformanceValidation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	tempDir := t.TempDir()

	// Create a reasonable number of target files for performance testing
	issuesDir := filepath.Join(tempDir, "projects", "PERF", "issues")
	if err := os.MkdirAll(issuesDir, 0755); err != nil {
		t.Fatalf("Failed to create issues directory: %v", err)
	}

	// Create 100 target files
	targetCount := 100
	for i := 0; i < targetCount; i++ {
		issueKey := fmt.Sprintf("PERF-%d", i)
		targetFile := filepath.Join(issuesDir, issueKey+".yaml")
		content := "key: " + issueKey
		if err := os.WriteFile(targetFile, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create target file %s: %v", issueKey, err)
		}
	}

	manager := NewSymbolicLinkManager()

	// Create directory structure once
	err := manager.CreateDirectoryStructure(tempDir, "PERF")
	if err != nil {
		t.Fatalf("Failed to create directory structure: %v", err)
	}

	// Test performance of creating many links
	// SPIKE-004 found 0.06ms per link creation on macOS
	startTime := time.Now()
	linkCount := 0

	for i := 0; i < targetCount; i++ {
		sourceKey := fmt.Sprintf("PERF-SRC-%d", i)
		targetKey := fmt.Sprintf("PERF-%d", i)

		issue := &client.Issue{
			Key: sourceKey,
			Relationships: &client.Relationships{
				EpicLink: targetKey,
			},
		}

		err := manager.CreateRelationshipLinks(issue, tempDir)
		if err != nil {
			t.Fatalf("Failed to create links for %s: %v", sourceKey, err)
		}
		linkCount++
	}

	duration := time.Since(startTime)
	avgPerLink := duration / time.Duration(linkCount)

	t.Logf("Created %d links in %v (avg: %v per link)", linkCount, duration, avgPerLink)

	// Performance should be reasonable (allowing for test environment variance)
	// SPIKE-004 found 0.06ms, we'll allow up to 10ms per link for CI environments
	maxPerLink := 10 * time.Millisecond
	if avgPerLink > maxPerLink {
		t.Errorf("Link creation performance degraded: %v per link (max expected: %v)", avgPerLink, maxPerLink)
	}

	// Validate a sample of the created links
	sampleSize := 10
	for i := 0; i < sampleSize; i++ {
		sourceKey := fmt.Sprintf("PERF-SRC-%d", i)
		linkPath := filepath.Join(tempDir, "projects", "PERF", "relationships", "epic", sourceKey)

		err := manager.ValidateLink(linkPath)
		if err != nil {
			t.Errorf("Link validation failed for %s: %v", sourceKey, err)
		}
	}
}

func TestSymbolicLinkIntegration_DirectoryStructureValidation(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewSymbolicLinkManager()

	err := manager.CreateDirectoryStructure(tempDir, "STRUCT")
	if err != nil {
		t.Fatalf("Failed to create directory structure: %v", err)
	}

	// Verify all expected relationship directories exist with correct permissions
	expectedDirs := []string{"epic", "subtasks", "parent", "blocks", "clones", "documents"}

	for _, relType := range expectedDirs {
		dirPath := filepath.Join(tempDir, "projects", "STRUCT", "relationships", relType)

		info, err := os.Stat(dirPath)
		if err != nil {
			t.Errorf("Directory not created: %s, error: %v", dirPath, err)
			continue
		}

		if !info.IsDir() {
			t.Errorf("Path is not a directory: %s", dirPath)
		}

		// Check permissions (0755)
		mode := info.Mode()
		expectedMode := os.FileMode(0755)
		if mode.Perm() != expectedMode {
			t.Errorf("Directory has incorrect permissions: %s, got %v, expected %v",
				dirPath, mode.Perm(), expectedMode)
		}
	}

	// Test GetRelationshipPath method
	for _, relType := range expectedDirs {
		path := manager.GetRelationshipPath(tempDir, "STRUCT", relType)
		expected := filepath.Join(tempDir, "projects", "STRUCT", "relationships", relType)

		if path != expected {
			t.Errorf("GetRelationshipPath returned incorrect path for %s: got %s, expected %s",
				relType, path, expected)
		}
	}
}
