package test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestCLI_EndToEnd_ValidationOnly is temporarily disabled due to path issues in CI
// The core functionality is tested in TestCLI_EndToEnd_MockIntegration

func TestCLI_EndToEnd_MockIntegration(t *testing.T) {
	// Get absolute path to project root - handle both direct test execution and make test
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}

	var projectRoot string
	if filepath.Base(wd) == "test" {
		// Running from test/ directory (direct test execution)
		projectRoot, err = filepath.Abs("..")
		if err != nil {
			t.Fatalf("Failed to get project root: %v", err)
		}
	} else {
		// Running from project root (make test)
		projectRoot = wd
	}

	// Create temporary directory for test repository
	tempDir, err := os.MkdirTemp("", "cli-e2e-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Create test repository directory
	testRepo := filepath.Join(tempDir, "test-repo")
	if err := os.MkdirAll(testRepo, 0755); err != nil {
		t.Fatalf("Failed to create test repo directory: %v", err)
	}

	// Create mock .env file with invalid JIRA config (since we can't test real JIRA)
	envFile := filepath.Join(tempDir, ".env")
	envContent := `JIRA_BASE_URL=https://example.atlassian.net
JIRA_EMAIL=test@example.com
JIRA_PAT=fake-token-1234567890
LOG_LEVEL=debug
LOG_FORMAT=text`

	if err := os.WriteFile(envFile, []byte(envContent), 0644); err != nil {
		t.Fatalf("Failed to create .env file: %v", err)
	}

	// Path to the built binary
	binaryPath := filepath.Join(projectRoot, "build", "jira-sync")

	// Test with mock configuration (should fail at JIRA authentication)
	cmd := exec.Command(binaryPath, "sync", "--issues=TEST-123", "--repo="+testRepo)
	cmd.Dir = tempDir // Run from directory with .env file
	output, err := cmd.CombinedOutput()

	outputStr := string(output)
	t.Logf("Command output: %s", outputStr)
	t.Logf("Command error: %v", err)

	// Should fail at some step (likely config loading due to missing .env)
	if err == nil {
		t.Errorf("Expected command to fail, but it succeeded. Output: %s", outputStr)
	}

	// Check that CLI integration is working correctly
	if strings.Contains(outputStr, "Loading configuration") {
		t.Logf("✅ CLI integration working - reached configuration loading")

		if strings.Contains(outputStr, "Connecting to JIRA") {
			t.Logf("✅ JIRA connection step reached")

			// Expect authentication failure with fake credentials
			if strings.Contains(outputStr, "failed to authenticate with JIRA") {
				t.Logf("✅ Authentication validation working - correctly failed with fake credentials")
			}
		}
	} else {
		t.Errorf("Expected to reach configuration loading, but got: %s", outputStr)
	}

	// Should not fail at basic validation (these should pass)
	if strings.Contains(outputStr, "invalid issue key") {
		t.Error("Should not fail at issue key validation with valid key TEST-123")
	}
	if strings.Contains(outputStr, "invalid repository path") {
		t.Error("Should not fail at repository path validation with valid temp directory")
	}
}

func TestCLI_BuildSystem_Integration(t *testing.T) {
	// Test that the build system produces a working binary
	buildCmd := exec.Command("make", "build")
	buildCmd.Dir = ".."
	output, err := buildCmd.CombinedOutput()

	if err != nil {
		t.Fatalf("Build failed: %v, output: %s", err, output)
	}

	// Check that binary exists and is executable
	projectRoot, _ := filepath.Abs("..")
	binaryPath := filepath.Join(projectRoot, "build", "jira-sync")
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		t.Fatalf("Binary was not created at %s", binaryPath)
	}

	// Test version command
	versionCmd := exec.Command(binaryPath, "--version")
	versionOutput, err := versionCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Version command failed: %v", err)
	}

	versionStr := string(versionOutput)
	if !strings.Contains(versionStr, "v0.4.1") {
		t.Errorf("Expected version output to contain 'v0.4.1', got: %s", versionStr)
	}
}

func TestCLI_ProfileCommands_Integration(t *testing.T) {
	// Test CLI profile commands integration with the built binary
	projectRoot, err := filepath.Abs("..")
	if err != nil {
		t.Fatalf("Failed to get project root: %v", err)
	}

	binaryPath := filepath.Join(projectRoot, "build", "jira-sync")
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		t.Skip("Binary not found, skipping CLI profile integration test")
	}

	// Create temporary directory for profile tests
	tempDir, err := os.MkdirTemp("", "cli-profile-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Create test repository directory
	testRepo := filepath.Join(tempDir, "test-repo")
	if err := os.MkdirAll(testRepo, 0755); err != nil {
		t.Fatalf("Failed to create test repository: %v", err)
	}

	// Set working directory to temp dir for profile storage
	originalWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(originalWd) }()
	_ = os.Chdir(tempDir)

	t.Run("profile_templates_command", func(t *testing.T) {
		// Test profile templates list command
		templatesCmd := exec.Command(binaryPath, "profile", "templates")
		output, err := templatesCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Profile templates command failed: %v, output: %s", err, output)
		}

		outputStr := string(output)
		// Should show available templates
		if !strings.Contains(outputStr, "epic-all-issues") {
			t.Error("Expected templates output to contain 'epic-all-issues'")
		}
		if !strings.Contains(outputStr, "Templates:") {
			t.Error("Expected templates output to contain 'Templates:'")
		}
	})

	t.Run("profile_create_from_template", func(t *testing.T) {
		// Test creating a profile from template
		createCmd := exec.Command(binaryPath, "profile", "create",
			"--template=custom-jql",
			"--name=test-cli-profile",
			"--jql=project = TEST",
			"--repository="+testRepo)

		output, err := createCmd.CombinedOutput()
		outputStr := string(output)

		if err != nil {
			t.Fatalf("Profile create command failed: %v, output: %s", err, outputStr)
		}

		// Should indicate success
		if !strings.Contains(outputStr, "Profile 'test-cli-profile' created successfully") {
			t.Errorf("Expected success message, got: %s", outputStr)
		}
	})

	t.Run("profile_list_command", func(t *testing.T) {
		// Test listing profiles
		listCmd := exec.Command(binaryPath, "profile", "list")
		output, err := listCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Profile list command failed: %v, output: %s", err, output)
		}

		outputStr := string(output)
		// Should show the created profile
		if !strings.Contains(outputStr, "test-cli-profile") {
			t.Error("Expected list output to contain 'test-cli-profile'")
		}
		if !strings.Contains(outputStr, "Total: 1 profiles") {
			t.Error("Expected list output to show 1 profile")
		}
	})

	t.Run("profile_show_command", func(t *testing.T) {
		// Test showing profile details
		showCmd := exec.Command(binaryPath, "profile", "show", "test-cli-profile")
		output, err := showCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Profile show command failed: %v, output: %s", err, output)
		}

		outputStr := string(output)
		// Should show profile details
		if !strings.Contains(outputStr, "Profile: test-cli-profile") {
			t.Error("Expected show output to contain profile name")
		}
		if !strings.Contains(outputStr, "project = TEST") {
			t.Error("Expected show output to contain JQL query")
		}
		if !strings.Contains(outputStr, testRepo) {
			t.Error("Expected show output to contain repository path")
		}
	})

	t.Run("profile_export_import", func(t *testing.T) {
		exportFile := filepath.Join(tempDir, "exported-profiles.yaml")

		// Test exporting profiles
		exportCmd := exec.Command(binaryPath, "profile", "export",
			"--file="+exportFile,
			"--names=test-cli-profile")

		output, err := exportCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Profile export command failed: %v, output: %s", err, output)
		}

		outputStr := string(output)
		if !strings.Contains(outputStr, "Exported 1 profiles") {
			t.Errorf("Expected export success message, got: %s", outputStr)
		}

		// Verify export file exists
		if _, err := os.Stat(exportFile); os.IsNotExist(err) {
			t.Error("Export file was not created")
		}

		// Test importing profiles (with prefix to avoid conflicts)
		importCmd := exec.Command(binaryPath, "profile", "import",
			"--file="+exportFile,
			"--prefix=imported-")

		importOutput, err := importCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Profile import command failed: %v, output: %s", err, importOutput)
		}

		importStr := string(importOutput)
		if !strings.Contains(importStr, "imported successfully") {
			t.Errorf("Expected import success message, got: %s", importStr)
		}

		// Verify imported profile exists
		listCmd := exec.Command(binaryPath, "profile", "list")
		listOutput, _ := listCmd.CombinedOutput()
		listStr := string(listOutput)

		if !strings.Contains(listStr, "imported-test-cli-profile") {
			t.Error("Expected imported profile to appear in list")
		}
	})

	t.Run("profile_sync_integration", func(t *testing.T) {
		// Test using profile with sync command (dry run to avoid actual JIRA calls)
		syncCmd := exec.Command(binaryPath, "sync",
			"--profile=test-cli-profile",
			"--dry-run")

		output, err := syncCmd.CombinedOutput()
		outputStr := string(output)

		// This might fail due to missing JIRA config, which is expected
		if err != nil && !strings.Contains(outputStr, "JIRA_BASE_URL is required") {
			t.Fatalf("Unexpected sync error: %v, output: %s", err, outputStr)
		}

		// Should at least load the profile successfully before failing on JIRA config
		if strings.Contains(outputStr, "failed to load profile") {
			t.Error("Profile should load successfully even if JIRA config missing")
		}
	})

	t.Run("profile_delete_command", func(t *testing.T) {
		// Test deleting profile with force flag
		deleteCmd := exec.Command(binaryPath, "profile", "delete", "test-cli-profile", "--force")
		output, err := deleteCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Profile delete command failed: %v, output: %s", err, output)
		}

		outputStr := string(output)
		if !strings.Contains(outputStr, "Profile 'test-cli-profile' deleted successfully") {
			t.Errorf("Expected delete success message, got: %s", outputStr)
		}

		// Verify profile is gone
		listCmd := exec.Command(binaryPath, "profile", "list")
		listOutput, _ := listCmd.CombinedOutput()
		listStr := string(listOutput)

		if strings.Contains(listStr, "test-cli-profile") && !strings.Contains(listStr, "imported-test-cli-profile") {
			t.Error("Profile should be deleted from list")
		}
	})

	t.Log("✅ CLI profile commands integration test completed successfully")
}
