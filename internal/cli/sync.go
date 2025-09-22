package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/chambrid/jira-cdc-git/internal/sync"
	"github.com/chambrid/jira-cdc-git/pkg/client"
	"github.com/chambrid/jira-cdc-git/pkg/config"
	"github.com/chambrid/jira-cdc-git/pkg/git"
	"github.com/chambrid/jira-cdc-git/pkg/schema"
	"github.com/spf13/cobra"
)

// syncCmd represents the sync command
var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync JIRA issue(s) to a Git repository with relationship mapping",
	Long: `Sync JIRA issues to a Git repository as structured YAML files with symbolic link relationships.

This command fetches issues from JIRA and stores them as YAML files in a organized directory 
structure, creating symbolic links to represent issue relationships (epic/story, subtasks, 
blocks/clones). Supports batch operations with rate limiting and progress feedback.

File Structure:
  {repo}/projects/{project-key}/issues/{issue-key}.yaml        # Issue data
  {repo}/projects/{project-key}/relationships/{type}/          # Relationship links

Sync Modes:
  • Single/Multiple Issues: --issues=PROJ-123 or --issues=PROJ-1,PROJ-2,PROJ-3
  • JQL Query: --jql="project = PROJ AND status = 'To Do'"

Performance:
  • Default: 5 workers, 100ms rate limit (recommended for most JIRA instances)
  • High load: --concurrency=2 --rate-limit=500ms (gentler on JIRA API)
  • Fast sync: --concurrency=10 --rate-limit=50ms (use carefully)`,
	Example: `  # Sync single issue
  jira-sync sync --issues=PROJ-123 --repo=./my-repo

  # Sync multiple issues with custom rate limiting
  jira-sync sync --issues=PROJ-1,PROJ-2,PROJ-3 --repo=./my-repo --rate-limit=200ms

  # Sync all issues in epic using JQL
  jira-sync sync --jql="Epic Link = PROJ-123" --repo=./my-repo

  # Sync with custom concurrency for faster processing
  jira-sync sync --jql="project = PROJ AND status = 'To Do'" --repo=./my-repo --concurrency=8

  # Sync to current directory with debug logging
  jira-sync sync --issues=TEAM-456 --repo=. --log-level=debug

  # Gentle sync for overloaded JIRA instances
  jira-sync sync --jql="assignee = currentUser()" --repo=./issues --concurrency=2 --rate-limit=1s`,
	RunE: runSync,
}

func runSync(cmd *cobra.Command, args []string) error {
	issuesArg, _ := cmd.Flags().GetString("issues")
	jqlArg, _ := cmd.Flags().GetString("jql")
	repo, _ := cmd.Flags().GetString("repo")
	concurrency, _ := cmd.Flags().GetInt("concurrency")
	rateLimitArg, _ := cmd.Flags().GetString("rate-limit")

	// Validate mutual exclusivity of --issues and --jql
	if issuesArg != "" && jqlArg != "" {
		return fmt.Errorf("cannot specify both --issues and --jql flags")
	}
	if issuesArg == "" && jqlArg == "" {
		return fmt.Errorf("must specify either --issues or --jql flag")
	}

	// Validate repository path
	if err := validateRepoPath(repo); err != nil {
		return fmt.Errorf("invalid repository path: %w", err)
	}

	// Validate rate limit if provided
	var rateLimitOverride *time.Duration
	if rateLimitArg != "" {
		parsed, err := parseRateLimit(rateLimitArg)
		if err != nil {
			return fmt.Errorf("invalid rate limit: %w", err)
		}
		rateLimitOverride = &parsed
	}

	// Step 1: Load configuration
	fmt.Println("📄 Loading configuration...")
	configLoader := config.NewDotEnvLoader()
	cfg, err := configLoader.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Apply rate limit override if provided
	if rateLimitOverride != nil {
		fmt.Printf("⏱️  Overriding rate limit delay to %v\n", *rateLimitOverride)
		cfg.RateLimitDelay = *rateLimitOverride
	}

	// Step 2: Initialize JIRA client
	fmt.Println("🔗 Connecting to JIRA...")
	jiraClient, err := client.NewClient(cfg)
	if err != nil {
		return fmt.Errorf("failed to create JIRA client: %w", err)
	}

	// Authenticate with JIRA
	if err := jiraClient.Authenticate(); err != nil {
		return fmt.Errorf("failed to authenticate with JIRA: %w", err)
	}

	// Step 3: Initialize Git repository
	fmt.Printf("📁 Preparing Git repository at %s...\n", repo)
	gitRepo := git.NewGitRepository("JIRA CDC Git Sync", "jira-sync@automated.local")

	// Initialize repository if needed
	if err := gitRepo.Initialize(repo); err != nil {
		return fmt.Errorf("failed to initialize Git repository: %w", err)
	}

	// Validate working tree is clean
	if err := gitRepo.ValidateWorkingTree(repo); err != nil {
		return fmt.Errorf("git repository validation failed: %w", err)
	}

	// Step 4: Initialize batch sync engine
	fileWriter := schema.NewYAMLFileWriter()
	batchEngine := sync.NewBatchSyncEngine(jiraClient, fileWriter, gitRepo, concurrency)

	// Step 5: Start progress monitoring
	ctx := context.Background()
	progressDone := make(chan bool, 1)

	go func() {
		defer func() { progressDone <- true }()
		monitorProgress(batchEngine.GetProgressChannel())
	}()

	// Step 6: Execute sync based on mode
	var result *sync.BatchResult
	if issuesArg != "" {
		// Issues list mode
		rawIssues, err := parseIssueList(issuesArg)
		if err != nil {
			return fmt.Errorf("failed to parse issues: %w", err)
		}

		issues, err := validateIssueList(rawIssues)
		if err != nil {
			return fmt.Errorf("issue validation failed: %w", err)
		}

		if len(issues) == 1 {
			fmt.Printf("🚀 Syncing JIRA issue %s to repository %s\n", issues[0], repo)
		} else {
			fmt.Printf("🚀 Syncing %d JIRA issues to repository %s\n", len(issues), repo)
			fmt.Printf("📋 Issues: %s\n", strings.Join(issues, ", "))
		}

		result, err = batchEngine.SyncIssues(ctx, issues, repo)
		if err != nil {
			return fmt.Errorf("batch sync failed: %w", err)
		}
	} else {
		// JQL mode
		fmt.Printf("🚀 Syncing JIRA issues matching JQL query to repository %s\n", repo)
		fmt.Printf("📋 JQL: %s\n", jqlArg)

		result, err = batchEngine.SyncJQL(ctx, jqlArg, repo)
		if err != nil {
			return fmt.Errorf("JQL sync failed: %w", err)
		}
	}

	// Wait for progress monitoring to complete
	<-progressDone

	// Step 7: Display results
	displaySyncResults(result)

	return nil
}

// validateIssueKey validates JIRA issue key format (e.g., PROJ-123)
func validateIssueKey(issueKey string) error {
	if issueKey == "" {
		return fmt.Errorf("issue key cannot be empty")
	}

	// JIRA issue key format: PROJECT-NUMBER (e.g., PROJ-123, MY-PROJECT-456)
	issueKeyRegex := regexp.MustCompile(`^[A-Z][A-Z0-9]*(-[A-Z0-9]+)*-\d+$`)
	if !issueKeyRegex.MatchString(issueKey) {
		return fmt.Errorf("issue key '%s' does not match JIRA format (e.g., PROJ-123)", issueKey)
	}

	return nil
}

// validateRepoPath validates repository path
func validateRepoPath(repoPath string) error {
	if repoPath == "" {
		return fmt.Errorf("repository path cannot be empty")
	}

	// Convert to absolute path
	absPath, err := filepath.Abs(repoPath)
	if err != nil {
		return fmt.Errorf("invalid path '%s': %w", repoPath, err)
	}

	// Check if parent directory exists (we'll create the repo dir if needed)
	parentDir := filepath.Dir(absPath)
	if _, err := os.Stat(parentDir); os.IsNotExist(err) {
		return fmt.Errorf("parent directory '%s' does not exist", parentDir)
	}

	return nil
}

// parseIssueList parses comma-separated issue keys and returns a deduplicated, validated list
func parseIssueList(issuesArg string) ([]string, error) {
	if issuesArg == "" {
		return nil, fmt.Errorf("issues list cannot be empty")
	}

	// Split by comma and clean up whitespace
	rawIssues := strings.Split(issuesArg, ",")
	var issues []string

	for _, issue := range rawIssues {
		trimmed := strings.TrimSpace(issue)
		if trimmed != "" {
			issues = append(issues, trimmed)
		}
	}

	if len(issues) == 0 {
		return nil, fmt.Errorf("no valid issues found in list")
	}

	return issues, nil
}

// validateIssueList validates a list of issue keys and removes duplicates
func validateIssueList(issues []string) ([]string, error) {
	if len(issues) == 0 {
		return nil, fmt.Errorf("issue list cannot be empty")
	}

	seen := make(map[string]bool)
	var validIssues []string
	var errors []string

	for _, issue := range issues {
		// Skip duplicates
		if seen[issue] {
			continue
		}
		seen[issue] = true

		// Validate individual issue key
		if err := validateIssueKey(issue); err != nil {
			errors = append(errors, fmt.Sprintf("invalid issue '%s': %v", issue, err))
			continue
		}

		validIssues = append(validIssues, issue)
	}

	// Report validation errors if any
	if len(errors) > 0 {
		return nil, fmt.Errorf("validation failed for %d issues:\n%s", len(errors), strings.Join(errors, "\n"))
	}

	if len(validIssues) == 0 {
		return nil, fmt.Errorf("no valid issues found after validation")
	}

	return validIssues, nil
}

// monitorProgress displays real-time progress updates
func monitorProgress(progressChan <-chan sync.ProgressUpdate) {
	lastPercentage := -1.0

	for update := range progressChan {
		// Only display percentage updates to avoid spam
		if update.Percentage > 0 && int(update.Percentage) != int(lastPercentage) {
			fmt.Printf("⏳ Progress: %.0f%% (%d processed)\n", update.Percentage, update.ProcessedCount)
			lastPercentage = update.Percentage
		}
	}
}

// displaySyncResults shows the final results of the sync operation
func displaySyncResults(result *sync.BatchResult) {
	fmt.Printf("\n🎯 Sync completed in %v\n", result.Duration)
	fmt.Printf("📊 Results:\n")
	fmt.Printf("  • Total issues: %d\n", result.TotalIssues)
	fmt.Printf("  • Processed: %d\n", result.ProcessedIssues)
	fmt.Printf("  • Successful: %d\n", result.SuccessfulSync)
	fmt.Printf("  • Failed: %d\n", result.FailedSync)

	// Performance metrics
	fmt.Printf("⚡ Performance:\n")
	fmt.Printf("  • Speed: %.1f issues/second\n", result.Performance.IssuesPerSecond)
	fmt.Printf("  • Workers: %d\n", result.Performance.WorkerCount)
	fmt.Printf("  • Avg time per issue: %v\n", result.Performance.AvgProcessTime)

	// Show errors if any
	if len(result.Errors) > 0 {
		fmt.Printf("\n❌ Errors:\n")
		for _, err := range result.Errors {
			fmt.Printf("  • %s (%s): %s\n", err.IssueKey, err.Step, err.Message)
		}
	}

	// Show successful files
	if len(result.ProcessedFiles) > 0 {
		fmt.Printf("\n✅ Successfully synced files:\n")
		for i, file := range result.ProcessedFiles {
			if i < 5 { // Show first 5 files
				fmt.Printf("  • %s\n", file)
			} else if i == 5 {
				fmt.Printf("  • ... and %d more files\n", len(result.ProcessedFiles)-5)
				break
			}
		}
	}
}

// parseRateLimit parses and validates a rate limit duration string
func parseRateLimit(rateLimitStr string) (time.Duration, error) {
	if rateLimitStr == "" {
		return 0, fmt.Errorf("rate limit cannot be empty")
	}

	duration, err := time.ParseDuration(rateLimitStr)
	if err != nil {
		return 0, fmt.Errorf("invalid duration format '%s': %w (expected format: 100ms, 1s, 2s, etc.)", rateLimitStr, err)
	}

	if duration < 0 {
		return 0, fmt.Errorf("rate limit delay must be non-negative, got %v", duration)
	}

	return duration, nil
}

func init() {
	rootCmd.AddCommand(syncCmd)

	// Sync command flags
	syncCmd.Flags().StringP("issues", "i", "", "JIRA issue key(s) - single issue (PROJ-123) or comma-separated list (PROJ-1,PROJ-2)")
	syncCmd.Flags().StringP("jql", "j", "", "JQL query to find issues (e.g., 'project = PROJ AND status = \"To Do\"')")
	syncCmd.Flags().StringP("repo", "r", "", "Target Git repository path - will be created if it doesn't exist (required)")
	syncCmd.Flags().IntP("concurrency", "c", 5, "Parallel workers for batch processing (1-10, default: 5, recommended: 2-5 for most instances)")
	syncCmd.Flags().String("rate-limit", "", "API call delay override (e.g., 100ms, 1s, 2s) - use higher values for busy JIRA instances")

	// Mark required flags
	_ = syncCmd.MarkFlagRequired("repo")
}
