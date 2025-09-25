package state

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/chambrid/jira-cdc-git/pkg/client"
	"gopkg.in/yaml.v3"
)

const (
	StateFileVersion  = "v0.3.0"
	StateFileName     = ".jira-sync-state.yaml"
	StateFileBackup   = ".jira-sync-state.backup.yaml"
	MaxHistoryEntries = 50
)

// StateManager defines the interface for managing sync state
type StateManager interface {
	// State Management
	LoadState(repoPath string) (*SyncState, error)
	SaveState(repoPath string, state *SyncState) error
	InitializeState(repoPath string, repoInfo RepositoryInfo) (*SyncState, error)
	BackupState(repoPath string) error
	RestoreState(repoPath string) error

	// Sync Operations
	StartSyncOperation(state *SyncState, syncType SyncType, config SyncConfig) *SyncOperation
	CompleteSyncOperation(state *SyncState, operation *SyncOperation, results OperationResults) error
	FailSyncOperation(state *SyncState, operation *SyncOperation, err error) error

	// Issue State Management
	UpdateIssueState(state *SyncState, issue *client.Issue, filePath string) error
	GetIssueState(state *SyncState, issueKey string) (*IssueState, bool)
	RemoveIssueState(state *SyncState, issueKey string) error

	// Incremental Sync Support
	GetChangedIssues(state *SyncState, options IncrementalSyncOptions) ([]string, error)
	ShouldSyncIssue(state *SyncState, issue *client.Issue) bool
	GetLastSyncTime(state *SyncState) time.Time

	// Validation and Recovery
	ValidateState(state *SyncState, repoPath string) (*StateValidationResult, error)
	RecoverState(state *SyncState, repoPath string, options StateRecoveryOptions) (*StateValidationResult, error)

	// Statistics and Reporting
	GetSyncStatistics(state *SyncState) SyncStatistics
	UpdateStatistics(state *SyncState, operation *SyncOperation) error
	GetHistoryReport(state *SyncState, limit int) []SyncOperation
}

// FileStateManager implements StateManager using file-based storage
type FileStateManager struct {
	format StateFileFormat
}

// StateFileFormat represents the file format for state storage
type StateFileFormat string

const (
	FormatYAML StateFileFormat = "yaml"
	FormatJSON StateFileFormat = "json"
)

// NewFileStateManager creates a new file-based state manager
func NewFileStateManager(format StateFileFormat) *FileStateManager {
	if format != FormatYAML && format != FormatJSON {
		format = FormatYAML // Default to YAML
	}
	return &FileStateManager{
		format: format,
	}
}

// getStateFilePath returns the path to the state file
func (m *FileStateManager) getStateFilePath(repoPath string) string {
	if m.format == FormatJSON {
		return filepath.Join(repoPath, ".jira-sync-state.json")
	}
	return filepath.Join(repoPath, StateFileName)
}

// getBackupFilePath returns the path to the backup state file
func (m *FileStateManager) getBackupFilePath(repoPath string) string {
	if m.format == FormatJSON {
		return filepath.Join(repoPath, ".jira-sync-state.backup.json")
	}
	return filepath.Join(repoPath, StateFileBackup)
}

// LoadState loads the sync state from the repository
func (m *FileStateManager) LoadState(repoPath string) (*SyncState, error) {
	stateFilePath := m.getStateFilePath(repoPath)

	// Check if state file exists
	if _, err := os.Stat(stateFilePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("state file does not exist at %s", stateFilePath)
	}

	// Read state file
	data, err := os.ReadFile(stateFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read state file: %w", err)
	}

	// Parse state file
	var state SyncState
	if m.format == FormatJSON {
		if err := json.Unmarshal(data, &state); err != nil {
			return nil, fmt.Errorf("failed to parse JSON state file: %w", err)
		}
	} else {
		if err := yaml.Unmarshal(data, &state); err != nil {
			return nil, fmt.Errorf("failed to parse YAML state file: %w", err)
		}
	}

	// Validate state version
	if state.Version == "" {
		state.Version = StateFileVersion
	}

	// Initialize maps if nil
	if state.Issues == nil {
		state.Issues = make(map[string]IssueState)
	}

	return &state, nil
}

// SaveState saves the sync state to the repository
func (m *FileStateManager) SaveState(repoPath string, state *SyncState) error {
	if state == nil {
		return fmt.Errorf("state cannot be nil")
	}

	// Update metadata
	state.Version = StateFileVersion
	state.UpdatedAt = time.Now()

	// Limit history size
	if len(state.History) > MaxHistoryEntries {
		// Keep the most recent entries
		state.History = state.History[len(state.History)-MaxHistoryEntries:]
	}

	// Marshal state to bytes
	var data []byte
	var err error
	if m.format == FormatJSON {
		data, err = json.MarshalIndent(state, "", "  ")
	} else {
		data, err = yaml.Marshal(state)
	}
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	// Write to temp file first for atomic operation
	stateFilePath := m.getStateFilePath(repoPath)
	tempFilePath := stateFilePath + ".tmp"

	if err := os.WriteFile(tempFilePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write temp state file: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tempFilePath, stateFilePath); err != nil {
		// Clean up temp file on failure
		_ = os.Remove(tempFilePath)
		return fmt.Errorf("failed to rename temp state file: %w", err)
	}

	return nil
}

// InitializeState creates a new state for a repository
func (m *FileStateManager) InitializeState(repoPath string, repoInfo RepositoryInfo) (*SyncState, error) {
	now := time.Now()

	state := &SyncState{
		Version:    StateFileVersion,
		Repository: repoInfo,
		LastSync:   nil,
		History:    make([]SyncOperation, 0),
		Issues:     make(map[string]IssueState),
		Stats: SyncStatistics{
			ActiveProjects: make([]string, 0),
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	// Save initial state
	if err := m.SaveState(repoPath, state); err != nil {
		return nil, fmt.Errorf("failed to save initial state: %w", err)
	}

	return state, nil
}

// BackupState creates a backup of the current state
func (m *FileStateManager) BackupState(repoPath string) error {
	stateFilePath := m.getStateFilePath(repoPath)
	backupFilePath := m.getBackupFilePath(repoPath)

	// Check if state file exists
	if _, err := os.Stat(stateFilePath); os.IsNotExist(err) {
		return fmt.Errorf("state file does not exist, cannot backup")
	}

	// Copy state file to backup location
	sourceFile, err := os.Open(stateFilePath)
	if err != nil {
		return fmt.Errorf("failed to open state file for backup: %w", err)
	}
	defer func() { _ = sourceFile.Close() }()

	backupFile, err := os.Create(backupFilePath)
	if err != nil {
		return fmt.Errorf("failed to create backup file: %w", err)
	}
	defer func() { _ = backupFile.Close() }()

	if _, err := io.Copy(backupFile, sourceFile); err != nil {
		return fmt.Errorf("failed to copy state to backup: %w", err)
	}

	return nil
}

// RestoreState restores state from backup
func (m *FileStateManager) RestoreState(repoPath string) error {
	stateFilePath := m.getStateFilePath(repoPath)
	backupFilePath := m.getBackupFilePath(repoPath)

	// Check if backup exists
	if _, err := os.Stat(backupFilePath); os.IsNotExist(err) {
		return fmt.Errorf("backup file does not exist")
	}

	// Copy backup to state file
	backupFile, err := os.Open(backupFilePath)
	if err != nil {
		return fmt.Errorf("failed to open backup file: %w", err)
	}
	defer func() { _ = backupFile.Close() }()

	stateFile, err := os.Create(stateFilePath)
	if err != nil {
		return fmt.Errorf("failed to create state file from backup: %w", err)
	}
	defer func() { _ = stateFile.Close() }()

	if _, err := io.Copy(stateFile, backupFile); err != nil {
		return fmt.Errorf("failed to restore state from backup: %w", err)
	}

	return nil
}

// StartSyncOperation creates and starts a new sync operation
func (m *FileStateManager) StartSyncOperation(state *SyncState, syncType SyncType, config SyncConfig) *SyncOperation {
	now := time.Now()
	operationID := fmt.Sprintf("sync-%d", now.Unix())

	operation := &SyncOperation{
		ID:        operationID,
		Type:      syncType,
		StartTime: now,
		Status:    SyncStatusRunning,
		Config:    config,
		Results:   OperationResults{},
		Metadata:  make(map[string]string),
	}

	// Set as current operation
	state.LastSync = operation

	return operation
}

// CompleteSyncOperation marks a sync operation as completed
func (m *FileStateManager) CompleteSyncOperation(state *SyncState, operation *SyncOperation, results OperationResults) error {
	now := time.Now()
	operation.EndTime = now
	operation.Duration = now.Sub(operation.StartTime)
	operation.Status = SyncStatusCompleted
	operation.Results = results

	// Add to history
	state.History = append(state.History, *operation)

	// Update statistics
	if err := m.UpdateStatistics(state, operation); err != nil {
		return fmt.Errorf("failed to update statistics: %w", err)
	}

	return nil
}

// FailSyncOperation marks a sync operation as failed
func (m *FileStateManager) FailSyncOperation(state *SyncState, operation *SyncOperation, err error) error {
	now := time.Now()
	operation.EndTime = now
	operation.Duration = now.Sub(operation.StartTime)
	operation.Status = SyncStatusFailed
	operation.Error = err.Error()

	// Add to history
	state.History = append(state.History, *operation)

	// Update statistics
	if updateErr := m.UpdateStatistics(state, operation); updateErr != nil {
		return fmt.Errorf("failed to update statistics: %w", updateErr)
	}

	return nil
}

// UpdateIssueState updates the state for a specific issue
func (m *FileStateManager) UpdateIssueState(state *SyncState, issue *client.Issue, filePath string) error {
	now := time.Now()

	// Calculate file checksum
	checksum, err := m.calculateFileChecksum(filePath)
	if err != nil {
		return fmt.Errorf("failed to calculate file checksum: %w", err)
	}

	// Get file size
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}

	// Update or create issue state
	existingState, exists := state.Issues[issue.Key]
	syncCount := 1
	if exists {
		syncCount = existingState.SyncCount + 1
	}

	issueState := IssueState{
		Key:          issue.Key,
		ProjectKey:   extractProjectKey(issue.Key), // Extract from issue key since we don't have Fields
		LastSynced:   now,
		LastModified: now,
		LastUpdated:  parseJIRATime(issue.Updated),
		Version:      1, // TODO: Get from JIRA if available
		FilePath:     filePath,
		FileSize:     fileInfo.Size(),
		Checksum:     checksum,
		SyncStatus:   "success",
		SyncCount:    syncCount,
	}

	state.Issues[issue.Key] = issueState

	// Update active projects list
	m.updateActiveProjects(state, extractProjectKey(issue.Key))

	return nil
}

// GetIssueState retrieves the state for a specific issue
func (m *FileStateManager) GetIssueState(state *SyncState, issueKey string) (*IssueState, bool) {
	issueState, exists := state.Issues[issueKey]
	return &issueState, exists
}

// RemoveIssueState removes the state for a specific issue
func (m *FileStateManager) RemoveIssueState(state *SyncState, issueKey string) error {
	delete(state.Issues, issueKey)
	return nil
}

// GetChangedIssues returns issues that need to be synced based on incremental options
func (m *FileStateManager) GetChangedIssues(state *SyncState, options IncrementalSyncOptions) ([]string, error) {
	var changedIssues []string

	// If no since time specified, use last sync time
	sinceTime := options.Since
	if sinceTime.IsZero() {
		sinceTime = m.GetLastSyncTime(state)
	}

	// If force is true, include all known issues
	if options.Force {
		for issueKey := range state.Issues {
			changedIssues = append(changedIssues, issueKey)
		}
		return changedIssues, nil
	}

	// Check each issue for changes
	for issueKey, issueState := range state.Issues {
		shouldInclude := false

		// Include if modified after since time
		if options.IncludeModified && issueState.LastUpdated.After(sinceTime) {
			shouldInclude = true
		}

		// Include if never synced before
		if options.IncludeNew && issueState.LastSynced.IsZero() {
			shouldInclude = true
		}

		// Check project filter
		if len(options.Projects) > 0 {
			projectMatch := false
			for _, project := range options.Projects {
				if issueState.ProjectKey == project {
					projectMatch = true
					break
				}
			}
			if !projectMatch {
				shouldInclude = false
			}
		}

		// Check max age
		if options.MaxAge > 0 && time.Since(issueState.LastUpdated) > options.MaxAge {
			shouldInclude = false
		}

		if shouldInclude {
			changedIssues = append(changedIssues, issueKey)
		}
	}

	return changedIssues, nil
}

// ShouldSyncIssue determines if an issue should be synced
func (m *FileStateManager) ShouldSyncIssue(state *SyncState, issue *client.Issue) bool {
	issueState, exists := state.Issues[issue.Key]
	if !exists {
		return true // New issue, should sync
	}

	// Check if issue was updated since last sync
	issueUpdated := parseJIRATime(issue.Updated)
	return issueUpdated.After(issueState.LastSynced)
}

// GetLastSyncTime returns the timestamp of the last successful sync
func (m *FileStateManager) GetLastSyncTime(state *SyncState) time.Time {
	if state.LastSync != nil && state.LastSync.Status == SyncStatusCompleted {
		return state.LastSync.EndTime
	}

	// Look through history for last successful sync
	for i := len(state.History) - 1; i >= 0; i-- {
		if state.History[i].Status == SyncStatusCompleted {
			return state.History[i].EndTime
		}
	}

	return time.Time{} // No successful sync found
}

// ValidateState validates the current state against the repository
func (m *FileStateManager) ValidateState(state *SyncState, repoPath string) (*StateValidationResult, error) {
	result := &StateValidationResult{
		Valid:              true,
		Errors:             make([]string, 0),
		Warnings:           make([]string, 0),
		MissingIssues:      make([]string, 0),
		OrphanedFiles:      make([]string, 0),
		CorruptedFiles:     make([]string, 0),
		RecommendedActions: make([]string, 0),
	}

	// Check if all tracked issues still have their files
	for issueKey, issueState := range state.Issues {
		if _, err := os.Stat(issueState.FilePath); os.IsNotExist(err) {
			result.MissingIssues = append(result.MissingIssues, issueKey)
			result.Valid = false
			result.Errors = append(result.Errors, fmt.Sprintf("File missing for issue %s: %s", issueKey, issueState.FilePath))
		} else if err != nil {
			result.CorruptedFiles = append(result.CorruptedFiles, issueState.FilePath)
			result.Valid = false
			result.Errors = append(result.Errors, fmt.Sprintf("Cannot access file for issue %s: %v", issueKey, err))
		} else {
			// Validate checksum if file exists
			checksum, err := m.calculateFileChecksum(issueState.FilePath)
			if err != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("Cannot calculate checksum for %s: %v", issueKey, err))
			} else if checksum != issueState.Checksum {
				result.Warnings = append(result.Warnings, fmt.Sprintf("File checksum mismatch for %s (file may have been modified outside of sync)", issueKey))
			}
		}
	}

	// Check for orphaned issue files (files that exist but aren't tracked)
	issuesDir := filepath.Join(repoPath, "projects")
	if _, err := os.Stat(issuesDir); err == nil {
		err := filepath.Walk(issuesDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() || filepath.Ext(path) != ".yaml" {
				return nil
			}

			// Check if this file is tracked in state
			tracked := false
			for _, issueState := range state.Issues {
				if issueState.FilePath == path {
					tracked = true
					break
				}
			}

			if !tracked {
				result.OrphanedFiles = append(result.OrphanedFiles, path)
				result.Warnings = append(result.Warnings, fmt.Sprintf("Orphaned file found: %s", path))
			}

			return nil
		})
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("Failed to scan issues directory: %v", err))
			result.Valid = false
		}
	}

	// Generate recommendations
	if len(result.MissingIssues) > 0 {
		result.RecommendedActions = append(result.RecommendedActions, "Run full resync to restore missing issue files")
	}
	if len(result.OrphanedFiles) > 0 {
		result.RecommendedActions = append(result.RecommendedActions, "Consider removing orphaned files or updating state to track them")
	}
	if len(result.CorruptedFiles) > 0 {
		result.RecommendedActions = append(result.RecommendedActions, "Repair or resync corrupted files")
	}

	return result, nil
}

// RecoverState attempts to recover from state issues
func (m *FileStateManager) RecoverState(state *SyncState, repoPath string, options StateRecoveryOptions) (*StateValidationResult, error) {
	// First validate to identify issues
	result, err := m.ValidateState(state, repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to validate state for recovery: %w", err)
	}

	// Backup state if requested
	if options.BackupFirst {
		if err := m.BackupState(repoPath); err != nil {
			return result, fmt.Errorf("failed to backup state before recovery: %w", err)
		}
	}

	// Process recovery actions
	for _, action := range options.Actions {
		switch action {
		case ActionRemoveOrphans:
			if !options.DryRun {
				for _, orphanPath := range result.OrphanedFiles {
					if err := os.Remove(orphanPath); err != nil {
						result.Warnings = append(result.Warnings, fmt.Sprintf("Failed to remove orphan file %s: %v", orphanPath, err))
					}
				}
			}
		case ActionRepairState:
			if !options.DryRun {
				// Remove missing issues from state
				for _, missingIssue := range result.MissingIssues {
					delete(state.Issues, missingIssue)
				}
			}
		case ActionValidateOnly:
			// Already validated above
		}
	}

	return result, nil
}

// GetSyncStatistics returns current sync statistics
func (m *FileStateManager) GetSyncStatistics(state *SyncState) SyncStatistics {
	return state.Stats
}

// UpdateStatistics updates statistics after a sync operation
func (m *FileStateManager) UpdateStatistics(state *SyncState, operation *SyncOperation) error {
	stats := &state.Stats

	stats.TotalOperations++
	switch operation.Status {
	case SyncStatusCompleted:
		stats.SuccessfulOps++
		stats.LastSuccessfulOp = operation.EndTime
		stats.TotalIssuesSynced += operation.Results.SuccessfulSync
	case SyncStatusFailed:
		stats.FailedOps++
		stats.LastFailedOp = operation.EndTime
	}

	stats.TotalSyncTime += operation.Duration
	if stats.TotalOperations > 0 {
		stats.AvgSyncTime = stats.TotalSyncTime / time.Duration(stats.TotalOperations)
	}

	// Update unique issues count
	stats.UniqueIssues = len(state.Issues)

	// Update active projects
	projectMap := make(map[string]bool)
	for _, issueState := range state.Issues {
		projectMap[issueState.ProjectKey] = true
	}
	stats.ActiveProjects = make([]string, 0, len(projectMap))
	for project := range projectMap {
		stats.ActiveProjects = append(stats.ActiveProjects, project)
	}
	sort.Strings(stats.ActiveProjects)

	return nil
}

// GetHistoryReport returns the sync operation history
func (m *FileStateManager) GetHistoryReport(state *SyncState, limit int) []SyncOperation {
	if limit <= 0 || limit > len(state.History) {
		limit = len(state.History)
	}

	// Return most recent entries
	start := len(state.History) - limit
	return state.History[start:]
}

// Helper functions

// calculateFileChecksum calculates SHA256 checksum of a file
func (m *FileStateManager) calculateFileChecksum(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer func() { _ = file.Close() }()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// updateActiveProjects updates the active projects list
func (m *FileStateManager) updateActiveProjects(state *SyncState, projectKey string) {
	// Check if project already exists
	for _, existing := range state.Stats.ActiveProjects {
		if existing == projectKey {
			return
		}
	}

	// Add new project and sort
	state.Stats.ActiveProjects = append(state.Stats.ActiveProjects, projectKey)
	sort.Strings(state.Stats.ActiveProjects)
}

// parseJIRATime parses JIRA timestamp string
func parseJIRATime(timeStr string) time.Time {
	// JIRA typically uses ISO 8601 format
	if timeStr == "" {
		return time.Time{}
	}

	// Try common JIRA time formats
	formats := []string{
		"2006-01-02T15:04:05.999-0700",
		"2006-01-02T15:04:05.999Z",
		"2006-01-02T15:04:05-0700",
		"2006-01-02T15:04:05Z",
		time.RFC3339,
		time.RFC3339Nano,
	}

	for _, format := range formats {
		if t, err := time.Parse(format, timeStr); err == nil {
			return t
		}
	}

	return time.Time{}
}

// extractProjectKey extracts the project key from an issue key (e.g., "PROJ-123" -> "PROJ")
func extractProjectKey(issueKey string) string {
	// JIRA issue keys are in format: PROJECT-NUMBER
	for i, char := range issueKey {
		if char == '-' {
			return issueKey[:i]
		}
	}
	return issueKey // Fallback if no dash found
}
