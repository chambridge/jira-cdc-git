package config

import (
	"context"
	"crypto/sha256"
	"fmt"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatortypes "github.com/chambrid/jira-cdc-git/internal/operator/types"
)

// DriftDetector detects configuration drift between desired and actual state
type DriftDetector struct {
	client client.Client
	log    logr.Logger
}

// NewDriftDetector creates a new configuration drift detector
func NewDriftDetector(client client.Client, log logr.Logger) *DriftDetector {
	return &DriftDetector{
		client: client,
		log:    log,
	}
}

// DriftResult represents the result of drift detection
type DriftResult struct {
	HasDrift        bool
	ConfigMapDrift  *ConfigMapDrift
	DeploymentDrift *DeploymentDrift
	ServiceDrift    *ServiceDrift
	SpecHash        string
	ActualHash      string
	Recommendation  string
}

// ConfigMapDrift represents drift in ConfigMap configuration
type ConfigMapDrift struct {
	Missing         bool
	DataDifferences map[string]DataDifference
}

// DeploymentDrift represents drift in Deployment configuration
type DeploymentDrift struct {
	Missing            bool
	ReplicasDrift      *int32
	ImageDrift         *string
	ResourcesDrift     map[string]interface{}
	EnvironmentDrift   map[string]string
	ConfigHashMismatch bool
}

// ServiceDrift represents drift in Service configuration
type ServiceDrift struct {
	Missing   bool
	PortDrift *int32
	TypeDrift *string
}

// DataDifference represents a difference in configuration data
type DataDifference struct {
	Expected string
	Actual   string
	Type     string // "missing", "extra", "different"
}

// DetectDrift detects configuration drift for an APIServer
func (d *DriftDetector) DetectDrift(ctx context.Context, apiServer *operatortypes.APIServer) (*DriftResult, error) {
	log := d.log.WithValues("apiserver", apiServer.Name, "namespace", apiServer.Namespace)

	result := &DriftResult{
		HasDrift: false,
	}

	// Generate expected configuration hash
	result.SpecHash = d.generateSpecHash(apiServer.Spec)

	// Check ConfigMap drift
	configMapDrift, err := d.detectConfigMapDrift(ctx, apiServer)
	if err != nil {
		return nil, fmt.Errorf("failed to detect ConfigMap drift: %w", err)
	}
	result.ConfigMapDrift = configMapDrift
	if configMapDrift.HasDrift() {
		result.HasDrift = true
		log.Info("ConfigMap drift detected", "differences", len(configMapDrift.DataDifferences))
	}

	// Check Deployment drift
	deploymentDrift, err := d.detectDeploymentDrift(ctx, apiServer)
	if err != nil {
		return nil, fmt.Errorf("failed to detect Deployment drift: %w", err)
	}
	result.DeploymentDrift = deploymentDrift
	if deploymentDrift.HasDrift() {
		result.HasDrift = true
		log.Info("Deployment drift detected")
	}

	// Check Service drift
	serviceDrift, err := d.detectServiceDrift(ctx, apiServer)
	if err != nil {
		return nil, fmt.Errorf("failed to detect Service drift: %w", err)
	}
	result.ServiceDrift = serviceDrift
	if serviceDrift.HasDrift() {
		result.HasDrift = true
		log.Info("Service drift detected")
	}

	// Generate actual configuration hash from live resources
	result.ActualHash = d.generateActualHash(ctx, apiServer)

	// Generate recommendation
	result.Recommendation = d.generateRecommendation(result)

	log.Info("Drift detection completed",
		"hasDrift", result.HasDrift,
		"specHash", result.SpecHash,
		"actualHash", result.ActualHash)

	return result, nil
}

// detectConfigMapDrift detects drift in ConfigMap
func (d *DriftDetector) detectConfigMapDrift(ctx context.Context, apiServer *operatortypes.APIServer) (*ConfigMapDrift, error) {
	configMapName := fmt.Sprintf("%s-config", apiServer.Name)

	// Get actual ConfigMap
	actualConfigMap := &corev1.ConfigMap{}
	err := d.client.Get(ctx, client.ObjectKey{
		Name:      configMapName,
		Namespace: apiServer.Namespace,
	}, actualConfigMap)

	drift := &ConfigMapDrift{
		DataDifferences: make(map[string]DataDifference),
	}

	if err != nil {
		if apierrors.IsNotFound(err) {
			drift.Missing = true
			return drift, nil
		}
		return nil, err
	}

	// Generate expected configuration data
	expectedData := d.buildExpectedConfigMapData(apiServer)

	// Compare expected vs actual
	for key, expectedValue := range expectedData {
		if actualValue, exists := actualConfigMap.Data[key]; exists {
			if actualValue != expectedValue {
				drift.DataDifferences[key] = DataDifference{
					Expected: expectedValue,
					Actual:   actualValue,
					Type:     "different",
				}
			}
		} else {
			drift.DataDifferences[key] = DataDifference{
				Expected: expectedValue,
				Actual:   "",
				Type:     "missing",
			}
		}
	}

	// Check for extra keys in actual ConfigMap
	for key, actualValue := range actualConfigMap.Data {
		if _, exists := expectedData[key]; !exists {
			drift.DataDifferences[key] = DataDifference{
				Expected: "",
				Actual:   actualValue,
				Type:     "extra",
			}
		}
	}

	return drift, nil
}

// detectDeploymentDrift detects drift in Deployment
func (d *DriftDetector) detectDeploymentDrift(ctx context.Context, apiServer *operatortypes.APIServer) (*DeploymentDrift, error) {
	deploymentName := fmt.Sprintf("%s-deployment", apiServer.Name)

	// Get actual Deployment
	actualDeployment := &appsv1.Deployment{}
	err := d.client.Get(ctx, client.ObjectKey{
		Name:      deploymentName,
		Namespace: apiServer.Namespace,
	}, actualDeployment)

	drift := &DeploymentDrift{
		ResourcesDrift:   make(map[string]interface{}),
		EnvironmentDrift: make(map[string]string),
	}

	if err != nil {
		if apierrors.IsNotFound(err) {
			drift.Missing = true
			return drift, nil
		}
		return nil, err
	}

	// Check replicas drift
	expectedReplicas := d.getExpectedReplicas(apiServer)
	if actualDeployment.Spec.Replicas != nil && *actualDeployment.Spec.Replicas != expectedReplicas {
		drift.ReplicasDrift = actualDeployment.Spec.Replicas
	}

	// Check image drift
	if len(actualDeployment.Spec.Template.Spec.Containers) > 0 {
		actualImage := actualDeployment.Spec.Template.Spec.Containers[0].Image
		expectedImage := d.getExpectedImage(apiServer)
		if actualImage != expectedImage {
			drift.ImageDrift = &actualImage
		}

		// Check environment variables drift - only if deployment uses environment variables
		actualEnv := make(map[string]string)
		for _, env := range actualDeployment.Spec.Template.Spec.Containers[0].Env {
			if env.Value != "" {
				actualEnv[env.Name] = env.Value
			}
		}

		// Only check environment drift if the deployment actually has environment variables
		if len(actualEnv) > 0 {
			expectedEnv := d.getExpectedEnvironment(apiServer)
			for key, expectedValue := range expectedEnv {
				if actualValue, exists := actualEnv[key]; exists {
					if actualValue != expectedValue {
						drift.EnvironmentDrift[key] = actualValue
					}
				} else {
					drift.EnvironmentDrift[key] = ""
				}
			}
		}
	}

	// Check for configuration hash annotation drift
	expectedHash := d.generateSpecHash(apiServer.Spec)
	if actualDeployment.Annotations != nil {
		if actualHash, exists := actualDeployment.Annotations["config-hash"]; exists {
			if actualHash != expectedHash {
				drift.ConfigHashMismatch = true
			}
		}
		// If annotation doesn't exist, we don't consider it drift unless other changes detected
	}

	return drift, nil
}

// detectServiceDrift detects drift in Service
func (d *DriftDetector) detectServiceDrift(ctx context.Context, apiServer *operatortypes.APIServer) (*ServiceDrift, error) {
	serviceName := fmt.Sprintf("%s-service", apiServer.Name)

	// Get actual Service
	actualService := &corev1.Service{}
	err := d.client.Get(ctx, client.ObjectKey{
		Name:      serviceName,
		Namespace: apiServer.Namespace,
	}, actualService)

	drift := &ServiceDrift{}

	if err != nil {
		if apierrors.IsNotFound(err) {
			drift.Missing = true
			return drift, nil
		}
		return nil, err
	}

	// Check port drift
	expectedPort := d.getExpectedServicePort(apiServer)
	if len(actualService.Spec.Ports) > 0 {
		actualPort := actualService.Spec.Ports[0].Port
		if actualPort != expectedPort {
			drift.PortDrift = &actualPort
		}
	}

	// Check service type drift
	expectedType := d.getExpectedServiceType(apiServer)
	actualType := string(actualService.Spec.Type)
	if actualType != expectedType {
		drift.TypeDrift = &actualType
	}

	return drift, nil
}

// Helper methods for checking drift

func (cd *ConfigMapDrift) HasDrift() bool {
	return cd.Missing || len(cd.DataDifferences) > 0
}

func (dd *DeploymentDrift) HasDrift() bool {
	return dd.Missing ||
		dd.ReplicasDrift != nil ||
		dd.ImageDrift != nil ||
		len(dd.ResourcesDrift) > 0 ||
		len(dd.EnvironmentDrift) > 0 ||
		dd.ConfigHashMismatch
}

func (sd *ServiceDrift) HasDrift() bool {
	return sd.Missing || sd.PortDrift != nil || sd.TypeDrift != nil
}

// Configuration generation helpers

func (d *DriftDetector) buildExpectedConfigMapData(apiServer *operatortypes.APIServer) map[string]string {
	config := map[string]string{
		"LOG_LEVEL":  d.getExpectedLogLevel(apiServer),
		"LOG_FORMAT": d.getExpectedLogFormat(apiServer),
		"API_PORT":   fmt.Sprintf("%d", d.getExpectedAPIPort(apiServer)),
		"API_HOST":   "0.0.0.0",
	}

	// Add job-related config if enabled (using default if not explicitly set)
	enableJobs := true // Default value
	if apiServer.Spec.Config != nil && apiServer.Spec.Config.EnableJobs != nil {
		enableJobs = *apiServer.Spec.Config.EnableJobs
	}
	if enableJobs {
		config["ENABLE_JOBS"] = "true"
		config["KUBERNETES_NAMESPACE"] = apiServer.Namespace
		if jobImage := d.getExpectedJobImage(apiServer); jobImage != "" {
			config["JOB_IMAGE"] = jobImage
		}
	}

	// Add safe mode config if enabled (using default if not explicitly set)
	safeModeEnabled := false // Default value
	if apiServer.Spec.Config != nil && apiServer.Spec.Config.SafeModeEnabled != nil {
		safeModeEnabled = *apiServer.Spec.Config.SafeModeEnabled
	}
	if safeModeEnabled {
		config["SPIKE_SAFE_MODE"] = "true"
	}

	return config
}

func (d *DriftDetector) getExpectedReplicas(apiServer *operatortypes.APIServer) int32 {
	if apiServer.Spec.Replicas != nil {
		return *apiServer.Spec.Replicas
	}
	return 2 // Default value
}

func (d *DriftDetector) getExpectedImage(apiServer *operatortypes.APIServer) string {
	return fmt.Sprintf("%s:%s", apiServer.Spec.Image.Repository, apiServer.Spec.Image.Tag)
}

func (d *DriftDetector) getExpectedLogLevel(apiServer *operatortypes.APIServer) string {
	if apiServer.Spec.Config != nil && apiServer.Spec.Config.LogLevel != "" {
		return apiServer.Spec.Config.LogLevel
	}
	return "INFO"
}

func (d *DriftDetector) getExpectedLogFormat(apiServer *operatortypes.APIServer) string {
	if apiServer.Spec.Config != nil && apiServer.Spec.Config.LogFormat != "" {
		return apiServer.Spec.Config.LogFormat
	}
	return "json"
}

func (d *DriftDetector) getExpectedAPIPort(apiServer *operatortypes.APIServer) int32 {
	if apiServer.Spec.Config != nil && apiServer.Spec.Config.Port != nil {
		return *apiServer.Spec.Config.Port
	}
	return 8080
}

func (d *DriftDetector) getExpectedServicePort(apiServer *operatortypes.APIServer) int32 {
	if apiServer.Spec.Service != nil && apiServer.Spec.Service.Port != nil {
		return *apiServer.Spec.Service.Port
	}
	return 80
}

func (d *DriftDetector) getExpectedServiceType(apiServer *operatortypes.APIServer) string {
	if apiServer.Spec.Service != nil && apiServer.Spec.Service.Type != "" {
		return apiServer.Spec.Service.Type
	}
	return "ClusterIP"
}

func (d *DriftDetector) getExpectedJobImage(apiServer *operatortypes.APIServer) string {
	if apiServer.Spec.Config != nil && apiServer.Spec.Config.JobImage != "" {
		return apiServer.Spec.Config.JobImage
	}
	return ""
}

func (d *DriftDetector) getExpectedEnvironment(apiServer *operatortypes.APIServer) map[string]string {
	return d.buildExpectedConfigMapData(apiServer)
}

// Hash generation methods

func (d *DriftDetector) generateSpecHash(spec operatortypes.APIServerSpec) string {
	hasher := sha256.New()
	_, _ = fmt.Fprintf(hasher, "%+v", spec)
	return fmt.Sprintf("%x", hasher.Sum(nil))[:16]
}

func (d *DriftDetector) generateActualHash(ctx context.Context, apiServer *operatortypes.APIServer) string {
	actualState := map[string]interface{}{}

	// Get ConfigMap data
	configMapName := fmt.Sprintf("%s-config", apiServer.Name)
	configMap := &corev1.ConfigMap{}
	if err := d.client.Get(ctx, client.ObjectKey{Name: configMapName, Namespace: apiServer.Namespace}, configMap); err == nil {
		actualState["configMap"] = configMap.Data
	}

	// Get Deployment spec
	deploymentName := fmt.Sprintf("%s-deployment", apiServer.Name)
	deployment := &appsv1.Deployment{}
	if err := d.client.Get(ctx, client.ObjectKey{Name: deploymentName, Namespace: apiServer.Namespace}, deployment); err == nil {
		actualState["deployment"] = map[string]interface{}{
			"replicas": deployment.Spec.Replicas,
			"image":    deployment.Spec.Template.Spec.Containers[0].Image,
		}
	}

	// Get Service spec
	serviceName := fmt.Sprintf("%s-service", apiServer.Name)
	service := &corev1.Service{}
	if err := d.client.Get(ctx, client.ObjectKey{Name: serviceName, Namespace: apiServer.Namespace}, service); err == nil {
		actualState["service"] = map[string]interface{}{
			"type":  string(service.Spec.Type),
			"ports": service.Spec.Ports,
		}
	}

	hasher := sha256.New()
	_, _ = fmt.Fprintf(hasher, "%+v", actualState)
	return fmt.Sprintf("%x", hasher.Sum(nil))[:16]
}

func (d *DriftDetector) generateRecommendation(result *DriftResult) string {
	if !result.HasDrift {
		return "No configuration drift detected. All resources are in sync."
	}

	recommendations := []string{}

	if result.ConfigMapDrift.HasDrift() {
		if result.ConfigMapDrift.Missing {
			recommendations = append(recommendations, "ConfigMap is missing and needs to be created")
		} else {
			recommendations = append(recommendations, fmt.Sprintf("ConfigMap has %d configuration differences that need reconciliation", len(result.ConfigMapDrift.DataDifferences)))
		}
	}

	if result.DeploymentDrift.HasDrift() {
		if result.DeploymentDrift.Missing {
			recommendations = append(recommendations, "Deployment is missing and needs to be created")
		} else {
			recommendations = append(recommendations, "Deployment configuration has drifted and needs reconciliation")
		}
	}

	if result.ServiceDrift.HasDrift() {
		if result.ServiceDrift.Missing {
			recommendations = append(recommendations, "Service is missing and needs to be created")
		} else {
			recommendations = append(recommendations, "Service configuration has drifted and needs reconciliation")
		}
	}

	if len(recommendations) == 1 {
		return recommendations[0]
	}

	return fmt.Sprintf("Multiple configuration issues detected: %v", recommendations)
}
