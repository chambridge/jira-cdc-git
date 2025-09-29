package main

import (
	"fmt"
	"os"

	"github.com/chambrid/jira-cdc-git/internal/api"
)

// Build-time variables set by ldflags
var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	// Set build information for API server
	buildInfo := api.BuildInfo{
		Version: version,
		Commit:  commit,
		Date:    date,
	}

	// Initialize and start API server
	if err := api.Execute(buildInfo); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
