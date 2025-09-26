package profile

// ProfileManager defines the interface for managing sync profiles
type ProfileManager interface {
	// Profile Management
	CreateProfile(profile *Profile) error
	GetProfile(name string) (*Profile, error)
	UpdateProfile(name string, profile *Profile) error
	DeleteProfile(name string) error
	ListProfiles(options *ProfileListOptions) ([]Profile, error)
	ProfileExists(name string) bool

	// Profile Operations
	ValidateProfile(profile *Profile) (*ProfileValidationResult, error)
	DuplicateProfile(sourceName, targetName string) error
	RenameProfile(oldName, newName string) error

	// Template Management
	GetTemplates() ([]ProfileTemplate, error)
	GetTemplate(id string) (*ProfileTemplate, error)
	CreateFromTemplate(templateID string, name string, variables map[string]string) (*Profile, error)

	// Import/Export
	ExportProfiles(options *ProfileExportOptions) (*ProfileCollection, error)
	ImportProfiles(collection *ProfileCollection, options *ProfileImportOptions) error
	ExportToFile(filePath string, options *ProfileExportOptions) error
	ImportFromFile(filePath string, options *ProfileImportOptions) error

	// Search and Discovery
	SearchProfiles(options *ProfileSearchOptions) ([]Profile, error)
	GetSimilarProfiles(profile *Profile, limit int) ([]Profile, error)

	// Usage Tracking
	RecordUsage(name string, syncDuration int64, success bool) error
	GetUsageStats(name string) (*UsageStats, error)
	GetMostUsedProfiles(limit int) ([]Profile, error)

	// Collection Management
	GetCollection() (*ProfileCollection, error)
	SaveCollection(collection *ProfileCollection) error
	BackupProfiles() error
	RestoreProfiles() error

	// Validation and Health
	ValidateCollection() (*ProfileValidationResult, error)
	RepairCollection() error
	CleanupOrphanedProfiles() error
}
