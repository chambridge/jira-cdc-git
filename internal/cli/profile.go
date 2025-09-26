package cli

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/chambrid/jira-cdc-git/pkg/profile"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// profileCmd represents the profile command
var profileCmd = &cobra.Command{
	Use:   "profile",
	Short: "Manage sync profiles for reusable configurations",
	Long: `Manage sync profiles to save and reuse common sync configurations.

Profiles allow you to save complex sync configurations (JQL queries, EPIC keys, options)
with a simple name for easy reuse. This is especially useful for:

• Regular sync operations with specific JQL queries
• EPIC-based syncs with consistent options
• Team-shared sync configurations
• Template-based profile creation

Profile Storage:
  Profiles are stored in .jira-sync-profiles/ directory in YAML format.
  You can export/import profiles for sharing with team members.

Common Workflow:
  1. Create profile from template or manually
  2. Use profile with 'jira-sync sync --profile=name'  
  3. Update profile as needed
  4. Share profiles via export/import`,
	Example: `  # List all profiles
  jira-sync profile list
  
  # Create profile from template
  jira-sync profile create --template=epic-all-issues --name=my-epic --epic_key=PROJ-123 --repository=./repo
  
  # Create custom profile
  jira-sync profile create --name=urgent --jql="priority = High AND status != Closed" --repository=./urgent
  
  # Use a profile
  jira-sync sync --profile=my-epic
  
  # Export profiles for sharing
  jira-sync profile export --file=team-profiles.yaml --names=my-epic,urgent`,
}

var profileListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all sync profiles",
	Long: `List all saved sync profiles with their basic information.

Use filters to narrow down the list or include usage statistics for analysis.`,
	Example: `  # List all profiles
  jira-sync profile list
  
  # List with usage statistics
  jira-sync profile list --stats
  
  # List profiles with specific tags
  jira-sync profile list --tags=epic,team
  
  # List most recently used profiles
  jira-sync profile list --sort=usage --limit=5`,
	RunE: runProfileListCommand,
}

var profileCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new sync profile",
	Long: `Create a new sync profile either from scratch or using a template.

Templates provide pre-configured profiles for common use cases like EPIC syncs,
project monitoring, or personal workflows. Use variables to customize templates.`,
	Example: `  # Create from template
  jira-sync profile create --template=epic-all-issues --name=feature-x --epic_key=PROJ-456 --repository=./feature-x
  
  # Create custom JQL profile
  jira-sync profile create --name=bugs --jql="type = Bug AND status != Closed" --repository=./bug-tracking
  
  # Create issue list profile
  jira-sync profile create --name=release --issues=PROJ-1,PROJ-2,PROJ-3 --repository=./release-prep
  
  # Create with custom options
  jira-sync profile create --name=large-sync --jql="project = PROJ" --repository=./full-proj --concurrency=8 --rate-limit=200ms`,
	RunE: runProfileCreateCommand,
}

var profileShowCmd = &cobra.Command{
	Use:   "show <profile-name>",
	Short: "Show details of a specific profile",
	Long: `Display detailed information about a specific profile including configuration,
usage statistics, and metadata.`,
	Example: `  # Show profile details
  jira-sync profile show my-epic
  
  # Show with usage statistics
  jira-sync profile show my-epic --stats`,
	Args: cobra.ExactArgs(1),
	RunE: runProfileShowCommand,
}

var profileUpdateCmd = &cobra.Command{
	Use:   "update <profile-name>",
	Short: "Update an existing profile",
	Long: `Update an existing profile's configuration. You can modify any aspect of the
profile including the sync query, repository, options, and metadata.`,
	Example: `  # Update JQL query
  jira-sync profile update my-profile --jql="project = PROJ AND status = 'In Progress'"
  
  # Update repository path
  jira-sync profile update my-profile --repository=./new-location
  
  # Update sync options
  jira-sync profile update my-profile --concurrency=3 --rate-limit=1s
  
  # Add tags
  jira-sync profile update my-profile --tags=production,critical`,
	Args: cobra.ExactArgs(1),
	RunE: runProfileUpdateCommand,
}

var profileDeleteCmd = &cobra.Command{
	Use:   "delete <profile-name>",
	Short: "Delete a sync profile",
	Long: `Delete a sync profile permanently. This action cannot be undone.

Use --force to skip confirmation prompt.`,
	Example: `  # Delete with confirmation
  jira-sync profile delete old-profile
  
  # Delete without confirmation
  jira-sync profile delete old-profile --force`,
	Args: cobra.ExactArgs(1),
	RunE: runProfileDeleteCommand,
}

var profileTemplatesCmd = &cobra.Command{
	Use:   "templates",
	Short: "List available profile templates",
	Long: `List all available profile templates that can be used to create new profiles.

Templates provide pre-configured profiles for common use cases and can be customized
using variables during profile creation.`,
	Example: `  # List all templates
  jira-sync profile templates
  
  # Show template details
  jira-sync profile templates --details`,
	RunE: runProfileTemplatesCommand,
}

var profileExportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export profiles to a file",
	Long: `Export one or more profiles to a file for backup or sharing.

Exported profiles can be imported by other team members or used to restore profiles
after system migration.`,
	Example: `  # Export all profiles
  jira-sync profile export --file=all-profiles.yaml
  
  # Export specific profiles
  jira-sync profile export --file=team-profiles.yaml --names=epic-sync,bug-tracking
  
  # Export profiles with specific tags
  jira-sync profile export --file=production-profiles.yaml --tags=production,critical
  
  # Export without usage statistics
  jira-sync profile export --file=clean-profiles.yaml --no-stats`,
	RunE: runProfileExportCommand,
}

var profileImportCmd = &cobra.Command{
	Use:   "import",
	Short: "Import profiles from a file",
	Long: `Import profiles from a previously exported file.

By default, import will not overwrite existing profiles. Use --overwrite to replace
existing profiles with imported ones.`,
	Example: `  # Import profiles
  jira-sync profile import --file=team-profiles.yaml
  
  # Import with overwrite
  jira-sync profile import --file=team-profiles.yaml --overwrite
  
  # Import with name prefix
  jira-sync profile import --file=shared-profiles.yaml --prefix=team-
  
  # Import with additional tags
  jira-sync profile import --file=external-profiles.yaml --tags=external,imported`,
	RunE: runProfileImportCommand,
}

// Profile command flags
var profileFlags struct {
	// List flags
	Stats bool
	Tags  []string
	Sort  string
	Order string
	Limit int

	// Create/Update flags
	Template     string
	Name         string
	Description  string
	JQL          string
	Issues       []string
	EpicKey      string
	Repository   string
	Concurrency  int
	RateLimit    string
	Incremental  bool
	Force        bool
	DryRun       bool
	IncludeLinks bool
	ProfileTags  []string

	// Show flags
	ShowStats bool

	// Delete flags
	ForceDelete bool

	// Template flags
	Details bool

	// Export flags
	ExportFile string
	Names      []string
	NoStats    bool
	Format     string

	// Import flags
	ImportFile string
	Overwrite  bool
	Prefix     string
	ImportTags []string
	Validate   bool

	// Template variables (dynamic)
	Variables map[string]string
}

func init() {
	// Add profile commands
	rootCmd.AddCommand(profileCmd)
	profileCmd.AddCommand(profileListCmd)
	profileCmd.AddCommand(profileCreateCmd)
	profileCmd.AddCommand(profileShowCmd)
	profileCmd.AddCommand(profileUpdateCmd)
	profileCmd.AddCommand(profileDeleteCmd)
	profileCmd.AddCommand(profileTemplatesCmd)
	profileCmd.AddCommand(profileExportCmd)
	profileCmd.AddCommand(profileImportCmd)

	// List command flags
	profileListCmd.Flags().BoolVar(&profileFlags.Stats, "stats", false, "Include usage statistics")
	profileListCmd.Flags().StringSliceVar(&profileFlags.Tags, "tags", nil, "Filter by tags")
	profileListCmd.Flags().StringVar(&profileFlags.Sort, "sort", "name", "Sort by: name, created, updated, usage")
	profileListCmd.Flags().StringVar(&profileFlags.Order, "order", "asc", "Sort order: asc, desc")
	profileListCmd.Flags().IntVar(&profileFlags.Limit, "limit", 0, "Limit number of results")

	// Create command flags
	profileCreateCmd.Flags().StringVar(&profileFlags.Template, "template", "", "Template to use for profile creation")
	profileCreateCmd.Flags().StringVar(&profileFlags.Name, "name", "", "Profile name (required)")
	profileCreateCmd.Flags().StringVar(&profileFlags.Description, "description", "", "Profile description")
	profileCreateCmd.Flags().StringVar(&profileFlags.JQL, "jql", "", "JQL query for sync")
	profileCreateCmd.Flags().StringSliceVar(&profileFlags.Issues, "issues", nil, "Comma-separated issue keys")
	profileCreateCmd.Flags().StringVar(&profileFlags.EpicKey, "epic-key", "", "EPIC key for EPIC-based sync")
	profileCreateCmd.Flags().StringVar(&profileFlags.Repository, "repository", "", "Target repository path (required)")
	profileCreateCmd.Flags().IntVar(&profileFlags.Concurrency, "concurrency", 5, "Concurrency level (1-10)")
	profileCreateCmd.Flags().StringVar(&profileFlags.RateLimit, "rate-limit", "500ms", "Rate limit between API calls")
	profileCreateCmd.Flags().BoolVar(&profileFlags.Incremental, "incremental", false, "Enable incremental sync")
	profileCreateCmd.Flags().BoolVar(&profileFlags.Force, "force", false, "Enable force sync")
	profileCreateCmd.Flags().BoolVar(&profileFlags.DryRun, "dry-run", false, "Enable dry run mode")
	profileCreateCmd.Flags().BoolVar(&profileFlags.IncludeLinks, "include-links", true, "Include relationship links")
	profileCreateCmd.Flags().StringSliceVar(&profileFlags.ProfileTags, "tags", nil, "Profile tags")

	// Mark required flags for create
	_ = profileCreateCmd.MarkFlagRequired("name")

	// Show command flags
	profileShowCmd.Flags().BoolVar(&profileFlags.ShowStats, "stats", false, "Show usage statistics")

	// Update command flags (reuse create flags)
	profileUpdateCmd.Flags().StringVar(&profileFlags.Description, "description", "", "Profile description")
	profileUpdateCmd.Flags().StringVar(&profileFlags.JQL, "jql", "", "JQL query for sync")
	profileUpdateCmd.Flags().StringSliceVar(&profileFlags.Issues, "issues", nil, "Comma-separated issue keys")
	profileUpdateCmd.Flags().StringVar(&profileFlags.EpicKey, "epic-key", "", "EPIC key for EPIC-based sync")
	profileUpdateCmd.Flags().StringVar(&profileFlags.Repository, "repository", "", "Target repository path")
	profileUpdateCmd.Flags().IntVar(&profileFlags.Concurrency, "concurrency", 0, "Concurrency level (1-10)")
	profileUpdateCmd.Flags().StringVar(&profileFlags.RateLimit, "rate-limit", "", "Rate limit between API calls")
	profileUpdateCmd.Flags().BoolVar(&profileFlags.Incremental, "incremental", false, "Enable incremental sync")
	profileUpdateCmd.Flags().BoolVar(&profileFlags.Force, "force", false, "Enable force sync")
	profileUpdateCmd.Flags().BoolVar(&profileFlags.DryRun, "dry-run", false, "Enable dry run mode")
	profileUpdateCmd.Flags().BoolVar(&profileFlags.IncludeLinks, "include-links", true, "Include relationship links")
	profileUpdateCmd.Flags().StringSliceVar(&profileFlags.ProfileTags, "tags", nil, "Profile tags")

	// Delete command flags
	profileDeleteCmd.Flags().BoolVar(&profileFlags.ForceDelete, "force", false, "Skip confirmation prompt")

	// Templates command flags
	profileTemplatesCmd.Flags().BoolVar(&profileFlags.Details, "details", false, "Show detailed template information")

	// Export command flags
	profileExportCmd.Flags().StringVar(&profileFlags.ExportFile, "file", "", "Export file path (required)")
	profileExportCmd.Flags().StringSliceVar(&profileFlags.Names, "names", nil, "Specific profile names to export")
	profileExportCmd.Flags().StringSliceVar(&profileFlags.Tags, "tags", nil, "Export profiles with these tags")
	profileExportCmd.Flags().BoolVar(&profileFlags.NoStats, "no-stats", false, "Exclude usage statistics")
	profileExportCmd.Flags().StringVar(&profileFlags.Format, "format", "", "Export format: yaml, json (auto-detected from file extension)")

	// Mark required flags for export
	_ = profileExportCmd.MarkFlagRequired("file")

	// Import command flags
	profileImportCmd.Flags().StringVar(&profileFlags.ImportFile, "file", "", "Import file path (required)")
	profileImportCmd.Flags().BoolVar(&profileFlags.Overwrite, "overwrite", false, "Overwrite existing profiles")
	profileImportCmd.Flags().StringVar(&profileFlags.Prefix, "prefix", "", "Add prefix to imported profile names")
	profileImportCmd.Flags().StringSliceVar(&profileFlags.ImportTags, "tags", nil, "Add these tags to imported profiles")
	profileImportCmd.Flags().BoolVar(&profileFlags.Validate, "validate", true, "Validate profiles before import")

	// Mark required flags for import
	_ = profileImportCmd.MarkFlagRequired("file")

	// Initialize variables map
	profileFlags.Variables = make(map[string]string)
}

func runProfileListCommand(cmd *cobra.Command, args []string) error {
	manager := profile.NewFileProfileManager(".", "yaml")

	options := &profile.ProfileListOptions{
		Tags:         profileFlags.Tags,
		IncludeStats: profileFlags.Stats,
		SortBy:       profileFlags.Sort,
		SortOrder:    profileFlags.Order,
		Limit:        profileFlags.Limit,
	}

	profiles, err := manager.ListProfiles(options)
	if err != nil {
		return fmt.Errorf("failed to list profiles: %w", err)
	}

	if len(profiles) == 0 {
		fmt.Println("No profiles found.")
		fmt.Println()
		fmt.Println("Create your first profile:")
		fmt.Println("  jira-sync profile create --template=epic-all-issues --name=my-epic --epic_key=PROJ-123 --repository=./repo")
		fmt.Println("  jira-sync profile templates  # See available templates")
		return nil
	}

	// Print profiles in table format
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	if profileFlags.Stats {
		_, _ = fmt.Fprintf(w, "NAME\tDESCRIPTION\tTYPE\tREPOSITORY\tUSED\tLAST USED\tTAGS\n")
	} else {
		_, _ = fmt.Fprintf(w, "NAME\tDESCRIPTION\tTYPE\tREPOSITORY\tTAGS\n")
	}

	for _, p := range profiles {
		syncType := getSyncType(p)
		description := p.Description
		if len(description) > 50 {
			description = description[:47] + "..."
		}

		tags := strings.Join(p.Tags, ",")
		if len(tags) > 20 {
			tags = tags[:17] + "..."
		}

		if profileFlags.Stats {
			lastUsed := "never"
			if !p.UsageStats.LastUsed.IsZero() {
				lastUsed = p.UsageStats.LastUsed.Format("2006-01-02")
			}
			_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%d\t%s\t%s\n",
				p.Name, description, syncType, p.Repository, p.UsageStats.TimesUsed, lastUsed, tags)
		} else {
			_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
				p.Name, description, syncType, p.Repository, tags)
		}
	}

	_ = w.Flush()
	fmt.Printf("\nTotal: %d profiles\n", len(profiles))

	return nil
}

func runProfileCreateCommand(cmd *cobra.Command, args []string) error {
	manager := profile.NewFileProfileManager(".", "yaml")

	// Check if profile already exists
	if manager.ProfileExists(profileFlags.Name) {
		return fmt.Errorf("profile '%s' already exists", profileFlags.Name)
	}

	var newProfile *profile.Profile
	var err error

	if profileFlags.Template != "" {
		// Create from template
		variables := make(map[string]string)
		variables["name"] = profileFlags.Name
		if profileFlags.Repository != "" {
			variables["repository"] = profileFlags.Repository
		}
		if profileFlags.EpicKey != "" {
			variables["epic_key"] = profileFlags.EpicKey
		}
		if profileFlags.JQL != "" {
			variables["jql"] = profileFlags.JQL
		}

		// Parse additional variables from flags
		cmd.Flags().Visit(func(flag *pflag.Flag) {
			if !strings.HasPrefix(flag.Name, "var-") {
				return
			}
			varName := strings.TrimPrefix(flag.Name, "var-")
			variables[varName] = flag.Value.String()
		})

		newProfile, err = manager.CreateFromTemplate(profileFlags.Template, profileFlags.Name, variables)
		if err != nil {
			return fmt.Errorf("failed to create profile from template: %w", err)
		}
	} else {
		// Create custom profile
		if profileFlags.Repository == "" {
			return fmt.Errorf("repository is required when not using a template")
		}

		// Validate sync mode
		syncModes := 0
		if profileFlags.JQL != "" {
			syncModes++
		}
		if len(profileFlags.Issues) > 0 {
			syncModes++
		}
		if profileFlags.EpicKey != "" {
			syncModes++
		}

		if syncModes == 0 {
			return fmt.Errorf("must specify one sync mode: --jql, --issues, or --epic-key")
		}
		if syncModes > 1 {
			return fmt.Errorf("can only specify one sync mode: --jql, --issues, or --epic-key")
		}

		newProfile = &profile.Profile{
			Name:        profileFlags.Name,
			Description: profileFlags.Description,
			JQL:         profileFlags.JQL,
			IssueKeys:   profileFlags.Issues,
			EpicKey:     profileFlags.EpicKey,
			Repository:  profileFlags.Repository,
			Options: profile.ProfileOptions{
				Concurrency:  profileFlags.Concurrency,
				RateLimit:    profileFlags.RateLimit,
				Incremental:  profileFlags.Incremental,
				Force:        profileFlags.Force,
				DryRun:       profileFlags.DryRun,
				IncludeLinks: profileFlags.IncludeLinks,
			},
			Tags: profileFlags.ProfileTags,
		}
	}

	// Override template values with command line flags if provided
	if cmd.Flags().Changed("description") {
		newProfile.Description = profileFlags.Description
	}
	if cmd.Flags().Changed("concurrency") {
		newProfile.Options.Concurrency = profileFlags.Concurrency
	}
	if cmd.Flags().Changed("rate-limit") {
		newProfile.Options.RateLimit = profileFlags.RateLimit
	}
	if cmd.Flags().Changed("tags") {
		newProfile.Tags = profileFlags.ProfileTags
	}

	// Create the profile
	if err := manager.CreateProfile(newProfile); err != nil {
		return fmt.Errorf("failed to create profile: %w", err)
	}

	fmt.Printf("✅ Profile '%s' created successfully\n", profileFlags.Name)
	fmt.Printf("📋 Type: %s\n", getSyncType(*newProfile))
	fmt.Printf("📁 Repository: %s\n", newProfile.Repository)

	if len(newProfile.Tags) > 0 {
		fmt.Printf("🏷️  Tags: %s\n", strings.Join(newProfile.Tags, ", "))
	}

	fmt.Printf("\nUse it with:\n")
	fmt.Printf("  jira-sync sync --profile=%s\n", profileFlags.Name)

	return nil
}

func runProfileShowCommand(cmd *cobra.Command, args []string) error {
	manager := profile.NewFileProfileManager(".", "yaml")
	profileName := args[0]

	p, err := manager.GetProfile(profileName)
	if err != nil {
		return fmt.Errorf("failed to get profile: %w", err)
	}

	fmt.Printf("Profile: %s\n", p.Name)
	fmt.Printf("Description: %s\n", p.Description)
	fmt.Printf("Type: %s\n", getSyncType(*p))
	fmt.Printf("Repository: %s\n", p.Repository)

	// Show sync configuration
	fmt.Printf("\nSync Configuration:\n")
	if p.JQL != "" {
		fmt.Printf("  JQL: %s\n", p.JQL)
	}
	if len(p.IssueKeys) > 0 {
		fmt.Printf("  Issues: %s\n", strings.Join(p.IssueKeys, ", "))
	}
	if p.EpicKey != "" {
		fmt.Printf("  EPIC: %s\n", p.EpicKey)
	}

	// Show options
	fmt.Printf("\nOptions:\n")
	fmt.Printf("  Concurrency: %d\n", p.Options.Concurrency)
	fmt.Printf("  Rate Limit: %s\n", p.Options.RateLimit)
	fmt.Printf("  Incremental: %t\n", p.Options.Incremental)
	fmt.Printf("  Force: %t\n", p.Options.Force)
	fmt.Printf("  Dry Run: %t\n", p.Options.DryRun)
	fmt.Printf("  Include Links: %t\n", p.Options.IncludeLinks)

	// Show metadata
	fmt.Printf("\nMetadata:\n")
	fmt.Printf("  Created: %s\n", p.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("  Updated: %s\n", p.UpdatedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("  Version: %s\n", p.Version)

	if len(p.Tags) > 0 {
		fmt.Printf("  Tags: %s\n", strings.Join(p.Tags, ", "))
	}

	// Show usage statistics if requested
	if profileFlags.ShowStats {
		fmt.Printf("\nUsage Statistics:\n")
		fmt.Printf("  Times Used: %d\n", p.UsageStats.TimesUsed)
		if !p.UsageStats.LastUsed.IsZero() {
			fmt.Printf("  Last Used: %s\n", p.UsageStats.LastUsed.Format("2006-01-02 15:04:05"))
		}
		if p.UsageStats.AvgSyncTime > 0 {
			fmt.Printf("  Avg Sync Time: %s\n", time.Duration(p.UsageStats.AvgSyncTime*int64(time.Millisecond)))
		}
		fmt.Printf("  Success Rate: %.1f%%\n", p.UsageStats.SuccessRate*100)
	}

	return nil
}

func runProfileUpdateCommand(cmd *cobra.Command, args []string) error {
	manager := profile.NewFileProfileManager(".", "yaml")
	profileName := args[0]

	// Get existing profile
	p, err := manager.GetProfile(profileName)
	if err != nil {
		return fmt.Errorf("failed to get profile: %w", err)
	}

	// Update fields that were provided
	updated := false

	if cmd.Flags().Changed("description") {
		p.Description = profileFlags.Description
		updated = true
	}

	if cmd.Flags().Changed("jql") {
		p.JQL = profileFlags.JQL
		p.IssueKeys = nil
		p.EpicKey = ""
		updated = true
	}

	if cmd.Flags().Changed("issues") {
		p.IssueKeys = profileFlags.Issues
		p.JQL = ""
		p.EpicKey = ""
		updated = true
	}

	if cmd.Flags().Changed("epic-key") {
		p.EpicKey = profileFlags.EpicKey
		p.JQL = ""
		p.IssueKeys = nil
		updated = true
	}

	if cmd.Flags().Changed("repository") {
		p.Repository = profileFlags.Repository
		updated = true
	}

	if cmd.Flags().Changed("concurrency") {
		p.Options.Concurrency = profileFlags.Concurrency
		updated = true
	}

	if cmd.Flags().Changed("rate-limit") {
		p.Options.RateLimit = profileFlags.RateLimit
		updated = true
	}

	if cmd.Flags().Changed("incremental") {
		p.Options.Incremental = profileFlags.Incremental
		updated = true
	}

	if cmd.Flags().Changed("force") {
		p.Options.Force = profileFlags.Force
		updated = true
	}

	if cmd.Flags().Changed("dry-run") {
		p.Options.DryRun = profileFlags.DryRun
		updated = true
	}

	if cmd.Flags().Changed("include-links") {
		p.Options.IncludeLinks = profileFlags.IncludeLinks
		updated = true
	}

	if cmd.Flags().Changed("tags") {
		p.Tags = profileFlags.ProfileTags
		updated = true
	}

	if !updated {
		return fmt.Errorf("no changes specified")
	}

	// Update the profile
	if err := manager.UpdateProfile(profileName, p); err != nil {
		return fmt.Errorf("failed to update profile: %w", err)
	}

	fmt.Printf("✅ Profile '%s' updated successfully\n", profileName)

	return nil
}

func runProfileDeleteCommand(cmd *cobra.Command, args []string) error {
	manager := profile.NewFileProfileManager(".", "yaml")
	profileName := args[0]

	// Check if profile exists
	if !manager.ProfileExists(profileName) {
		return fmt.Errorf("profile '%s' does not exist", profileName)
	}

	// Confirm deletion unless force flag is used
	if !profileFlags.ForceDelete {
		fmt.Printf("Are you sure you want to delete profile '%s'? [y/N]: ", profileName)
		var response string
		_, _ = fmt.Scanln(&response)
		if strings.ToLower(response) != "y" && strings.ToLower(response) != "yes" {
			fmt.Println("Delete cancelled")
			return nil
		}
	}

	// Delete the profile
	if err := manager.DeleteProfile(profileName); err != nil {
		return fmt.Errorf("failed to delete profile: %w", err)
	}

	fmt.Printf("✅ Profile '%s' deleted successfully\n", profileName)

	return nil
}

func runProfileTemplatesCommand(cmd *cobra.Command, args []string) error {
	templates := profile.GetBuiltinTemplates()

	if len(templates) == 0 {
		fmt.Println("No templates available")
		return nil
	}

	if profileFlags.Details {
		// Show detailed template information
		for _, tmpl := range templates {
			fmt.Printf("\nTemplate: %s\n", tmpl.ID)
			fmt.Printf("Name: %s\n", tmpl.Name)
			fmt.Printf("Category: %s\n", tmpl.Category)
			fmt.Printf("Description: %s\n", tmpl.Description)

			if len(tmpl.Variables) > 0 {
				fmt.Printf("Variables:\n")
				for _, variable := range tmpl.Variables {
					required := ""
					if variable.Required {
						required = " (required)"
					}
					fmt.Printf("  %s: %s%s\n", variable.Name, variable.Description, required)
					if variable.Example != "" {
						fmt.Printf("    Example: %s\n", variable.Example)
					}
				}
			}

			if len(tmpl.Examples) > 0 {
				fmt.Printf("Examples:\n")
				for _, example := range tmpl.Examples {
					fmt.Printf("  %s\n", example)
				}
			}
		}
	} else {
		// Show compact template list
		categories := profile.GetTemplatesByCategory()

		for category, categoryTemplates := range categories {
			fmt.Printf("\n%s Templates:\n", strings.ToUpper(category[:1])+category[1:])

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			_, _ = fmt.Fprintf(w, "  ID\tNAME\tDESCRIPTION\n")

			sort.Slice(categoryTemplates, func(i, j int) bool {
				return categoryTemplates[i].Name < categoryTemplates[j].Name
			})

			for _, tmpl := range categoryTemplates {
				description := tmpl.Description
				if len(description) > 60 {
					description = description[:57] + "..."
				}
				_, _ = fmt.Fprintf(w, "  %s\t%s\t%s\n", tmpl.ID, tmpl.Name, description)
			}
			_ = w.Flush()
		}

		fmt.Printf("\nUse 'jira-sync profile templates --details' for more information\n")
	}

	return nil
}

func runProfileExportCommand(cmd *cobra.Command, args []string) error {
	manager := profile.NewFileProfileManager(".", "yaml")

	options := &profile.ProfileExportOptions{
		Names:        profileFlags.Names,
		Tags:         profileFlags.Tags,
		IncludeStats: !profileFlags.NoStats,
		Format:       profileFlags.Format,
	}

	if err := manager.ExportToFile(profileFlags.ExportFile, options); err != nil {
		return fmt.Errorf("failed to export profiles: %w", err)
	}

	// Count exported profiles
	collection, err := manager.ExportProfiles(options)
	if err != nil {
		return fmt.Errorf("failed to count exported profiles: %w", err)
	}

	fmt.Printf("✅ Exported %d profiles to %s\n", len(collection.Profiles), profileFlags.ExportFile)

	return nil
}

func runProfileImportCommand(cmd *cobra.Command, args []string) error {
	manager := profile.NewFileProfileManager(".", "yaml")

	// Validate import file first if requested
	if profileFlags.Validate {
		fmt.Printf("🔍 Validating import file...\n")
		validation, err := profile.ValidateImportFile(profileFlags.ImportFile)
		if err != nil {
			return fmt.Errorf("failed to validate import file: %w", err)
		}

		if !validation.Valid {
			fmt.Printf("❌ Import file validation failed:\n")
			for _, err := range validation.Errors {
				fmt.Printf("  • %s\n", err)
			}
			return fmt.Errorf("import file is invalid")
		}

		if len(validation.Warnings) > 0 {
			fmt.Printf("⚠️  Import file warnings:\n")
			for _, warning := range validation.Warnings {
				fmt.Printf("  • %s\n", warning)
			}
		}

		fmt.Printf("✅ Import file is valid\n")
	}

	options := &profile.ProfileImportOptions{
		Overwrite:   profileFlags.Overwrite,
		MergeStats:  false,
		NamePrefix:  profileFlags.Prefix,
		DefaultTags: profileFlags.ImportTags,
		Validate:    profileFlags.Validate,
	}

	if err := manager.ImportFromFile(profileFlags.ImportFile, options); err != nil {
		return fmt.Errorf("failed to import profiles: %w", err)
	}

	fmt.Printf("✅ Profiles imported successfully from %s\n", profileFlags.ImportFile)

	return nil
}

// Helper functions

func getSyncType(p profile.Profile) string {
	if p.EpicKey != "" {
		return "epic"
	}
	if p.JQL != "" {
		return "jql"
	}
	if len(p.IssueKeys) > 0 {
		return "issues"
	}
	return "unknown"
}
