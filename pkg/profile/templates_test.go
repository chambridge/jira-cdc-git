package profile

import (
	"testing"
)

func TestGetBuiltinTemplates(t *testing.T) {
	templates := GetBuiltinTemplates()

	if len(templates) == 0 {
		t.Error("Expected at least one builtin template")
	}

	// Check that all templates have required fields
	for _, tmpl := range templates {
		if tmpl.ID == "" {
			t.Error("Template ID cannot be empty")
		}
		if tmpl.Name == "" {
			t.Error("Template name cannot be empty")
		}
		if tmpl.Category == "" {
			t.Error("Template category cannot be empty")
		}
		if len(tmpl.Variables) == 0 {
			t.Error("Template should have at least one variable")
		}

		// Check that template has name variable
		hasNameVar := false
		for _, variable := range tmpl.Variables {
			if variable.Name == "name" {
				hasNameVar = true
				break
			}
		}
		if !hasNameVar {
			t.Errorf("Template %s should have 'name' variable", tmpl.ID)
		}
	}
}

func TestGetTemplatesByCategory(t *testing.T) {
	categories := GetTemplatesByCategory()

	if len(categories) == 0 {
		t.Error("Expected at least one category")
	}

	// Check that templates are properly categorized
	for category, templates := range categories {
		if category == "" {
			t.Error("Category name cannot be empty")
		}
		if len(templates) == 0 {
			t.Errorf("Category %s should have at least one template", category)
		}

		for _, tmpl := range templates {
			if tmpl.Category != category && category != "other" {
				t.Errorf("Template %s has category %s but is in category %s",
					tmpl.ID, tmpl.Category, category)
			}
		}
	}
}

func TestFileProfileManager_GetTemplate(t *testing.T) {
	manager := NewFileProfileManager("", "yaml")

	// Test getting existing template
	template, err := manager.GetTemplate("epic-all-issues")
	if err != nil {
		t.Fatalf("Failed to get template: %v", err)
	}

	if template.ID != "epic-all-issues" {
		t.Errorf("Expected ID 'epic-all-issues', got %s", template.ID)
	}

	// Test getting non-existent template
	_, err = manager.GetTemplate("non-existent")
	if err == nil {
		t.Error("Expected error for non-existent template")
	}
}

func TestFileProfileManager_CreateFromTemplate(t *testing.T) {
	manager := NewFileProfileManager("", "yaml")

	tests := []struct {
		name        string
		templateID  string
		profileName string
		variables   map[string]string
		expectError bool
	}{
		{
			name:        "valid epic template",
			templateID:  "epic-all-issues",
			profileName: "test-epic",
			variables: map[string]string{
				"epic_key":   "TEST-123",
				"repository": "./test-repo",
			},
			expectError: false,
		},
		{
			name:        "valid JQL template",
			templateID:  "custom-jql",
			profileName: "test-jql",
			variables: map[string]string{
				"jql":        "project = TEST",
				"repository": "./test-repo",
			},
			expectError: false,
		},
		{
			name:        "missing required variable",
			templateID:  "epic-all-issues",
			profileName: "test-missing",
			variables: map[string]string{
				"epic_key": "TEST-123",
				// missing repository
			},
			expectError: true,
		},
		{
			name:        "non-existent template",
			templateID:  "non-existent",
			profileName: "test-invalid",
			variables:   map[string]string{},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			profile, err := manager.CreateFromTemplate(tt.templateID, tt.profileName, tt.variables)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if profile.Name != tt.profileName {
				t.Errorf("Expected name %s, got %s", tt.profileName, profile.Name)
			}

			// Validate that required fields are set
			if profile.Repository == "" {
				t.Error("Repository should be set from template")
			}

			// Check that variables were substituted
			for varName, varValue := range tt.variables {
				switch varName {
				case "epic_key":
					if profile.EpicKey != varValue {
						t.Errorf("Expected EpicKey %s, got %s", varValue, profile.EpicKey)
					}
				case "jql":
					if profile.JQL != varValue {
						t.Errorf("Expected JQL %s, got %s", varValue, profile.JQL)
					}
				case "repository":
					if profile.Repository != varValue {
						t.Errorf("Expected Repository %s, got %s", varValue, profile.Repository)
					}
				}
			}
		})
	}
}

func TestValidateTemplateVariables(t *testing.T) {
	template := &ProfileTemplate{
		ID:   "test-template",
		Name: "Test Template",
		Variables: []TemplateVar{
			{
				Name:     "required_var",
				Required: true,
			},
			{
				Name:     "optional_var",
				Required: false,
			},
		},
	}

	tests := []struct {
		name        string
		variables   map[string]string
		expectError bool
	}{
		{
			name: "all required variables provided",
			variables: map[string]string{
				"required_var": "value1",
				"optional_var": "value2",
			},
			expectError: false,
		},
		{
			name: "only required variables provided",
			variables: map[string]string{
				"required_var": "value1",
			},
			expectError: false,
		},
		{
			name: "missing required variable",
			variables: map[string]string{
				"optional_var": "value2",
			},
			expectError: true,
		},
		{
			name: "empty required variable",
			variables: map[string]string{
				"required_var": "",
				"optional_var": "value2",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTemplateVariables(template, tt.variables)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}

func TestTemplateWithDefaults(t *testing.T) {
	manager := NewFileProfileManager("", "yaml")

	// Test template that uses default values
	variables := map[string]string{
		"name":       "test-recent",
		"repository": "./test-repo",
		// days should use default value from template
	}

	profile, err := manager.CreateFromTemplate("recent-updates", "test-recent", variables)
	if err != nil {
		t.Fatalf("Failed to create profile from template: %v", err)
	}

	// Check that JQL contains the default days value (7)
	if !contains(profile.JQL, "7d") {
		t.Errorf("Expected JQL to contain default days value, got: %s", profile.JQL)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			stringContains(s, substr)))
}

func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
