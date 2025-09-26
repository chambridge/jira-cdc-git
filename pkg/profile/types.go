package profile

import (
	"time"
)

// Profile represents a named sync configuration that can be reused
type Profile struct {
	Name        string            `json:"name" yaml:"name"`
	Description string            `json:"description" yaml:"description"`
	JQL         string            `json:"jql,omitempty" yaml:"jql,omitempty"`
	IssueKeys   []string          `json:"issue_keys,omitempty" yaml:"issue_keys,omitempty"`
	EpicKey     string            `json:"epic_key,omitempty" yaml:"epic_key,omitempty"`
	Repository  string            `json:"repository" yaml:"repository"`
	Options     ProfileOptions    `json:"options" yaml:"options"`
	Tags        []string          `json:"tags,omitempty" yaml:"tags,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	CreatedAt   time.Time         `json:"created_at" yaml:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at" yaml:"updated_at"`
	CreatedBy   string            `json:"created_by,omitempty" yaml:"created_by,omitempty"`
	Version     string            `json:"version" yaml:"version"`
	UsageStats  UsageStats        `json:"usage_stats" yaml:"usage_stats"`
}

// ProfileOptions contains sync configuration options for a profile
type ProfileOptions struct {
	Concurrency  int    `json:"concurrency" yaml:"concurrency"`
	RateLimit    string `json:"rate_limit" yaml:"rate_limit"`
	Incremental  bool   `json:"incremental" yaml:"incremental"`
	Force        bool   `json:"force" yaml:"force"`
	DryRun       bool   `json:"dry_run" yaml:"dry_run"`
	IncludeLinks bool   `json:"include_links" yaml:"include_links"`
}

// UsageStats tracks how often a profile is used
type UsageStats struct {
	TimesUsed     int       `json:"times_used" yaml:"times_used"`
	LastUsed      time.Time `json:"last_used" yaml:"last_used"`
	TotalSyncTime int64     `json:"total_sync_time_ms" yaml:"total_sync_time_ms"`
	AvgSyncTime   int64     `json:"avg_sync_time_ms" yaml:"avg_sync_time_ms"`
	LastSuccess   time.Time `json:"last_success" yaml:"last_success"`
	LastFailure   time.Time `json:"last_failure" yaml:"last_failure"`
	SuccessRate   float64   `json:"success_rate" yaml:"success_rate"`
}

// ProfileCollection represents a collection of profiles
type ProfileCollection struct {
	Version   string             `json:"version" yaml:"version"`
	Profiles  map[string]Profile `json:"profiles" yaml:"profiles"`
	Templates map[string]Profile `json:"templates" yaml:"templates"`
	UpdatedAt time.Time          `json:"updated_at" yaml:"updated_at"`
	Metadata  map[string]string  `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

// ProfileType represents the type of profile
type ProfileType string

const (
	ProfileTypeCustom    ProfileType = "custom"
	ProfileTypeTemplate  ProfileType = "template"
	ProfileTypeImported  ProfileType = "imported"
	ProfileTypeGenerated ProfileType = "generated"
)

// SyncMode represents the sync mode for a profile
type SyncMode string

const (
	SyncModeIssues      SyncMode = "issues"
	SyncModeJQL         SyncMode = "jql"
	SyncModeEpic        SyncMode = "epic"
	SyncModeIncremental SyncMode = "incremental"
)

// ProfileValidationResult contains validation results for a profile
type ProfileValidationResult struct {
	Valid    bool     `json:"valid" yaml:"valid"`
	Errors   []string `json:"errors" yaml:"errors"`
	Warnings []string `json:"warnings" yaml:"warnings"`
}

// ProfileTemplate represents a profile template for common use cases
type ProfileTemplate struct {
	ID          string        `json:"id" yaml:"id"`
	Name        string        `json:"name" yaml:"name"`
	Description string        `json:"description" yaml:"description"`
	Category    string        `json:"category" yaml:"category"`
	Template    Profile       `json:"template" yaml:"template"`
	Variables   []TemplateVar `json:"variables" yaml:"variables"`
	Examples    []string      `json:"examples" yaml:"examples"`
}

// TemplateVar represents a variable in a profile template
type TemplateVar struct {
	Name        string `json:"name" yaml:"name"`
	Description string `json:"description" yaml:"description"`
	Type        string `json:"type" yaml:"type"`
	Required    bool   `json:"required" yaml:"required"`
	Default     string `json:"default,omitempty" yaml:"default,omitempty"`
	Example     string `json:"example,omitempty" yaml:"example,omitempty"`
}

// ProfileListOptions contains options for listing profiles
type ProfileListOptions struct {
	Tags         []string `json:"tags,omitempty" yaml:"tags,omitempty"`
	Category     string   `json:"category,omitempty" yaml:"category,omitempty"`
	IncludeStats bool     `json:"include_stats" yaml:"include_stats"`
	SortBy       string   `json:"sort_by" yaml:"sort_by"`
	SortOrder    string   `json:"sort_order" yaml:"sort_order"`
	Limit        int      `json:"limit,omitempty" yaml:"limit,omitempty"`
}

// ProfileExportOptions contains options for exporting profiles
type ProfileExportOptions struct {
	Names        []string `json:"names,omitempty" yaml:"names,omitempty"`
	Tags         []string `json:"tags,omitempty" yaml:"tags,omitempty"`
	IncludeStats bool     `json:"include_stats" yaml:"include_stats"`
	Format       string   `json:"format" yaml:"format"`
}

// ProfileImportOptions contains options for importing profiles
type ProfileImportOptions struct {
	Overwrite   bool     `json:"overwrite" yaml:"overwrite"`
	MergeStats  bool     `json:"merge_stats" yaml:"merge_stats"`
	NamePrefix  string   `json:"name_prefix,omitempty" yaml:"name_prefix,omitempty"`
	DefaultTags []string `json:"default_tags,omitempty" yaml:"default_tags,omitempty"`
	Validate    bool     `json:"validate" yaml:"validate"`
}

// ProfileSearchOptions contains options for searching profiles
type ProfileSearchOptions struct {
	Query      string   `json:"query" yaml:"query"`
	Tags       []string `json:"tags,omitempty" yaml:"tags,omitempty"`
	Repository string   `json:"repository,omitempty" yaml:"repository,omitempty"`
	IncludeJQL bool     `json:"include_jql" yaml:"include_jql"`
	MatchMode  string   `json:"match_mode" yaml:"match_mode"` // exact, contains, regex
}

const (
	ProfileVersion = "v0.3.0"
	ProfilesDir    = ".jira-sync-profiles"
	ProfilesFile   = "profiles.yaml"
	TemplatesFile  = "templates.yaml"
)
