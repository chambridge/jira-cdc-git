package profile

import (
	"fmt"
	"sort"
	"time"
)

// MockProfileManager implements ProfileManager for testing
type MockProfileManager struct {
	profiles  map[string]Profile
	templates []ProfileTemplate
}

// NewMockProfileManager creates a new mock profile manager
func NewMockProfileManager() *MockProfileManager {
	return &MockProfileManager{
		profiles:  make(map[string]Profile),
		templates: GetBuiltinTemplates(),
	}
}

// CreateProfile creates a new profile
func (m *MockProfileManager) CreateProfile(profile *Profile) error {
	if profile == nil {
		return fmt.Errorf("profile cannot be nil")
	}

	if _, exists := m.profiles[profile.Name]; exists {
		return fmt.Errorf("profile '%s' already exists", profile.Name)
	}

	// Set timestamps
	now := time.Now()
	profile.CreatedAt = now
	profile.UpdatedAt = now
	profile.Version = ProfileVersion

	m.profiles[profile.Name] = *profile
	return nil
}

// GetProfile retrieves a profile by name
func (m *MockProfileManager) GetProfile(name string) (*Profile, error) {
	profile, exists := m.profiles[name]
	if !exists {
		return nil, fmt.Errorf("profile '%s' not found", name)
	}
	return &profile, nil
}

// UpdateProfile updates an existing profile
func (m *MockProfileManager) UpdateProfile(name string, profile *Profile) error {
	if profile == nil {
		return fmt.Errorf("profile cannot be nil")
	}

	existing, exists := m.profiles[name]
	if !exists {
		return fmt.Errorf("profile '%s' not found", name)
	}

	// Preserve creation time and usage stats
	profile.CreatedAt = existing.CreatedAt
	profile.UsageStats = existing.UsageStats
	profile.UpdatedAt = time.Now()
	profile.Version = ProfileVersion
	profile.Name = name

	m.profiles[name] = *profile
	return nil
}

// DeleteProfile deletes a profile
func (m *MockProfileManager) DeleteProfile(name string) error {
	if _, exists := m.profiles[name]; !exists {
		return fmt.Errorf("profile '%s' not found", name)
	}

	delete(m.profiles, name)
	return nil
}

// ListProfiles lists profiles with optional filtering
func (m *MockProfileManager) ListProfiles(options *ProfileListOptions) ([]Profile, error) {
	var profiles []Profile

	for _, profile := range m.profiles {
		// Apply filters
		if options != nil && len(options.Tags) > 0 {
			hasMatchingTag := false
			for _, reqTag := range options.Tags {
				for _, profileTag := range profile.Tags {
					if profileTag == reqTag {
						hasMatchingTag = true
						break
					}
				}
				if hasMatchingTag {
					break
				}
			}
			if !hasMatchingTag {
				continue
			}
		}

		profiles = append(profiles, profile)
	}

	// Sort profiles by name by default
	sort.Slice(profiles, func(i, j int) bool {
		return profiles[i].Name < profiles[j].Name
	})

	// Apply limit
	if options != nil && options.Limit > 0 && len(profiles) > options.Limit {
		profiles = profiles[:options.Limit]
	}

	return profiles, nil
}

// ProfileExists checks if a profile exists
func (m *MockProfileManager) ProfileExists(name string) bool {
	_, exists := m.profiles[name]
	return exists
}

// ValidateProfile validates a profile configuration
func (m *MockProfileManager) ValidateProfile(profile *Profile) (*ProfileValidationResult, error) {
	result := &ProfileValidationResult{
		Valid:    true,
		Errors:   make([]string, 0),
		Warnings: make([]string, 0),
	}

	// Basic validation
	if profile.Name == "" {
		result.Valid = false
		result.Errors = append(result.Errors, "profile name cannot be empty")
	}

	if profile.Repository == "" {
		result.Valid = false
		result.Errors = append(result.Errors, "repository path is required")
	}

	// Validate sync mode
	syncModes := 0
	if profile.JQL != "" {
		syncModes++
	}
	if len(profile.IssueKeys) > 0 {
		syncModes++
	}
	if profile.EpicKey != "" {
		syncModes++
	}

	if syncModes == 0 {
		result.Valid = false
		result.Errors = append(result.Errors, "must specify at least one sync mode")
	} else if syncModes > 1 {
		result.Valid = false
		result.Errors = append(result.Errors, "can only specify one sync mode")
	}

	return result, nil
}

// DuplicateProfile creates a copy of an existing profile
func (m *MockProfileManager) DuplicateProfile(sourceName, targetName string) error {
	source, err := m.GetProfile(sourceName)
	if err != nil {
		return err
	}

	newProfile := *source
	newProfile.Name = targetName
	newProfile.CreatedAt = time.Time{}
	newProfile.UpdatedAt = time.Time{}
	newProfile.UsageStats = UsageStats{}

	return m.CreateProfile(&newProfile)
}

// RenameProfile renames an existing profile
func (m *MockProfileManager) RenameProfile(oldName, newName string) error {
	profile, err := m.GetProfile(oldName)
	if err != nil {
		return err
	}

	if _, exists := m.profiles[newName]; exists {
		return fmt.Errorf("profile '%s' already exists", newName)
	}

	profile.Name = newName
	profile.UpdatedAt = time.Now()

	m.profiles[newName] = *profile
	delete(m.profiles, oldName)

	return nil
}

// GetTemplates returns available templates
func (m *MockProfileManager) GetTemplates() ([]ProfileTemplate, error) {
	return m.templates, nil
}

// GetTemplate returns a specific template
func (m *MockProfileManager) GetTemplate(id string) (*ProfileTemplate, error) {
	for _, tmpl := range m.templates {
		if tmpl.ID == id {
			return &tmpl, nil
		}
	}
	return nil, fmt.Errorf("template '%s' not found", id)
}

// CreateFromTemplate creates a profile from a template
func (m *MockProfileManager) CreateFromTemplate(templateID string, name string, variables map[string]string) (*Profile, error) {
	template, err := m.GetTemplate(templateID)
	if err != nil {
		return nil, err
	}

	// Validate required variables (excluding 'name' which is provided as parameter)
	for _, variable := range template.Variables {
		if variable.Required && variable.Name != "name" {
			if _, exists := variables[variable.Name]; !exists {
				return nil, fmt.Errorf("required variable '%s' not provided", variable.Name)
			}
		}
	}

	// Create profile from template (simplified)
	profile := template.Template
	profile.Name = name

	// Apply basic variable substitution
	if epicKey, exists := variables["epic_key"]; exists {
		profile.EpicKey = epicKey
	}
	if jql, exists := variables["jql"]; exists {
		profile.JQL = jql
	}
	if repo, exists := variables["repository"]; exists {
		profile.Repository = repo
	}

	return &profile, nil
}

// RecordUsage records usage statistics
func (m *MockProfileManager) RecordUsage(name string, syncDuration int64, success bool) error {
	profile, exists := m.profiles[name]
	if !exists {
		return fmt.Errorf("profile '%s' not found", name)
	}

	stats := &profile.UsageStats
	stats.TimesUsed++
	stats.LastUsed = time.Now()
	stats.TotalSyncTime += syncDuration

	if stats.TimesUsed > 0 {
		stats.AvgSyncTime = stats.TotalSyncTime / int64(stats.TimesUsed)
	}

	if success {
		stats.LastSuccess = time.Now()
		stats.SuccessRate = 1.0 // Simplified
	}

	m.profiles[name] = profile
	return nil
}

// GetUsageStats retrieves usage statistics
func (m *MockProfileManager) GetUsageStats(name string) (*UsageStats, error) {
	profile, err := m.GetProfile(name)
	if err != nil {
		return nil, err
	}
	return &profile.UsageStats, nil
}

// GetMostUsedProfiles returns most used profiles
func (m *MockProfileManager) GetMostUsedProfiles(limit int) ([]Profile, error) {
	var profiles []Profile
	for _, profile := range m.profiles {
		profiles = append(profiles, profile)
	}

	sort.Slice(profiles, func(i, j int) bool {
		return profiles[i].UsageStats.TimesUsed > profiles[j].UsageStats.TimesUsed
	})

	if limit > 0 && len(profiles) > limit {
		profiles = profiles[:limit]
	}

	return profiles, nil
}

// Stubs for remaining interface methods
func (m *MockProfileManager) ExportProfiles(options *ProfileExportOptions) (*ProfileCollection, error) {
	return &ProfileCollection{
		Version:  ProfileVersion,
		Profiles: m.profiles,
	}, nil
}

func (m *MockProfileManager) ImportProfiles(collection *ProfileCollection, options *ProfileImportOptions) error {
	for name, profile := range collection.Profiles {
		m.profiles[name] = profile
	}
	return nil
}

func (m *MockProfileManager) ExportToFile(filePath string, options *ProfileExportOptions) error {
	return nil // Stub
}

func (m *MockProfileManager) ImportFromFile(filePath string, options *ProfileImportOptions) error {
	return nil // Stub
}

func (m *MockProfileManager) SearchProfiles(options *ProfileSearchOptions) ([]Profile, error) {
	return m.ListProfiles(nil) // Simplified
}

func (m *MockProfileManager) GetSimilarProfiles(profile *Profile, limit int) ([]Profile, error) {
	return []Profile{}, nil // Stub
}

func (m *MockProfileManager) GetCollection() (*ProfileCollection, error) {
	return &ProfileCollection{
		Version:  ProfileVersion,
		Profiles: m.profiles,
	}, nil
}

func (m *MockProfileManager) SaveCollection(collection *ProfileCollection) error {
	return nil // Stub
}

func (m *MockProfileManager) BackupProfiles() error {
	return nil // Stub
}

func (m *MockProfileManager) RestoreProfiles() error {
	return nil // Stub
}

func (m *MockProfileManager) ValidateCollection() (*ProfileValidationResult, error) {
	return &ProfileValidationResult{Valid: true}, nil
}

func (m *MockProfileManager) RepairCollection() error {
	return nil // Stub
}

func (m *MockProfileManager) CleanupOrphanedProfiles() error {
	return nil // Stub
}
