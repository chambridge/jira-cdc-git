package profile

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// FileProfileManager implements ProfileManager using file-based storage
type FileProfileManager struct {
	profilesDir string
	format      string // "yaml" or "json"
}

// NewFileProfileManager creates a new file-based profile manager
func NewFileProfileManager(baseDir string, format string) *FileProfileManager {
	if format != "yaml" && format != "json" {
		format = "yaml" // Default to YAML
	}

	profilesDir := filepath.Join(baseDir, ProfilesDir)

	return &FileProfileManager{
		profilesDir: profilesDir,
		format:      format,
	}
}

// getProfilesFilePath returns the path to the profiles file
func (m *FileProfileManager) getProfilesFilePath() string {
	if m.format == "json" {
		return filepath.Join(m.profilesDir, "profiles.json")
	}
	return filepath.Join(m.profilesDir, ProfilesFile)
}

// ensureProfilesDir creates the profiles directory if it doesn't exist
func (m *FileProfileManager) ensureProfilesDir() error {
	return os.MkdirAll(m.profilesDir, 0755)
}

// loadCollection loads the profile collection from file
func (m *FileProfileManager) loadCollection() (*ProfileCollection, error) {
	if err := m.ensureProfilesDir(); err != nil {
		return nil, fmt.Errorf("failed to create profiles directory: %w", err)
	}

	profilesFile := m.getProfilesFilePath()

	// Check if profiles file exists
	if _, err := os.Stat(profilesFile); os.IsNotExist(err) {
		// Create empty collection
		collection := &ProfileCollection{
			Version:   ProfileVersion,
			Profiles:  make(map[string]Profile),
			Templates: make(map[string]Profile),
			UpdatedAt: time.Now(),
			Metadata:  make(map[string]string),
		}

		if err := m.SaveCollection(collection); err != nil {
			return nil, fmt.Errorf("failed to create initial profiles file: %w", err)
		}

		return collection, nil
	}

	// Read existing file
	data, err := os.ReadFile(profilesFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read profiles file: %w", err)
	}

	var collection ProfileCollection
	if m.format == "json" {
		if err := json.Unmarshal(data, &collection); err != nil {
			return nil, fmt.Errorf("failed to parse JSON profiles file: %w", err)
		}
	} else {
		if err := yaml.Unmarshal(data, &collection); err != nil {
			return nil, fmt.Errorf("failed to parse YAML profiles file: %w", err)
		}
	}

	// Initialize maps if nil
	if collection.Profiles == nil {
		collection.Profiles = make(map[string]Profile)
	}
	if collection.Templates == nil {
		collection.Templates = make(map[string]Profile)
	}
	if collection.Metadata == nil {
		collection.Metadata = make(map[string]string)
	}

	return &collection, nil
}

// GetCollection returns the current profile collection
func (m *FileProfileManager) GetCollection() (*ProfileCollection, error) {
	return m.loadCollection()
}

// SaveCollection saves the profile collection to file
func (m *FileProfileManager) SaveCollection(collection *ProfileCollection) error {
	if collection == nil {
		return fmt.Errorf("collection cannot be nil")
	}

	if err := m.ensureProfilesDir(); err != nil {
		return fmt.Errorf("failed to create profiles directory: %w", err)
	}

	// Update metadata
	collection.Version = ProfileVersion
	collection.UpdatedAt = time.Now()

	// Marshal to bytes
	var data []byte
	var err error
	if m.format == "json" {
		data, err = json.MarshalIndent(collection, "", "  ")
	} else {
		data, err = yaml.Marshal(collection)
	}
	if err != nil {
		return fmt.Errorf("failed to marshal collection: %w", err)
	}

	// Write to temp file first for atomic operation
	profilesFile := m.getProfilesFilePath()
	tempFile := profilesFile + ".tmp"

	if err := os.WriteFile(tempFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write temp profiles file: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tempFile, profilesFile); err != nil {
		_ = os.Remove(tempFile)
		return fmt.Errorf("failed to rename temp profiles file: %w", err)
	}

	return nil
}

// CreateProfile creates a new profile
func (m *FileProfileManager) CreateProfile(profile *Profile) error {
	if profile == nil {
		return fmt.Errorf("profile cannot be nil")
	}

	if err := m.validateProfileName(profile.Name); err != nil {
		return err
	}

	collection, err := m.loadCollection()
	if err != nil {
		return fmt.Errorf("failed to load collection: %w", err)
	}

	if _, exists := collection.Profiles[profile.Name]; exists {
		return fmt.Errorf("profile '%s' already exists", profile.Name)
	}

	// Set metadata
	now := time.Now()
	profile.CreatedAt = now
	profile.UpdatedAt = now
	profile.Version = ProfileVersion

	// Initialize usage stats if not set
	if profile.UsageStats == (UsageStats{}) {
		profile.UsageStats = UsageStats{
			TimesUsed:   0,
			SuccessRate: 0.0,
		}
	}

	collection.Profiles[profile.Name] = *profile

	return m.SaveCollection(collection)
}

// GetProfile retrieves a profile by name
func (m *FileProfileManager) GetProfile(name string) (*Profile, error) {
	collection, err := m.loadCollection()
	if err != nil {
		return nil, fmt.Errorf("failed to load collection: %w", err)
	}

	profile, exists := collection.Profiles[name]
	if !exists {
		return nil, fmt.Errorf("profile '%s' not found", name)
	}

	return &profile, nil
}

// UpdateProfile updates an existing profile
func (m *FileProfileManager) UpdateProfile(name string, profile *Profile) error {
	if profile == nil {
		return fmt.Errorf("profile cannot be nil")
	}

	collection, err := m.loadCollection()
	if err != nil {
		return fmt.Errorf("failed to load collection: %w", err)
	}

	existing, exists := collection.Profiles[name]
	if !exists {
		return fmt.Errorf("profile '%s' not found", name)
	}

	// Preserve creation time and usage stats
	profile.CreatedAt = existing.CreatedAt
	profile.UsageStats = existing.UsageStats
	profile.UpdatedAt = time.Now()
	profile.Version = ProfileVersion
	profile.Name = name // Ensure name consistency

	collection.Profiles[name] = *profile

	return m.SaveCollection(collection)
}

// DeleteProfile deletes a profile
func (m *FileProfileManager) DeleteProfile(name string) error {
	collection, err := m.loadCollection()
	if err != nil {
		return fmt.Errorf("failed to load collection: %w", err)
	}

	if _, exists := collection.Profiles[name]; !exists {
		return fmt.Errorf("profile '%s' not found", name)
	}

	delete(collection.Profiles, name)

	return m.SaveCollection(collection)
}

// ListProfiles lists profiles with optional filtering
func (m *FileProfileManager) ListProfiles(options *ProfileListOptions) ([]Profile, error) {
	collection, err := m.loadCollection()
	if err != nil {
		return nil, fmt.Errorf("failed to load collection: %w", err)
	}

	var profiles []Profile
	for _, profile := range collection.Profiles {
		// Apply filters
		if options != nil {
			// Filter by tags
			if len(options.Tags) > 0 {
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
		}

		profiles = append(profiles, profile)
	}

	// Sort profiles
	if options != nil && options.SortBy != "" {
		m.sortProfiles(profiles, options.SortBy, options.SortOrder)
	} else {
		// Default sort by name
		sort.Slice(profiles, func(i, j int) bool {
			return profiles[i].Name < profiles[j].Name
		})
	}

	// Apply limit
	if options != nil && options.Limit > 0 && len(profiles) > options.Limit {
		profiles = profiles[:options.Limit]
	}

	return profiles, nil
}

// ProfileExists checks if a profile exists
func (m *FileProfileManager) ProfileExists(name string) bool {
	_, err := m.GetProfile(name)
	return err == nil
}

// ValidateProfile validates a profile configuration
func (m *FileProfileManager) ValidateProfile(profile *Profile) (*ProfileValidationResult, error) {
	result := &ProfileValidationResult{
		Valid:    true,
		Errors:   make([]string, 0),
		Warnings: make([]string, 0),
	}

	// Validate name
	if err := m.validateProfileName(profile.Name); err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, err.Error())
	}

	// Validate sync mode
	syncModeCount := 0
	if profile.JQL != "" {
		syncModeCount++
	}
	if len(profile.IssueKeys) > 0 {
		syncModeCount++
	}
	if profile.EpicKey != "" {
		syncModeCount++
	}

	if syncModeCount == 0 {
		result.Valid = false
		result.Errors = append(result.Errors, "profile must specify at least one sync mode (JQL, issue keys, or epic key)")
	} else if syncModeCount > 1 {
		result.Valid = false
		result.Errors = append(result.Errors, "profile can only specify one sync mode (JQL, issue keys, or epic key)")
	}

	// Validate repository path
	if profile.Repository == "" {
		result.Valid = false
		result.Errors = append(result.Errors, "repository path is required")
	}

	// Validate options
	if profile.Options.Concurrency < 1 || profile.Options.Concurrency > 10 {
		result.Warnings = append(result.Warnings, "concurrency should be between 1 and 10")
	}

	// Validate rate limit format
	if profile.Options.RateLimit != "" {
		if _, err := time.ParseDuration(profile.Options.RateLimit); err != nil {
			result.Valid = false
			result.Errors = append(result.Errors, fmt.Sprintf("invalid rate limit format: %v", err))
		}
	}

	// Validate mutually exclusive options
	if profile.Options.Incremental && profile.Options.Force {
		result.Valid = false
		result.Errors = append(result.Errors, "incremental and force options are mutually exclusive")
	}

	return result, nil
}

// DuplicateProfile creates a copy of an existing profile with a new name
func (m *FileProfileManager) DuplicateProfile(sourceName, targetName string) error {
	sourceProfile, err := m.GetProfile(sourceName)
	if err != nil {
		return fmt.Errorf("failed to get source profile: %w", err)
	}

	// Create copy with new name
	newProfile := *sourceProfile
	newProfile.Name = targetName
	newProfile.Description = fmt.Sprintf("Copy of %s", sourceProfile.Description)

	// Reset creation metadata
	newProfile.CreatedAt = time.Time{}
	newProfile.UpdatedAt = time.Time{}
	newProfile.UsageStats = UsageStats{}

	return m.CreateProfile(&newProfile)
}

// RenameProfile renames an existing profile
func (m *FileProfileManager) RenameProfile(oldName, newName string) error {
	if err := m.validateProfileName(newName); err != nil {
		return err
	}

	profile, err := m.GetProfile(oldName)
	if err != nil {
		return fmt.Errorf("failed to get profile: %w", err)
	}

	collection, err := m.loadCollection()
	if err != nil {
		return fmt.Errorf("failed to load collection: %w", err)
	}

	if _, exists := collection.Profiles[newName]; exists {
		return fmt.Errorf("profile '%s' already exists", newName)
	}

	// Update profile name
	profile.Name = newName
	profile.UpdatedAt = time.Now()

	// Add with new name and remove old
	collection.Profiles[newName] = *profile
	delete(collection.Profiles, oldName)

	return m.SaveCollection(collection)
}

// RecordUsage records usage statistics for a profile
func (m *FileProfileManager) RecordUsage(name string, syncDuration int64, success bool) error {
	collection, err := m.loadCollection()
	if err != nil {
		return fmt.Errorf("failed to load collection: %w", err)
	}

	profile, exists := collection.Profiles[name]
	if !exists {
		return fmt.Errorf("profile '%s' not found", name)
	}

	now := time.Now()
	stats := &profile.UsageStats

	// Update usage stats
	stats.TimesUsed++
	stats.LastUsed = now
	stats.TotalSyncTime += syncDuration

	if stats.TimesUsed > 0 {
		stats.AvgSyncTime = stats.TotalSyncTime / int64(stats.TimesUsed)
	}

	if success {
		stats.LastSuccess = now
	} else {
		stats.LastFailure = now
	}

	// Calculate success rate
	// For now, use a simple heuristic based on recent usage
	// In a full implementation, we'd track detailed success/failure counts
	if success {
		stats.SuccessRate = 1.0 // Simplified for this implementation
	}

	collection.Profiles[name] = profile

	return m.SaveCollection(collection)
}

// GetUsageStats retrieves usage statistics for a profile
func (m *FileProfileManager) GetUsageStats(name string) (*UsageStats, error) {
	profile, err := m.GetProfile(name)
	if err != nil {
		return nil, err
	}

	return &profile.UsageStats, nil
}

// GetMostUsedProfiles returns the most frequently used profiles
func (m *FileProfileManager) GetMostUsedProfiles(limit int) ([]Profile, error) {
	collection, err := m.loadCollection()
	if err != nil {
		return nil, fmt.Errorf("failed to load collection: %w", err)
	}

	var profiles []Profile
	for _, profile := range collection.Profiles {
		profiles = append(profiles, profile)
	}

	// Sort by usage count
	sort.Slice(profiles, func(i, j int) bool {
		return profiles[i].UsageStats.TimesUsed > profiles[j].UsageStats.TimesUsed
	})

	if limit > 0 && len(profiles) > limit {
		profiles = profiles[:limit]
	}

	return profiles, nil
}

// SearchProfiles searches profiles based on criteria
func (m *FileProfileManager) SearchProfiles(options *ProfileSearchOptions) ([]Profile, error) {
	collection, err := m.loadCollection()
	if err != nil {
		return nil, fmt.Errorf("failed to load collection: %w", err)
	}

	var matches []Profile

	for _, profile := range collection.Profiles {
		if m.profileMatches(profile, options) {
			matches = append(matches, profile)
		}
	}

	return matches, nil
}

// GetSimilarProfiles finds profiles similar to the given profile
func (m *FileProfileManager) GetSimilarProfiles(profile *Profile, limit int) ([]Profile, error) {
	collection, err := m.loadCollection()
	if err != nil {
		return nil, fmt.Errorf("failed to load collection: %w", err)
	}

	var similar []Profile

	for _, candidate := range collection.Profiles {
		if candidate.Name == profile.Name {
			continue // Skip self
		}

		similarity := m.calculateSimilarity(profile, &candidate)
		if similarity > 0.5 { // 50% similarity threshold
			similar = append(similar, candidate)
		}
	}

	// Sort by similarity (simplified - in practice we'd track similarity scores)
	sort.Slice(similar, func(i, j int) bool {
		return similar[i].UsageStats.TimesUsed > similar[j].UsageStats.TimesUsed
	})

	if limit > 0 && len(similar) > limit {
		similar = similar[:limit]
	}

	return similar, nil
}

// BackupProfiles creates a backup of the profiles
func (m *FileProfileManager) BackupProfiles() error {
	profilesFile := m.getProfilesFilePath()
	backupFile := profilesFile + ".backup"

	data, err := os.ReadFile(profilesFile)
	if err != nil {
		return fmt.Errorf("failed to read profiles file: %w", err)
	}

	if err := os.WriteFile(backupFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write backup file: %w", err)
	}

	return nil
}

// RestoreProfiles restores profiles from backup
func (m *FileProfileManager) RestoreProfiles() error {
	profilesFile := m.getProfilesFilePath()
	backupFile := profilesFile + ".backup"

	data, err := os.ReadFile(backupFile)
	if err != nil {
		return fmt.Errorf("failed to read backup file: %w", err)
	}

	if err := os.WriteFile(profilesFile, data, 0644); err != nil {
		return fmt.Errorf("failed to restore profiles file: %w", err)
	}

	return nil
}

// ValidateCollection validates the entire profile collection
func (m *FileProfileManager) ValidateCollection() (*ProfileValidationResult, error) {
	collection, err := m.loadCollection()
	if err != nil {
		return nil, fmt.Errorf("failed to load collection: %w", err)
	}

	result := &ProfileValidationResult{
		Valid:    true,
		Errors:   make([]string, 0),
		Warnings: make([]string, 0),
	}

	for name, profile := range collection.Profiles {
		profileResult, err := m.ValidateProfile(&profile)
		if err != nil {
			result.Valid = false
			result.Errors = append(result.Errors, fmt.Sprintf("profile '%s': %v", name, err))
			continue
		}

		if !profileResult.Valid {
			result.Valid = false
			for _, err := range profileResult.Errors {
				result.Errors = append(result.Errors, fmt.Sprintf("profile '%s': %s", name, err))
			}
		}

		for _, warning := range profileResult.Warnings {
			result.Warnings = append(result.Warnings, fmt.Sprintf("profile '%s': %s", name, warning))
		}
	}

	return result, nil
}

// RepairCollection attempts to repair issues in the collection
func (m *FileProfileManager) RepairCollection() error {
	// For now, just validate and remove invalid profiles
	validation, err := m.ValidateCollection()
	if err != nil {
		return fmt.Errorf("failed to validate collection: %w", err)
	}

	if validation.Valid {
		return nil // Nothing to repair
	}

	// In a more sophisticated implementation, we would:
	// 1. Try to fix individual profile issues
	// 2. Remove profiles that can't be fixed
	// 3. Update references and dependencies

	return fmt.Errorf("collection repair not fully implemented")
}

// CleanupOrphanedProfiles removes unused or corrupted profiles
func (m *FileProfileManager) CleanupOrphanedProfiles() error {
	// For now, just validate the collection
	_, err := m.ValidateCollection()
	return err
}

// Helper methods

// validateProfileName validates a profile name
func (m *FileProfileManager) validateProfileName(name string) error {
	if name == "" {
		return fmt.Errorf("profile name cannot be empty")
	}

	// Check for valid characters (alphanumeric, hyphens, underscores)
	validName := regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
	if !validName.MatchString(name) {
		return fmt.Errorf("profile name can only contain letters, numbers, hyphens, and underscores")
	}

	return nil
}

// sortProfiles sorts profiles based on the specified criteria
func (m *FileProfileManager) sortProfiles(profiles []Profile, sortBy, sortOrder string) {
	reverse := sortOrder == "desc"

	switch sortBy {
	case "name":
		sort.Slice(profiles, func(i, j int) bool {
			if reverse {
				return profiles[i].Name > profiles[j].Name
			}
			return profiles[i].Name < profiles[j].Name
		})
	case "created":
		sort.Slice(profiles, func(i, j int) bool {
			if reverse {
				return profiles[i].CreatedAt.After(profiles[j].CreatedAt)
			}
			return profiles[i].CreatedAt.Before(profiles[j].CreatedAt)
		})
	case "updated":
		sort.Slice(profiles, func(i, j int) bool {
			if reverse {
				return profiles[i].UpdatedAt.After(profiles[j].UpdatedAt)
			}
			return profiles[i].UpdatedAt.Before(profiles[j].UpdatedAt)
		})
	case "usage":
		sort.Slice(profiles, func(i, j int) bool {
			if reverse {
				return profiles[i].UsageStats.TimesUsed < profiles[j].UsageStats.TimesUsed
			}
			return profiles[i].UsageStats.TimesUsed > profiles[j].UsageStats.TimesUsed
		})
	}
}

// profileMatches checks if a profile matches search criteria
func (m *FileProfileManager) profileMatches(profile Profile, options *ProfileSearchOptions) bool {
	if options == nil {
		return true
	}

	// Query matching
	if options.Query != "" {
		query := strings.ToLower(options.Query)

		// Check name and description
		if strings.Contains(strings.ToLower(profile.Name), query) ||
			strings.Contains(strings.ToLower(profile.Description), query) {
			return true
		}

		// Check JQL if requested
		if options.IncludeJQL && strings.Contains(strings.ToLower(profile.JQL), query) {
			return true
		}

		// Check tags
		for _, tag := range profile.Tags {
			if strings.Contains(strings.ToLower(tag), query) {
				return true
			}
		}

		return false
	}

	// Tag matching
	if len(options.Tags) > 0 {
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
			return false
		}
	}

	// Repository matching
	if options.Repository != "" && profile.Repository != options.Repository {
		return false
	}

	return true
}

// calculateSimilarity calculates similarity between two profiles (simplified)
func (m *FileProfileManager) calculateSimilarity(a, b *Profile) float64 {
	score := 0.0
	total := 0.0

	// Compare repository (weight: 20%)
	total += 0.2
	if a.Repository == b.Repository {
		score += 0.2
	}

	// Compare options (weight: 30%)
	total += 0.3
	optionsMatch := 0.0
	if a.Options.Concurrency == b.Options.Concurrency {
		optionsMatch += 0.25
	}
	if a.Options.RateLimit == b.Options.RateLimit {
		optionsMatch += 0.25
	}
	if a.Options.Incremental == b.Options.Incremental {
		optionsMatch += 0.25
	}
	if a.Options.IncludeLinks == b.Options.IncludeLinks {
		optionsMatch += 0.25
	}
	score += 0.3 * optionsMatch

	// Compare tags (weight: 20%)
	total += 0.2
	if len(a.Tags) > 0 && len(b.Tags) > 0 {
		commonTags := 0
		for _, tagA := range a.Tags {
			for _, tagB := range b.Tags {
				if tagA == tagB {
					commonTags++
					break
				}
			}
		}
		maxTags := len(a.Tags)
		if len(b.Tags) > maxTags {
			maxTags = len(b.Tags)
		}
		score += 0.2 * float64(commonTags) / float64(maxTags)
	}

	// Compare JQL similarity (weight: 30%)
	total += 0.3
	if a.JQL != "" && b.JQL != "" {
		// Simplified JQL comparison - in practice, we'd parse and compare semantically
		if strings.Contains(a.JQL, b.JQL) || strings.Contains(b.JQL, a.JQL) {
			score += 0.3
		} else {
			// Check for common keywords
			aWords := strings.Fields(strings.ToLower(a.JQL))
			bWords := strings.Fields(strings.ToLower(b.JQL))
			commonWords := 0
			for _, wordA := range aWords {
				for _, wordB := range bWords {
					if wordA == wordB {
						commonWords++
						break
					}
				}
			}
			maxWords := len(aWords)
			if len(bWords) > maxWords {
				maxWords = len(bWords)
			}
			if maxWords > 0 {
				score += 0.3 * float64(commonWords) / float64(maxWords)
			}
		}
	}

	if total > 0 {
		return score / total
	}
	return 0.0
}
