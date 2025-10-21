package config

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	operatortypes "github.com/chambrid/jira-cdc-git/internal/operator/types"
)

func TestDriftDetector_DetectDrift(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = appsv1.AddToScheme(scheme)

	tests := []struct {
		name             string
		apiServer        *operatortypes.APIServer
		existingObjects  []client.Object
		expectedHasDrift bool
		expectedDrifts   []string // List of expected drift types
	}{
		{
			name: "No drift - all resources match specification",
			apiServer: &operatortypes.APIServer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-api-server",
					Namespace: "test-namespace",
				},
				Spec: operatortypes.APIServerSpec{
					JIRACredentials: operatortypes.JIRACredentialsSpec{
						SecretRef: operatortypes.SecretRef{Name: "jira-secret"},
					},
					Image: operatortypes.ImageSpec{
						Repository: "registry.example.com/jira-sync",
						Tag:        "v1.0.0",
					},
					Replicas: &[]int32{2}[0],
					Config: &operatortypes.APIServerConfig{
						LogLevel:  "INFO",
						LogFormat: "json",
						Port:      &[]int32{8080}[0],
					},
				},
			},
			existingObjects: []client.Object{
				&corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-api-server-config",
						Namespace: "test-namespace",
					},
					Data: map[string]string{
						"LOG_LEVEL":            "INFO",
						"LOG_FORMAT":           "json",
						"API_PORT":             "8080",
						"API_HOST":             "0.0.0.0",
						"ENABLE_JOBS":          "true",
						"KUBERNETES_NAMESPACE": "test-namespace",
					},
				},
				&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-api-server-deployment",
						Namespace: "test-namespace",
					},
					Spec: appsv1.DeploymentSpec{
						Replicas: &[]int32{2}[0],
						Template: corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{
									{
										Image: "registry.example.com/jira-sync:v1.0.0",
									},
								},
							},
						},
					},
				},
				&corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-api-server-service",
						Namespace: "test-namespace",
					},
					Spec: corev1.ServiceSpec{
						Type: corev1.ServiceTypeClusterIP,
						Ports: []corev1.ServicePort{
							{
								Port: 80,
							},
						},
					},
				},
			},
			expectedHasDrift: false,
			expectedDrifts:   []string{},
		},
		{
			name: "ConfigMap drift - different configuration values",
			apiServer: &operatortypes.APIServer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-api-server",
					Namespace: "test-namespace",
				},
				Spec: operatortypes.APIServerSpec{
					JIRACredentials: operatortypes.JIRACredentialsSpec{
						SecretRef: operatortypes.SecretRef{Name: "jira-secret"},
					},
					Image: operatortypes.ImageSpec{
						Repository: "registry.example.com/jira-sync",
						Tag:        "v1.0.0",
					},
					Config: &operatortypes.APIServerConfig{
						LogLevel:  "DEBUG",           // Expected DEBUG, but actual will be INFO
						LogFormat: "text",            // Expected text, but actual will be json
						Port:      &[]int32{9090}[0], // Expected 9090, but actual will be 8080
					},
				},
			},
			existingObjects: []client.Object{
				&corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-api-server-config",
						Namespace: "test-namespace",
					},
					Data: map[string]string{
						"LOG_LEVEL":  "INFO", // Drift: should be DEBUG
						"LOG_FORMAT": "json", // Drift: should be text
						"API_PORT":   "8080", // Drift: should be 9090
						"API_HOST":   "0.0.0.0",
					},
				},
			},
			expectedHasDrift: true,
			expectedDrifts:   []string{"configmap"},
		},
		{
			name: "Deployment drift - different replica count and image",
			apiServer: &operatortypes.APIServer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-api-server",
					Namespace: "test-namespace",
				},
				Spec: operatortypes.APIServerSpec{
					JIRACredentials: operatortypes.JIRACredentialsSpec{
						SecretRef: operatortypes.SecretRef{Name: "jira-secret"},
					},
					Image: operatortypes.ImageSpec{
						Repository: "registry.example.com/jira-sync",
						Tag:        "v2.0.0", // Expected v2.0.0, but actual will be v1.0.0
					},
					Replicas: &[]int32{3}[0], // Expected 3, but actual will be 2
				},
			},
			existingObjects: []client.Object{
				&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-api-server-deployment",
						Namespace: "test-namespace",
					},
					Spec: appsv1.DeploymentSpec{
						Replicas: &[]int32{2}[0], // Drift: should be 3
						Template: corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{
									{
										Image: "registry.example.com/jira-sync:v1.0.0", // Drift: should be v2.0.0
									},
								},
							},
						},
					},
				},
			},
			expectedHasDrift: true,
			expectedDrifts:   []string{"deployment"},
		},
		{
			name: "Service drift - different port and type",
			apiServer: &operatortypes.APIServer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-api-server",
					Namespace: "test-namespace",
				},
				Spec: operatortypes.APIServerSpec{
					JIRACredentials: operatortypes.JIRACredentialsSpec{
						SecretRef: operatortypes.SecretRef{Name: "jira-secret"},
					},
					Image: operatortypes.ImageSpec{
						Repository: "registry.example.com/jira-sync",
						Tag:        "v1.0.0",
					},
					Service: &operatortypes.ServiceConfig{
						Type: "NodePort",        // Expected NodePort, but actual will be ClusterIP
						Port: &[]int32{8080}[0], // Expected 8080, but actual will be 80
					},
				},
			},
			existingObjects: []client.Object{
				&corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-api-server-service",
						Namespace: "test-namespace",
					},
					Spec: corev1.ServiceSpec{
						Type: corev1.ServiceTypeClusterIP, // Drift: should be NodePort
						Ports: []corev1.ServicePort{
							{
								Port: 80, // Drift: should be 8080
							},
						},
					},
				},
			},
			expectedHasDrift: true,
			expectedDrifts:   []string{"service"},
		},
		{
			name: "Missing resources - all resources absent",
			apiServer: &operatortypes.APIServer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-api-server",
					Namespace: "test-namespace",
				},
				Spec: operatortypes.APIServerSpec{
					JIRACredentials: operatortypes.JIRACredentialsSpec{
						SecretRef: operatortypes.SecretRef{Name: "jira-secret"},
					},
					Image: operatortypes.ImageSpec{
						Repository: "registry.example.com/jira-sync",
						Tag:        "v1.0.0",
					},
				},
			},
			existingObjects:  []client.Object{}, // No existing resources
			expectedHasDrift: true,
			expectedDrifts:   []string{"configmap", "deployment", "service"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(tt.existingObjects...).
				Build()

			detector := NewDriftDetector(fakeClient, logr.Discard())

			result, err := detector.DetectDrift(context.Background(), tt.apiServer)

			require.NoError(t, err)
			assert.Equal(t, tt.expectedHasDrift, result.HasDrift, "Drift detection result should match expected")

			// Check that expected drift types are detected
			for _, expectedDrift := range tt.expectedDrifts {
				switch expectedDrift {
				case "configmap":
					assert.True(t, result.ConfigMapDrift.HasDrift(), "ConfigMap drift should be detected")
				case "deployment":
					assert.True(t, result.DeploymentDrift.HasDrift(), "Deployment drift should be detected")
				case "service":
					assert.True(t, result.ServiceDrift.HasDrift(), "Service drift should be detected")
				}
			}

			// Verify recommendation is provided when drift is detected
			if tt.expectedHasDrift {
				assert.NotEmpty(t, result.Recommendation, "Recommendation should be provided when drift is detected")
			} else {
				assert.Contains(t, result.Recommendation, "No configuration drift detected", "Recommendation should indicate no drift")
			}

			// Verify hash values are set
			assert.NotEmpty(t, result.SpecHash, "Spec hash should be generated")
			assert.NotEmpty(t, result.ActualHash, "Actual hash should be generated")
		})
	}
}

func TestDriftDetector_DetectConfigMapDrift(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	tests := []struct {
		name                string
		apiServer           *operatortypes.APIServer
		existingConfigMap   *corev1.ConfigMap
		expectedMissing     bool
		expectedDifferences int
		expectedDriftTypes  []string
	}{
		{
			name: "No ConfigMap drift",
			apiServer: &operatortypes.APIServer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-api-server",
					Namespace: "test-namespace",
				},
				Spec: operatortypes.APIServerSpec{
					Config: &operatortypes.APIServerConfig{
						LogLevel:  "INFO",
						LogFormat: "json",
						Port:      &[]int32{8080}[0],
					},
				},
			},
			existingConfigMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-api-server-config",
					Namespace: "test-namespace",
				},
				Data: map[string]string{
					"LOG_LEVEL":            "INFO",
					"LOG_FORMAT":           "json",
					"API_PORT":             "8080",
					"API_HOST":             "0.0.0.0",
					"ENABLE_JOBS":          "true",
					"KUBERNETES_NAMESPACE": "test-namespace",
				},
			},
			expectedMissing:     false,
			expectedDifferences: 0,
			expectedDriftTypes:  []string{},
		},
		{
			name: "ConfigMap missing",
			apiServer: &operatortypes.APIServer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-api-server",
					Namespace: "test-namespace",
				},
				Spec: operatortypes.APIServerSpec{},
			},
			existingConfigMap:   nil,
			expectedMissing:     true,
			expectedDifferences: 0,
			expectedDriftTypes:  []string{},
		},
		{
			name: "ConfigMap has different values",
			apiServer: &operatortypes.APIServer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-api-server",
					Namespace: "test-namespace",
				},
				Spec: operatortypes.APIServerSpec{
					Config: &operatortypes.APIServerConfig{
						LogLevel:  "DEBUG",
						LogFormat: "text",
						Port:      &[]int32{9090}[0],
					},
				},
			},
			existingConfigMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-api-server-config",
					Namespace: "test-namespace",
				},
				Data: map[string]string{
					"LOG_LEVEL":  "INFO",    // Different: should be DEBUG
					"LOG_FORMAT": "json",    // Different: should be text
					"API_PORT":   "8080",    // Different: should be 9090
					"API_HOST":   "0.0.0.0", // Same
					"EXTRA_KEY":  "extra",   // Extra key not in expected
				},
			},
			expectedMissing:     false,
			expectedDifferences: 6, // 3 different values + 1 extra key + 2 missing default job config
			expectedDriftTypes:  []string{"different", "extra", "missing"},
		},
		{
			name: "ConfigMap missing expected keys",
			apiServer: &operatortypes.APIServer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-api-server",
					Namespace: "test-namespace",
				},
				Spec: operatortypes.APIServerSpec{
					Config: &operatortypes.APIServerConfig{
						LogLevel:        "INFO",
						LogFormat:       "json",
						Port:            &[]int32{8080}[0],
						EnableJobs:      &[]bool{true}[0],
						SafeModeEnabled: &[]bool{true}[0],
					},
				},
			},
			existingConfigMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-api-server-config",
					Namespace: "test-namespace",
				},
				Data: map[string]string{
					"LOG_LEVEL":  "INFO",
					"LOG_FORMAT": "json",
					"API_PORT":   "8080",
					"API_HOST":   "0.0.0.0",
					// Missing ENABLE_JOBS and SPIKE_SAFE_MODE
				},
			},
			expectedMissing:     false,
			expectedDifferences: 3, // Missing ENABLE_JOBS, KUBERNETES_NAMESPACE, and SPIKE_SAFE_MODE
			expectedDriftTypes:  []string{"missing"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			objects := []client.Object{}
			if tt.existingConfigMap != nil {
				objects = append(objects, tt.existingConfigMap)
			}

			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(objects...).
				Build()

			detector := NewDriftDetector(fakeClient, logr.Discard())

			drift, err := detector.detectConfigMapDrift(context.Background(), tt.apiServer)

			require.NoError(t, err)
			assert.Equal(t, tt.expectedMissing, drift.Missing, "ConfigMap missing status should match expected")
			assert.Len(t, drift.DataDifferences, tt.expectedDifferences, "Number of data differences should match expected")

			// Check specific drift types
			for _, expectedType := range tt.expectedDriftTypes {
				found := false
				for _, diff := range drift.DataDifferences {
					if diff.Type == expectedType {
						found = true
						break
					}
				}
				assert.True(t, found, "Expected drift type %s should be found", expectedType)
			}
		})
	}
}

func TestDriftDetector_HasDrift(t *testing.T) {
	tests := []struct {
		name     string
		drift    interface{}
		expected bool
	}{
		{
			name: "ConfigMapDrift - has drift with differences",
			drift: &ConfigMapDrift{
				Missing: false,
				DataDifferences: map[string]DataDifference{
					"key1": {Type: "different"},
				},
			},
			expected: true,
		},
		{
			name: "ConfigMapDrift - has drift missing",
			drift: &ConfigMapDrift{
				Missing:         true,
				DataDifferences: map[string]DataDifference{},
			},
			expected: true,
		},
		{
			name: "ConfigMapDrift - no drift",
			drift: &ConfigMapDrift{
				Missing:         false,
				DataDifferences: map[string]DataDifference{},
			},
			expected: false,
		},
		{
			name: "DeploymentDrift - has drift with replicas",
			drift: &DeploymentDrift{
				Missing:       false,
				ReplicasDrift: &[]int32{5}[0],
			},
			expected: true,
		},
		{
			name: "DeploymentDrift - has drift missing",
			drift: &DeploymentDrift{
				Missing: true,
			},
			expected: true,
		},
		{
			name: "DeploymentDrift - no drift",
			drift: &DeploymentDrift{
				Missing:            false,
				ReplicasDrift:      nil,
				ImageDrift:         nil,
				ResourcesDrift:     map[string]interface{}{},
				EnvironmentDrift:   map[string]string{},
				ConfigHashMismatch: false,
			},
			expected: false,
		},
		{
			name: "ServiceDrift - has drift with port",
			drift: &ServiceDrift{
				Missing:   false,
				PortDrift: &[]int32{8080}[0],
			},
			expected: true,
		},
		{
			name: "ServiceDrift - no drift",
			drift: &ServiceDrift{
				Missing:   false,
				PortDrift: nil,
				TypeDrift: nil,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result bool
			switch d := tt.drift.(type) {
			case *ConfigMapDrift:
				result = d.HasDrift()
			case *DeploymentDrift:
				result = d.HasDrift()
			case *ServiceDrift:
				result = d.HasDrift()
			default:
				t.Fatalf("Unknown drift type: %T", d)
			}

			assert.Equal(t, tt.expected, result)
		})
	}
}
