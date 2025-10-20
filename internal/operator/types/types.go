package types

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// GroupVersion is group version used to register these objects
var GroupVersion = schema.GroupVersion{Group: "sync.jira.io", Version: "v1alpha1"}

// JIRASyncSpec defines the desired state of JIRASync
type JIRASyncSpec struct {
	// Type of sync operation to perform
	SyncType string `json:"syncType"`

	// Target specification for sync operation
	Target SyncTarget `json:"target"`

	// Git repository destination configuration
	Destination GitDestination `json:"destination"`

	// Cron expression for scheduled syncs (optional)
	Schedule string `json:"schedule,omitempty"`

	// Retry configuration for failed sync operations
	RetryPolicy *RetryPolicy `json:"retryPolicy,omitempty"`
}

// SyncTarget defines what JIRA issues to sync
type SyncTarget struct {
	// List of specific JIRA issue keys to sync
	IssueKeys []string `json:"issueKeys,omitempty"`

	// JQL query to select issues for sync
	JQLQuery string `json:"jqlQuery,omitempty"`

	// JIRA project key for project-wide sync
	ProjectKey string `json:"projectKey,omitempty"`

	// EPIC key for epic-focused sync
	EpicKey string `json:"epicKey,omitempty"`
}

// GitDestination defines git repository destination
type GitDestination struct {
	// Git repository URL or path
	Repository string `json:"repository"`

	// Target Git branch
	Branch string `json:"branch,omitempty"`

	// Path within repository for issue files
	Path string `json:"path,omitempty"`
}

// RetryPolicy defines retry configuration
type RetryPolicy struct {
	// Maximum number of retry attempts
	MaxRetries int `json:"maxRetries,omitempty"`

	// Exponential backoff multiplier
	BackoffMultiplier float64 `json:"backoffMultiplier,omitempty"`

	// Initial delay before first retry (in seconds)
	InitialDelay int `json:"initialDelay,omitempty"`
}

// JIRASyncStatus defines the observed state of JIRASync
type JIRASyncStatus struct {
	// Current phase of the sync operation
	Phase string `json:"phase,omitempty"`

	// Conditions represent the latest available observations
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Statistics about the sync operation
	SyncStats *SyncStats `json:"syncStats,omitempty"`

	// Reference to the Kubernetes Job executing this sync
	JobRef *JobReference `json:"jobRef,omitempty"`

	// The generation observed by the controller
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Progress information for long-running operations
	Progress *ProgressInfo `json:"progress,omitempty"`

	// Current sync operation state
	SyncState *SyncState `json:"syncState,omitempty"`

	// Last error message if any
	LastError string `json:"lastError,omitempty"`

	// Number of consecutive retry attempts
	RetryCount int `json:"retryCount,omitempty"`

	// Timestamp of last status update
	LastStatusUpdate *metav1.Time `json:"lastStatusUpdate,omitempty"`
}

// SyncStats provides statistics about sync operations
type SyncStats struct {
	// Total number of issues to be synced
	TotalIssues int `json:"totalIssues,omitempty"`

	// Number of issues successfully processed
	ProcessedIssues int `json:"processedIssues,omitempty"`

	// Number of issues that failed to sync
	FailedIssues int `json:"failedIssues,omitempty"`

	// Timestamp of last successful sync
	LastSyncTime *metav1.Time `json:"lastSyncTime,omitempty"`

	// Duration of the last sync operation
	Duration string `json:"duration,omitempty"`

	// Start time of current sync operation
	StartTime *metav1.Time `json:"startTime,omitempty"`
}

// JobReference points to a Kubernetes Job
type JobReference struct {
	Name      string `json:"name,omitempty"`
	Namespace string `json:"namespace,omitempty"`
}

// ProgressInfo provides detailed progress information
type ProgressInfo struct {
	// Current progress percentage (0-100)
	Percentage int `json:"percentage,omitempty"`

	// Current operation being performed
	CurrentOperation string `json:"currentOperation,omitempty"`

	// Total number of operations to be completed
	TotalOperations int `json:"totalOperations,omitempty"`

	// Number of completed operations
	CompletedOperations int `json:"completedOperations,omitempty"`

	// Estimated completion time
	EstimatedCompletion *metav1.Time `json:"estimatedCompletion,omitempty"`

	// Processing rate (operations per minute)
	ProcessingRate float64 `json:"processingRate,omitempty"`

	// Current stage of the sync operation
	Stage string `json:"stage,omitempty"`
}

// SyncState provides current state information
type SyncState struct {
	// Current sync operation ID
	OperationID string `json:"operationID,omitempty"`

	// Sync configuration hash for change detection
	ConfigHash string `json:"configHash,omitempty"`

	// List of issues currently being processed
	ActiveIssues []string `json:"activeIssues,omitempty"`

	// Last successful sync configuration
	LastSuccessfulConfig string `json:"lastSuccessfulConfig,omitempty"`

	// Sync operation metadata
	Metadata map[string]string `json:"metadata,omitempty"`

	// Resource health status
	HealthStatus string `json:"healthStatus,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Type",type="string",JSONPath=".spec.syncType"
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Issues",type="string",JSONPath=".status.syncStats.processedIssues"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// JIRASync is the Schema for the jirasyncs API
type JIRASync struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   JIRASyncSpec   `json:"spec,omitempty"`
	Status JIRASyncStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// JIRASyncList contains a list of JIRASync
type JIRASyncList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []JIRASync `json:"items"`
}

// DeepCopyInto copies all properties of this object into another object of the
// same type that is provided as a pointer.
func (in *JIRASyncList) DeepCopyInto(out *JIRASyncList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]JIRASync, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy copies the receiver, creating a new JIRASyncList.
func (in *JIRASyncList) DeepCopy() *JIRASyncList {
	if in == nil {
		return nil
	}
	out := new(JIRASyncList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject copies the receiver, creating a new runtime.Object.
func (in *JIRASyncList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// JIRAProjectSpec defines the desired state of JIRAProject
type JIRAProjectSpec struct {
	// JIRA project key
	ProjectKey string `json:"projectKey"`

	// JIRA instance URL
	JIRAInstance string `json:"jiraInstance"`

	// Configuration for project sync operations
	SyncConfiguration *ProjectSyncConfig `json:"syncConfiguration,omitempty"`

	// Git repository destination configuration
	Destination GitDestination `json:"destination"`

	// Reference to credentials for JIRA and Git access
	Credentials *CredentialRefs `json:"credentials,omitempty"`
}

// ProjectSyncConfig defines project-level sync configuration
type ProjectSyncConfig struct {
	// Whether to sync issue relationships as symbolic links
	IncludeRelationships bool `json:"includeRelationships,omitempty"`

	// List of issue types to include (empty = all)
	IssueTypes []string `json:"issueTypes,omitempty"`

	// List of custom fields to include in sync
	CustomFields []string `json:"customFields,omitempty"`

	// List of statuses to exclude from sync
	ExcludeStatuses []string `json:"excludeStatuses,omitempty"`

	// How often to perform full project sync (cron format)
	SyncFrequency string `json:"syncFrequency,omitempty"`

	// Enable incremental sync for this project
	IncrementalSync bool `json:"incrementalSync,omitempty"`
}

// CredentialRefs defines references to secrets containing credentials
type CredentialRefs struct {
	// Secret containing JIRA credentials
	JIRASecretRef *SecretRef `json:"jiraSecretRef,omitempty"`

	// Secret containing Git credentials
	GitSecretRef *SecretRef `json:"gitSecretRef,omitempty"`
}

// SecretRef defines a reference to a Kubernetes secret
type SecretRef struct {
	Name string `json:"name"`
	Key  string `json:"key,omitempty"`
}

// JIRAProjectStatus defines the observed state of JIRAProject
type JIRAProjectStatus struct {
	// Current phase of project management
	Phase string `json:"phase,omitempty"`

	// Conditions represent the latest available observations
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Timestamp of last successful project sync
	LastSyncTime *metav1.Time `json:"lastSyncTime,omitempty"`

	// Total number of issues in this project
	TotalIssues int `json:"totalIssues,omitempty"`

	// Number of currently active sync operations
	ActiveSyncs int `json:"activeSyncs,omitempty"`

	// The generation observed by the controller
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Project",type="string",JSONPath=".spec.projectKey"
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Issues",type="integer",JSONPath=".status.totalIssues"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// JIRAProject is the Schema for the jiraprojects API
type JIRAProject struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   JIRAProjectSpec   `json:"spec,omitempty"`
	Status JIRAProjectStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// JIRAProjectList contains a list of JIRAProject
type JIRAProjectList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []JIRAProject `json:"items"`
}

// DeepCopyInto copies all properties of this object into another object of the
// same type that is provided as a pointer.
func (in *JIRASync) DeepCopyInto(out *JIRASync) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy copies the receiver, creating a new JIRASync.
func (in *JIRASync) DeepCopy() *JIRASync {
	if in == nil {
		return nil
	}
	out := new(JIRASync)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject copies the receiver, creating a new runtime.Object.
func (in *JIRASync) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto copies all properties of this object into another object of the
// same type that is provided as a pointer.
func (in *JIRASyncSpec) DeepCopyInto(out *JIRASyncSpec) {
	*out = *in
	in.Target.DeepCopyInto(&out.Target)
	out.Destination = in.Destination
	if in.RetryPolicy != nil {
		in, out := &in.RetryPolicy, &out.RetryPolicy
		*out = new(RetryPolicy)
		**out = **in
	}
}

// DeepCopy copies the receiver, creating a new JIRASyncSpec.
func (in *JIRASyncSpec) DeepCopy() *JIRASyncSpec {
	if in == nil {
		return nil
	}
	out := new(JIRASyncSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto copies all properties of this object into another object of the
// same type that is provided as a pointer.
func (in *JIRASyncStatus) DeepCopyInto(out *JIRASyncStatus) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]metav1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.SyncStats != nil {
		in, out := &in.SyncStats, &out.SyncStats
		*out = new(SyncStats)
		(*in).DeepCopyInto(*out)
	}
	if in.JobRef != nil {
		in, out := &in.JobRef, &out.JobRef
		*out = new(JobReference)
		**out = **in
	}
	if in.Progress != nil {
		in, out := &in.Progress, &out.Progress
		*out = new(ProgressInfo)
		(*in).DeepCopyInto(*out)
	}
	if in.SyncState != nil {
		in, out := &in.SyncState, &out.SyncState
		*out = new(SyncState)
		(*in).DeepCopyInto(*out)
	}
	if in.LastStatusUpdate != nil {
		in, out := &in.LastStatusUpdate, &out.LastStatusUpdate
		*out = (*in).DeepCopy()
	}
}

// DeepCopy copies the receiver, creating a new JIRASyncStatus.
func (in *JIRASyncStatus) DeepCopy() *JIRASyncStatus {
	if in == nil {
		return nil
	}
	out := new(JIRASyncStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto copies all properties of this object into another object of the
// same type that is provided as a pointer.
func (in *SyncStats) DeepCopyInto(out *SyncStats) {
	*out = *in
	if in.LastSyncTime != nil {
		in, out := &in.LastSyncTime, &out.LastSyncTime
		*out = (*in).DeepCopy()
	}
	if in.StartTime != nil {
		in, out := &in.StartTime, &out.StartTime
		*out = (*in).DeepCopy()
	}
}

// DeepCopy copies the receiver, creating a new SyncStats.
func (in *SyncStats) DeepCopy() *SyncStats {
	if in == nil {
		return nil
	}
	out := new(SyncStats)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto copies all properties of this object into another object of the
// same type that is provided as a pointer.
func (in *SyncTarget) DeepCopyInto(out *SyncTarget) {
	*out = *in
	if in.IssueKeys != nil {
		in, out := &in.IssueKeys, &out.IssueKeys
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy copies the receiver, creating a new SyncTarget.
func (in *SyncTarget) DeepCopy() *SyncTarget {
	if in == nil {
		return nil
	}
	out := new(SyncTarget)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto for JIRAProject
func (in *JIRAProject) DeepCopyInto(out *JIRAProject) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy copies the receiver, creating a new JIRAProject.
func (in *JIRAProject) DeepCopy() *JIRAProject {
	if in == nil {
		return nil
	}
	out := new(JIRAProject)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject copies the receiver, creating a new runtime.Object.
func (in *JIRAProject) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto for JIRAProjectList
func (in *JIRAProjectList) DeepCopyInto(out *JIRAProjectList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]JIRAProject, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy copies the receiver, creating a new JIRAProjectList.
func (in *JIRAProjectList) DeepCopy() *JIRAProjectList {
	if in == nil {
		return nil
	}
	out := new(JIRAProjectList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject copies the receiver, creating a new runtime.Object.
func (in *JIRAProjectList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto for JIRAProjectSpec
func (in *JIRAProjectSpec) DeepCopyInto(out *JIRAProjectSpec) {
	*out = *in
	if in.SyncConfiguration != nil {
		in, out := &in.SyncConfiguration, &out.SyncConfiguration
		*out = new(ProjectSyncConfig)
		(*in).DeepCopyInto(*out)
	}
	out.Destination = in.Destination
	if in.Credentials != nil {
		in, out := &in.Credentials, &out.Credentials
		*out = new(CredentialRefs)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy copies the receiver, creating a new JIRAProjectSpec.
func (in *JIRAProjectSpec) DeepCopy() *JIRAProjectSpec {
	if in == nil {
		return nil
	}
	out := new(JIRAProjectSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto for JIRAProjectStatus
func (in *JIRAProjectStatus) DeepCopyInto(out *JIRAProjectStatus) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]metav1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.LastSyncTime != nil {
		in, out := &in.LastSyncTime, &out.LastSyncTime
		*out = (*in).DeepCopy()
	}
}

// DeepCopy copies the receiver, creating a new JIRAProjectStatus.
func (in *JIRAProjectStatus) DeepCopy() *JIRAProjectStatus {
	if in == nil {
		return nil
	}
	out := new(JIRAProjectStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto for ProjectSyncConfig
func (in *ProjectSyncConfig) DeepCopyInto(out *ProjectSyncConfig) {
	*out = *in
	if in.IssueTypes != nil {
		in, out := &in.IssueTypes, &out.IssueTypes
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.CustomFields != nil {
		in, out := &in.CustomFields, &out.CustomFields
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.ExcludeStatuses != nil {
		in, out := &in.ExcludeStatuses, &out.ExcludeStatuses
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy copies the receiver, creating a new ProjectSyncConfig.
func (in *ProjectSyncConfig) DeepCopy() *ProjectSyncConfig {
	if in == nil {
		return nil
	}
	out := new(ProjectSyncConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto for CredentialRefs
func (in *CredentialRefs) DeepCopyInto(out *CredentialRefs) {
	*out = *in
	if in.JIRASecretRef != nil {
		in, out := &in.JIRASecretRef, &out.JIRASecretRef
		*out = new(SecretRef)
		**out = **in
	}
	if in.GitSecretRef != nil {
		in, out := &in.GitSecretRef, &out.GitSecretRef
		*out = new(SecretRef)
		**out = **in
	}
}

// DeepCopy copies the receiver, creating a new CredentialRefs.
func (in *CredentialRefs) DeepCopy() *CredentialRefs {
	if in == nil {
		return nil
	}
	out := new(CredentialRefs)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto for ProgressInfo
func (in *ProgressInfo) DeepCopyInto(out *ProgressInfo) {
	*out = *in
	if in.EstimatedCompletion != nil {
		in, out := &in.EstimatedCompletion, &out.EstimatedCompletion
		*out = (*in).DeepCopy()
	}
}

// DeepCopy copies the receiver, creating a new ProgressInfo.
func (in *ProgressInfo) DeepCopy() *ProgressInfo {
	if in == nil {
		return nil
	}
	out := new(ProgressInfo)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto for SyncState
func (in *SyncState) DeepCopyInto(out *SyncState) {
	*out = *in
	if in.ActiveIssues != nil {
		in, out := &in.ActiveIssues, &out.ActiveIssues
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.Metadata != nil {
		in, out := &in.Metadata, &out.Metadata
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
}

// DeepCopy copies the receiver, creating a new SyncState.
func (in *SyncState) DeepCopy() *SyncState {
	if in == nil {
		return nil
	}
	out := new(SyncState)
	in.DeepCopyInto(out)
	return out
}

// APIServerSpec defines the desired state of APIServer
type APIServerSpec struct {
	// JIRA connection credentials
	JIRACredentials JIRACredentialsSpec `json:"jiraCredentials"`

	// Container image configuration
	Image ImageSpec `json:"image"`

	// Number of API server replicas
	Replicas *int32 `json:"replicas,omitempty"`

	// Resource requirements for API server
	Resources *ResourceRequirements `json:"resources,omitempty"`

	// API server configuration
	Config *APIServerConfig `json:"config,omitempty"`

	// Service configuration
	Service *ServiceConfig `json:"service,omitempty"`
}

// JIRACredentialsSpec defines JIRA connection credentials
type JIRACredentialsSpec struct {
	// Reference to secret containing JIRA credentials
	SecretRef SecretRef `json:"secretRef"`
}

// ImageSpec defines container image configuration
type ImageSpec struct {
	// Container image repository
	Repository string `json:"repository"`

	// Container image tag
	Tag string `json:"tag"`

	// Image pull policy
	PullPolicy string `json:"pullPolicy,omitempty"`
}

// ResourceRequirements defines resource requirements
type ResourceRequirements struct {
	// Resource requests
	Requests *ResourceList `json:"requests,omitempty"`

	// Resource limits
	Limits *ResourceList `json:"limits,omitempty"`
}

// ResourceList defines CPU and memory resources
type ResourceList struct {
	// CPU resource
	CPU string `json:"cpu,omitempty"`

	// Memory resource
	Memory string `json:"memory,omitempty"`
}

// APIServerConfig defines API server configuration
type APIServerConfig struct {
	// Log level for API server
	LogLevel string `json:"logLevel,omitempty"`

	// Log format for API server
	LogFormat string `json:"logFormat,omitempty"`

	// API server port
	Port *int32 `json:"port,omitempty"`

	// Enable Kubernetes job creation
	EnableJobs *bool `json:"enableJobs,omitempty"`

	// Container image for sync jobs
	JobImage string `json:"jobImage,omitempty"`

	// Enable safe mode for testing
	SafeModeEnabled *bool `json:"safeModeEnabled,omitempty"`
}

// ServiceConfig defines service configuration
type ServiceConfig struct {
	// Kubernetes service type
	Type string `json:"type,omitempty"`

	// Service port
	Port *int32 `json:"port,omitempty"`

	// Service annotations
	Annotations map[string]string `json:"annotations,omitempty"`
}

// APIServerStatus defines the observed state of APIServer
type APIServerStatus struct {
	// Current phase of the API server
	Phase string `json:"phase,omitempty"`

	// Current conditions of the API server
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Status of managed Deployment
	DeploymentStatus *DeploymentStatus `json:"deploymentStatus,omitempty"`

	// Status of managed Service
	ServiceStatus *ServiceStatus `json:"serviceStatus,omitempty"`

	// API server endpoint URL
	Endpoint string `json:"endpoint,omitempty"`

	// Health status of API server
	HealthStatus *HealthStatus `json:"healthStatus,omitempty"`
}

// DeploymentStatus defines deployment status information
type DeploymentStatus struct {
	// Total number of replicas
	Replicas int32 `json:"replicas,omitempty"`

	// Number of ready replicas
	ReadyReplicas int32 `json:"readyReplicas,omitempty"`

	// Number of updated replicas
	UpdatedReplicas int32 `json:"updatedReplicas,omitempty"`
}

// ServiceStatus defines service status information
type ServiceStatus struct {
	// Cluster IP of the service
	ClusterIP string `json:"clusterIP,omitempty"`

	// Service ports
	Ports []ServicePort `json:"ports,omitempty"`
}

// ServicePort defines a service port
type ServicePort struct {
	// Port name
	Name string `json:"name,omitempty"`

	// Port number
	Port int32 `json:"port,omitempty"`

	// Target port
	TargetPort int32 `json:"targetPort,omitempty"`
}

// HealthStatus defines health status information
type HealthStatus struct {
	// Whether API server is healthy
	Healthy bool `json:"healthy,omitempty"`

	// Last health check time
	LastCheck *metav1.Time `json:"lastCheck,omitempty"`

	// Health check message
	Message string `json:"message,omitempty"`
}

// APIServer is the Schema for the apiservers API
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Replicas",type="integer",JSONPath=".spec.replicas"
// +kubebuilder:printcolumn:name="Ready",type="integer",JSONPath=".status.deploymentStatus.readyReplicas"
// +kubebuilder:printcolumn:name="Endpoint",type="string",JSONPath=".status.endpoint"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type APIServer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   APIServerSpec   `json:"spec,omitempty"`
	Status APIServerStatus `json:"status,omitempty"`
}

// APIServerList contains a list of APIServer
// +kubebuilder:object:root=true
type APIServerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []APIServer `json:"items"`
}

// DeepCopyInto for APIServer
func (in *APIServer) DeepCopyInto(out *APIServer) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy copies the receiver, creating a new APIServer.
func (in *APIServer) DeepCopy() *APIServer {
	if in == nil {
		return nil
	}
	out := new(APIServer)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject copies the receiver, creating a new runtime.Object.
func (in *APIServer) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto for APIServerList
func (in *APIServerList) DeepCopyInto(out *APIServerList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]APIServer, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy copies the receiver, creating a new APIServerList.
func (in *APIServerList) DeepCopy() *APIServerList {
	if in == nil {
		return nil
	}
	out := new(APIServerList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject copies the receiver, creating a new runtime.Object.
func (in *APIServerList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto for APIServerSpec
func (in *APIServerSpec) DeepCopyInto(out *APIServerSpec) {
	*out = *in
	out.JIRACredentials = in.JIRACredentials
	out.Image = in.Image
	if in.Replicas != nil {
		in, out := &in.Replicas, &out.Replicas
		*out = new(int32)
		**out = **in
	}
	if in.Resources != nil {
		in, out := &in.Resources, &out.Resources
		*out = new(ResourceRequirements)
		(*in).DeepCopyInto(*out)
	}
	if in.Config != nil {
		in, out := &in.Config, &out.Config
		*out = new(APIServerConfig)
		(*in).DeepCopyInto(*out)
	}
	if in.Service != nil {
		in, out := &in.Service, &out.Service
		*out = new(ServiceConfig)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy copies the receiver, creating a new APIServerSpec.
func (in *APIServerSpec) DeepCopy() *APIServerSpec {
	if in == nil {
		return nil
	}
	out := new(APIServerSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto for APIServerStatus
func (in *APIServerStatus) DeepCopyInto(out *APIServerStatus) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]metav1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.DeploymentStatus != nil {
		in, out := &in.DeploymentStatus, &out.DeploymentStatus
		*out = new(DeploymentStatus)
		**out = **in
	}
	if in.ServiceStatus != nil {
		in, out := &in.ServiceStatus, &out.ServiceStatus
		*out = new(ServiceStatus)
		(*in).DeepCopyInto(*out)
	}
	if in.HealthStatus != nil {
		in, out := &in.HealthStatus, &out.HealthStatus
		*out = new(HealthStatus)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy copies the receiver, creating a new APIServerStatus.
func (in *APIServerStatus) DeepCopy() *APIServerStatus {
	if in == nil {
		return nil
	}
	out := new(APIServerStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto for ResourceRequirements
func (in *ResourceRequirements) DeepCopyInto(out *ResourceRequirements) {
	*out = *in
	if in.Requests != nil {
		in, out := &in.Requests, &out.Requests
		*out = new(ResourceList)
		**out = **in
	}
	if in.Limits != nil {
		in, out := &in.Limits, &out.Limits
		*out = new(ResourceList)
		**out = **in
	}
}

// DeepCopy copies the receiver, creating a new ResourceRequirements.
func (in *ResourceRequirements) DeepCopy() *ResourceRequirements {
	if in == nil {
		return nil
	}
	out := new(ResourceRequirements)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto for APIServerConfig
func (in *APIServerConfig) DeepCopyInto(out *APIServerConfig) {
	*out = *in
	if in.Port != nil {
		in, out := &in.Port, &out.Port
		*out = new(int32)
		**out = **in
	}
	if in.EnableJobs != nil {
		in, out := &in.EnableJobs, &out.EnableJobs
		*out = new(bool)
		**out = **in
	}
	if in.SafeModeEnabled != nil {
		in, out := &in.SafeModeEnabled, &out.SafeModeEnabled
		*out = new(bool)
		**out = **in
	}
}

// DeepCopy copies the receiver, creating a new APIServerConfig.
func (in *APIServerConfig) DeepCopy() *APIServerConfig {
	if in == nil {
		return nil
	}
	out := new(APIServerConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto for ServiceConfig
func (in *ServiceConfig) DeepCopyInto(out *ServiceConfig) {
	*out = *in
	if in.Port != nil {
		in, out := &in.Port, &out.Port
		*out = new(int32)
		**out = **in
	}
	if in.Annotations != nil {
		in, out := &in.Annotations, &out.Annotations
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
}

// DeepCopy copies the receiver, creating a new ServiceConfig.
func (in *ServiceConfig) DeepCopy() *ServiceConfig {
	if in == nil {
		return nil
	}
	out := new(ServiceConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto for ServiceStatus
func (in *ServiceStatus) DeepCopyInto(out *ServiceStatus) {
	*out = *in
	if in.Ports != nil {
		in, out := &in.Ports, &out.Ports
		*out = make([]ServicePort, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy copies the receiver, creating a new ServiceStatus.
func (in *ServiceStatus) DeepCopy() *ServiceStatus {
	if in == nil {
		return nil
	}
	out := new(ServiceStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto for HealthStatus
func (in *HealthStatus) DeepCopyInto(out *HealthStatus) {
	*out = *in
	if in.LastCheck != nil {
		in, out := &in.LastCheck, &out.LastCheck
		*out = (*in).DeepCopy()
	}
}

// DeepCopy copies the receiver, creating a new HealthStatus.
func (in *HealthStatus) DeepCopy() *HealthStatus {
	if in == nil {
		return nil
	}
	out := new(HealthStatus)
	in.DeepCopyInto(out)
	return out
}

func init() {
	SchemeBuilder.Register(&JIRASync{}, &JIRASyncList{}, &JIRAProject{}, &JIRAProjectList{}, &APIServer{}, &APIServerList{})
}
