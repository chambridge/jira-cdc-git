package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// BuildInfo contains build-time information
type BuildInfo struct {
	Version string
	Commit  string
	Date    string
}

var buildInfo BuildInfo

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "jira-sync",
	Short: "Sync JIRA issues to Git repositories with relationship mapping and batch processing",
	Long: `JIRA CDC Git Sync - A Kubernetes-native tool for synchronizing JIRA data into Git repositories.

Fetches JIRA issues and stores them as structured YAML files with symbolic link relationships,
enabling GitOps workflows and version-controlled project tracking. Supports batch operations,
rate limiting, and comprehensive relationship mapping (epic/story, subtasks, blocks/clones).

Key Features:
  • Issue-to-file mapping with structured YAML output
  • Symbolic link relationships for issue dependencies  
  • Batch processing with configurable concurrency and rate limiting
  • JQL query support for targeted issue synchronization
  • Conventional Git commits with proper metadata
  • Progress reporting and comprehensive error handling

Configuration:
  Create a .env file with JIRA credentials:
    JIRA_BASE_URL=https://your-instance.atlassian.net
    JIRA_EMAIL=your-email@company.com
    JIRA_PAT=your-personal-access-token

Getting Started:
  jira-sync sync --issues=PROJ-123 --repo=./my-repo`,
	Version: buildInfo.Version,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute(info BuildInfo) error {
	buildInfo = info
	rootCmd.Version = fmt.Sprintf("%s (commit: %s, built: %s)", info.Version, info.Commit, info.Date)
	return rootCmd.Execute()
}

func init() {
	// Global flags can be added here
	rootCmd.PersistentFlags().StringP("log-level", "l", "info", "Log level (debug, info, warn, error)")
	rootCmd.PersistentFlags().String("log-format", "text", "Log format (text, json)")
}
