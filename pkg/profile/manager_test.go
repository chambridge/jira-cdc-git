package profile

import (
	"os"
	"testing"
)

func TestFileProfileManager_CreateProfile(t *testing.T) {
	// Create temporary directory for test
	tmpDir, err := os.MkdirTemp("", "profile-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	manager := NewFileProfileManager(tmpDir, "yaml")

	// Test profile creation
	profile := &Profile{
		Name:        "test-profile",
		Description: "Test profile for unit testing",
		JQL:         "project = TEST",
		Repository:  "./test-repo",
		Options: ProfileOptions{
			Concurrency:  5,
			RateLimit:    "500ms",
			Incremental:  false,
			Force:        false,
			DryRun:       false,
			IncludeLinks: true,
		},
		Tags: []string{"test", "automation"},
	}

	err = manager.CreateProfile(profile)
	if err != nil {
		t.Fatalf("Failed to create profile: %v", err)
	}

	// Verify profile was created
	if !manager.ProfileExists("test-profile") {
		t.Error("Profile should exist after creation")
	}

	// Verify profile can be retrieved
	retrieved, err := manager.GetProfile("test-profile")
	if err != nil {
		t.Fatalf("Failed to get profile: %v", err)
	}

	if retrieved.Name != profile.Name {
		t.Errorf("Expected name %s, got %s", profile.Name, retrieved.Name)
	}
	if retrieved.JQL != profile.JQL {
		t.Errorf("Expected JQL %s, got %s", profile.JQL, retrieved.JQL)
	}
}

func TestFileProfileManager_CreateDuplicateProfile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "profile-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	manager := NewFileProfileManager(tmpDir, "yaml")

	profile := &Profile{
		Name:       "duplicate-test",
		JQL:        "project = TEST",
		Repository: "./test-repo",
	}

	// Create first profile
	err = manager.CreateProfile(profile)
	if err != nil {
		t.Fatalf("Failed to create first profile: %v", err)
	}

	// Try to create duplicate
	err = manager.CreateProfile(profile)
	if err == nil {
		t.Error("Expected error when creating duplicate profile")
	}
}

func TestFileProfileManager_UpdateProfile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "profile-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	manager := NewFileProfileManager(tmpDir, "yaml")

	// Create initial profile
	profile := &Profile{
		Name:        "update-test",
		Description: "Original description",
		JQL:         "project = TEST",
		Repository:  "./test-repo",
	}

	err = manager.CreateProfile(profile)
	if err != nil {
		t.Fatalf("Failed to create profile: %v", err)
	}

	// Update profile
	updatedProfile := *profile
	updatedProfile.Description = "Updated description"
	updatedProfile.JQL = "project = UPDATED"

	err = manager.UpdateProfile("update-test", &updatedProfile)
	if err != nil {
		t.Fatalf("Failed to update profile: %v", err)
	}

	// Verify update
	retrieved, err := manager.GetProfile("update-test")
	if err != nil {
		t.Fatalf("Failed to get updated profile: %v", err)
	}

	if retrieved.Description != "Updated description" {
		t.Errorf("Expected updated description, got %s", retrieved.Description)
	}
	if retrieved.JQL != "project = UPDATED" {
		t.Errorf("Expected updated JQL, got %s", retrieved.JQL)
	}
}

func TestFileProfileManager_DeleteProfile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "profile-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	manager := NewFileProfileManager(tmpDir, "yaml")

	// Create profile
	profile := &Profile{
		Name:       "delete-test",
		JQL:        "project = TEST",
		Repository: "./test-repo",
	}

	err = manager.CreateProfile(profile)
	if err != nil {
		t.Fatalf("Failed to create profile: %v", err)
	}

	// Verify profile exists
	if !manager.ProfileExists("delete-test") {
		t.Error("Profile should exist before deletion")
	}

	// Delete profile
	err = manager.DeleteProfile("delete-test")
	if err != nil {
		t.Fatalf("Failed to delete profile: %v", err)
	}

	// Verify profile no longer exists
	if manager.ProfileExists("delete-test") {
		t.Error("Profile should not exist after deletion")
	}
}

func TestFileProfileManager_ListProfiles(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "profile-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	manager := NewFileProfileManager(tmpDir, "yaml")

	// Create multiple profiles
	profiles := []*Profile{
		{
			Name:       "profile-1",
			JQL:        "project = TEST1",
			Repository: "./repo1",
			Tags:       []string{"tag1", "common"},
		},
		{
			Name:       "profile-2",
			JQL:        "project = TEST2",
			Repository: "./repo2",
			Tags:       []string{"tag2", "common"},
		},
		{
			Name:       "profile-3",
			JQL:        "project = TEST3",
			Repository: "./repo3",
			Tags:       []string{"tag1"},
		},
	}

	for _, profile := range profiles {
		err = manager.CreateProfile(profile)
		if err != nil {
			t.Fatalf("Failed to create profile %s: %v", profile.Name, err)
		}
	}

	// List all profiles
	allProfiles, err := manager.ListProfiles(nil)
	if err != nil {
		t.Fatalf("Failed to list profiles: %v", err)
	}

	if len(allProfiles) != 3 {
		t.Errorf("Expected 3 profiles, got %d", len(allProfiles))
	}

	// List profiles with tag filter
	options := &ProfileListOptions{
		Tags: []string{"tag1"},
	}

	filteredProfiles, err := manager.ListProfiles(options)
	if err != nil {
		t.Fatalf("Failed to list filtered profiles: %v", err)
	}

	if len(filteredProfiles) != 2 {
		t.Errorf("Expected 2 profiles with tag1, got %d", len(filteredProfiles))
	}
}

func TestFileProfileManager_ValidateProfile(t *testing.T) {
	manager := NewFileProfileManager("", "yaml")

	tests := []struct {
		name      string
		profile   *Profile
		wantValid bool
	}{
		{
			name: "valid JQL profile",
			profile: &Profile{
				Name:       "valid-jql",
				JQL:        "project = TEST",
				Repository: "./repo",
				Options: ProfileOptions{
					Concurrency: 5,
				},
			},
			wantValid: true,
		},
		{
			name: "valid issue keys profile",
			profile: &Profile{
				Name:       "valid-issues",
				IssueKeys:  []string{"TEST-1", "TEST-2"},
				Repository: "./repo",
				Options: ProfileOptions{
					Concurrency: 3,
				},
			},
			wantValid: true,
		},
		{
			name: "valid epic profile",
			profile: &Profile{
				Name:       "valid-epic",
				EpicKey:    "TEST-123",
				Repository: "./repo",
				Options: ProfileOptions{
					Concurrency: 5,
				},
			},
			wantValid: true,
		},
		{
			name: "invalid - no sync mode",
			profile: &Profile{
				Name:       "no-sync-mode",
				Repository: "./repo",
			},
			wantValid: false,
		},
		{
			name: "invalid - multiple sync modes",
			profile: &Profile{
				Name:       "multiple-modes",
				JQL:        "project = TEST",
				EpicKey:    "TEST-123",
				Repository: "./repo",
			},
			wantValid: false,
		},
		{
			name: "invalid - no repository",
			profile: &Profile{
				Name: "no-repo",
				JQL:  "project = TEST",
			},
			wantValid: false,
		},
		{
			name: "invalid - empty name",
			profile: &Profile{
				Name:       "",
				JQL:        "project = TEST",
				Repository: "./repo",
			},
			wantValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := manager.ValidateProfile(tt.profile)
			if err != nil {
				t.Fatalf("ValidateProfile failed: %v", err)
			}

			if result.Valid != tt.wantValid {
				t.Errorf("Expected valid=%t, got valid=%t, errors=%v",
					tt.wantValid, result.Valid, result.Errors)
			}
		})
	}
}

func TestFileProfileManager_RecordUsage(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "profile-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	manager := NewFileProfileManager(tmpDir, "yaml")

	// Create profile
	profile := &Profile{
		Name:       "usage-test",
		JQL:        "project = TEST",
		Repository: "./repo",
	}

	err = manager.CreateProfile(profile)
	if err != nil {
		t.Fatalf("Failed to create profile: %v", err)
	}

	// Record usage
	err = manager.RecordUsage("usage-test", 1500, true)
	if err != nil {
		t.Fatalf("Failed to record usage: %v", err)
	}

	// Get updated profile
	updated, err := manager.GetProfile("usage-test")
	if err != nil {
		t.Fatalf("Failed to get profile: %v", err)
	}

	if updated.UsageStats.TimesUsed != 1 {
		t.Errorf("Expected TimesUsed=1, got %d", updated.UsageStats.TimesUsed)
	}

	if updated.UsageStats.TotalSyncTime != 1500 {
		t.Errorf("Expected TotalSyncTime=1500, got %d", updated.UsageStats.TotalSyncTime)
	}

	if updated.UsageStats.AvgSyncTime != 1500 {
		t.Errorf("Expected AvgSyncTime=1500, got %d", updated.UsageStats.AvgSyncTime)
	}
}

func TestFileProfileManager_BackupAndRestore(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "profile-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	manager := NewFileProfileManager(tmpDir, "yaml")

	// Create profile
	profile := &Profile{
		Name:       "backup-test",
		JQL:        "project = TEST",
		Repository: "./repo",
	}

	err = manager.CreateProfile(profile)
	if err != nil {
		t.Fatalf("Failed to create profile: %v", err)
	}

	// Create backup
	err = manager.BackupProfiles()
	if err != nil {
		t.Fatalf("Failed to backup profiles: %v", err)
	}

	// Verify backup file exists
	backupFile := manager.getProfilesFilePath() + ".backup"
	if _, err := os.Stat(backupFile); os.IsNotExist(err) {
		t.Error("Backup file should exist")
	}

	// Modify original (delete profile)
	err = manager.DeleteProfile("backup-test")
	if err != nil {
		t.Fatalf("Failed to delete profile: %v", err)
	}

	// Verify profile is gone
	if manager.ProfileExists("backup-test") {
		t.Error("Profile should be deleted")
	}

	// Restore from backup
	err = manager.RestoreProfiles()
	if err != nil {
		t.Fatalf("Failed to restore profiles: %v", err)
	}

	// Verify profile is restored
	if !manager.ProfileExists("backup-test") {
		t.Error("Profile should be restored")
	}
}
