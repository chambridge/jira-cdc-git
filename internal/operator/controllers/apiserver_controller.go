package controllers

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	operatortypes "github.com/chambrid/jira-cdc-git/internal/operator/types"
)

// APIServerReconciler reconciles an APIServer object
type APIServerReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Log    logr.Logger
}

const (
	// Phase constants for APIServer
	APIServerPhasePending  = "Pending"
	APIServerPhaseCreating = "Creating"
	APIServerPhaseRunning  = "Running"
	APIServerPhaseFailed   = "Failed"
	APIServerPhaseDeleting = "Deleting"

	// Finalizer for APIServer
	APIServerFinalizer = "sync.jira.io/apiserver-finalizer"

	// Default configuration
	DefaultAPIServerPort   = 8080
	DefaultServicePort     = 80
	DefaultReplicas        = 2
	DefaultLogLevel        = "INFO"
	DefaultLogFormat       = "json"
	DefaultEnableJobs      = true
	DefaultSafeModeEnabled = false
	DefaultImagePullPolicy = "IfNotPresent"
	DefaultServiceType     = "ClusterIP"
	DefaultRequestsCPU     = "100m"
	DefaultRequestsMemory  = "128Mi"
	DefaultLimitsCPU       = "500m"
	DefaultLimitsMemory    = "512Mi"
)

// +kubebuilder:rbac:groups=sync.jira.io,resources=apiservers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=sync.jira.io,resources=apiservers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=sync.jira.io,resources=apiservers/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

// NewAPIServerReconciler creates a new APIServerReconciler
func NewAPIServerReconciler(mgr ctrl.Manager) *APIServerReconciler {
	return &APIServerReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
		Log:    ctrl.Log.WithName("controllers").WithName("APIServer"),
	}
}

// Reconcile handles APIServer reconciliation
func (r *APIServerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("apiserver", req.NamespacedName)

	// Fetch the APIServer instance
	apiServer := &operatortypes.APIServer{}
	err := r.Get(ctx, req.NamespacedName, apiServer)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("APIServer resource not found, ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get APIServer")
		return ctrl.Result{}, err
	}

	// Handle deletion
	if apiServer.DeletionTimestamp != nil {
		return r.handleDeletion(ctx, apiServer, log)
	}

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(apiServer, APIServerFinalizer) {
		controllerutil.AddFinalizer(apiServer, APIServerFinalizer)
		if err := r.Update(ctx, apiServer); err != nil {
			log.Error(err, "Failed to add finalizer")
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Update phase to Creating if Pending
	if apiServer.Status.Phase == "" || apiServer.Status.Phase == APIServerPhasePending {
		apiServer.Status.Phase = APIServerPhaseCreating
		if err := r.Status().Update(ctx, apiServer); err != nil {
			log.Error(err, "Failed to update status to Creating")
			return ctrl.Result{}, err
		}
	}

	// Reconcile ConfigMap first
	configMap, err := r.reconcileConfigMap(ctx, apiServer, log)
	if err != nil {
		log.Error(err, "Failed to reconcile ConfigMap")
		r.updateStatusFailed(ctx, apiServer, "ConfigMapFailed", err.Error())
		return ctrl.Result{RequeueAfter: 30 * time.Second}, err
	}

	// Reconcile Deployment
	deployment, err := r.reconcileDeployment(ctx, apiServer, configMap, log)
	if err != nil {
		log.Error(err, "Failed to reconcile Deployment")
		r.updateStatusFailed(ctx, apiServer, "DeploymentFailed", err.Error())
		return ctrl.Result{RequeueAfter: 30 * time.Second}, err
	}

	// Reconcile Service
	service, err := r.reconcileService(ctx, apiServer, log)
	if err != nil {
		log.Error(err, "Failed to reconcile Service")
		r.updateStatusFailed(ctx, apiServer, "ServiceFailed", err.Error())
		return ctrl.Result{RequeueAfter: 30 * time.Second}, err
	}

	// Update status based on deployment readiness
	err = r.updateStatus(ctx, apiServer, deployment, service, log)
	if err != nil {
		log.Error(err, "Failed to update status")
		return ctrl.Result{RequeueAfter: 10 * time.Second}, err
	}

	// Perform health check if running
	if apiServer.Status.Phase == APIServerPhaseRunning {
		err = r.performHealthCheck(ctx, apiServer, log)
		if err != nil {
			log.Error(err, "Health check failed")
			// Don't fail reconciliation on health check failure, just log it
		}
	}

	// Requeue to check status periodically
	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

// handleDeletion handles the deletion of APIServer resources
func (r *APIServerReconciler) handleDeletion(ctx context.Context, apiServer *operatortypes.APIServer, log logr.Logger) (ctrl.Result, error) {
	log.Info("Handling APIServer deletion")

	// Update phase to Deleting
	if apiServer.Status.Phase != APIServerPhaseDeleting {
		apiServer.Status.Phase = APIServerPhaseDeleting
		if err := r.Status().Update(ctx, apiServer); err != nil {
			log.Error(err, "Failed to update status to Deleting")
		}
	}

	// Clean up owned resources (Deployment, Service, ConfigMap)
	// These will be automatically cleaned up by Kubernetes ownership, but we can be explicit

	// Remove finalizer
	controllerutil.RemoveFinalizer(apiServer, APIServerFinalizer)
	if err := r.Update(ctx, apiServer); err != nil {
		log.Error(err, "Failed to remove finalizer")
		return ctrl.Result{}, err
	}

	log.Info("APIServer deletion completed")
	return ctrl.Result{}, nil
}

// reconcileConfigMap creates or updates the ConfigMap for API server configuration
func (r *APIServerReconciler) reconcileConfigMap(ctx context.Context, apiServer *operatortypes.APIServer, log logr.Logger) (*corev1.ConfigMap, error) {
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.getConfigMapName(apiServer),
			Namespace: apiServer.Namespace,
		},
	}

	op, err := controllerutil.CreateOrUpdate(ctx, r.Client, configMap, func() error {
		// Set owner reference
		if err := controllerutil.SetControllerReference(apiServer, configMap, r.Scheme); err != nil {
			return err
		}

		// Build configuration data
		configData := r.buildConfigMapData(apiServer)
		configMap.Data = configData

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to reconcile ConfigMap: %w", err)
	}

	log.Info("ConfigMap reconciled", "operation", op, "name", configMap.Name)
	return configMap, nil
}

// reconcileDeployment creates or updates the Deployment for the API server
func (r *APIServerReconciler) reconcileDeployment(ctx context.Context, apiServer *operatortypes.APIServer, configMap *corev1.ConfigMap, log logr.Logger) (*appsv1.Deployment, error) {
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.getDeploymentName(apiServer),
			Namespace: apiServer.Namespace,
		},
	}

	op, err := controllerutil.CreateOrUpdate(ctx, r.Client, deployment, func() error {
		// Set owner reference
		if err := controllerutil.SetControllerReference(apiServer, deployment, r.Scheme); err != nil {
			return err
		}

		// Build deployment spec
		return r.buildDeploymentSpec(apiServer, configMap, deployment)
	})

	if err != nil {
		return nil, fmt.Errorf("failed to reconcile Deployment: %w", err)
	}

	log.Info("Deployment reconciled", "operation", op, "name", deployment.Name)
	return deployment, nil
}

// reconcileService creates or updates the Service for the API server
func (r *APIServerReconciler) reconcileService(ctx context.Context, apiServer *operatortypes.APIServer, log logr.Logger) (*corev1.Service, error) {
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.getServiceName(apiServer),
			Namespace: apiServer.Namespace,
		},
	}

	op, err := controllerutil.CreateOrUpdate(ctx, r.Client, service, func() error {
		// Set owner reference
		if err := controllerutil.SetControllerReference(apiServer, service, r.Scheme); err != nil {
			return err
		}

		// Build service spec
		return r.buildServiceSpec(apiServer, service)
	})

	if err != nil {
		return nil, fmt.Errorf("failed to reconcile Service: %w", err)
	}

	log.Info("Service reconciled", "operation", op, "name", service.Name)
	return service, nil
}

// Helper functions for resource names
func (r *APIServerReconciler) getDeploymentName(apiServer *operatortypes.APIServer) string {
	return fmt.Sprintf("%s-api", apiServer.Name)
}

func (r *APIServerReconciler) getServiceName(apiServer *operatortypes.APIServer) string {
	return fmt.Sprintf("%s-api", apiServer.Name)
}

func (r *APIServerReconciler) getConfigMapName(apiServer *operatortypes.APIServer) string {
	return fmt.Sprintf("%s-api-config", apiServer.Name)
}

// buildConfigMapData builds the configuration data for the API server
func (r *APIServerReconciler) buildConfigMapData(apiServer *operatortypes.APIServer) map[string]string {
	config := map[string]string{
		"LOG_LEVEL":  r.getLogLevel(apiServer),
		"LOG_FORMAT": r.getLogFormat(apiServer),
		"API_PORT":   fmt.Sprintf("%d", r.getAPIPort(apiServer)),
		"API_HOST":   "0.0.0.0",
	}

	if r.getEnableJobs(apiServer) {
		config["ENABLE_JOBS"] = "true"
		config["KUBERNETES_NAMESPACE"] = apiServer.Namespace
		if jobImage := r.getJobImage(apiServer); jobImage != "" {
			config["JOB_IMAGE"] = jobImage
		}
	}

	if r.getSafeModeEnabled(apiServer) {
		config["SPIKE_SAFE_MODE"] = "true"
	}

	return config
}

// buildDeploymentSpec builds the deployment specification
func (r *APIServerReconciler) buildDeploymentSpec(apiServer *operatortypes.APIServer, configMap *corev1.ConfigMap, deployment *appsv1.Deployment) error {
	replicas := r.getReplicas(apiServer)
	labels := r.getLabels(apiServer)

	deployment.Spec = appsv1.DeploymentSpec{
		Replicas: &replicas,
		Strategy: appsv1.DeploymentStrategy{
			Type: appsv1.RollingUpdateDeploymentStrategyType,
			RollingUpdate: &appsv1.RollingUpdateDeployment{
				MaxUnavailable: &intstr.IntOrString{Type: intstr.Int, IntVal: 1},
				MaxSurge:       &intstr.IntOrString{Type: intstr.Int, IntVal: 1},
			},
		},
		Selector: &metav1.LabelSelector{
			MatchLabels: labels,
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: labels,
				Annotations: map[string]string{
					"config-hash": r.getConfigHash(configMap),
				},
			},
			Spec: corev1.PodSpec{
				SecurityContext: &corev1.PodSecurityContext{
					RunAsNonRoot: &[]bool{true}[0],
					RunAsUser:    &[]int64{1000}[0],
					FSGroup:      &[]int64{1000}[0],
				},
				Containers: []corev1.Container{
					{
						Name:            "api-server",
						Image:           r.getImage(apiServer),
						ImagePullPolicy: corev1.PullPolicy(r.getImagePullPolicy(apiServer)),
						Command:         []string{"/bin/api-server"},
						Args:            r.getContainerArgs(apiServer),
						Ports: []corev1.ContainerPort{
							{
								Name:          "http",
								ContainerPort: r.getAPIPort(apiServer),
								Protocol:      corev1.ProtocolTCP,
							},
						},
						Env:            r.getContainerEnv(apiServer),
						Resources:      r.getResources(apiServer),
						LivenessProbe:  r.getLivenessProbe(apiServer),
						ReadinessProbe: r.getReadinessProbe(apiServer),
						VolumeMounts:   r.getVolumeMounts(apiServer),
					},
				},
				Volumes: r.getVolumes(apiServer, configMap),
			},
		},
	}

	return nil
}

// buildServiceSpec builds the service specification
func (r *APIServerReconciler) buildServiceSpec(apiServer *operatortypes.APIServer, service *corev1.Service) error {
	labels := r.getLabels(apiServer)
	servicePort := r.getServicePort(apiServer)
	apiPort := r.getAPIPort(apiServer)

	service.Spec = corev1.ServiceSpec{
		Type:     corev1.ServiceType(r.getServiceType(apiServer)),
		Selector: labels,
		Ports: []corev1.ServicePort{
			{
				Name:       "http",
				Port:       servicePort,
				TargetPort: intstr.FromInt(int(apiPort)),
				Protocol:   corev1.ProtocolTCP,
			},
		},
	}

	// Add annotations if specified
	if apiServer.Spec.Service != nil && apiServer.Spec.Service.Annotations != nil {
		if service.Annotations == nil {
			service.Annotations = make(map[string]string)
		}
		for k, v := range apiServer.Spec.Service.Annotations {
			service.Annotations[k] = v
		}
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager
func (r *APIServerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&operatortypes.APIServer{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.ConfigMap{}).
		Complete(r)
}
