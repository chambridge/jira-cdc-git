package config

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	operatortypes "github.com/chambrid/jira-cdc-git/internal/operator/types"
)

func TestConfigValidator_ValidateAPIServerSpec(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	tests := []struct {
		name            string
		spec            *operatortypes.APIServerSpec
		namespace       string
		existingSecrets []*corev1.Secret
		expectedValid   bool
		expectedErrors  int
	}{
		{
			name: "Valid complete specification",
			spec: &operatortypes.APIServerSpec{
				JIRACredentials: operatortypes.JIRACredentialsSpec{
					SecretRef: operatortypes.SecretRef{
						Name: "valid-jira-secret",
					},
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
					Port:            &[]int32{8080}[0],
					EnableJobs:      &[]bool{true}[0],
					SafeModeEnabled: &[]bool{false}[0],
				},
				Service: &operatortypes.ServiceConfig{
					Type: "ClusterIP",
					Port: &[]int32{80}[0],
				},
			},
			namespace: "test-namespace",
			existingSecrets: []*corev1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "valid-jira-secret",
						Namespace: "test-namespace",
					},
					Data: map[string][]byte{
						"base-url": []byte("https://company.atlassian.net"),
						"email":    []byte("user@company.com"),
						"pat":      []byte("secret-token"),
					},
				},
			},
			expectedValid:  true,
			expectedErrors: 0,
		},
		{
			name: "Missing secret reference",
			spec: &operatortypes.APIServerSpec{
				JIRACredentials: operatortypes.JIRACredentialsSpec{
					SecretRef: operatortypes.SecretRef{
						Name: "",
					},
				},
				Image: operatortypes.ImageSpec{
					Repository: "registry.example.com/jira-sync",
					Tag:        "v1.0.0",
				},
			},
			namespace:      "test-namespace",
			expectedValid:  false,
			expectedErrors: 1,
		},
		{
			name: "Secret not found",
			spec: &operatortypes.APIServerSpec{
				JIRACredentials: operatortypes.JIRACredentialsSpec{
					SecretRef: operatortypes.SecretRef{
						Name: "missing-secret",
					},
				},
				Image: operatortypes.ImageSpec{
					Repository: "registry.example.com/jira-sync",
					Tag:        "v1.0.0",
				},
			},
			namespace:      "test-namespace",
			expectedValid:  false,
			expectedErrors: 1,
		},
		{
			name: "Secret missing required keys",
			spec: &operatortypes.APIServerSpec{
				JIRACredentials: operatortypes.JIRACredentialsSpec{
					SecretRef: operatortypes.SecretRef{
						Name: "incomplete-secret",
					},
				},
				Image: operatortypes.ImageSpec{
					Repository: "registry.example.com/jira-sync",
					Tag:        "v1.0.0",
				},
			},
			namespace: "test-namespace",
			existingSecrets: []*corev1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "incomplete-secret",
						Namespace: "test-namespace",
					},
					Data: map[string][]byte{
						"base-url": []byte("https://company.atlassian.net"),
						// Missing email and pat
					},
				},
			},
			expectedValid:  false,
			expectedErrors: 2, // Missing email and pat keys
		},
		{
			name: "Invalid JIRA base URL",
			spec: &operatortypes.APIServerSpec{
				JIRACredentials: operatortypes.JIRACredentialsSpec{
					SecretRef: operatortypes.SecretRef{
						Name: "invalid-url-secret",
					},
				},
				Image: operatortypes.ImageSpec{
					Repository: "registry.example.com/jira-sync",
					Tag:        "v1.0.0",
				},
			},
			namespace: "test-namespace",
			existingSecrets: []*corev1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "invalid-url-secret",
						Namespace: "test-namespace",
					},
					Data: map[string][]byte{
						"base-url": []byte("http://insecure.example.com"), // Not HTTPS
						"email":    []byte("user@company.com"),
						"pat":      []byte("secret-token"),
					},
				},
			},
			expectedValid:  false,
			expectedErrors: 1,
		},
		{
			name: "Invalid image configuration",
			spec: &operatortypes.APIServerSpec{
				JIRACredentials: operatortypes.JIRACredentialsSpec{
					SecretRef: operatortypes.SecretRef{
						Name: "valid-jira-secret",
					},
				},
				Image: operatortypes.ImageSpec{
					Repository: "", // Missing repository
					Tag:        "", // Missing tag
					PullPolicy: "InvalidPolicy",
				},
			},
			namespace: "test-namespace",
			existingSecrets: []*corev1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "valid-jira-secret",
						Namespace: "test-namespace",
					},
					Data: map[string][]byte{
						"base-url": []byte("https://company.atlassian.net"),
						"email":    []byte("user@company.com"),
						"pat":      []byte("secret-token"),
					},
				},
			},
			expectedValid:  false,
			expectedErrors: 3, // Missing repository, tag, and invalid pull policy
		},
		{
			name: "Invalid replica count",
			spec: &operatortypes.APIServerSpec{
				JIRACredentials: operatortypes.JIRACredentialsSpec{
					SecretRef: operatortypes.SecretRef{
						Name: "valid-jira-secret",
					},
				},
				Image: operatortypes.ImageSpec{
					Repository: "registry.example.com/jira-sync",
					Tag:        "v1.0.0",
				},
				Replicas: &[]int32{0}[0], // Invalid replica count
			},
			namespace: "test-namespace",
			existingSecrets: []*corev1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "valid-jira-secret",
						Namespace: "test-namespace",
					},
					Data: map[string][]byte{
						"base-url": []byte("https://company.atlassian.net"),
						"email":    []byte("user@company.com"),
						"pat":      []byte("secret-token"),
					},
				},
			},
			expectedValid:  false,
			expectedErrors: 1,
		},
		{
			name: "Invalid configuration values",
			spec: &operatortypes.APIServerSpec{
				JIRACredentials: operatortypes.JIRACredentialsSpec{
					SecretRef: operatortypes.SecretRef{
						Name: "valid-jira-secret",
					},
				},
				Image: operatortypes.ImageSpec{
					Repository: "registry.example.com/jira-sync",
					Tag:        "v1.0.0",
				},
				Config: &operatortypes.APIServerConfig{
					LogLevel:  "INVALID_LEVEL",
					LogFormat: "invalid_format",
					Port:      &[]int32{100}[0], // Port too low
				},
			},
			namespace: "test-namespace",
			existingSecrets: []*corev1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "valid-jira-secret",
						Namespace: "test-namespace",
					},
					Data: map[string][]byte{
						"base-url": []byte("https://company.atlassian.net"),
						"email":    []byte("user@company.com"),
						"pat":      []byte("secret-token"),
					},
				},
			},
			expectedValid:  false,
			expectedErrors: 3, // Invalid log level, format, and port
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fake client with existing secrets
			objects := make([]client.Object, len(tt.existingSecrets))
			for i, secret := range tt.existingSecrets {
				objects[i] = secret
			}

			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(objects...).
				Build()

			validator := NewConfigValidator(fakeClient)

			result := validator.ValidateAPIServerSpec(context.Background(), tt.spec, tt.namespace)

			assert.Equal(t, tt.expectedValid, result.Valid, "Validation result should match expected")
			assert.Len(t, result.Errors, tt.expectedErrors, "Number of validation errors should match expected")

			if !tt.expectedValid {
				// Ensure we have meaningful error messages
				for _, err := range result.Errors {
					assert.NotEmpty(t, err.Field, "Error should specify the field")
					assert.NotEmpty(t, err.Message, "Error should have a message")
				}
			}
		})
	}
}

func TestConfigValidator_ValidateJIRABaseURL(t *testing.T) {
	validator := &ConfigValidator{}

	tests := []struct {
		name        string
		baseURL     string
		expectError bool
	}{
		{
			name:        "Valid HTTPS URL",
			baseURL:     "https://company.atlassian.net",
			expectError: false,
		},
		{
			name:        "Valid HTTPS URL with path",
			baseURL:     "https://company.atlassian.net/jira",
			expectError: false,
		},
		{
			name:        "Invalid HTTP URL",
			baseURL:     "http://company.atlassian.net",
			expectError: true,
		},
		{
			name:        "Invalid empty URL",
			baseURL:     "",
			expectError: true,
		},
		{
			name:        "Invalid malformed URL",
			baseURL:     "not-a-url",
			expectError: true,
		},
		{
			name:        "Invalid URL without host",
			baseURL:     "https://",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.validateJIRABaseURL(tt.baseURL)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConfigValidator_ValidateEmail(t *testing.T) {
	validator := &ConfigValidator{}

	tests := []struct {
		name        string
		email       string
		expectError bool
	}{
		{
			name:        "Valid email",
			email:       "user@company.com",
			expectError: false,
		},
		{
			name:        "Valid email with subdomain",
			email:       "user@mail.company.com",
			expectError: false,
		},
		{
			name:        "Invalid empty email",
			email:       "",
			expectError: true,
		},
		{
			name:        "Invalid email without @",
			email:       "usercompany.com",
			expectError: true,
		},
		{
			name:        "Invalid email without domain",
			email:       "user@",
			expectError: true,
		},
		{
			name:        "Invalid email without user",
			email:       "@company.com",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.validateEmail(tt.email)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConfigValidator_ValidationError(t *testing.T) {
	err := ValidationError{
		Field:   "test.field",
		Message: "test message",
		Value:   "test value",
	}

	expected := "validation failed for field 'test.field': test message (value: test value)"
	assert.Equal(t, expected, err.Error())
}
