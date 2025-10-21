package controllers

import (
	"context"
	"crypto/md5"
	"fmt"
	"net/http"
	"time"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	operatortypes "github.com/chambrid/jira-cdc-git/internal/operator/types"
)

// Helper functions for configuration values with defaults

func (r *APIServerReconciler) getReplicas(apiServer *operatortypes.APIServer) int32 {
	if apiServer.Spec.Replicas != nil {
		return *apiServer.Spec.Replicas
	}
	return DefaultReplicas
}

func (r *APIServerReconciler) getLogLevel(apiServer *operatortypes.APIServer) string {
	if apiServer.Spec.Config != nil && apiServer.Spec.Config.LogLevel != "" {
		return apiServer.Spec.Config.LogLevel
	}
	return DefaultLogLevel
}

func (r *APIServerReconciler) getLogFormat(apiServer *operatortypes.APIServer) string {
	if apiServer.Spec.Config != nil && apiServer.Spec.Config.LogFormat != "" {
		return apiServer.Spec.Config.LogFormat
	}
	return DefaultLogFormat
}

func (r *APIServerReconciler) getAPIPort(apiServer *operatortypes.APIServer) int32 {
	if apiServer.Spec.Config != nil && apiServer.Spec.Config.Port != nil {
		return *apiServer.Spec.Config.Port
	}
	return DefaultAPIServerPort
}

func (r *APIServerReconciler) getServicePort(apiServer *operatortypes.APIServer) int32 {
	if apiServer.Spec.Service != nil && apiServer.Spec.Service.Port != nil {
		return *apiServer.Spec.Service.Port
	}
	return DefaultServicePort
}

func (r *APIServerReconciler) getServiceType(apiServer *operatortypes.APIServer) string {
	if apiServer.Spec.Service != nil && apiServer.Spec.Service.Type != "" {
		return apiServer.Spec.Service.Type
	}
	return DefaultServiceType
}

func (r *APIServerReconciler) getEnableJobs(apiServer *operatortypes.APIServer) bool {
	if apiServer.Spec.Config != nil && apiServer.Spec.Config.EnableJobs != nil {
		return *apiServer.Spec.Config.EnableJobs
	}
	return DefaultEnableJobs
}

func (r *APIServerReconciler) getSafeModeEnabled(apiServer *operatortypes.APIServer) bool {
	if apiServer.Spec.Config != nil && apiServer.Spec.Config.SafeModeEnabled != nil {
		return *apiServer.Spec.Config.SafeModeEnabled
	}
	return DefaultSafeModeEnabled
}

func (r *APIServerReconciler) getJobImage(apiServer *operatortypes.APIServer) string {
	if apiServer.Spec.Config != nil && apiServer.Spec.Config.JobImage != "" {
		return apiServer.Spec.Config.JobImage
	}
	// Default to the same image as the API server if not specified
	return r.getImage(apiServer)
}

func (r *APIServerReconciler) getImage(apiServer *operatortypes.APIServer) string {
	return fmt.Sprintf("%s:%s", apiServer.Spec.Image.Repository, apiServer.Spec.Image.Tag)
}

func (r *APIServerReconciler) getImagePullPolicy(apiServer *operatortypes.APIServer) string {
	if apiServer.Spec.Image.PullPolicy != "" {
		return apiServer.Spec.Image.PullPolicy
	}
	return DefaultImagePullPolicy
}

func (r *APIServerReconciler) getLabels(apiServer *operatortypes.APIServer) map[string]string {
	return map[string]string{
		"app":                          fmt.Sprintf("%s-api", apiServer.Name),
		"app.kubernetes.io/name":       "jira-sync-api",
		"app.kubernetes.io/instance":   apiServer.Name,
		"app.kubernetes.io/component":  "api-server",
		"app.kubernetes.io/part-of":    "jira-sync",
		"app.kubernetes.io/managed-by": "jira-sync-operator",
		"sync.jira.io/apiserver":       apiServer.Name,
	}
}

func (r *APIServerReconciler) getConfigHash(configMap *corev1.ConfigMap) string {
	hash := md5.New()
	for k, v := range configMap.Data {
		_, _ = fmt.Fprintf(hash, "%s=%s", k, v)
	}
	return fmt.Sprintf("%x", hash.Sum(nil))[:8]
}

func (r *APIServerReconciler) getContainerArgs(apiServer *operatortypes.APIServer) []string {
	args := []string{"serve"}

	if r.getEnableJobs(apiServer) {
		args = append(args, "--enable-jobs")
		args = append(args, fmt.Sprintf("--namespace=%s", apiServer.Namespace))
		if jobImage := r.getJobImage(apiServer); jobImage != "" {
			args = append(args, fmt.Sprintf("--image=%s", jobImage))
		}
	}

	return args
}

func (r *APIServerReconciler) getContainerEnv(apiServer *operatortypes.APIServer) []corev1.EnvVar {
	env := []corev1.EnvVar{
		{
			Name: "KUBERNETES_NAMESPACE",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.namespace",
				},
			},
		},
	}

	// Add JIRA credentials from secret
	secretName := apiServer.Spec.JIRACredentials.SecretRef.Name

	env = append(env, []corev1.EnvVar{
		{
			Name: "JIRA_BASE_URL",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
					Key:                  "base-url",
				},
			},
		},
		{
			Name: "JIRA_EMAIL",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
					Key:                  "email",
				},
			},
		},
		{
			Name: "JIRA_PAT",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
					Key:                  "token",
				},
			},
		},
	}...)

	return env
}

func (r *APIServerReconciler) getResources(apiServer *operatortypes.APIServer) corev1.ResourceRequirements {
	resources := corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(DefaultRequestsCPU),
			corev1.ResourceMemory: resource.MustParse(DefaultRequestsMemory),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(DefaultLimitsCPU),
			corev1.ResourceMemory: resource.MustParse(DefaultLimitsMemory),
		},
	}

	if apiServer.Spec.Resources != nil {
		if apiServer.Spec.Resources.Requests != nil {
			if apiServer.Spec.Resources.Requests.CPU != "" {
				resources.Requests[corev1.ResourceCPU] = resource.MustParse(apiServer.Spec.Resources.Requests.CPU)
			}
			if apiServer.Spec.Resources.Requests.Memory != "" {
				resources.Requests[corev1.ResourceMemory] = resource.MustParse(apiServer.Spec.Resources.Requests.Memory)
			}
		}
		if apiServer.Spec.Resources.Limits != nil {
			if apiServer.Spec.Resources.Limits.CPU != "" {
				resources.Limits[corev1.ResourceCPU] = resource.MustParse(apiServer.Spec.Resources.Limits.CPU)
			}
			if apiServer.Spec.Resources.Limits.Memory != "" {
				resources.Limits[corev1.ResourceMemory] = resource.MustParse(apiServer.Spec.Resources.Limits.Memory)
			}
		}
	}

	return resources
}

func (r *APIServerReconciler) getLivenessProbe(apiServer *operatortypes.APIServer) *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path: "/api/v1/health",
				Port: intstr.FromString("http"),
			},
		},
		InitialDelaySeconds: 30,
		PeriodSeconds:       10,
		TimeoutSeconds:      5,
		FailureThreshold:    3,
	}
}

func (r *APIServerReconciler) getReadinessProbe(apiServer *operatortypes.APIServer) *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path: "/api/v1/health",
				Port: intstr.FromString("http"),
			},
		},
		InitialDelaySeconds: 5,
		PeriodSeconds:       5,
		TimeoutSeconds:      3,
		FailureThreshold:    3,
	}
}

func (r *APIServerReconciler) getVolumeMounts(apiServer *operatortypes.APIServer) []corev1.VolumeMount {
	return []corev1.VolumeMount{
		{
			Name:      "config",
			MountPath: "/etc/jira-sync",
			ReadOnly:  true,
		},
		{
			Name:      "credentials",
			MountPath: "/etc/jira-sync/secrets",
			ReadOnly:  true,
		},
	}
}

func (r *APIServerReconciler) getVolumes(apiServer *operatortypes.APIServer, configMap *corev1.ConfigMap) []corev1.Volume {
	secretName := apiServer.Spec.JIRACredentials.SecretRef.Name

	return []corev1.Volume{
		{
			Name: "config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: configMap.Name,
					},
				},
			},
		},
		{
			Name: "credentials",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName:  secretName,
					DefaultMode: &[]int32{0400}[0],
				},
			},
		},
	}
}

// Status update methods

func (r *APIServerReconciler) updateStatus(ctx context.Context, apiServer *operatortypes.APIServer, deployment *appsv1.Deployment, service *corev1.Service, log logr.Logger) error {
	// Update deployment status
	if deployment.Status.Replicas > 0 {
		apiServer.Status.DeploymentStatus = &operatortypes.DeploymentStatus{
			Replicas:        deployment.Status.Replicas,
			ReadyReplicas:   deployment.Status.ReadyReplicas,
			UpdatedReplicas: deployment.Status.UpdatedReplicas,
		}
	}

	// Update service status
	if service.Spec.ClusterIP != "" {
		ports := make([]operatortypes.ServicePort, len(service.Spec.Ports))
		for i, port := range service.Spec.Ports {
			ports[i] = operatortypes.ServicePort{
				Name:       port.Name,
				Port:       port.Port,
				TargetPort: port.TargetPort.IntVal,
			}
		}
		apiServer.Status.ServiceStatus = &operatortypes.ServiceStatus{
			ClusterIP: service.Spec.ClusterIP,
			Ports:     ports,
		}
	}

	// Update endpoint
	apiServer.Status.Endpoint = fmt.Sprintf("http://%s.%s.svc.cluster.local:%d",
		service.Name, service.Namespace, r.getServicePort(apiServer))

	// Update phase based on deployment readiness
	if deployment.Status.ReadyReplicas == deployment.Status.Replicas && deployment.Status.Replicas > 0 {
		apiServer.Status.Phase = APIServerPhaseRunning
		r.setCondition(apiServer, "Ready", metav1.ConditionTrue, "DeploymentReady", "All replicas are ready")
	} else if deployment.Status.Replicas > 0 {
		apiServer.Status.Phase = APIServerPhaseCreating
		r.setCondition(apiServer, "Ready", metav1.ConditionFalse, "DeploymentNotReady",
			fmt.Sprintf("Ready replicas: %d/%d", deployment.Status.ReadyReplicas, deployment.Status.Replicas))
	} else {
		apiServer.Status.Phase = APIServerPhaseCreating
		r.setCondition(apiServer, "Ready", metav1.ConditionFalse, "DeploymentCreating", "Deployment is being created")
	}

	if err := r.Status().Update(ctx, apiServer); err != nil {
		log.Error(err, "Failed to update APIServer status")
		return err
	}
	return nil
}

func (r *APIServerReconciler) updateStatusFailed(ctx context.Context, apiServer *operatortypes.APIServer, reason, message string) {
	apiServer.Status.Phase = APIServerPhaseFailed
	r.setCondition(apiServer, "Ready", metav1.ConditionFalse, reason, message)
	_ = r.Status().Update(ctx, apiServer)
}

func (r *APIServerReconciler) setCondition(apiServer *operatortypes.APIServer, conditionType string, status metav1.ConditionStatus, reason, message string) {
	condition := metav1.Condition{
		Type:               conditionType,
		Status:             status,
		LastTransitionTime: metav1.NewTime(time.Now()),
		Reason:             reason,
		Message:            message,
	}

	// Find existing condition
	for i, existingCondition := range apiServer.Status.Conditions {
		if existingCondition.Type == conditionType {
			// Update existing condition only if status changed
			if existingCondition.Status != status {
				apiServer.Status.Conditions[i] = condition
			}
			return
		}
	}

	// Add new condition
	apiServer.Status.Conditions = append(apiServer.Status.Conditions, condition)
}

// performHealthCheck performs a health check on the running API server
func (r *APIServerReconciler) performHealthCheck(ctx context.Context, apiServer *operatortypes.APIServer, log logr.Logger) error {
	if apiServer.Status.Endpoint == "" {
		return fmt.Errorf("no endpoint available for health check")
	}

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// Perform health check
	healthURL := fmt.Sprintf("%s/api/v1/health", apiServer.Status.Endpoint)
	resp, err := client.Get(healthURL)

	now := metav1.NewTime(time.Now())
	if err != nil {
		// Health check failed
		apiServer.Status.HealthStatus = &operatortypes.HealthStatus{
			Healthy:   false,
			LastCheck: &now,
			Message:   fmt.Sprintf("Health check failed: %v", err),
		}
		log.Error(err, "Health check failed", "url", healthURL)
		return err
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			log.Error(closeErr, "Failed to close response body")
		}
	}()

	// Check response status
	if resp.StatusCode == http.StatusOK {
		apiServer.Status.HealthStatus = &operatortypes.HealthStatus{
			Healthy:   true,
			LastCheck: &now,
			Message:   "Health check passed",
		}
		log.V(1).Info("Health check passed", "url", healthURL, "status", resp.StatusCode)
	} else {
		apiServer.Status.HealthStatus = &operatortypes.HealthStatus{
			Healthy:   false,
			LastCheck: &now,
			Message:   fmt.Sprintf("Health check returned status %d", resp.StatusCode),
		}
		log.Error(fmt.Errorf("unexpected status code"), "Health check returned non-200 status", "url", healthURL, "status", resp.StatusCode)
		return fmt.Errorf("health check failed with status %d", resp.StatusCode)
	}

	// Update status
	if err := r.Status().Update(ctx, apiServer); err != nil {
		log.Error(err, "Failed to update APIServer health status")
		return err
	}
	return nil
}

// updateStatusCondition updates a specific condition in the APIServer status
func (r *APIServerReconciler) updateStatusCondition(ctx context.Context, apiServer *operatortypes.APIServer, condition metav1.Condition) {
	r.setCondition(apiServer, condition.Type, condition.Status, condition.Reason, condition.Message)
	_ = r.Status().Update(ctx, apiServer)
}
