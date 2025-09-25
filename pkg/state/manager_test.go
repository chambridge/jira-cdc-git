package state

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/chambrid/jira-cdc-git/pkg/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFileStateManager_InitializeState(t *testing.T) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "state-test-*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	manager := NewFileStateManager(FormatYAML)

	repoInfo := RepositoryInfo{
		Path:        tempDir,
		Branch:      "main",
		InitialSync: true,
	}

	state, err := manager.InitializeState(tempDir, repoInfo)
	require.NoError(t, err)
	assert.NotNil(t, state)

	// Verify state structure
	assert.Equal(t, StateFileVersion, state.Version)
	assert.Equal(t, tempDir, state.Repository.Path)
	assert.Equal(t, "main", state.Repository.Branch)
	assert.True(t, state.Repository.InitialSync)
	assert.NotNil(t, state.Issues)
	assert.NotNil(t, state.History)
	assert.NotZero(t, state.CreatedAt)
	assert.NotZero(t, state.UpdatedAt)

	// Verify state file was created
	stateFilePath := filepath.Join(tempDir, StateFileName)
	assert.FileExists(t, stateFilePath)
}

func TestFileStateManager_LoadAndSaveState(t *testing.T) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "state-test-*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	manager := NewFileStateManager(FormatYAML)

	// Create a state
	originalState := &SyncState{
		Version: StateFileVersion,
		Repository: RepositoryInfo{
			Path:   tempDir,
			Branch: "main",
		},
		Issues:    make(map[string]IssueState),
		History:   make([]SyncOperation, 0),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Add some test data
	originalState.Issues["TEST-123"] = IssueState{
		Key:         "TEST-123",
		ProjectKey:  "TEST",
		LastSynced:  time.Now(),
		LastUpdated: time.Now(),
		FilePath:    "/test/path/TEST-123.yaml",
		SyncStatus:  "success",
		SyncCount:   1,
	}

	// Save state
	err = manager.SaveState(tempDir, originalState)
	require.NoError(t, err)

	// Load state
	loadedState, err := manager.LoadState(tempDir)
	require.NoError(t, err)
	assert.NotNil(t, loadedState)

	// Verify loaded state matches original
	assert.Equal(t, originalState.Version, loadedState.Version)
	assert.Equal(t, originalState.Repository.Path, loadedState.Repository.Path)
	assert.Equal(t, originalState.Repository.Branch, loadedState.Repository.Branch)
	assert.Len(t, loadedState.Issues, 1)

	issue, exists := loadedState.Issues["TEST-123"]
	assert.True(t, exists)
	assert.Equal(t, "TEST-123", issue.Key)
	assert.Equal(t, "TEST", issue.ProjectKey)
	assert.Equal(t, "success", issue.SyncStatus)
	assert.Equal(t, 1, issue.SyncCount)
}

func TestFileStateManager_BackupAndRestore(t *testing.T) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "state-test-*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	manager := NewFileStateManager(FormatYAML)

	// Initialize state
	repoInfo := RepositoryInfo{Path: tempDir, Branch: "main"}
	state, err := manager.InitializeState(tempDir, repoInfo)
	require.NoError(t, err)

	// Add test data
	state.Issues["TEST-456"] = IssueState{
		Key:        "TEST-456",
		ProjectKey: "TEST",
		SyncStatus: "success",
	}

	// Save updated state
	err = manager.SaveState(tempDir, state)
	require.NoError(t, err)

	// Create backup
	err = manager.BackupState(tempDir)
	require.NoError(t, err)

	// Verify backup file exists
	backupPath := filepath.Join(tempDir, StateFileBackup)
	assert.FileExists(t, backupPath)

	// Modify original state
	state.Issues["TEST-789"] = IssueState{Key: "TEST-789"}
	err = manager.SaveState(tempDir, state)
	require.NoError(t, err)

	// Restore from backup
	err = manager.RestoreState(tempDir)
	require.NoError(t, err)

	// Load restored state
	restoredState, err := manager.LoadState(tempDir)
	require.NoError(t, err)

	// Verify restored state doesn't have the modification
	_, exists := restoredState.Issues["TEST-789"]
	assert.False(t, exists)

	// But has the original data
	_, exists = restoredState.Issues["TEST-456"]
	assert.True(t, exists)
}

func TestFileStateManager_SyncOperations(t *testing.T) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "state-test-*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	manager := NewFileStateManager(FormatYAML)

	// Initialize state
	repoInfo := RepositoryInfo{Path: tempDir, Branch: "main"}
	state, err := manager.InitializeState(tempDir, repoInfo)
	require.NoError(t, err)

	// Start sync operation
	config := SyncConfig{
		Concurrency: 5,
		Incremental: true,
		Force:       false,
	}

	operation := manager.StartSyncOperation(state, SyncTypeIssues, config)
	assert.NotNil(t, operation)
	assert.NotEmpty(t, operation.ID)
	assert.Equal(t, SyncTypeIssues, operation.Type)
	assert.Equal(t, SyncStatusRunning, operation.Status)
	assert.Equal(t, config, operation.Config)
	assert.NotZero(t, operation.StartTime)

	// Complete sync operation
	results := OperationResults{
		TotalIssues:     5,
		ProcessedIssues: 5,
		SuccessfulSync:  4,
		FailedSync:      1,
		ProcessedFiles:  []string{"file1.yaml", "file2.yaml"},
	}

	err = manager.CompleteSyncOperation(state, operation, results)
	require.NoError(t, err)

	assert.Equal(t, SyncStatusCompleted, operation.Status)
	assert.Equal(t, results, operation.Results)
	assert.NotZero(t, operation.EndTime)
	assert.Positive(t, operation.Duration)

	// Verify operation was added to history
	assert.Len(t, state.History, 1)
	assert.Equal(t, operation.ID, state.History[0].ID)
}

func TestFileStateManager_IssueStateManagement(t *testing.T) {
	manager := NewFileStateManager(FormatYAML)

	// Create test state
	state := &SyncState{
		Issues: make(map[string]IssueState),
		Stats: SyncStatistics{
			ActiveProjects: make([]string, 0),
		},
	}

	// Create test issue
	issue := &client.Issue{
		Key:     "TEST-999",
		Updated: "2023-01-01T12:00:00.000Z",
	}

	// Create test file for checksum calculation
	tempDir, err := os.MkdirTemp("", "state-test-*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	testFilePath := filepath.Join(tempDir, "test.yaml")
	err = os.WriteFile(testFilePath, []byte("test content"), 0644)
	require.NoError(t, err)

	// Update issue state
	err = manager.UpdateIssueState(state, issue, testFilePath)
	require.NoError(t, err)

	// Verify issue state was created
	issueState, exists := manager.GetIssueState(state, "TEST-999")
	assert.True(t, exists)
	assert.NotNil(t, issueState)
	assert.Equal(t, "TEST-999", issueState.Key)
	assert.Equal(t, "TEST", issueState.ProjectKey)
	assert.Equal(t, testFilePath, issueState.FilePath)
	assert.Equal(t, "success", issueState.SyncStatus)
	assert.Equal(t, 1, issueState.SyncCount)
	assert.NotEmpty(t, issueState.Checksum)

	// Update again (should increment sync count)
	err = manager.UpdateIssueState(state, issue, testFilePath)
	require.NoError(t, err)

	issueState, exists = manager.GetIssueState(state, "TEST-999")
	assert.True(t, exists)
	assert.Equal(t, 2, issueState.SyncCount)

	// Remove issue state
	err = manager.RemoveIssueState(state, "TEST-999")
	require.NoError(t, err)

	_, exists = manager.GetIssueState(state, "TEST-999")
	assert.False(t, exists)
}

func TestFileStateManager_IncrementalSync(t *testing.T) {
	manager := NewFileStateManager(FormatYAML)

	// Create test state with some issues
	now := time.Now()
	oldTime := now.Add(-24 * time.Hour)

	state := &SyncState{
		Issues: map[string]IssueState{
			"OLD-1": {
				Key:         "OLD-1",
				ProjectKey:  "OLD",
				LastSynced:  oldTime,
				LastUpdated: oldTime,
			},
			"NEW-1": {
				Key:         "NEW-1",
				ProjectKey:  "NEW",
				LastSynced:  now,
				LastUpdated: now,
			},
		},
		History: []SyncOperation{
			{
				Status:  SyncStatusCompleted,
				EndTime: oldTime,
			},
		},
	}

	// Test getting changed issues
	options := IncrementalSyncOptions{
		Since:           oldTime.Add(1 * time.Hour),
		IncludeNew:      true,
		IncludeModified: true,
	}

	changedIssues, err := manager.GetChangedIssues(state, options)
	require.NoError(t, err)

	// Should include NEW-1 (modified after since time) but not OLD-1
	assert.Contains(t, changedIssues, "NEW-1")
	assert.NotContains(t, changedIssues, "OLD-1")

	// Test force option
	options.Force = true
	changedIssues, err = manager.GetChangedIssues(state, options)
	require.NoError(t, err)

	// Should include all issues when force is true
	assert.Contains(t, changedIssues, "OLD-1")
	assert.Contains(t, changedIssues, "NEW-1")

	// Test project filter
	options.Force = false
	options.Projects = []string{"OLD"}
	changedIssues, err = manager.GetChangedIssues(state, options)
	require.NoError(t, err)

	// Should only include issues from OLD project
	assert.NotContains(t, changedIssues, "NEW-1")

	// Test last sync time
	lastSync := manager.GetLastSyncTime(state)
	assert.Equal(t, oldTime, lastSync)
}

func TestFileStateManager_StateValidation(t *testing.T) {
	// Create temporary directory with test files
	tempDir, err := os.MkdirTemp("", "state-test-*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Create test file structure
	projectDir := filepath.Join(tempDir, "projects", "TEST", "issues")
	err = os.MkdirAll(projectDir, 0755)
	require.NoError(t, err)

	existingFile := filepath.Join(projectDir, "TEST-123.yaml")
	err = os.WriteFile(existingFile, []byte("test content"), 0644)
	require.NoError(t, err)

	orphanFile := filepath.Join(projectDir, "TEST-456.yaml")
	err = os.WriteFile(orphanFile, []byte("orphan content"), 0644)
	require.NoError(t, err)

	manager := NewFileStateManager(FormatYAML)

	// Create state that tracks only one file
	state := &SyncState{
		Issues: map[string]IssueState{
			"TEST-123": {
				Key:      "TEST-123",
				FilePath: existingFile,
				Checksum: "invalid-checksum", // This should trigger a warning
			},
			"TEST-MISSING": {
				Key:      "TEST-MISSING",
				FilePath: filepath.Join(projectDir, "TEST-MISSING.yaml"),
			},
		},
	}

	// Validate state
	result, err := manager.ValidateState(state, tempDir)
	require.NoError(t, err)
	assert.NotNil(t, result)

	// Should be invalid due to missing files
	assert.False(t, result.Valid)

	// Should detect missing issue
	assert.Contains(t, result.MissingIssues, "TEST-MISSING")
	assert.Len(t, result.Errors, 1)

	// Should detect orphaned file
	assert.Contains(t, result.OrphanedFiles, orphanFile)

	// Should have warnings about checksum mismatch
	assert.NotEmpty(t, result.Warnings)

	// Should have recommendations
	assert.NotEmpty(t, result.RecommendedActions)
}

func TestFileStateManager_Statistics(t *testing.T) {
	manager := NewFileStateManager(FormatYAML)

	state := &SyncState{
		Stats: SyncStatistics{
			TotalOperations: 0,
			SuccessfulOps:   0,
			FailedOps:       0,
		},
		Issues: map[string]IssueState{
			"PROJ1-1": {ProjectKey: "PROJ1"},
			"PROJ1-2": {ProjectKey: "PROJ1"},
			"PROJ2-1": {ProjectKey: "PROJ2"},
		},
	}

	// Test successful operation
	operation := &SyncOperation{
		Status:   SyncStatusCompleted,
		Duration: 5 * time.Second,
		EndTime:  time.Now(),
		Results: OperationResults{
			SuccessfulSync: 3,
		},
	}

	err := manager.UpdateStatistics(state, operation)
	require.NoError(t, err)

	stats := manager.GetSyncStatistics(state)
	assert.Equal(t, 1, stats.TotalOperations)
	assert.Equal(t, 1, stats.SuccessfulOps)
	assert.Equal(t, 0, stats.FailedOps)
	assert.Equal(t, 3, stats.TotalIssuesSynced)
	assert.Equal(t, 3, stats.UniqueIssues)
	assert.Len(t, stats.ActiveProjects, 2)
	assert.Contains(t, stats.ActiveProjects, "PROJ1")
	assert.Contains(t, stats.ActiveProjects, "PROJ2")

	// Test failed operation
	failedOperation := &SyncOperation{
		Status:   SyncStatusFailed,
		Duration: 2 * time.Second,
		EndTime:  time.Now(),
	}

	err = manager.UpdateStatistics(state, failedOperation)
	require.NoError(t, err)

	stats = manager.GetSyncStatistics(state)
	assert.Equal(t, 2, stats.TotalOperations)
	assert.Equal(t, 1, stats.SuccessfulOps)
	assert.Equal(t, 1, stats.FailedOps)
}

func TestFileStateManager_JSONFormat(t *testing.T) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "state-test-*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Test JSON format
	manager := NewFileStateManager(FormatJSON)

	repoInfo := RepositoryInfo{
		Path:   tempDir,
		Branch: "main",
	}

	state, err := manager.InitializeState(tempDir, repoInfo)
	require.NoError(t, err)

	// Verify JSON state file was created
	stateFilePath := filepath.Join(tempDir, ".jira-sync-state.json")
	assert.FileExists(t, stateFilePath)

	// Load and verify
	loadedState, err := manager.LoadState(tempDir)
	require.NoError(t, err)
	assert.Equal(t, state.Version, loadedState.Version)
}

func TestFileStateManager_HistoryLimit(t *testing.T) {
	manager := NewFileStateManager(FormatYAML)

	state := &SyncState{
		History: make([]SyncOperation, 0),
	}

	// Add more than MaxHistoryEntries operations
	for i := 0; i < MaxHistoryEntries+10; i++ {
		operation := SyncOperation{
			ID:     fmt.Sprintf("op-%d", i),
			Status: SyncStatusCompleted,
		}
		state.History = append(state.History, operation)
	}

	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "state-test-*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Save state (should limit history)
	err = manager.SaveState(tempDir, state)
	require.NoError(t, err)

	// Load and verify history was limited
	loadedState, err := manager.LoadState(tempDir)
	require.NoError(t, err)

	assert.Len(t, loadedState.History, MaxHistoryEntries)

	// Verify it kept the most recent entries
	assert.Equal(t, fmt.Sprintf("op-%d", MaxHistoryEntries+10-MaxHistoryEntries), loadedState.History[0].ID)
	assert.Equal(t, fmt.Sprintf("op-%d", MaxHistoryEntries+10-1), loadedState.History[MaxHistoryEntries-1].ID)
}
