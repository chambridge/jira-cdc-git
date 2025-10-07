package api

import (
	"fmt"
	"regexp"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// CRDConverter provides conversion from API requests to CRD specifications
type CRDConverter struct {
	// Default values for CRD fields not present in API
	defaultBranch      string
	defaultPath        string
	defaultPriority    string
	defaultTimeout     int
	defaultRetryPolicy *CRDRetryPolicy
}

// CRDRetryPolicy represents the retry policy structure for CRDs
type CRDRetryPolicy struct {
	MaxRetries        int     `json:"maxRetries"`
	BackoffMultiplier float64 `json:"backoffMultiplier"`
	InitialDelay      int     `json:"initialDelay"`
}

// CRDTarget represents the target structure for CRDs
type CRDTarget struct {
	IssueKeys  []string `json:"issueKeys,omitempty"`
	JQLQuery   string   `json:"jqlQuery,omitempty"`
	ProjectKey string   `json:"projectKey,omitempty"`
	EpicKey    string   `json:"epicKey,omitempty"`
}

// CRDDestination represents the destination structure for CRDs
type CRDDestination struct {
	Repository string `json:"repository"`
	Branch     string `json:"branch"`
	Path       string `json:"path"`
}

// CRDSpec represents the complete CRD specification
type CRDSpec struct {
	SyncType    string            `json:"syncType"`
	Target      CRDTarget         `json:"target"`
	Destination CRDDestination    `json:"destination"`
	Priority    string            `json:"priority"`
	Timeout     int               `json:"timeout"`
	RetryPolicy *CRDRetryPolicy   `json:"retryPolicy"`
	Labels      map[string]string `json:"labels,omitempty"`
}

// ConversionResult contains the CRD and metadata from conversion
type ConversionResult struct {
	CRDSpec     *CRDSpec
	CRDResource *unstructured.Unstructured
	Annotations map[string]string // API-specific fields preserved as annotations
}

// NewCRDConverter creates a new converter with default values
func NewCRDConverter() *CRDConverter {
	return &CRDConverter{
		defaultBranch:   "main",
		defaultPath:     "/",
		defaultPriority: "normal",
		defaultTimeout:  1800, // 30 minutes
		defaultRetryPolicy: &CRDRetryPolicy{
			MaxRetries:        3,
			BackoffMultiplier: 2.0,
			InitialDelay:      30,
		},
	}
}

// ConvertSingleSync converts a SingleSyncRequest to CRD specification
func (c *CRDConverter) ConvertSingleSync(req *SingleSyncRequest) (*ConversionResult, error) {
	// Enhanced validation using CRD security patterns
	if err := c.validateSingleSyncRequestSecure(req); err != nil {
		return nil, fmt.Errorf("security validation failed: %w", err)
	}

	// Create CRD spec
	spec := &CRDSpec{
		SyncType: "single",
		Target: CRDTarget{
			IssueKeys: []string{req.IssueKey},
		},
		Destination: CRDDestination{
			Repository: req.Repository,
			Branch:     c.defaultBranch,
			Path:       c.defaultPath,
		},
		Priority:    c.defaultPriority,
		Timeout:     c.defaultTimeout,
		RetryPolicy: c.defaultRetryPolicy,
	}

	// Apply options if provided
	if req.Options != nil {
		c.applySyncOptionsToSpec(spec, req.Options)
	}

	// Create annotations for API-specific fields
	annotations := map[string]string{
		"sync.jira.io/safe-mode": fmt.Sprintf("%t", req.SafeMode),
		"sync.jira.io/async":     fmt.Sprintf("%t", req.Async),
		"sync.jira.io/source":    "api-single-sync",
	}

	// Create CRD resource
	crdResource, err := c.createCRDResource(spec, annotations, "single")
	if err != nil {
		return nil, fmt.Errorf("failed to create CRD resource: %w", err)
	}

	return &ConversionResult{
		CRDSpec:     spec,
		CRDResource: crdResource,
		Annotations: annotations,
	}, nil
}

// ConvertBatchSync converts a BatchSyncRequest to CRD specification
func (c *CRDConverter) ConvertBatchSync(req *BatchSyncRequest) (*ConversionResult, error) {
	// Enhanced validation using CRD security patterns
	if err := c.validateBatchSyncRequestSecure(req); err != nil {
		return nil, fmt.Errorf("security validation failed: %w", err)
	}

	// Create CRD spec
	spec := &CRDSpec{
		SyncType: "batch",
		Target: CRDTarget{
			IssueKeys: req.IssueKeys,
		},
		Destination: CRDDestination{
			Repository: req.Repository,
			Branch:     c.defaultBranch,
			Path:       c.defaultPath,
		},
		Priority:    c.defaultPriority,
		Timeout:     c.defaultTimeout,
		RetryPolicy: c.defaultRetryPolicy,
	}

	// Apply options if provided
	if req.Options != nil {
		c.applySyncOptionsToSpec(spec, req.Options)
	}

	// Create annotations for API-specific fields
	annotations := map[string]string{
		"sync.jira.io/safe-mode":   fmt.Sprintf("%t", req.SafeMode),
		"sync.jira.io/async":       fmt.Sprintf("%t", req.Async),
		"sync.jira.io/parallelism": fmt.Sprintf("%d", req.Parallelism),
		"sync.jira.io/source":      "api-batch-sync",
		"sync.jira.io/issue-count": fmt.Sprintf("%d", len(req.IssueKeys)),
	}

	// Create CRD resource
	crdResource, err := c.createCRDResource(spec, annotations, "batch")
	if err != nil {
		return nil, fmt.Errorf("failed to create CRD resource: %w", err)
	}

	return &ConversionResult{
		CRDSpec:     spec,
		CRDResource: crdResource,
		Annotations: annotations,
	}, nil
}

// ConvertJQLSync converts a JQLSyncRequest to CRD specification
func (c *CRDConverter) ConvertJQLSync(req *JQLSyncRequest) (*ConversionResult, error) {
	// Enhanced validation using CRD security patterns
	if err := c.validateJQLSyncRequestSecure(req); err != nil {
		return nil, fmt.Errorf("security validation failed: %w", err)
	}

	// Create CRD spec
	spec := &CRDSpec{
		SyncType: "jql",
		Target: CRDTarget{
			JQLQuery: req.JQL,
		},
		Destination: CRDDestination{
			Repository: req.Repository,
			Branch:     c.defaultBranch,
			Path:       c.defaultPath,
		},
		Priority:    c.defaultPriority,
		Timeout:     c.defaultTimeout,
		RetryPolicy: c.defaultRetryPolicy,
	}

	// Apply options if provided
	if req.Options != nil {
		c.applySyncOptionsToSpec(spec, req.Options)
	}

	// Create annotations for API-specific fields
	annotations := map[string]string{
		"sync.jira.io/safe-mode":   fmt.Sprintf("%t", req.SafeMode),
		"sync.jira.io/async":       fmt.Sprintf("%t", req.Async),
		"sync.jira.io/parallelism": fmt.Sprintf("%d", req.Parallelism),
		"sync.jira.io/source":      "api-jql-sync",
		"sync.jira.io/jql-query":   req.JQL, // Preserved for debugging
	}

	// Create CRD resource
	crdResource, err := c.createCRDResource(spec, annotations, "jql")
	if err != nil {
		return nil, fmt.Errorf("failed to create CRD resource: %w", err)
	}

	return &ConversionResult{
		CRDSpec:     spec,
		CRDResource: crdResource,
		Annotations: annotations,
	}, nil
}

// Enhanced validation methods using CRD security patterns

func (c *CRDConverter) validateSingleSyncRequestSecure(req *SingleSyncRequest) error {
	if req.IssueKey == "" {
		return fmt.Errorf("issue_key is required")
	}
	if req.Repository == "" {
		return fmt.Errorf("repository is required")
	}

	// Use CRD-compatible issue key validation
	if !c.isValidIssueKeySecure(req.IssueKey) {
		return fmt.Errorf("invalid issue key format: %s (must match pattern ^[A-Z][A-Z0-9]*-[1-9][0-9]*$)", req.IssueKey)
	}

	// Use CRD-compatible repository URL validation
	if !c.isValidRepositoryURLSecure(req.Repository) {
		return fmt.Errorf("invalid repository URL: %s (must be HTTPS or SSH)", req.Repository)
	}

	return c.validateSyncOptionsSecure(req.Options)
}

func (c *CRDConverter) validateBatchSyncRequestSecure(req *BatchSyncRequest) error {
	if len(req.IssueKeys) == 0 {
		return fmt.Errorf("issue_keys is required and cannot be empty")
	}
	if len(req.IssueKeys) > 100 {
		return fmt.Errorf("too many issue keys: %d (maximum 100 allowed)", len(req.IssueKeys))
	}
	if req.Repository == "" {
		return fmt.Errorf("repository is required")
	}

	// Validate each issue key with CRD pattern
	for _, issueKey := range req.IssueKeys {
		if !c.isValidIssueKeySecure(issueKey) {
			return fmt.Errorf("invalid issue key format: %s (must match pattern ^[A-Z][A-Z0-9]*-[1-9][0-9]*$)", issueKey)
		}
	}

	// Use CRD-compatible repository URL validation
	if !c.isValidRepositoryURLSecure(req.Repository) {
		return fmt.Errorf("invalid repository URL: %s (must be HTTPS or SSH)", req.Repository)
	}

	// Validate parallelism
	if req.Parallelism < 0 || req.Parallelism > 10 {
		return fmt.Errorf("parallelism must be between 0 and 10")
	}

	return c.validateSyncOptionsSecure(req.Options)
}

func (c *CRDConverter) validateJQLSyncRequestSecure(req *JQLSyncRequest) error {
	if req.JQL == "" {
		return fmt.Errorf("jql is required")
	}
	if req.Repository == "" {
		return fmt.Errorf("repository is required")
	}

	// Use CRD-compatible JQL validation (prevent SQL injection)
	if !c.isValidJQLSecure(req.JQL) {
		return fmt.Errorf("invalid JQL query contains prohibited characters (;\\<>\" or control characters)")
	}

	// Use CRD-compatible repository URL validation
	if !c.isValidRepositoryURLSecure(req.Repository) {
		return fmt.Errorf("invalid repository URL: %s (must be HTTPS or SSH)", req.Repository)
	}

	// Validate parallelism
	if req.Parallelism < 0 || req.Parallelism > 10 {
		return fmt.Errorf("parallelism must be between 0 and 10")
	}

	return c.validateSyncOptionsSecure(req.Options)
}

func (c *CRDConverter) validateSyncOptionsSecure(options *SyncOptions) error {
	if options == nil {
		return nil
	}

	if options.Concurrency < 0 || options.Concurrency > 10 {
		return fmt.Errorf("concurrency must be between 0 and 10")
	}

	if options.Incremental && options.Force {
		return fmt.Errorf("incremental and force options are mutually exclusive")
	}

	return nil
}

// CRD-compatible validation methods

func (c *CRDConverter) isValidIssueKeySecure(issueKey string) bool {
	// Enhanced pattern matching CRD validation
	pattern := `^[A-Z][A-Z0-9]*-[1-9][0-9]*$`
	matched, err := regexp.MatchString(pattern, issueKey)
	if err != nil {
		return false
	}
	return matched && len(issueKey) >= 4 && len(issueKey) <= 50
}

func (c *CRDConverter) isValidRepositoryURLSecure(repo string) bool {
	// Enhanced pattern matching CRD validation for repository URLs
	pattern := `^(https://[a-zA-Z0-9][a-zA-Z0-9.-]*[a-zA-Z0-9]/[a-zA-Z0-9_.-]+/[a-zA-Z0-9_.-]+(\\.git)?|git@[a-zA-Z0-9][a-zA-Z0-9.-]*[a-zA-Z0-9]:[a-zA-Z0-9_.-]+/[a-zA-Z0-9_.-]+(\\.git)?)$`
	matched, err := regexp.MatchString(pattern, repo)
	if err != nil {
		return false
	}
	return matched && len(repo) >= 1 && len(repo) <= 500
}

func (c *CRDConverter) isValidJQLSecure(jql string) bool {
	// Enhanced pattern matching CRD validation for JQL (prevent SQL injection)
	pattern := `^[^;\\<>"\x00-\x1f]*$`
	matched, err := regexp.MatchString(pattern, jql)
	if err != nil {
		return false
	}
	return matched && len(jql) >= 1 && len(jql) <= 1000
}

// Helper methods

func (c *CRDConverter) applySyncOptionsToSpec(spec *CRDSpec, options *SyncOptions) {
	// Map API options to CRD labels for controller interpretation
	if spec.Labels == nil {
		spec.Labels = make(map[string]string)
	}

	if options.Incremental {
		spec.Labels["sync.jira.io/incremental"] = "true"
	}
	if options.Force {
		spec.Labels["sync.jira.io/force"] = "true"
	}
	if options.DryRun {
		spec.Labels["sync.jira.io/dry-run"] = "true"
	}
	if options.IncludeLinks {
		spec.Labels["sync.jira.io/include-links"] = "true"
	}
	if options.Concurrency > 0 {
		spec.Labels["sync.jira.io/concurrency"] = fmt.Sprintf("%d", options.Concurrency)
	}
	if options.RateLimit > 0 {
		spec.Labels["sync.jira.io/rate-limit"] = options.RateLimit.String()
	}
}

func (c *CRDConverter) createCRDResource(spec *CRDSpec, annotations map[string]string, syncType string) (*unstructured.Unstructured, error) {
	// Generate unique name based on sync type and timestamp
	name := fmt.Sprintf("jirasync-%s-%d", syncType, time.Now().Unix())

	// Convert annotations to map[string]interface{} for deep copy compatibility
	annotationsInterface := make(map[string]interface{})
	for k, v := range annotations {
		annotationsInterface[k] = v
	}

	// Create CRD resource
	crd := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "sync.jira.io/v1alpha1",
			"kind":       "JIRASync",
			"metadata": map[string]interface{}{
				"name":        name,
				"namespace":   "default", // TODO: Make configurable
				"annotations": annotationsInterface,
				"labels": map[string]interface{}{
					"app.kubernetes.io/name":      "jira-sync-operator",
					"app.kubernetes.io/component": "sync-job",
					"sync.jira.io/type":           syncType,
					"sync.jira.io/source":         "api",
				},
			},
		},
	}

	// Convert spec to unstructured format
	specMap, err := c.structToMap(spec)
	if err != nil {
		return nil, fmt.Errorf("failed to convert spec to map: %w", err)
	}

	crd.Object["spec"] = specMap

	return crd, nil
}

func (c *CRDConverter) structToMap(obj interface{}) (map[string]interface{}, error) {
	// Convert struct to map via JSON marshaling/unmarshaling
	// This ensures proper handling of JSON tags and nested structures
	result := make(map[string]interface{})

	switch v := obj.(type) {
	case *CRDSpec:
		result["syncType"] = v.SyncType
		result["priority"] = v.Priority
		result["timeout"] = int64(v.Timeout) // Convert to int64 for deep copy compatibility

		// Handle target (only include non-empty fields)
		target := make(map[string]interface{})
		if len(v.Target.IssueKeys) > 0 {
			// Convert []string to []interface{} for deep copy compatibility
			issueKeysInterface := make([]interface{}, len(v.Target.IssueKeys))
			for i, key := range v.Target.IssueKeys {
				issueKeysInterface[i] = key
			}
			target["issueKeys"] = issueKeysInterface
		}
		if v.Target.JQLQuery != "" {
			target["jqlQuery"] = v.Target.JQLQuery
		}
		if v.Target.ProjectKey != "" {
			target["projectKey"] = v.Target.ProjectKey
		}
		if v.Target.EpicKey != "" {
			target["epicKey"] = v.Target.EpicKey
		}
		result["target"] = target

		// Handle destination
		destination := map[string]interface{}{
			"repository": v.Destination.Repository,
			"branch":     v.Destination.Branch,
			"path":       v.Destination.Path,
		}
		result["destination"] = destination

		// Handle retry policy
		if v.RetryPolicy != nil {
			retryPolicy := map[string]interface{}{
				"maxRetries":        int64(v.RetryPolicy.MaxRetries),   // Convert to int64
				"backoffMultiplier": v.RetryPolicy.BackoffMultiplier,   // float64 is fine
				"initialDelay":      int64(v.RetryPolicy.InitialDelay), // Convert to int64
			}
			result["retryPolicy"] = retryPolicy
		}

		// Handle labels
		if len(v.Labels) > 0 {
			// Convert map[string]string to map[string]interface{} for deep copy compatibility
			labelsInterface := make(map[string]interface{})
			for k, v := range v.Labels {
				labelsInterface[k] = v
			}
			result["labels"] = labelsInterface
		}

	default:
		return nil, fmt.Errorf("unsupported type for conversion: %T", obj)
	}

	return result, nil
}

// ValidateConversion validates that a conversion result is valid
func (c *CRDConverter) ValidateConversion(result *ConversionResult) error {
	if result.CRDSpec == nil {
		return fmt.Errorf("CRD spec is nil")
	}
	if result.CRDResource == nil {
		return fmt.Errorf("CRD resource is nil")
	}

	// Validate CRD spec structure
	if result.CRDSpec.SyncType == "" {
		return fmt.Errorf("syncType is required")
	}

	validSyncTypes := []string{"single", "batch", "jql", "incremental"}
	found := false
	for _, validType := range validSyncTypes {
		if result.CRDSpec.SyncType == validType {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("invalid syncType: %s", result.CRDSpec.SyncType)
	}

	// Validate target has appropriate field for sync type
	switch result.CRDSpec.SyncType {
	case "single", "batch":
		if len(result.CRDSpec.Target.IssueKeys) == 0 {
			return fmt.Errorf("issueKeys required for %s syncType", result.CRDSpec.SyncType)
		}
	case "jql":
		if result.CRDSpec.Target.JQLQuery == "" {
			return fmt.Errorf("jqlQuery required for jql syncType")
		}
	case "incremental":
		if result.CRDSpec.Target.ProjectKey == "" {
			return fmt.Errorf("projectKey required for incremental syncType")
		}
	}

	// Validate destination
	if result.CRDSpec.Destination.Repository == "" {
		return fmt.Errorf("repository is required in destination")
	}

	return nil
}
