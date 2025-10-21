package config

import (
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"

	operatortypes "github.com/chambrid/jira-cdc-git/internal/operator/types"
)

func TestChangeDetector_DetectChanges(t *testing.T) {
	detector := NewChangeDetector(logr.Discard())

	tests := []struct {
		name                    string
		previous                *operatortypes.APIServerSpec
		current                 *operatortypes.APIServerSpec
		expectedHasChanges      bool
		expectedRequiresUpdate  bool
		expectedRequiresRestart bool
		expectedChangeCount     int
		expectedImpacts         []ChangeImpact
	}{
		{
			name: "No changes - identical specs",
			previous: &operatortypes.APIServerSpec{
				JIRACredentials: operatortypes.JIRACredentialsSpec{
					SecretRef: operatortypes.SecretRef{Name: "jira-secret"},
				},
				Image: operatortypes.ImageSpec{
					Repository: "registry.example.com/jira-sync",
					Tag:        "v1.0.0",
				},
				Replicas: &[]int32{2}[0],
			},
			current: &operatortypes.APIServerSpec{
				JIRACredentials: operatortypes.JIRACredentialsSpec{
					SecretRef: operatortypes.SecretRef{Name: "jira-secret"},
				},
				Image: operatortypes.ImageSpec{
					Repository: "registry.example.com/jira-sync",
					Tag:        "v1.0.0",
				},
				Replicas: &[]int32{2}[0],
			},
			expectedHasChanges:      false,
			expectedRequiresUpdate:  false,
			expectedRequiresRestart: false,
			expectedChangeCount:     0,
			expectedImpacts:         []ChangeImpact{},
		},
		{
			name: "JIRA credentials secret changed - critical impact",
			previous: &operatortypes.APIServerSpec{
				JIRACredentials: operatortypes.JIRACredentialsSpec{
					SecretRef: operatortypes.SecretRef{Name: "old-jira-secret"},
				},
				Image: operatortypes.ImageSpec{
					Repository: "registry.example.com/jira-sync",
					Tag:        "v1.0.0",
				},
			},
			current: &operatortypes.APIServerSpec{
				JIRACredentials: operatortypes.JIRACredentialsSpec{
					SecretRef: operatortypes.SecretRef{Name: "new-jira-secret"},
				},
				Image: operatortypes.ImageSpec{
					Repository: "registry.example.com/jira-sync",
					Tag:        "v1.0.0",
				},
			},
			expectedHasChanges:      true,
			expectedRequiresUpdate:  true,
			expectedRequiresRestart: true,
			expectedChangeCount:     2, // Overall credentials change + specific secret name change
			expectedImpacts:         []ChangeImpact{ImpactHigh, ImpactCritical},
		},
		{
			name: "Image tag changed - medium impact",
			previous: &operatortypes.APIServerSpec{
				JIRACredentials: operatortypes.JIRACredentialsSpec{
					SecretRef: operatortypes.SecretRef{Name: "jira-secret"},
				},
				Image: operatortypes.ImageSpec{
					Repository: "registry.example.com/jira-sync",
					Tag:        "v1.0.0",
				},
			},
			current: &operatortypes.APIServerSpec{
				JIRACredentials: operatortypes.JIRACredentialsSpec{
					SecretRef: operatortypes.SecretRef{Name: "jira-secret"},
				},
				Image: operatortypes.ImageSpec{
					Repository: "registry.example.com/jira-sync",
					Tag:        "v2.0.0",
				},
			},
			expectedHasChanges:      true,
			expectedRequiresUpdate:  true,
			expectedRequiresRestart: false,
			expectedChangeCount:     1,
			expectedImpacts:         []ChangeImpact{ImpactMedium},
		},
		{
			name: "Image repository changed - high impact",
			previous: &operatortypes.APIServerSpec{
				JIRACredentials: operatortypes.JIRACredentialsSpec{
					SecretRef: operatortypes.SecretRef{Name: "jira-secret"},
				},
				Image: operatortypes.ImageSpec{
					Repository: "old-registry.example.com/jira-sync",
					Tag:        "v1.0.0",
				},
			},
			current: &operatortypes.APIServerSpec{
				JIRACredentials: operatortypes.JIRACredentialsSpec{
					SecretRef: operatortypes.SecretRef{Name: "jira-secret"},
				},
				Image: operatortypes.ImageSpec{
					Repository: "new-registry.example.com/jira-sync",
					Tag:        "v1.0.0",
				},
			},
			expectedHasChanges:      true,
			expectedRequiresUpdate:  true,
			expectedRequiresRestart: true,
			expectedChangeCount:     1,
			expectedImpacts:         []ChangeImpact{ImpactHigh},
		},
		{
			name: "Replica count changed - low to medium impact",
			previous: &operatortypes.APIServerSpec{
				JIRACredentials: operatortypes.JIRACredentialsSpec{
					SecretRef: operatortypes.SecretRef{Name: "jira-secret"},
				},
				Image: operatortypes.ImageSpec{
					Repository: "registry.example.com/jira-sync",
					Tag:        "v1.0.0",
				},
				Replicas: &[]int32{3}[0],
			},
			current: &operatortypes.APIServerSpec{
				JIRACredentials: operatortypes.JIRACredentialsSpec{
					SecretRef: operatortypes.SecretRef{Name: "jira-secret"},
				},
				Image: operatortypes.ImageSpec{
					Repository: "registry.example.com/jira-sync",
					Tag:        "v1.0.0",
				},
				Replicas: &[]int32{1}[0], // Scaling down
			},
			expectedHasChanges:      true,
			expectedRequiresUpdate:  true,
			expectedRequiresRestart: false,
			expectedChangeCount:     1,
			expectedImpacts:         []ChangeImpact{ImpactMedium}, // Scaling down has medium impact
		},
		{
			name: "Log level changed - low impact",
			previous: &operatortypes.APIServerSpec{
				JIRACredentials: operatortypes.JIRACredentialsSpec{
					SecretRef: operatortypes.SecretRef{Name: "jira-secret"},
				},
				Image: operatortypes.ImageSpec{
					Repository: "registry.example.com/jira-sync",
					Tag:        "v1.0.0",
				},
				Config: &operatortypes.APIServerConfig{
					LogLevel: "INFO",
				},
			},
			current: &operatortypes.APIServerSpec{
				JIRACredentials: operatortypes.JIRACredentialsSpec{
					SecretRef: operatortypes.SecretRef{Name: "jira-secret"},
				},
				Image: operatortypes.ImageSpec{
					Repository: "registry.example.com/jira-sync",
					Tag:        "v1.0.0",
				},
				Config: &operatortypes.APIServerConfig{
					LogLevel: "DEBUG",
				},
			},
			expectedHasChanges:      true,
			expectedRequiresUpdate:  true,
			expectedRequiresRestart: false,
			expectedChangeCount:     1,
			expectedImpacts:         []ChangeImpact{ImpactLow},
		},
		{
			name: "API port changed - high impact",
			previous: &operatortypes.APIServerSpec{
				JIRACredentials: operatortypes.JIRACredentialsSpec{
					SecretRef: operatortypes.SecretRef{Name: "jira-secret"},
				},
				Image: operatortypes.ImageSpec{
					Repository: "registry.example.com/jira-sync",
					Tag:        "v1.0.0",
				},
				Config: &operatortypes.APIServerConfig{
					Port: &[]int32{8080}[0],
				},
			},
			current: &operatortypes.APIServerSpec{
				JIRACredentials: operatortypes.JIRACredentialsSpec{
					SecretRef: operatortypes.SecretRef{Name: "jira-secret"},
				},
				Image: operatortypes.ImageSpec{
					Repository: "registry.example.com/jira-sync",
					Tag:        "v1.0.0",
				},
				Config: &operatortypes.APIServerConfig{
					Port: &[]int32{9090}[0],
				},
			},
			expectedHasChanges:      true,
			expectedRequiresUpdate:  true,
			expectedRequiresRestart: true,
			expectedChangeCount:     1,
			expectedImpacts:         []ChangeImpact{ImpactHigh},
		},
		{
			name: "Multiple changes with mixed impacts",
			previous: &operatortypes.APIServerSpec{
				JIRACredentials: operatortypes.JIRACredentialsSpec{
					SecretRef: operatortypes.SecretRef{Name: "jira-secret"},
				},
				Image: operatortypes.ImageSpec{
					Repository: "registry.example.com/jira-sync",
					Tag:        "v1.0.0",
					PullPolicy: "IfNotPresent",
				},
				Replicas: &[]int32{2}[0],
				Config: &operatortypes.APIServerConfig{
					LogLevel:        "INFO",
					LogFormat:       "json",
					EnableJobs:      &[]bool{false}[0],
					SafeModeEnabled: &[]bool{false}[0],
				},
			},
			current: &operatortypes.APIServerSpec{
				JIRACredentials: operatortypes.JIRACredentialsSpec{
					SecretRef: operatortypes.SecretRef{Name: "jira-secret"},
				},
				Image: operatortypes.ImageSpec{
					Repository: "registry.example.com/jira-sync",
					Tag:        "v2.0.0", // Medium impact
					PullPolicy: "Always", // Low impact
				},
				Replicas: &[]int32{4}[0], // Low impact (scaling up)
				Config: &operatortypes.APIServerConfig{
					LogLevel:        "DEBUG",          // Low impact
					LogFormat:       "text",           // Low impact
					EnableJobs:      &[]bool{true}[0], // Medium impact
					SafeModeEnabled: &[]bool{true}[0], // Medium impact
				},
			},
			expectedHasChanges:      true,
			expectedRequiresUpdate:  true,
			expectedRequiresRestart: false,                                   // No critical/high impact changes
			expectedChangeCount:     7,                                       // tag, pull policy, replicas, log level, log format, enable jobs, safe mode
			expectedImpacts:         []ChangeImpact{ImpactLow, ImpactMedium}, // Mix of low and medium impacts
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detector.DetectChanges(tt.previous, tt.current, 1, 2)

			assert.Equal(t, tt.expectedHasChanges, result.HasChanges, "HasChanges should match expected")
			assert.Equal(t, tt.expectedRequiresUpdate, result.RequiresUpdate, "RequiresUpdate should match expected")
			assert.Equal(t, tt.expectedRequiresRestart, result.RequiresRestart, "RequiresRestart should match expected")
			assert.Len(t, result.ConfigChanges, tt.expectedChangeCount, "Number of changes should match expected")

			// Verify that all expected impact levels are present
			if len(tt.expectedImpacts) > 0 {
				foundImpacts := make(map[ChangeImpact]bool)
				for _, change := range result.ConfigChanges {
					foundImpacts[change.Impact] = true
				}

				for _, expectedImpact := range tt.expectedImpacts {
					assert.True(t, foundImpacts[expectedImpact], "Expected impact level %s should be found", expectedImpact)
				}
			}

			// Verify that hashes are different when changes are detected
			if tt.expectedHasChanges {
				assert.NotEqual(t, result.PreviousHash, result.CurrentHash, "Hashes should be different when changes are detected")
				assert.NotEmpty(t, result.UpdateRecommendation, "Update recommendation should be provided")
			} else {
				assert.Equal(t, result.PreviousHash, result.CurrentHash, "Hashes should be the same when no changes are detected")
			}

			// Verify change details have meaningful information
			for _, change := range result.ConfigChanges {
				assert.NotEmpty(t, change.Field, "Change should specify field")
				assert.NotEmpty(t, change.FieldPath, "Change should specify field path")
				assert.NotEmpty(t, change.Description, "Change should have description")
				assert.NotEqual(t, change.OldValue, change.NewValue, "Old and new values should be different")
			}
		})
	}
}

func TestChangeDetector_ImpactLevel(t *testing.T) {
	detector := NewChangeDetector(logr.Discard())

	tests := []struct {
		impact   ChangeImpact
		expected int
	}{
		{ImpactLow, 1},
		{ImpactMedium, 2},
		{ImpactHigh, 3},
		{ImpactCritical, 4},
		{ChangeImpact("unknown"), 0},
	}

	for _, tt := range tests {
		t.Run(string(tt.impact), func(t *testing.T) {
			result := detector.impactLevel(tt.impact)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestChangeDetector_GetConfigValue(t *testing.T) {
	detector := NewChangeDetector(logr.Discard())

	tests := []struct {
		name     string
		config   *operatortypes.APIServerConfig
		expected operatortypes.APIServerConfig
	}{
		{
			name: "Non-nil config",
			config: &operatortypes.APIServerConfig{
				LogLevel:        "DEBUG",
				LogFormat:       "text",
				Port:            &[]int32{9090}[0],
				EnableJobs:      &[]bool{true}[0],
				SafeModeEnabled: &[]bool{true}[0],
			},
			expected: operatortypes.APIServerConfig{
				LogLevel:        "DEBUG",
				LogFormat:       "text",
				Port:            &[]int32{9090}[0],
				EnableJobs:      &[]bool{true}[0],
				SafeModeEnabled: &[]bool{true}[0],
			},
		},
		{
			name:     "Nil config",
			config:   nil,
			expected: operatortypes.APIServerConfig{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detector.getConfigValue(tt.config)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestChangeDetector_GetReplicaValue(t *testing.T) {
	detector := NewChangeDetector(logr.Discard())

	tests := []struct {
		name     string
		replicas *int32
		expected int32
	}{
		{
			name:     "Non-nil replicas",
			replicas: &[]int32{5}[0],
			expected: 5,
		},
		{
			name:     "Nil replicas",
			replicas: nil,
			expected: 2, // Default value
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detector.getReplicaValue(tt.replicas)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestChangeDetector_DetermineUpdateRequirements(t *testing.T) {
	detector := NewChangeDetector(logr.Discard())

	tests := []struct {
		name                    string
		changes                 []ConfigChange
		expectedRequiresUpdate  bool
		expectedRequiresRestart bool
		expectedRecommendation  string
	}{
		{
			name:                    "No changes",
			changes:                 []ConfigChange{},
			expectedRequiresUpdate:  false,
			expectedRequiresRestart: false,
			expectedRecommendation:  "No updates required",
		},
		{
			name: "Low impact changes only",
			changes: []ConfigChange{
				{Field: "config.logLevel", Impact: ImpactLow},
				{Field: "config.logFormat", Impact: ImpactLow},
			},
			expectedRequiresUpdate:  true,
			expectedRequiresRestart: false,
			expectedRecommendation:  "Low-impact configuration changes detected - ConfigMap update sufficient",
		},
		{
			name: "Medium impact changes",
			changes: []ConfigChange{
				{Field: "replicas", Impact: ImpactMedium},
				{Field: "config.enableJobs", Impact: ImpactMedium},
			},
			expectedRequiresUpdate:  true,
			expectedRequiresRestart: false,
			expectedRecommendation:  "Medium-impact configuration changes detected - rolling update recommended",
		},
		{
			name: "High impact changes",
			changes: []ConfigChange{
				{Field: "image.repository", Impact: ImpactHigh},
				{Field: "config.port", Impact: ImpactHigh},
			},
			expectedRequiresUpdate:  true,
			expectedRequiresRestart: true,
			expectedRecommendation:  "High-impact configuration changes detected - rolling restart required",
		},
		{
			name: "Critical impact changes",
			changes: []ConfigChange{
				{Field: "jiraCredentials.secretRef.name", Impact: ImpactCritical},
			},
			expectedRequiresUpdate:  true,
			expectedRequiresRestart: true,
			expectedRecommendation:  "Critical configuration changes detected - immediate restart required",
		},
		{
			name: "Mixed impact changes - highest wins",
			changes: []ConfigChange{
				{Field: "config.logLevel", Impact: ImpactLow},
				{Field: "replicas", Impact: ImpactMedium},
				{Field: "image.repository", Impact: ImpactHigh},
			},
			expectedRequiresUpdate:  true,
			expectedRequiresRestart: true,
			expectedRecommendation:  "High-impact configuration changes detected - rolling restart required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &ChangeResult{
				ConfigChanges: tt.changes,
			}

			detector.determineUpdateRequirements(result)

			assert.Equal(t, tt.expectedRequiresUpdate, result.RequiresUpdate, "RequiresUpdate should match expected")
			assert.Equal(t, tt.expectedRequiresRestart, result.RequiresRestart, "RequiresRestart should match expected")
			assert.Equal(t, tt.expectedRecommendation, result.UpdateRecommendation, "UpdateRecommendation should match expected")
		})
	}
}
