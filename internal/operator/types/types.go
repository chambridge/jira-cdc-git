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

func init() {
	SchemeBuilder.Register(&JIRASync{}, &JIRASyncList{}, &JIRAProject{}, &JIRAProjectList{})
}
