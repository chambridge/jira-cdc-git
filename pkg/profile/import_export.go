package profile

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// ExportProfiles exports profiles matching the criteria to a collection
func (m *FileProfileManager) ExportProfiles(options *ProfileExportOptions) (*ProfileCollection, error) {
	collection, err := m.loadCollection()
	if err != nil {
		return nil, NewExportError("failed to load collection", err)
	}

	exportCollection := &ProfileCollection{
		Version:   ProfileVersion,
		Profiles:  make(map[string]Profile),
		Templates: make(map[string]Profile),
		UpdatedAt: time.Now(),
		Metadata: map[string]string{
			"exported_at": time.Now().Format(time.RFC3339),
			"exported_by": "jira-sync",
		},
	}

	// Filter profiles based on options
	if options != nil {
		// Export specific profiles by name
		if len(options.Names) > 0 {
			for _, name := range options.Names {
				if profile, exists := collection.Profiles[name]; exists {
					exportProfile := profile

					// Optionally exclude stats
					if !options.IncludeStats {
						exportProfile.UsageStats = UsageStats{}
					}

					exportCollection.Profiles[name] = exportProfile
				}
			}
		} else {
			// Export all profiles, optionally filtered by tags
			for name, profile := range collection.Profiles {
				shouldInclude := true

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
						shouldInclude = false
					}
				}

				if shouldInclude {
					exportProfile := profile

					// Optionally exclude stats
					if !options.IncludeStats {
						exportProfile.UsageStats = UsageStats{}
					}

					exportCollection.Profiles[name] = exportProfile
				}
			}
		}
	} else {
		// Export all profiles
		for name, profile := range collection.Profiles {
			exportProfile := profile
			exportProfile.UsageStats = UsageStats{} // Default to no stats
			exportCollection.Profiles[name] = exportProfile
		}
	}

	return exportCollection, nil
}

// ImportProfiles imports profiles from a collection
func (m *FileProfileManager) ImportProfiles(importCollection *ProfileCollection, options *ProfileImportOptions) error {
	if importCollection == nil {
		return NewImportError("import collection cannot be nil", nil)
	}

	collection, err := m.loadCollection()
	if err != nil {
		return NewImportError("failed to load existing collection", err)
	}

	// Validate import collection if requested
	if options != nil && options.Validate {
		for name, profile := range importCollection.Profiles {
			validation, err := m.ValidateProfile(&profile)
			if err != nil {
				return NewImportError(fmt.Sprintf("validation failed for profile '%s'", name), err)
			}
			if !validation.Valid {
				return NewImportError(fmt.Sprintf("profile '%s' is invalid: %s", name,
					strings.Join(validation.Errors, "; ")), nil)
			}
		}
	}

	// Import profiles
	conflicts := make([]string, 0)
	imported := 0

	for name, profile := range importCollection.Profiles {
		finalName := name

		// Apply name prefix if specified
		if options != nil && options.NamePrefix != "" {
			finalName = options.NamePrefix + name
		}

		// Check for conflicts
		if _, exists := collection.Profiles[finalName]; exists {
			if options == nil || !options.Overwrite {
				conflicts = append(conflicts, finalName)
				continue
			}
		}

		// Prepare profile for import
		importProfile := profile
		importProfile.Name = finalName

		// Add default tags if specified
		if options != nil && len(options.DefaultTags) > 0 {
			tagMap := make(map[string]bool)
			for _, tag := range importProfile.Tags {
				tagMap[tag] = true
			}
			for _, defaultTag := range options.DefaultTags {
				if !tagMap[defaultTag] {
					importProfile.Tags = append(importProfile.Tags, defaultTag)
				}
			}
		}

		// Merge usage stats if requested
		if options != nil && options.MergeStats {
			if existing, exists := collection.Profiles[finalName]; exists {
				// Merge statistics from existing profile
				importProfile.UsageStats.TimesUsed += existing.UsageStats.TimesUsed
				importProfile.UsageStats.TotalSyncTime += existing.UsageStats.TotalSyncTime

				if importProfile.UsageStats.TimesUsed > 0 {
					importProfile.UsageStats.AvgSyncTime = importProfile.UsageStats.TotalSyncTime / int64(importProfile.UsageStats.TimesUsed)
				}

				// Keep the most recent timestamps
				if existing.UsageStats.LastUsed.After(importProfile.UsageStats.LastUsed) {
					importProfile.UsageStats.LastUsed = existing.UsageStats.LastUsed
				}
				if existing.UsageStats.LastSuccess.After(importProfile.UsageStats.LastSuccess) {
					importProfile.UsageStats.LastSuccess = existing.UsageStats.LastSuccess
				}
			}
		}

		// Set import metadata
		now := time.Now()
		importProfile.UpdatedAt = now
		if importProfile.CreatedAt.IsZero() {
			importProfile.CreatedAt = now
		}
		if importProfile.Version == "" {
			importProfile.Version = ProfileVersion
		}

		collection.Profiles[finalName] = importProfile
		imported++
	}

	// Save updated collection
	if err := m.SaveCollection(collection); err != nil {
		return NewImportError("failed to save imported profiles", err)
	}

	// Report conflicts if any
	if len(conflicts) > 0 {
		return NewImportError(fmt.Sprintf("imported %d profiles, %d conflicts (use --overwrite to replace): %s",
			imported, len(conflicts), strings.Join(conflicts, ", ")), nil)
	}

	return nil
}

// ExportToFile exports profiles to a file
func (m *FileProfileManager) ExportToFile(filePath string, options *ProfileExportOptions) error {
	collection, err := m.ExportProfiles(options)
	if err != nil {
		return NewExportError("failed to export profiles", err)
	}

	// Determine format from file extension or options
	format := "yaml"
	if options != nil && options.Format != "" {
		format = options.Format
	} else {
		ext := strings.ToLower(filepath.Ext(filePath))
		if ext == ".json" {
			format = "json"
		}
	}

	// Marshal to bytes
	var data []byte
	switch format {
	case "json":
		data, err = json.MarshalIndent(collection, "", "  ")
	case "yaml":
		data, err = yaml.Marshal(collection)
	default:
		return NewExportError(fmt.Sprintf("unsupported export format: %s", format), nil)
	}

	if err != nil {
		return NewExportError("failed to marshal export data", err)
	}

	// Ensure directory exists
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return NewExportError("failed to create export directory", err)
	}

	// Write to file
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return NewExportError("failed to write export file", err)
	}

	return nil
}

// ImportFromFile imports profiles from a file
func (m *FileProfileManager) ImportFromFile(filePath string, options *ProfileImportOptions) error {
	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return NewImportError(fmt.Sprintf("import file does not exist: %s", filePath), nil)
	}

	// Read file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return NewImportError("failed to read import file", err)
	}

	// Determine format from file extension
	var collection ProfileCollection
	ext := strings.ToLower(filepath.Ext(filePath))

	switch ext {
	case ".json":
		if err := json.Unmarshal(data, &collection); err != nil {
			return NewImportError("failed to parse JSON import file", err)
		}
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, &collection); err != nil {
			return NewImportError("failed to parse YAML import file", err)
		}
	default:
		// Try YAML first, then JSON
		if err := yaml.Unmarshal(data, &collection); err != nil {
			if jsonErr := json.Unmarshal(data, &collection); jsonErr != nil {
				return NewImportError("failed to parse import file (tried both YAML and JSON)", err)
			}
		}
	}

	// Validate collection structure
	if collection.Profiles == nil {
		return NewImportError("import file does not contain profiles", nil)
	}

	return m.ImportProfiles(&collection, options)
}

// ExportProfilesForSharing creates a shareable export with minimal metadata
func (m *FileProfileManager) ExportProfilesForSharing(profileNames []string) (*ProfileCollection, error) {
	options := &ProfileExportOptions{
		Names:        profileNames,
		IncludeStats: false,
		Format:       "yaml",
	}

	collection, err := m.ExportProfiles(options)
	if err != nil {
		return nil, err
	}

	// Clean up profiles for sharing
	for name, profile := range collection.Profiles {
		// Remove personal/sensitive information
		profile.CreatedBy = ""
		profile.UsageStats = UsageStats{}

		// Clear metadata that's not relevant for sharing
		if profile.Metadata == nil {
			profile.Metadata = make(map[string]string)
		}
		delete(profile.Metadata, "local_path")
		delete(profile.Metadata, "user_id")

		collection.Profiles[name] = profile
	}

	// Add sharing metadata
	collection.Metadata["purpose"] = "sharing"
	collection.Metadata["cleaned"] = "true"

	return collection, nil
}

// ImportSharedProfiles imports shared profiles with appropriate safety checks
func (m *FileProfileManager) ImportSharedProfiles(collection *ProfileCollection) error {
	options := &ProfileImportOptions{
		Overwrite:   false,
		MergeStats:  false,
		NamePrefix:  "shared-",
		DefaultTags: []string{"imported", "shared"},
		Validate:    true,
	}

	return m.ImportProfiles(collection, options)
}

// CreateQuickExport creates a quick export of the most commonly used profiles
func (m *FileProfileManager) CreateQuickExport(limit int) (*ProfileCollection, error) {
	// Get most used profiles
	profiles, err := m.GetMostUsedProfiles(limit)
	if err != nil {
		return nil, NewExportError("failed to get most used profiles", err)
	}

	names := make([]string, len(profiles))
	for i, profile := range profiles {
		names[i] = profile.Name
	}

	options := &ProfileExportOptions{
		Names:        names,
		IncludeStats: true,
		Format:       "yaml",
	}

	return m.ExportProfiles(options)
}

// ValidateImportFile validates an import file without actually importing
func ValidateImportFile(filePath string) (*ProfileValidationResult, error) {
	// Read and parse file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, NewImportError("failed to read import file", err)
	}

	var collection ProfileCollection
	ext := strings.ToLower(filepath.Ext(filePath))

	switch ext {
	case ".json":
		if err := json.Unmarshal(data, &collection); err != nil {
			return nil, NewImportError("failed to parse JSON import file", err)
		}
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, &collection); err != nil {
			return nil, NewImportError("failed to parse YAML import file", err)
		}
	default:
		// Try both formats
		if err := yaml.Unmarshal(data, &collection); err != nil {
			if jsonErr := json.Unmarshal(data, &collection); jsonErr != nil {
				return nil, NewImportError("failed to parse import file (tried both YAML and JSON)", err)
			}
		}
	}

	// Validate collection
	result := &ProfileValidationResult{
		Valid:    true,
		Errors:   make([]string, 0),
		Warnings: make([]string, 0),
	}

	if collection.Profiles == nil {
		result.Valid = false
		result.Errors = append(result.Errors, "import file does not contain profiles")
		return result, nil
	}

	// Create temporary manager for validation
	tempManager := NewFileProfileManager("", "yaml")

	for name, profile := range collection.Profiles {
		validation, err := tempManager.ValidateProfile(&profile)
		if err != nil {
			result.Valid = false
			result.Errors = append(result.Errors, fmt.Sprintf("profile '%s': validation error: %v", name, err))
			continue
		}

		if !validation.Valid {
			result.Valid = false
			for _, err := range validation.Errors {
				result.Errors = append(result.Errors, fmt.Sprintf("profile '%s': %s", name, err))
			}
		}

		for _, warning := range validation.Warnings {
			result.Warnings = append(result.Warnings, fmt.Sprintf("profile '%s': %s", name, warning))
		}
	}

	return result, nil
}
