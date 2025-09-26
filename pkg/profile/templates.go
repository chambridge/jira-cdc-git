package profile

import (
	"fmt"
	"strings"
	"text/template"
)

// GetBuiltinTemplates returns the built-in profile templates
func GetBuiltinTemplates() []ProfileTemplate {
	return []ProfileTemplate{
		{
			ID:          "epic-all-issues",
			Name:        "EPIC - All Issues",
			Description: "Sync all issues associated with an EPIC (stories, subtasks, and related issues)",
			Category:    "epic",
			Template: Profile{
				Name:        "{{.name}}",
				Description: "Sync for EPIC {{.epic_key}} - all associated issues",
				EpicKey:     "{{.epic_key}}",
				Repository:  "{{.repository}}",
				Options: ProfileOptions{
					Concurrency:  5,
					RateLimit:    "500ms",
					Incremental:  false,
					Force:        false,
					DryRun:       false,
					IncludeLinks: true,
				},
				Tags: []string{"epic", "comprehensive"},
			},
			Variables: []TemplateVar{
				{
					Name:        "name",
					Description: "Profile name",
					Type:        "string",
					Required:    true,
					Example:     "my-epic-sync",
				},
				{
					Name:        "epic_key",
					Description: "JIRA EPIC key",
					Type:        "string",
					Required:    true,
					Example:     "PROJ-123",
				},
				{
					Name:        "repository",
					Description: "Target Git repository path",
					Type:        "string",
					Required:    true,
					Example:     "./my-repo",
				},
			},
			Examples: []string{
				"jira-sync profile create --template=epic-all-issues --name=product-epic --epic_key=RHOAIENG-456 --repository=./product-repo",
			},
		},
		{
			ID:          "epic-stories-only",
			Name:        "EPIC - Stories Only",
			Description: "Sync only the stories under an EPIC (exclude subtasks and related issues)",
			Category:    "epic",
			Template: Profile{
				Name:        "{{.name}}",
				Description: "Sync for EPIC {{.epic_key}} - stories only",
				JQL:         "\"Epic Link\" = {{.epic_key}} AND type = Story",
				Repository:  "{{.repository}}",
				Options: ProfileOptions{
					Concurrency:  5,
					RateLimit:    "500ms",
					Incremental:  false,
					Force:        false,
					DryRun:       false,
					IncludeLinks: true,
				},
				Tags: []string{"epic", "stories", "filtered"},
			},
			Variables: []TemplateVar{
				{
					Name:        "name",
					Description: "Profile name",
					Type:        "string",
					Required:    true,
					Example:     "epic-stories",
				},
				{
					Name:        "epic_key",
					Description: "JIRA EPIC key",
					Type:        "string",
					Required:    true,
					Example:     "PROJ-123",
				},
				{
					Name:        "repository",
					Description: "Target Git repository path",
					Type:        "string",
					Required:    true,
					Example:     "./my-repo",
				},
			},
			Examples: []string{
				"jira-sync profile create --template=epic-stories-only --name=feature-stories --epic_key=RHOAIENG-789 --repository=./feature-repo",
			},
		},
		{
			ID:          "project-active-issues",
			Name:        "Project - Active Issues",
			Description: "Sync all active (non-closed) issues in a project",
			Category:    "project",
			Template: Profile{
				Name:        "{{.name}}",
				Description: "Active issues for project {{.project_key}}",
				JQL:         "project = {{.project_key}} AND status != Closed AND status != Done",
				Repository:  "{{.repository}}",
				Options: ProfileOptions{
					Concurrency:  8,
					RateLimit:    "300ms",
					Incremental:  true,
					Force:        false,
					DryRun:       false,
					IncludeLinks: true,
				},
				Tags: []string{"project", "active", "incremental"},
			},
			Variables: []TemplateVar{
				{
					Name:        "name",
					Description: "Profile name",
					Type:        "string",
					Required:    true,
					Example:     "project-active",
				},
				{
					Name:        "project_key",
					Description: "JIRA project key",
					Type:        "string",
					Required:    true,
					Example:     "RHOAIENG",
				},
				{
					Name:        "repository",
					Description: "Target Git repository path",
					Type:        "string",
					Required:    true,
					Example:     "./project-repo",
				},
			},
			Examples: []string{
				"jira-sync profile create --template=project-active-issues --name=rhoai-active --project_key=RHOAIENG --repository=./rhoai-issues",
			},
		},
		{
			ID:          "my-current-sprint",
			Name:        "My Current Sprint",
			Description: "Sync issues assigned to you in the current sprint",
			Category:    "personal",
			Template: Profile{
				Name:        "{{.name}}",
				Description: "My issues in current sprint",
				JQL:         "assignee = currentUser() AND sprint in openSprints()",
				Repository:  "{{.repository}}",
				Options: ProfileOptions{
					Concurrency:  3,
					RateLimit:    "1s",
					Incremental:  true,
					Force:        false,
					DryRun:       false,
					IncludeLinks: false,
				},
				Tags: []string{"personal", "sprint", "current"},
			},
			Variables: []TemplateVar{
				{
					Name:        "name",
					Description: "Profile name",
					Type:        "string",
					Required:    true,
					Example:     "my-sprint",
				},
				{
					Name:        "repository",
					Description: "Target Git repository path",
					Type:        "string",
					Required:    true,
					Example:     "./my-work",
				},
			},
			Examples: []string{
				"jira-sync profile create --template=my-current-sprint --name=my-current-work --repository=./current-sprint",
			},
		},
		{
			ID:          "recent-updates",
			Name:        "Recent Updates",
			Description: "Sync recently updated issues across projects",
			Category:    "monitoring",
			Template: Profile{
				Name:        "{{.name}}",
				Description: "Recently updated issues (last {{.days}} days)",
				JQL:         "updated >= -{{.days}}d{{if .project_filter}} AND project in ({{.project_filter}}){{end}}",
				Repository:  "{{.repository}}",
				Options: ProfileOptions{
					Concurrency:  6,
					RateLimit:    "400ms",
					Incremental:  true,
					Force:        false,
					DryRun:       false,
					IncludeLinks: true,
				},
				Tags: []string{"recent", "updates", "monitoring"},
			},
			Variables: []TemplateVar{
				{
					Name:        "name",
					Description: "Profile name",
					Type:        "string",
					Required:    true,
					Example:     "recent-changes",
				},
				{
					Name:        "repository",
					Description: "Target Git repository path",
					Type:        "string",
					Required:    true,
					Example:     "./recent-updates",
				},
				{
					Name:        "days",
					Description: "Number of days to look back",
					Type:        "number",
					Required:    false,
					Default:     "7",
					Example:     "14",
				},
				{
					Name:        "project_filter",
					Description: "Comma-separated list of project keys (optional)",
					Type:        "string",
					Required:    false,
					Example:     "RHOAIENG,KUBEFLOW",
				},
			},
			Examples: []string{
				"jira-sync profile create --template=recent-updates --name=weekly-updates --repository=./updates --days=7",
				"jira-sync profile create --template=recent-updates --name=ai-updates --repository=./ai-updates --days=3 --project_filter=RHOAIENG",
			},
		},
		{
			ID:          "custom-jql",
			Name:        "Custom JQL Query",
			Description: "Create a profile with a custom JQL query",
			Category:    "custom",
			Template: Profile{
				Name:        "{{.name}}",
				Description: "Custom JQL: {{.jql}}",
				JQL:         "{{.jql}}",
				Repository:  "{{.repository}}",
				Options: ProfileOptions{
					Concurrency:  5,
					RateLimit:    "500ms",
					Incremental:  false,
					Force:        false,
					DryRun:       false,
					IncludeLinks: true,
				},
				Tags: []string{"custom", "jql"},
			},
			Variables: []TemplateVar{
				{
					Name:        "name",
					Description: "Profile name",
					Type:        "string",
					Required:    true,
					Example:     "custom-query",
				},
				{
					Name:        "jql",
					Description: "Custom JQL query",
					Type:        "string",
					Required:    true,
					Example:     "project = PROJ AND priority = High",
				},
				{
					Name:        "repository",
					Description: "Target Git repository path",
					Type:        "string",
					Required:    true,
					Example:     "./custom-sync",
				},
			},
			Examples: []string{
				"jira-sync profile create --template=custom-jql --name=high-priority --jql=\"priority = High AND status != Closed\" --repository=./urgent",
			},
		},
		{
			ID:          "issue-list",
			Name:        "Specific Issue List",
			Description: "Sync a specific list of issue keys",
			Category:    "custom",
			Template: Profile{
				Name:        "{{.name}}",
				Description: "Specific issues: {{.issue_keys}}",
				IssueKeys:   []string{}, // Will be populated from variable
				Repository:  "{{.repository}}",
				Options: ProfileOptions{
					Concurrency:  3,
					RateLimit:    "500ms",
					Incremental:  false,
					Force:        false,
					DryRun:       false,
					IncludeLinks: true,
				},
				Tags: []string{"specific", "issues"},
			},
			Variables: []TemplateVar{
				{
					Name:        "name",
					Description: "Profile name",
					Type:        "string",
					Required:    true,
					Example:     "specific-issues",
				},
				{
					Name:        "issue_keys",
					Description: "Comma-separated list of issue keys",
					Type:        "string",
					Required:    true,
					Example:     "PROJ-123,PROJ-456,PROJ-789",
				},
				{
					Name:        "repository",
					Description: "Target Git repository path",
					Type:        "string",
					Required:    true,
					Example:     "./specific-issues",
				},
			},
			Examples: []string{
				"jira-sync profile create --template=issue-list --name=release-issues --issue_keys=PROJ-123,PROJ-456 --repository=./release",
			},
		},
	}
}

// GetTemplates returns built-in templates (implements ProfileManager interface)
func (m *FileProfileManager) GetTemplates() ([]ProfileTemplate, error) {
	return GetBuiltinTemplates(), nil
}

// GetTemplate returns a specific template by ID
func (m *FileProfileManager) GetTemplate(id string) (*ProfileTemplate, error) {
	templates := GetBuiltinTemplates()

	for _, tmpl := range templates {
		if tmpl.ID == id {
			return &tmpl, nil
		}
	}

	return nil, fmt.Errorf("template '%s' not found", id)
}

// CreateFromTemplate creates a profile from a template with variable substitution
func (m *FileProfileManager) CreateFromTemplate(templateID string, name string, variables map[string]string) (*Profile, error) {
	tmpl, err := m.GetTemplate(templateID)
	if err != nil {
		return nil, fmt.Errorf("failed to get template: %w", err)
	}

	// Validate required variables (excluding 'name' which is provided as parameter)
	for _, variable := range tmpl.Variables {
		if variable.Required && variable.Name != "name" {
			if _, exists := variables[variable.Name]; !exists {
				return nil, fmt.Errorf("required variable '%s' not provided", variable.Name)
			}
		}
	}

	// Apply defaults for missing optional variables and add name parameter
	variablesWithDefaults := make(map[string]string)
	variablesWithDefaults["name"] = name // Add name parameter to variables
	for k, v := range variables {
		variablesWithDefaults[k] = v
	}

	for _, variable := range tmpl.Variables {
		if !variable.Required && variable.Default != "" {
			if _, exists := variablesWithDefaults[variable.Name]; !exists {
				variablesWithDefaults[variable.Name] = variable.Default
			}
		}
	}

	// Ensure name is set
	variablesWithDefaults["name"] = name

	// Create profile from template
	profile := tmpl.Template
	profile.Name = name

	// Apply template substitution
	if err := m.applyTemplateSubstitution(&profile, variablesWithDefaults); err != nil {
		return nil, fmt.Errorf("failed to apply template substitution: %w", err)
	}

	// Special handling for issue_keys variable (comma-separated to slice)
	if issueKeysStr, exists := variablesWithDefaults["issue_keys"]; exists && issueKeysStr != "" {
		issueKeys := strings.Split(issueKeysStr, ",")
		for i, key := range issueKeys {
			issueKeys[i] = strings.TrimSpace(key)
		}
		profile.IssueKeys = issueKeys
	}

	// Validate the generated profile
	validation, err := m.ValidateProfile(&profile)
	if err != nil {
		return nil, fmt.Errorf("failed to validate generated profile: %w", err)
	}

	if !validation.Valid {
		return nil, fmt.Errorf("generated profile is invalid: %s", strings.Join(validation.Errors, "; "))
	}

	return &profile, nil
}

// applyTemplateSubstitution applies template variable substitution to a profile
func (m *FileProfileManager) applyTemplateSubstitution(profile *Profile, variables map[string]string) error {
	// Substitute in description
	if profile.Description != "" {
		tmpl, err := template.New("description").Parse(profile.Description)
		if err != nil {
			return fmt.Errorf("failed to parse description template: %w", err)
		}

		var result strings.Builder
		if err := tmpl.Execute(&result, variables); err != nil {
			return fmt.Errorf("failed to execute description template: %w", err)
		}
		profile.Description = result.String()
	}

	// Substitute in JQL
	if profile.JQL != "" {
		tmpl, err := template.New("jql").Parse(profile.JQL)
		if err != nil {
			return fmt.Errorf("failed to parse JQL template: %w", err)
		}

		var result strings.Builder
		if err := tmpl.Execute(&result, variables); err != nil {
			return fmt.Errorf("failed to execute JQL template: %w", err)
		}
		profile.JQL = result.String()
	}

	// Substitute in EpicKey
	if profile.EpicKey != "" {
		tmpl, err := template.New("epic").Parse(profile.EpicKey)
		if err != nil {
			return fmt.Errorf("failed to parse epic key template: %w", err)
		}

		var result strings.Builder
		if err := tmpl.Execute(&result, variables); err != nil {
			return fmt.Errorf("failed to execute epic key template: %w", err)
		}
		profile.EpicKey = result.String()
	}

	// Substitute in Repository
	if profile.Repository != "" {
		tmpl, err := template.New("repo").Parse(profile.Repository)
		if err != nil {
			return fmt.Errorf("failed to parse repository template: %w", err)
		}

		var result strings.Builder
		if err := tmpl.Execute(&result, variables); err != nil {
			return fmt.Errorf("failed to execute repository template: %w", err)
		}
		profile.Repository = result.String()
	}

	return nil
}

// GetTemplatesByCategory returns templates grouped by category
func GetTemplatesByCategory() map[string][]ProfileTemplate {
	templates := GetBuiltinTemplates()
	byCategory := make(map[string][]ProfileTemplate)

	for _, tmpl := range templates {
		category := tmpl.Category
		if category == "" {
			category = "other"
		}
		byCategory[category] = append(byCategory[category], tmpl)
	}

	return byCategory
}

// ValidateTemplateVariables validates that all required variables are provided
func ValidateTemplateVariables(template *ProfileTemplate, variables map[string]string) error {
	for _, variable := range template.Variables {
		if variable.Required {
			if value, exists := variables[variable.Name]; !exists || value == "" {
				return fmt.Errorf("required variable '%s' is missing or empty", variable.Name)
			}
		}
	}
	return nil
}
