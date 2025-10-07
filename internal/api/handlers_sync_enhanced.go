package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

// SyncMode determines how sync operations are executed
type SyncMode string

const (
	SyncModeDirectJob SyncMode = "direct-job" // v0.4.0 mode - direct Job creation
	SyncModeCRD       SyncMode = "crd"        // v0.4.1 mode - CRD creation + controller
	SyncModeHybrid    SyncMode = "hybrid"     // Support both modes based on request
)

// EnhancedSyncServer provides v0.4.1 CRD-compatible sync operations
type EnhancedSyncServer struct {
	*Server                        // Embed existing server for backward compatibility
	crdConverter *CRDConverter     // API-to-CRD conversion
	k8sClient    dynamic.Interface // Kubernetes client for CRD operations
	syncMode     SyncMode          // Operation mode
}

// CRDSyncResponse represents a CRD-based sync response
type CRDSyncResponse struct {
	*SyncResponse                  // Embed standard response
	CRDName        string          `json:"crd_name"`
	CRDNamespace   string          `json:"crd_namespace"`
	Mode           SyncMode        `json:"mode"`
	ConversionInfo *ConversionInfo `json:"conversion_info,omitempty"`
}

// ConversionInfo provides details about API-to-CRD conversion
type ConversionInfo struct {
	OriginalRequest string            `json:"original_request_type"`
	CRDFields       map[string]string `json:"crd_fields"`
	Annotations     map[string]string `json:"annotations"`
	Warnings        []string          `json:"warnings,omitempty"`
}

// NewEnhancedSyncServer creates a new enhanced sync server
func NewEnhancedSyncServer(server *Server, k8sClient dynamic.Interface, mode SyncMode) *EnhancedSyncServer {
	return &EnhancedSyncServer{
		Server:       server,
		crdConverter: NewCRDConverter(),
		k8sClient:    k8sClient,
		syncMode:     mode,
	}
}

// HandleEnhancedSingleSync handles single issue sync with CRD support
func (s *EnhancedSyncServer) HandleEnhancedSingleSync(w http.ResponseWriter, r *http.Request) {
	var req SingleSyncRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid JSON request body", err.Error())
		return
	}

	// Determine operation mode based on request headers or server config
	mode := s.determineSyncMode(r)

	switch mode {
	case SyncModeDirectJob:
		// Use existing v0.4.0 implementation
		s.handleSingleSync(w, r)
		return

	case SyncModeCRD:
		// Use new v0.4.1 CRD-based implementation
		response, err := s.createCRDSingleSync(r.Context(), &req)
		if err != nil {
			s.writeError(w, http.StatusInternalServerError, "CRD_SYNC_ERROR", "Failed to create CRD sync", err.Error())
			return
		}
		s.writeJSON(w, http.StatusAccepted, response)
		return

	case SyncModeHybrid:
		// Try CRD first, fallback to direct job if needed
		response, err := s.createCRDSingleSync(r.Context(), &req)
		if err != nil {
			// Fallback to direct job mode
			s.handleSingleSync(w, r)
			return
		}
		s.writeJSON(w, http.StatusAccepted, response)
		return

	default:
		s.writeError(w, http.StatusInternalServerError, "INVALID_MODE", "Invalid sync mode configuration", string(mode))
		return
	}
}

// HandleEnhancedBatchSync handles batch issue sync with CRD support
func (s *EnhancedSyncServer) HandleEnhancedBatchSync(w http.ResponseWriter, r *http.Request) {
	var req BatchSyncRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid JSON request body", err.Error())
		return
	}

	// Determine operation mode
	mode := s.determineSyncMode(r)

	switch mode {
	case SyncModeDirectJob:
		s.handleBatchSync(w, r)
		return

	case SyncModeCRD:
		response, err := s.createCRDBatchSync(r.Context(), &req)
		if err != nil {
			s.writeError(w, http.StatusInternalServerError, "CRD_SYNC_ERROR", "Failed to create CRD batch sync", err.Error())
			return
		}
		s.writeJSON(w, http.StatusAccepted, response)
		return

	case SyncModeHybrid:
		response, err := s.createCRDBatchSync(r.Context(), &req)
		if err != nil {
			s.handleBatchSync(w, r)
			return
		}
		s.writeJSON(w, http.StatusAccepted, response)
		return

	default:
		s.writeError(w, http.StatusInternalServerError, "INVALID_MODE", "Invalid sync mode configuration", string(mode))
		return
	}
}

// handleEnhancedJQLSync handles JQL query sync with CRD support
func (s *EnhancedSyncServer) HandleEnhancedJQLSync(w http.ResponseWriter, r *http.Request) {
	var req JQLSyncRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid JSON request body", err.Error())
		return
	}

	// Determine operation mode
	mode := s.determineSyncMode(r)

	switch mode {
	case SyncModeDirectJob:
		s.handleJQLSync(w, r)
		return

	case SyncModeCRD:
		response, err := s.createCRDJQLSync(r.Context(), &req)
		if err != nil {
			s.writeError(w, http.StatusInternalServerError, "CRD_SYNC_ERROR", "Failed to create CRD JQL sync", err.Error())
			return
		}
		s.writeJSON(w, http.StatusAccepted, response)
		return

	case SyncModeHybrid:
		response, err := s.createCRDJQLSync(r.Context(), &req)
		if err != nil {
			s.handleJQLSync(w, r)
			return
		}
		s.writeJSON(w, http.StatusAccepted, response)
		return

	default:
		s.writeError(w, http.StatusInternalServerError, "INVALID_MODE", "Invalid sync mode configuration", string(mode))
		return
	}
}

// CRD creation methods

func (s *EnhancedSyncServer) createCRDSingleSync(ctx context.Context, req *SingleSyncRequest) (*CRDSyncResponse, error) {
	// Convert API request to CRD
	result, err := s.crdConverter.ConvertSingleSync(req)
	if err != nil {
		return nil, fmt.Errorf("conversion failed: %w", err)
	}

	// Validate conversion
	if err := s.crdConverter.ValidateConversion(result); err != nil {
		return nil, fmt.Errorf("conversion validation failed: %w", err)
	}

	// Create CRD in Kubernetes
	crdName, crdNamespace, err := s.createCRDResource(ctx, result)
	if err != nil {
		return nil, fmt.Errorf("CRD creation failed: %w", err)
	}

	// Create response
	response := &CRDSyncResponse{
		SyncResponse: &SyncResponse{
			JobID:     fmt.Sprintf("crd-%s", crdName),
			Status:    "crd-created",
			CreatedAt: time.Now(),
		},
		CRDName:      crdName,
		CRDNamespace: crdNamespace,
		Mode:         s.syncMode,
		ConversionInfo: &ConversionInfo{
			OriginalRequest: "SingleSyncRequest",
			CRDFields: map[string]string{
				"syncType":   result.CRDSpec.SyncType,
				"repository": result.CRDSpec.Destination.Repository,
				"issueKeys":  fmt.Sprintf("%v", result.CRDSpec.Target.IssueKeys),
			},
			Annotations: result.Annotations,
		},
	}

	return response, nil
}

func (s *EnhancedSyncServer) createCRDBatchSync(ctx context.Context, req *BatchSyncRequest) (*CRDSyncResponse, error) {
	// Convert API request to CRD
	result, err := s.crdConverter.ConvertBatchSync(req)
	if err != nil {
		return nil, fmt.Errorf("conversion failed: %w", err)
	}

	// Validate conversion
	if err := s.crdConverter.ValidateConversion(result); err != nil {
		return nil, fmt.Errorf("conversion validation failed: %w", err)
	}

	// Create CRD in Kubernetes
	crdName, crdNamespace, err := s.createCRDResource(ctx, result)
	if err != nil {
		return nil, fmt.Errorf("CRD creation failed: %w", err)
	}

	// Create response
	response := &CRDSyncResponse{
		SyncResponse: &SyncResponse{
			JobID:     fmt.Sprintf("crd-%s", crdName),
			Status:    "crd-created",
			CreatedAt: time.Now(),
		},
		CRDName:      crdName,
		CRDNamespace: crdNamespace,
		Mode:         s.syncMode,
		ConversionInfo: &ConversionInfo{
			OriginalRequest: "BatchSyncRequest",
			CRDFields: map[string]string{
				"syncType":   result.CRDSpec.SyncType,
				"repository": result.CRDSpec.Destination.Repository,
				"issueKeys":  fmt.Sprintf("%v", result.CRDSpec.Target.IssueKeys),
				"issueCount": fmt.Sprintf("%d", len(result.CRDSpec.Target.IssueKeys)),
			},
			Annotations: result.Annotations,
		},
	}

	return response, nil
}

func (s *EnhancedSyncServer) createCRDJQLSync(ctx context.Context, req *JQLSyncRequest) (*CRDSyncResponse, error) {
	// Convert API request to CRD
	result, err := s.crdConverter.ConvertJQLSync(req)
	if err != nil {
		return nil, fmt.Errorf("conversion failed: %w", err)
	}

	// Validate conversion
	if err := s.crdConverter.ValidateConversion(result); err != nil {
		return nil, fmt.Errorf("conversion validation failed: %w", err)
	}

	// Create CRD in Kubernetes
	crdName, crdNamespace, err := s.createCRDResource(ctx, result)
	if err != nil {
		return nil, fmt.Errorf("CRD creation failed: %w", err)
	}

	// Create response
	response := &CRDSyncResponse{
		SyncResponse: &SyncResponse{
			JobID:     fmt.Sprintf("crd-%s", crdName),
			Status:    "crd-created",
			CreatedAt: time.Now(),
		},
		CRDName:      crdName,
		CRDNamespace: crdNamespace,
		Mode:         s.syncMode,
		ConversionInfo: &ConversionInfo{
			OriginalRequest: "JQLSyncRequest",
			CRDFields: map[string]string{
				"syncType":   result.CRDSpec.SyncType,
				"repository": result.CRDSpec.Destination.Repository,
				"jqlQuery":   result.CRDSpec.Target.JQLQuery,
			},
			Annotations: result.Annotations,
		},
	}

	return response, nil
}

// Helper methods

func (s *EnhancedSyncServer) determineSyncMode(r *http.Request) SyncMode {
	// Check for explicit mode header
	if modeHeader := r.Header.Get("X-Sync-Mode"); modeHeader != "" {
		switch SyncMode(modeHeader) {
		case SyncModeDirectJob, SyncModeCRD, SyncModeHybrid:
			return SyncMode(modeHeader)
		}
	}

	// Check for CRD preference header
	if r.Header.Get("X-Prefer-CRD") == "true" {
		return SyncModeCRD
	}

	// Use server default
	return s.syncMode
}

func (s *EnhancedSyncServer) createCRDResource(ctx context.Context, result *ConversionResult) (string, string, error) {
	// Define the GVR for JIRASync CRDs
	gvr := schema.GroupVersionResource{
		Group:    "sync.jira.io",
		Version:  "v1alpha1",
		Resource: "jirasyncs",
	}

	// Get namespace from CRD metadata
	namespace, found, err := unstructured.NestedString(result.CRDResource.Object, "metadata", "namespace")
	if err != nil || !found {
		namespace = "default" // Default namespace
	}

	// Create the CRD resource
	created, err := s.k8sClient.Resource(gvr).Namespace(namespace).Create(
		ctx, result.CRDResource, metav1.CreateOptions{})
	if err != nil {
		return "", "", fmt.Errorf("failed to create CRD in Kubernetes: %w", err)
	}

	return created.GetName(), created.GetNamespace(), nil
}

// Additional endpoint handlers for CRD management will be added as needed
