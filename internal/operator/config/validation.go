package config

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatortypes "github.com/chambrid/jira-cdc-git/internal/operator/types"
)

// ValidationError represents a configuration validation error
type ValidationError struct {
	Field   string
	Message string
	Value   interface{}
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("validation failed for field '%s': %s (value: %v)", e.Field, e.Message, e.Value)
}

// ValidationResult contains the results of configuration validation
type ValidationResult struct {
	Valid  bool
	Errors []ValidationError
}

// ConfigValidator provides configuration validation for APIServer specs
type ConfigValidator struct {
	client client.Client
}

// NewConfigValidator creates a new configuration validator
func NewConfigValidator(client client.Client) *ConfigValidator {
	return &ConfigValidator{
		client: client,
	}
}

// ValidateAPIServerSpec performs comprehensive validation of APIServer specification
func (v *ConfigValidator) ValidateAPIServerSpec(ctx context.Context, spec *operatortypes.APIServerSpec, namespace string) *ValidationResult {
	result := &ValidationResult{
		Valid:  true,
		Errors: []ValidationError{},
	}

	// Validate JIRA credentials
	v.validateJIRACredentials(ctx, &spec.JIRACredentials, namespace, result)

	// Validate image configuration
	v.validateImageSpec(&spec.Image, result)

	// Validate replica configuration
	v.validateReplicas(spec.Replicas, result)

	// Validate resource requirements
	v.validateResources(spec.Resources, result)

	// Validate API server config
	v.validateAPIServerConfig(spec.Config, result)

	// Validate service config
	v.validateServiceConfig(spec.Service, result)

	// Set overall validation status
	result.Valid = len(result.Errors) == 0

	return result
}

// validateJIRACredentials validates JIRA credentials configuration
func (v *ConfigValidator) validateJIRACredentials(ctx context.Context, creds *operatortypes.JIRACredentialsSpec, namespace string, result *ValidationResult) {
	// Validate secret reference
	if creds.SecretRef.Name == "" {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "jiraCredentials.secretRef.name",
			Message: "secret name is required",
			Value:   creds.SecretRef.Name,
		})
		return
	}

	// Validate that the secret exists and has required keys
	secret := &corev1.Secret{}
	err := v.client.Get(ctx, client.ObjectKey{
		Name:      creds.SecretRef.Name,
		Namespace: namespace,
	}, secret)

	if err != nil {
		if apierrors.IsNotFound(err) {
			result.Errors = append(result.Errors, ValidationError{
				Field:   "jiraCredentials.secretRef.name",
				Message: "referenced secret does not exist",
				Value:   creds.SecretRef.Name,
			})
		} else {
			result.Errors = append(result.Errors, ValidationError{
				Field:   "jiraCredentials.secretRef.name",
				Message: fmt.Sprintf("failed to validate secret: %v", err),
				Value:   creds.SecretRef.Name,
			})
		}
		return
	}

	// Validate required secret keys
	requiredKeys := []string{"base-url", "email", "pat"}
	for _, key := range requiredKeys {
		if _, exists := secret.Data[key]; !exists {
			result.Errors = append(result.Errors, ValidationError{
				Field:   fmt.Sprintf("jiraCredentials.secretRef[%s]", key),
				Message: "required secret key is missing",
				Value:   key,
			})
		}
	}

	// Validate JIRA base URL format
	if baseURL, exists := secret.Data["base-url"]; exists {
		if err := v.validateJIRABaseURL(string(baseURL)); err != nil {
			result.Errors = append(result.Errors, ValidationError{
				Field:   "jiraCredentials.secretRef[base-url]",
				Message: err.Error(),
				Value:   string(baseURL),
			})
		}
	}

	// Validate email format
	if email, exists := secret.Data["email"]; exists {
		if err := v.validateEmail(string(email)); err != nil {
			result.Errors = append(result.Errors, ValidationError{
				Field:   "jiraCredentials.secretRef[email]",
				Message: err.Error(),
				Value:   string(email),
			})
		}
	}
}

// validateImageSpec validates container image configuration
func (v *ConfigValidator) validateImageSpec(image *operatortypes.ImageSpec, result *ValidationResult) {
	if image.Repository == "" {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "image.repository",
			Message: "image repository is required",
			Value:   image.Repository,
		})
	}

	if image.Tag == "" {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "image.tag",
			Message: "image tag is required",
			Value:   image.Tag,
		})
	}

	// Validate pull policy
	validPullPolicies := []string{"Always", "IfNotPresent", "Never"}
	if image.PullPolicy != "" {
		valid := false
		for _, policy := range validPullPolicies {
			if image.PullPolicy == policy {
				valid = true
				break
			}
		}
		if !valid {
			result.Errors = append(result.Errors, ValidationError{
				Field:   "image.pullPolicy",
				Message: fmt.Sprintf("invalid pull policy, must be one of: %v", validPullPolicies),
				Value:   image.PullPolicy,
			})
		}
	}
}

// validateReplicas validates replica count configuration
func (v *ConfigValidator) validateReplicas(replicas *int32, result *ValidationResult) {
	if replicas != nil {
		if *replicas < 1 {
			result.Errors = append(result.Errors, ValidationError{
				Field:   "replicas",
				Message: "replica count must be at least 1",
				Value:   *replicas,
			})
		}
		if *replicas > 10 {
			result.Errors = append(result.Errors, ValidationError{
				Field:   "replicas",
				Message: "replica count should not exceed 10 for performance reasons",
				Value:   *replicas,
			})
		}
	}
}

// validateResources validates resource requirements
func (v *ConfigValidator) validateResources(resources *operatortypes.ResourceRequirements, result *ValidationResult) {
	if resources == nil {
		return
	}

	// Validate CPU requests/limits
	if resources.Requests != nil && resources.Requests.CPU != "" {
		if err := v.validateResourceQuantity(resources.Requests.CPU, "CPU"); err != nil {
			result.Errors = append(result.Errors, ValidationError{
				Field:   "resources.requests.cpu",
				Message: err.Error(),
				Value:   resources.Requests.CPU,
			})
		}
	}

	if resources.Limits != nil && resources.Limits.CPU != "" {
		if err := v.validateResourceQuantity(resources.Limits.CPU, "CPU"); err != nil {
			result.Errors = append(result.Errors, ValidationError{
				Field:   "resources.limits.cpu",
				Message: err.Error(),
				Value:   resources.Limits.CPU,
			})
		}
	}

	// Validate memory requests/limits
	if resources.Requests != nil && resources.Requests.Memory != "" {
		if err := v.validateResourceQuantity(resources.Requests.Memory, "memory"); err != nil {
			result.Errors = append(result.Errors, ValidationError{
				Field:   "resources.requests.memory",
				Message: err.Error(),
				Value:   resources.Requests.Memory,
			})
		}
	}

	if resources.Limits != nil && resources.Limits.Memory != "" {
		if err := v.validateResourceQuantity(resources.Limits.Memory, "memory"); err != nil {
			result.Errors = append(result.Errors, ValidationError{
				Field:   "resources.limits.memory",
				Message: err.Error(),
				Value:   resources.Limits.Memory,
			})
		}
	}
}

// validateAPIServerConfig validates API server specific configuration
func (v *ConfigValidator) validateAPIServerConfig(config *operatortypes.APIServerConfig, result *ValidationResult) {
	if config == nil {
		return
	}

	// Validate log level
	if config.LogLevel != "" {
		validLogLevels := []string{"DEBUG", "INFO", "WARN", "ERROR"}
		valid := false
		for _, level := range validLogLevels {
			if strings.ToUpper(config.LogLevel) == level {
				valid = true
				break
			}
		}
		if !valid {
			result.Errors = append(result.Errors, ValidationError{
				Field:   "config.logLevel",
				Message: fmt.Sprintf("invalid log level, must be one of: %v", validLogLevels),
				Value:   config.LogLevel,
			})
		}
	}

	// Validate log format
	if config.LogFormat != "" {
		validLogFormats := []string{"json", "text"}
		valid := false
		for _, format := range validLogFormats {
			if strings.ToLower(config.LogFormat) == format {
				valid = true
				break
			}
		}
		if !valid {
			result.Errors = append(result.Errors, ValidationError{
				Field:   "config.logFormat",
				Message: fmt.Sprintf("invalid log format, must be one of: %v", validLogFormats),
				Value:   config.LogFormat,
			})
		}
	}

	// Validate port
	if config.Port != nil {
		if *config.Port < 1024 || *config.Port > 65535 {
			result.Errors = append(result.Errors, ValidationError{
				Field:   "config.port",
				Message: "port must be between 1024 and 65535",
				Value:   *config.Port,
			})
		}
	}
}

// validateServiceConfig validates service configuration
func (v *ConfigValidator) validateServiceConfig(service *operatortypes.ServiceConfig, result *ValidationResult) {
	if service == nil {
		return
	}

	// Validate service type
	if service.Type != "" {
		validServiceTypes := []string{"ClusterIP", "NodePort", "LoadBalancer"}
		valid := false
		for _, serviceType := range validServiceTypes {
			if service.Type == serviceType {
				valid = true
				break
			}
		}
		if !valid {
			result.Errors = append(result.Errors, ValidationError{
				Field:   "service.type",
				Message: fmt.Sprintf("invalid service type, must be one of: %v", validServiceTypes),
				Value:   service.Type,
			})
		}
	}

	// Validate port
	if service.Port != nil {
		if *service.Port < 1 || *service.Port > 65535 {
			result.Errors = append(result.Errors, ValidationError{
				Field:   "service.port",
				Message: "service port must be between 1 and 65535",
				Value:   *service.Port,
			})
		}
	}
}

// Helper validation functions

func (v *ConfigValidator) validateJIRABaseURL(baseURL string) error {
	if baseURL == "" {
		return fmt.Errorf("JIRA base URL cannot be empty")
	}

	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		return fmt.Errorf("invalid URL format: %v", err)
	}

	if parsedURL.Scheme != "https" {
		return fmt.Errorf("JIRA base URL must use HTTPS protocol")
	}

	if parsedURL.Host == "" {
		return fmt.Errorf("JIRA base URL must include a valid host")
	}

	return nil
}

func (v *ConfigValidator) validateEmail(email string) error {
	if email == "" {
		return fmt.Errorf("email cannot be empty")
	}

	if !strings.Contains(email, "@") {
		return fmt.Errorf("invalid email format")
	}

	parts := strings.Split(email, "@")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return fmt.Errorf("invalid email format")
	}

	return nil
}

func (v *ConfigValidator) validateResourceQuantity(quantity, resourceType string) error {
	if quantity == "" {
		return fmt.Errorf("%s quantity cannot be empty", resourceType)
	}

	// Basic validation - could be enhanced with proper resource.Quantity parsing
	if strings.Contains(quantity, " ") {
		return fmt.Errorf("invalid %s quantity format: cannot contain spaces", resourceType)
	}

	return nil
}
