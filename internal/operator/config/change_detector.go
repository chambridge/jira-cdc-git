package config

import (
	"crypto/sha256"
	"fmt"
	"reflect"
	"time"

	"github.com/go-logr/logr"

	operatortypes "github.com/chambrid/jira-cdc-git/internal/operator/types"
)

// ChangeDetector detects configuration changes requiring API server updates
type ChangeDetector struct {
	log logr.Logger
}

// NewChangeDetector creates a new configuration change detector
func NewChangeDetector(log logr.Logger) *ChangeDetector {
	return &ChangeDetector{
		log: log,
	}
}

// ChangeResult represents the result of change detection
type ChangeResult struct {
	HasChanges           bool
	RequiresUpdate       bool
	RequiresRestart      bool
	ConfigChanges        []ConfigChange
	PreviousHash         string
	CurrentHash          string
	ChangeTimestamp      time.Time
	UpdateRecommendation string
}

// ConfigChange represents a specific configuration change
type ConfigChange struct {
	Field       string
	FieldPath   string
	OldValue    interface{}
	NewValue    interface{}
	Impact      ChangeImpact
	Description string
}

// ChangeImpact represents the impact level of a configuration change
type ChangeImpact string

const (
	ImpactLow      ChangeImpact = "low"      // Hot reload possible
	ImpactMedium   ChangeImpact = "medium"   // Rolling update required
	ImpactHigh     ChangeImpact = "high"     // Full restart required
	ImpactCritical ChangeImpact = "critical" // Service disruption expected
)

// DetectChanges detects configuration changes between previous and current APIServer spec
func (cd *ChangeDetector) DetectChanges(previous, current *operatortypes.APIServerSpec, previousGeneration, currentGeneration int64) *ChangeResult {
	log := cd.log.WithValues("previousGeneration", previousGeneration, "currentGeneration", currentGeneration)

	result := &ChangeResult{
		ConfigChanges:   []ConfigChange{},
		PreviousHash:    cd.generateSpecHash(previous),
		CurrentHash:     cd.generateSpecHash(current),
		ChangeTimestamp: time.Now(),
	}

	// Quick hash comparison
	if result.PreviousHash == result.CurrentHash {
		result.HasChanges = false
		result.UpdateRecommendation = "No configuration changes detected"
		log.V(1).Info("No configuration changes detected")
		return result
	}

	result.HasChanges = true
	log.Info("Configuration changes detected", "previousHash", result.PreviousHash, "currentHash", result.CurrentHash)

	// Detect specific changes
	cd.detectJIRACredentialsChanges(previous, current, result)
	cd.detectImageChanges(previous, current, result)
	cd.detectReplicaChanges(previous, current, result)
	cd.detectResourceChanges(previous, current, result)
	cd.detectConfigChanges(previous, current, result)
	cd.detectServiceChanges(previous, current, result)

	// Determine overall update requirements
	cd.determineUpdateRequirements(result)

	log.Info("Change detection completed",
		"totalChanges", len(result.ConfigChanges),
		"requiresUpdate", result.RequiresUpdate,
		"requiresRestart", result.RequiresRestart)

	return result
}

// detectJIRACredentialsChanges detects changes in JIRA credentials configuration
func (cd *ChangeDetector) detectJIRACredentialsChanges(previous, current *operatortypes.APIServerSpec, result *ChangeResult) {
	if !reflect.DeepEqual(previous.JIRACredentials, current.JIRACredentials) {
		change := ConfigChange{
			Field:       "jiraCredentials",
			FieldPath:   "spec.jiraCredentials",
			OldValue:    previous.JIRACredentials,
			NewValue:    current.JIRACredentials,
			Impact:      ImpactHigh,
			Description: "JIRA credentials configuration changed - requires pod restart to reload credentials",
		}
		result.ConfigChanges = append(result.ConfigChanges, change)

		// Check specific credential changes
		if previous.JIRACredentials.SecretRef.Name != current.JIRACredentials.SecretRef.Name {
			secretChange := ConfigChange{
				Field:       "jiraCredentials.secretRef.name",
				FieldPath:   "spec.jiraCredentials.secretRef.name",
				OldValue:    previous.JIRACredentials.SecretRef.Name,
				NewValue:    current.JIRACredentials.SecretRef.Name,
				Impact:      ImpactCritical,
				Description: "JIRA credentials secret reference changed - requires immediate restart",
			}
			result.ConfigChanges = append(result.ConfigChanges, secretChange)
		}
	}
}

// detectImageChanges detects changes in container image configuration
func (cd *ChangeDetector) detectImageChanges(previous, current *operatortypes.APIServerSpec, result *ChangeResult) {
	if !reflect.DeepEqual(previous.Image, current.Image) {
		if previous.Image.Repository != current.Image.Repository {
			change := ConfigChange{
				Field:       "image.repository",
				FieldPath:   "spec.image.repository",
				OldValue:    previous.Image.Repository,
				NewValue:    current.Image.Repository,
				Impact:      ImpactHigh,
				Description: "Container image repository changed - requires rolling update",
			}
			result.ConfigChanges = append(result.ConfigChanges, change)
		}

		if previous.Image.Tag != current.Image.Tag {
			change := ConfigChange{
				Field:       "image.tag",
				FieldPath:   "spec.image.tag",
				OldValue:    previous.Image.Tag,
				NewValue:    current.Image.Tag,
				Impact:      ImpactMedium,
				Description: "Container image tag changed - rolling update recommended",
			}
			result.ConfigChanges = append(result.ConfigChanges, change)
		}

		if previous.Image.PullPolicy != current.Image.PullPolicy {
			change := ConfigChange{
				Field:       "image.pullPolicy",
				FieldPath:   "spec.image.pullPolicy",
				OldValue:    previous.Image.PullPolicy,
				NewValue:    current.Image.PullPolicy,
				Impact:      ImpactLow,
				Description: "Image pull policy changed - will take effect on next pod restart",
			}
			result.ConfigChanges = append(result.ConfigChanges, change)
		}
	}
}

// detectReplicaChanges detects changes in replica count
func (cd *ChangeDetector) detectReplicaChanges(previous, current *operatortypes.APIServerSpec, result *ChangeResult) {
	prevReplicas := cd.getReplicaValue(previous.Replicas)
	currReplicas := cd.getReplicaValue(current.Replicas)

	if prevReplicas != currReplicas {
		impact := ImpactLow
		if currReplicas < prevReplicas {
			impact = ImpactMedium // Scaling down has more impact
		}

		change := ConfigChange{
			Field:       "replicas",
			FieldPath:   "spec.replicas",
			OldValue:    prevReplicas,
			NewValue:    currReplicas,
			Impact:      impact,
			Description: fmt.Sprintf("Replica count changed from %d to %d - will trigger scaling", prevReplicas, currReplicas),
		}
		result.ConfigChanges = append(result.ConfigChanges, change)
	}
}

// detectResourceChanges detects changes in resource requirements
func (cd *ChangeDetector) detectResourceChanges(previous, current *operatortypes.APIServerSpec, result *ChangeResult) {
	if !reflect.DeepEqual(previous.Resources, current.Resources) {
		change := ConfigChange{
			Field:       "resources",
			FieldPath:   "spec.resources",
			OldValue:    previous.Resources,
			NewValue:    current.Resources,
			Impact:      ImpactMedium,
			Description: "Resource requirements changed - rolling update required",
		}
		result.ConfigChanges = append(result.ConfigChanges, change)

		// Detect specific resource changes
		cd.detectSpecificResourceChanges(previous.Resources, current.Resources, result)
	}
}

// detectConfigChanges detects changes in API server configuration
func (cd *ChangeDetector) detectConfigChanges(previous, current *operatortypes.APIServerSpec, result *ChangeResult) {
	prevConfig := cd.getConfigValue(previous.Config)
	currConfig := cd.getConfigValue(current.Config)

	if !reflect.DeepEqual(prevConfig, currConfig) {
		// Log level changes
		if prevConfig.LogLevel != currConfig.LogLevel {
			change := ConfigChange{
				Field:       "config.logLevel",
				FieldPath:   "spec.config.logLevel",
				OldValue:    prevConfig.LogLevel,
				NewValue:    currConfig.LogLevel,
				Impact:      ImpactLow,
				Description: "Log level changed - can be updated via ConfigMap without restart",
			}
			result.ConfigChanges = append(result.ConfigChanges, change)
		}

		// Log format changes
		if prevConfig.LogFormat != currConfig.LogFormat {
			change := ConfigChange{
				Field:       "config.logFormat",
				FieldPath:   "spec.config.logFormat",
				OldValue:    prevConfig.LogFormat,
				NewValue:    currConfig.LogFormat,
				Impact:      ImpactLow,
				Description: "Log format changed - can be updated via ConfigMap without restart",
			}
			result.ConfigChanges = append(result.ConfigChanges, change)
		}

		// Port changes
		prevPort := cd.getPortValue(prevConfig.Port)
		currPort := cd.getPortValue(currConfig.Port)
		if prevPort != currPort {
			change := ConfigChange{
				Field:       "config.port",
				FieldPath:   "spec.config.port",
				OldValue:    prevPort,
				NewValue:    currPort,
				Impact:      ImpactHigh,
				Description: "API server port changed - requires pod restart and service update",
			}
			result.ConfigChanges = append(result.ConfigChanges, change)
		}

		// Job configuration changes
		prevEnableJobs := cd.getEnableJobsValue(prevConfig.EnableJobs)
		currEnableJobs := cd.getEnableJobsValue(currConfig.EnableJobs)
		if prevEnableJobs != currEnableJobs {
			change := ConfigChange{
				Field:       "config.enableJobs",
				FieldPath:   "spec.config.enableJobs",
				OldValue:    prevEnableJobs,
				NewValue:    currEnableJobs,
				Impact:      ImpactMedium,
				Description: "Job creation setting changed - requires restart to reload permissions",
			}
			result.ConfigChanges = append(result.ConfigChanges, change)
		}

		// Job image changes
		if prevConfig.JobImage != currConfig.JobImage {
			change := ConfigChange{
				Field:       "config.jobImage",
				FieldPath:   "spec.config.jobImage",
				OldValue:    prevConfig.JobImage,
				NewValue:    currConfig.JobImage,
				Impact:      ImpactLow,
				Description: "Job image changed - will affect new job creation only",
			}
			result.ConfigChanges = append(result.ConfigChanges, change)
		}

		// Safe mode changes
		prevSafeMode := cd.getSafeModeValue(prevConfig.SafeModeEnabled)
		currSafeMode := cd.getSafeModeValue(currConfig.SafeModeEnabled)
		if prevSafeMode != currSafeMode {
			change := ConfigChange{
				Field:       "config.safeModeEnabled",
				FieldPath:   "spec.config.safeModeEnabled",
				OldValue:    prevSafeMode,
				NewValue:    currSafeMode,
				Impact:      ImpactMedium,
				Description: "Safe mode setting changed - requires restart to take effect",
			}
			result.ConfigChanges = append(result.ConfigChanges, change)
		}
	}
}

// detectServiceChanges detects changes in service configuration
func (cd *ChangeDetector) detectServiceChanges(previous, current *operatortypes.APIServerSpec, result *ChangeResult) {
	prevService := cd.getServiceValue(previous.Service)
	currService := cd.getServiceValue(current.Service)

	if !reflect.DeepEqual(prevService, currService) {
		// Service type changes
		if prevService.Type != currService.Type {
			change := ConfigChange{
				Field:       "service.type",
				FieldPath:   "spec.service.type",
				OldValue:    prevService.Type,
				NewValue:    currService.Type,
				Impact:      ImpactMedium,
				Description: "Service type changed - will trigger service update",
			}
			result.ConfigChanges = append(result.ConfigChanges, change)
		}

		// Service port changes
		prevPort := cd.getServicePortValue(prevService.Port)
		currPort := cd.getServicePortValue(currService.Port)
		if prevPort != currPort {
			change := ConfigChange{
				Field:       "service.port",
				FieldPath:   "spec.service.port",
				OldValue:    prevPort,
				NewValue:    currPort,
				Impact:      ImpactLow,
				Description: "Service port changed - will trigger service update",
			}
			result.ConfigChanges = append(result.ConfigChanges, change)
		}
	}
}

// detectSpecificResourceChanges provides detailed resource change detection
func (cd *ChangeDetector) detectSpecificResourceChanges(previous, current *operatortypes.ResourceRequirements, result *ChangeResult) {
	// This could be expanded with specific CPU/memory change detection
	// For now, we handle it as a general resource change
}

// determineUpdateRequirements analyzes changes to determine update strategy
func (cd *ChangeDetector) determineUpdateRequirements(result *ChangeResult) {
	if len(result.ConfigChanges) == 0 {
		result.RequiresUpdate = false
		result.RequiresRestart = false
		result.UpdateRecommendation = "No updates required"
		return
	}

	result.RequiresUpdate = true
	maxImpact := ImpactLow

	for _, change := range result.ConfigChanges {
		if cd.impactLevel(change.Impact) > cd.impactLevel(maxImpact) {
			maxImpact = change.Impact
		}

		switch change.Impact {
		case ImpactLow:
			// recommendations = append(recommendations, fmt.Sprintf("Update %s via ConfigMap", change.Field))
		case ImpactMedium:
			// recommendations = append(recommendations, fmt.Sprintf("Rolling update required for %s", change.Field))
		case ImpactHigh:
			// recommendations = append(recommendations, fmt.Sprintf("Pod restart required for %s", change.Field))
		case ImpactCritical:
			// recommendations = append(recommendations, fmt.Sprintf("Immediate restart required for %s", change.Field))
		}
	}

	// Determine restart requirement
	result.RequiresRestart = maxImpact == ImpactHigh || maxImpact == ImpactCritical

	// Generate comprehensive recommendation
	switch maxImpact {
	case ImpactCritical:
		result.UpdateRecommendation = "Critical configuration changes detected - immediate restart required"
	case ImpactHigh:
		result.UpdateRecommendation = "High-impact configuration changes detected - rolling restart required"
	case ImpactMedium:
		result.UpdateRecommendation = "Medium-impact configuration changes detected - rolling update recommended"
	default:
		result.UpdateRecommendation = "Low-impact configuration changes detected - ConfigMap update sufficient"
	}
}

// Helper methods

func (cd *ChangeDetector) generateSpecHash(spec *operatortypes.APIServerSpec) string {
	if spec == nil {
		return ""
	}

	// Create a deterministic representation of the spec
	// by converting to a consistent string format
	hasher := sha256.New()

	// Hash JIRA credentials
	_, _ = fmt.Fprintf(hasher, "jira:%s", spec.JIRACredentials.SecretRef.Name)

	// Hash image spec
	_, _ = fmt.Fprintf(hasher, "image:%s:%s:%s", spec.Image.Repository, spec.Image.Tag, spec.Image.PullPolicy)

	// Hash replicas (with default value handling)
	replicas := cd.getReplicaValue(spec.Replicas)
	_, _ = fmt.Fprintf(hasher, "replicas:%d", replicas)

	// Hash resources if present
	if spec.Resources != nil {
		if spec.Resources.Requests != nil {
			_, _ = fmt.Fprintf(hasher, "req-cpu:%s,req-mem:%s", spec.Resources.Requests.CPU, spec.Resources.Requests.Memory)
		}
		if spec.Resources.Limits != nil {
			_, _ = fmt.Fprintf(hasher, "lim-cpu:%s,lim-mem:%s", spec.Resources.Limits.CPU, spec.Resources.Limits.Memory)
		}
	}

	// Hash config values (with default value handling)
	config := cd.getConfigValue(spec.Config)
	logLevel := config.LogLevel
	if logLevel == "" {
		logLevel = "INFO"
	}
	logFormat := config.LogFormat
	if logFormat == "" {
		logFormat = "json"
	}
	port := cd.getPortValue(config.Port)
	enableJobs := cd.getEnableJobsValue(config.EnableJobs)
	safeModeEnabled := cd.getSafeModeValue(config.SafeModeEnabled)
	jobImage := config.JobImage

	_, _ = fmt.Fprintf(hasher, "config:%s:%s:%d:%t:%t:%s",
		logLevel, logFormat, port, enableJobs, safeModeEnabled, jobImage)

	// Hash service values (with default value handling)
	service := cd.getServiceValue(spec.Service)
	serviceType := service.Type
	if serviceType == "" {
		serviceType = "ClusterIP"
	}
	servicePort := cd.getServicePortValue(service.Port)
	_, _ = fmt.Fprintf(hasher, "service:%s:%d", serviceType, servicePort)

	return fmt.Sprintf("%x", hasher.Sum(nil))[:16]
}

func (cd *ChangeDetector) impactLevel(impact ChangeImpact) int {
	switch impact {
	case ImpactLow:
		return 1
	case ImpactMedium:
		return 2
	case ImpactHigh:
		return 3
	case ImpactCritical:
		return 4
	default:
		return 0
	}
}

func (cd *ChangeDetector) getReplicaValue(replicas *int32) int32 {
	if replicas != nil {
		return *replicas
	}
	return 2 // Default value
}

func (cd *ChangeDetector) getConfigValue(config *operatortypes.APIServerConfig) operatortypes.APIServerConfig {
	if config != nil {
		return *config
	}
	return operatortypes.APIServerConfig{} // Default empty config
}

func (cd *ChangeDetector) getServiceValue(service *operatortypes.ServiceConfig) operatortypes.ServiceConfig {
	if service != nil {
		return *service
	}
	return operatortypes.ServiceConfig{} // Default empty service config
}

func (cd *ChangeDetector) getPortValue(port *int32) int32 {
	if port != nil {
		return *port
	}
	return 8080 // Default port
}

func (cd *ChangeDetector) getServicePortValue(port *int32) int32 {
	if port != nil {
		return *port
	}
	return 80 // Default service port
}

func (cd *ChangeDetector) getEnableJobsValue(value *bool) bool {
	if value != nil {
		return *value
	}
	return true // Default true to match DefaultEnableJobs
}

func (cd *ChangeDetector) getSafeModeValue(value *bool) bool {
	if value != nil {
		return *value
	}
	return false // Default false to match DefaultSafeModeEnabled
}
