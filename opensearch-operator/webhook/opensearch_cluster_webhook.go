/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package webhook

import (
	"context"
	"fmt"

	opensearchv1 "github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/api/opensearch.org/v1"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/helpers"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

//+kubebuilder:webhook:path=/validate-opensearch-org-v1-opensearchcluster,mutating=false,failurePolicy=fail,sideEffects=None,groups=opensearch.org,resources=opensearchclusters,verbs=create;update,versions=v1,name=vopensearchcluster.opensearch.org,admissionReviewVersions=v1

type OpenSearchClusterValidator struct {
	Client  client.Client
	decoder admission.Decoder
}

func (v *OpenSearchClusterValidator) SetupWithManager(mgr ctrl.Manager) error {
	v.Client = mgr.GetClient()
	v.decoder = admission.NewDecoder(mgr.GetScheme())
	return ctrl.NewWebhookManagedBy(mgr).
		For(&opensearchv1.OpenSearchCluster{}).
		WithValidator(v).
		Complete()
}

func (v *OpenSearchClusterValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	cluster := obj.(*opensearchv1.OpenSearchCluster)
	return v.validateTlsConfig(cluster)
}

func (v *OpenSearchClusterValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	oldCluster := oldObj.(*opensearchv1.OpenSearchCluster)
	newCluster := newObj.(*opensearchv1.OpenSearchCluster)

	if !newCluster.DeletionTimestamp.IsZero() {
		return nil, nil
	}

	// Validate storage class changes - storage class is immutable in StatefulSets
	if err := v.validateStorageClassChanges(oldCluster, newCluster); err != nil {
		return nil, err
	}

	return v.validateTlsConfig(newCluster)
}

func (v *OpenSearchClusterValidator) validateStorageClassChanges(oldCluster, newCluster *opensearchv1.OpenSearchCluster) error {
	// Create a map of old node pools by component name for easy lookup
	oldNodePools := make(map[string]*opensearchv1.NodePool)
	for i := range oldCluster.Spec.NodePools {
		nodePool := &oldCluster.Spec.NodePools[i]
		oldNodePools[nodePool.Component] = nodePool
	}

	// Check each new node pool for storage class changes
	for _, newNodePool := range newCluster.Spec.NodePools {
		oldNodePool, exists := oldNodePools[newNodePool.Component]
		if !exists {
			// New node pool, no validation needed
			continue
		}

		// Get old storage class
		var oldStorageClass *string
		if oldNodePool.Persistence != nil && oldNodePool.Persistence.PVC != nil {
			oldStorageClass = oldNodePool.Persistence.PVC.StorageClassName
		}

		// Get new storage class
		var newStorageClass *string
		if newNodePool.Persistence != nil && newNodePool.Persistence.PVC != nil {
			newStorageClass = newNodePool.Persistence.PVC.StorageClassName
		}

		// Compare storage classes (handling nil cases)
		oldSC := ""
		if oldStorageClass != nil {
			oldSC = *oldStorageClass
		}
		newSC := ""
		if newStorageClass != nil {
			newSC = *newStorageClass
		}

		// Reject if storage class has changed
		if oldSC != newSC {
			return fmt.Errorf("storage class cannot be changed for node pool '%s' (was '%s', attempting to change to '%s'). Storage class is immutable in StatefulSets. Please delete the cluster and recreate it with the new storage class", newNodePool.Component, oldSC, newSC)
		}
	}

	return nil
}

func (v *OpenSearchClusterValidator) validateTlsConfig(cluster *opensearchv1.OpenSearchCluster) (admission.Warnings, error) {
	if cluster.Spec.Security == nil || cluster.Spec.Security.Tls == nil {
		return nil, nil
	}

	tlsConfig := cluster.Spec.Security.Tls

	// Validate transport TLS: if enabled=true, transport config must be provided
	if tlsConfig.Transport != nil && tlsConfig.Transport.Enabled != nil && *tlsConfig.Transport.Enabled {
		// Transport TLS is explicitly enabled, config is already provided (Transport != nil)
		// Validation: if enabled=true, we need either Generate=true or existing certs via Secret
		if !tlsConfig.Transport.Generate && tlsConfig.Transport.Secret.Name == "" {
			return nil, fmt.Errorf("transport TLS is enabled but neither generate nor secret is provided")
		}
	}

	// Validate HTTP TLS: if enabled=true, HTTP config must be provided
	if tlsConfig.Http != nil && tlsConfig.Http.Enabled != nil && *tlsConfig.Http.Enabled {
		// HTTP TLS is explicitly enabled, config is already provided (Http != nil)
		// Validation: if enabled=true, we need either Generate=true or existing certs via Secret
		if !tlsConfig.Http.Generate && tlsConfig.Http.Secret.Name == "" {
			return nil, fmt.Errorf("HTTP TLS is enabled but neither generate nor secret is provided")
		}
	}

	// Validate admin secret name: if AdminSecret is empty, tls generate should be true.
	if helpers.IsSecurityPluginEnabled(cluster) {
		if cluster.Spec.Security.Config != nil && cluster.Spec.Security.Config.AdminSecret.Name != "" {
			return nil, nil
		} else {
			if helpers.SecurityChangeVersion(cluster) {
				if tlsConfig.Http != nil && tlsConfig.Http.Generate {
					return nil, nil
				} else {
					return nil, fmt.Errorf("admin secret name is not provided but http.tls generate is not true")
				}
			} else {
				if tlsConfig.Transport != nil && tlsConfig.Transport.Generate {
					return nil, nil
				} else {
					return nil, fmt.Errorf("admin secret name is not provided but transport.tls generate is not true")
				}
			}
		}
	}

	return nil, nil
}

func (v *OpenSearchClusterValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}
