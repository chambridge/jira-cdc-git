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
	Short: "Sync JIRA issues to Git repositories",
	Long: `JIRA CDC Git Sync - A tool for synchronizing JIRA issues into Git repositories.

This tool fetches JIRA issues and stores them as YAML files in a structured
Git repository, maintaining relationships and enabling GitOps workflows.`,
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
