package controllers

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatortypes "github.com/chambrid/jira-cdc-git/internal/operator/types"
)

// checkAPIServerDependency checks if there's a ready APIServer in the same namespace
func (r *JIRASyncReconciler) checkAPIServerDependency(ctx context.Context, jiraSync *operatortypes.JIRASync) (bool, error) {
	log := r.Log.WithValues("jirasync", client.ObjectKeyFromObject(jiraSync))

	// Look for APIServer resources in the same namespace
	apiServerList := &operatortypes.APIServerList{}
	err := r.List(ctx, apiServerList, client.InNamespace(jiraSync.Namespace))
	if err != nil {
		return false, fmt.Errorf("failed to list APIServer resources: %w", err)
	}

	if len(apiServerList.Items) == 0 {
		log.Info("No APIServer found in namespace, JIRASync operations cannot proceed", "namespace", jiraSync.Namespace)
		return false, nil
	}

	// Check if any APIServer is ready
	var readyAPIServer *operatortypes.APIServer
	for i := range apiServerList.Items {
		apiServer := &apiServerList.Items[i]

		// Check if this APIServer is in Running phase and has Ready condition
		if apiServer.Status.Phase == APIServerPhaseRunning {
			for _, condition := range apiServer.Status.Conditions {
				if condition.Type == "Ready" && condition.Status == metav1.ConditionTrue {
					readyAPIServer = apiServer
					break
				}
			}
		}

		if readyAPIServer != nil {
			break
		}
	}

	if readyAPIServer == nil {
		log.Info("No ready APIServer found in namespace", "namespace", jiraSync.Namespace, "totalAPIServers", len(apiServerList.Items))
		return false, nil
	}

	// Update the API client configuration to use the ready API server
	if readyAPIServer.Status.Endpoint != "" {
		log.Info("Found ready APIServer, updating API client configuration",
			"apiServerName", readyAPIServer.Name,
			"endpoint", readyAPIServer.Status.Endpoint)

		// Update the APIClient host if it's different
		if r.APIHost != readyAPIServer.Status.Endpoint {
			r.APIHost = readyAPIServer.Status.Endpoint
			// Recreate the API client with the new endpoint
			r.APIClient = r.APIClient.WithHost(readyAPIServer.Status.Endpoint)
		}
	}

	log.V(1).Info("API server dependency check passed", "apiServerName", readyAPIServer.Name, "endpoint", readyAPIServer.Status.Endpoint)
	return true, nil
}
