package main

import (
	"fmt"
	"os"

	"github.com/chambrid/jira-cdc-git/internal/cli"
)

// Build-time variables set by ldflags
var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	// Set build information for CLI
	buildInfo := cli.BuildInfo{
		Version: version,
		Commit:  commit,
		Date:    date,
	}

	// Initialize and execute CLI
	if err := cli.Execute(buildInfo); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
