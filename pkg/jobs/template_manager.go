package jobs

import (
	"fmt"
	"os"
	"path/filepath"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
)

// FileJobTemplateManager implements JobTemplateManager using file-based templates
type FileJobTemplateManager struct {
	templateDir string
	templates   map[JobType]*JobTemplate
}

// NewFileJobTemplateManager creates a new file-based job template manager
func NewFileJobTemplateManager() *FileJobTemplateManager {
	manager := &FileJobTemplateManager{
		templateDir: "deployments/jobs",
		templates:   make(map[JobType]*JobTemplate),
	}

	// Initialize with built-in templates based on SPIKE-001
	manager.initializeBuiltinTemplates()

	return manager
}

// GetTemplate retrieves a job template by type
func (m *FileJobTemplateManager) GetTemplate(jobType JobType) (*JobTemplate, error) {
	template, exists := m.templates[jobType]
	if !exists {
		return nil, fmt.Errorf("no template found for job type: %s", jobType)
	}

	return template.DeepCopy(), nil
}

// LoadTemplate loads a template from a file
func (m *FileJobTemplateManager) LoadTemplate(jobType JobType, templatePath string) error {
	if !filepath.IsAbs(templatePath) {
		templatePath = filepath.Join(m.templateDir, templatePath)
	}

	data, err := os.ReadFile(templatePath)
	if err != nil {
		return fmt.Errorf("failed to read template file %s: %w", templatePath, err)
	}

	var job batchv1.Job
	if err := yaml.Unmarshal(data, &job); err != nil {
		return fmt.Errorf("failed to unmarshal template: %w", err)
	}

	template := &JobTemplate{
		JobType:  jobType,
		Template: &job,
	}

	if err := m.ValidateTemplate(template); err != nil {
		return fmt.Errorf("template validation failed: %w", err)
	}

	m.templates[jobType] = template
	return nil
}

// ValidateTemplate validates a job template
func (m *FileJobTemplateManager) ValidateTemplate(template *JobTemplate) error {
	if template.Template == nil {
		return fmt.Errorf("template job is nil")
	}

	job := template.Template

	// Validate basic job structure
	if len(job.Spec.Template.Spec.Containers) == 0 {
		return fmt.Errorf("job template must have at least one container")
	}

	container := job.Spec.Template.Spec.Containers[0]

	// Validate container has required fields
	if container.Name == "" {
		return fmt.Errorf("container must have a name")
	}

	if container.Image == "" {
		return fmt.Errorf("container must have an image")
	}

	// Validate restart policy
	if job.Spec.Template.Spec.RestartPolicy != corev1.RestartPolicyNever &&
		job.Spec.Template.Spec.RestartPolicy != corev1.RestartPolicyOnFailure {
		return fmt.Errorf("job restart policy must be Never or OnFailure")
	}

	return nil
}

// initializeBuiltinTemplates creates the default templates based on SPIKE-001
func (m *FileJobTemplateManager) initializeBuiltinTemplates() {
	// Single issue sync template
	m.templates[JobTypeSingle] = m.createSingleJobTemplate()

	// Batch sync template
	m.templates[JobTypeBatch] = m.createBatchJobTemplate()

	// JQL sync template (based on batch template)
	m.templates[JobTypeJQL] = m.createJQLJobTemplate()
}

// createSingleJobTemplate creates template for single issue sync
func (m *FileJobTemplateManager) createSingleJobTemplate() *JobTemplate {
	template := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				"app":        "jira-sync",
				"sync-type":  "single",
				"managed-by": "jira-sync-scheduler",
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:          int32Ptr(3),
			ActiveDeadlineSeconds: int64Ptr(600), // 10 minutes
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":       "jira-sync",
						"sync-type": "single",
					},
				},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					SecurityContext: &corev1.PodSecurityContext{
						RunAsNonRoot: boolPtr(true),
						RunAsUser:    int64Ptr(1000),
						FSGroup:      int64Ptr(1000),
					},
					Containers: []corev1.Container{
						{
							Name:            "sync-worker",
							Image:           "jira-sync:latest",
							ImagePullPolicy: corev1.PullIfNotPresent,
							Command:         []string{"./jira-sync"},
							Args:            []string{"sync"}, // Will be overridden by scheduler
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("100m"),
									corev1.ResourceMemory: resource.MustParse("128Mi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("500m"),
									corev1.ResourceMemory: resource.MustParse("512Mi"),
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "git-repo",
									MountPath: "/workspace/repo",
								},
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
							},
							Env: []corev1.EnvVar{
								{
									Name: "JIRA_BASE_URL",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: "jira-credentials",
											},
											Key: "base-url",
										},
									},
								},
								{
									Name: "JIRA_PAT",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: "jira-credentials",
											},
											Key: "token",
										},
									},
								},
								{
									Name:  "LOG_LEVEL",
									Value: "INFO",
								},
								{
									Name:  "SPIKE_SAFE_MODE",
									Value: "true",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "git-repo",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: "git-repo-pvc",
								},
							},
						},
						{
							Name: "config",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "jira-sync-config",
									},
								},
							},
						},
						{
							Name: "credentials",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName:  "jira-credentials",
									DefaultMode: int32Ptr(0400),
								},
							},
						},
					},
				},
			},
		},
	}

	return &JobTemplate{
		JobType:  JobTypeSingle,
		Template: template,
		Resources: &JobResourceRequirements{
			RequestsCPU:    "100m",
			RequestsMemory: "128Mi",
			LimitsCPU:      "500m",
			LimitsMemory:   "512Mi",
		},
		TimeoutSec: int64Ptr(600),
	}
}

// createBatchJobTemplate creates template for batch sync
func (m *FileJobTemplateManager) createBatchJobTemplate() *JobTemplate {
	template := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				"app":        "jira-sync",
				"sync-type":  "batch",
				"managed-by": "jira-sync-scheduler",
			},
		},
		Spec: batchv1.JobSpec{
			Parallelism:           int32Ptr(2),
			Completions:           int32Ptr(1),
			BackoffLimit:          int32Ptr(3),
			ActiveDeadlineSeconds: int64Ptr(1800), // 30 minutes
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":       "jira-sync",
						"sync-type": "batch",
					},
				},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					SecurityContext: &corev1.PodSecurityContext{
						RunAsNonRoot: boolPtr(true),
						RunAsUser:    int64Ptr(1000),
						FSGroup:      int64Ptr(1000),
					},
					Containers: []corev1.Container{
						{
							Name:            "batch-sync-worker",
							Image:           "jira-sync:latest",
							ImagePullPolicy: corev1.PullIfNotPresent,
							Command:         []string{"./jira-sync"},
							Args:            []string{"sync"}, // Will be overridden by scheduler
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("200m"),
									corev1.ResourceMemory: resource.MustParse("256Mi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("1000m"),
									corev1.ResourceMemory: resource.MustParse("1Gi"),
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "git-repo",
									MountPath: "/workspace/repo",
								},
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
								{
									Name:      "shared-state",
									MountPath: "/workspace/shared",
								},
							},
							Env: []corev1.EnvVar{
								{
									Name: "JIRA_BASE_URL",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: "jira-credentials",
											},
											Key: "base-url",
										},
									},
								},
								{
									Name: "JIRA_PAT",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: "jira-credentials",
											},
											Key: "token",
										},
									},
								},
								{
									Name:  "LOG_LEVEL",
									Value: "INFO",
								},
								{
									Name:  "SPIKE_SAFE_MODE",
									Value: "true",
								},
								{
									Name:  "RATE_LIMIT_PER_MINUTE",
									Value: "30",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "git-repo",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: "git-repo-pvc",
								},
							},
						},
						{
							Name: "config",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "jira-sync-config",
									},
								},
							},
						},
						{
							Name: "credentials",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName:  "jira-credentials",
									DefaultMode: int32Ptr(0400),
								},
							},
						},
						{
							Name: "shared-state",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
					},
				},
			},
		},
	}

	return &JobTemplate{
		JobType:     JobTypeBatch,
		Template:    template,
		Parallelism: int32Ptr(2),
		Completions: int32Ptr(1),
		Resources: &JobResourceRequirements{
			RequestsCPU:    "200m",
			RequestsMemory: "256Mi",
			LimitsCPU:      "1000m",
			LimitsMemory:   "1Gi",
		},
		TimeoutSec: int64Ptr(1800),
	}
}

// createJQLJobTemplate creates template for JQL-based sync
func (m *FileJobTemplateManager) createJQLJobTemplate() *JobTemplate {
	// JQL sync is essentially the same as batch sync
	batchTemplate := m.createBatchJobTemplate()

	// Update labels and type
	batchTemplate.JobType = JobTypeJQL
	batchTemplate.Template.Labels["sync-type"] = "jql"
	batchTemplate.Template.Spec.Template.Labels["sync-type"] = "jql"

	return batchTemplate
}

// DeepCopy creates a deep copy of a JobTemplate
func (t *JobTemplate) DeepCopy() *JobTemplate {
	return &JobTemplate{
		JobType:     t.JobType,
		Template:    t.Template.DeepCopy(),
		Resources:   t.copyResources(),
		Parallelism: t.copyInt32Ptr(t.Parallelism),
		Completions: t.copyInt32Ptr(t.Completions),
		TimeoutSec:  t.copyInt64Ptr(t.TimeoutSec),
	}
}

func (t *JobTemplate) copyResources() *JobResourceRequirements {
	if t.Resources == nil {
		return nil
	}
	return &JobResourceRequirements{
		RequestsCPU:    t.Resources.RequestsCPU,
		RequestsMemory: t.Resources.RequestsMemory,
		LimitsCPU:      t.Resources.LimitsCPU,
		LimitsMemory:   t.Resources.LimitsMemory,
	}
}

func (t *JobTemplate) copyInt32Ptr(ptr *int32) *int32 {
	if ptr == nil {
		return nil
	}
	val := *ptr
	return &val
}

func (t *JobTemplate) copyInt64Ptr(ptr *int64) *int64 {
	if ptr == nil {
		return nil
	}
	val := *ptr
	return &val
}

// Helper functions for pointer creation
func int32Ptr(i int32) *int32 {
	return &i
}

func int64Ptr(i int64) *int64 {
	return &i
}

func boolPtr(b bool) *bool {
	return &b
}
