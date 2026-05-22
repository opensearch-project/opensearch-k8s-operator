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

	opsterv1 "github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/api/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

//+kubebuilder:webhook:path=/validate-opensearch-opster-io-v1-opensearchcluster,mutating=false,failurePolicy=fail,sideEffects=None,groups=opensearch.opster.io,resources=opensearchclusters,verbs=create;update,versions=v1,name=vopensearchcluster.opensearch.opster.io,admissionReviewVersions=v1

// OpenSearchClusterLegacyValidator validates old API group resources (opensearch.opster.io/v1)
// It denies all user updates to support revert functionality - only the operator can update old CRs
type OpenSearchClusterLegacyValidator struct {
	Client  client.Client
	decoder admission.Decoder
}

func (v *OpenSearchClusterLegacyValidator) SetupWithManager(mgr ctrl.Manager) error {
	v.Client = mgr.GetClient()
	v.decoder = admission.NewDecoder(mgr.GetScheme())
	return ctrl.NewWebhookManagedBy(mgr).
		For(&opsterv1.OpenSearchCluster{}).
		WithValidator(v).
		Complete()
}

// ValidateCreate implements webhook.Validator
func (v *OpenSearchClusterLegacyValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return nil, fmt.Errorf("opensearch.opster.io/v1 API group is deprecated. Please use opensearch.org/v1 instead. Creation of old API group OpenSearchCluster resources is not allowed")
}

// ValidateUpdate implements webhook.Validator
func (v *OpenSearchClusterLegacyValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	oldCluster := oldObj.(*opsterv1.OpenSearchCluster)
	newCluster := newObj.(*opsterv1.OpenSearchCluster)

	// Allow deletion
	if !newCluster.DeletionTimestamp.IsZero() {
		return nil, nil
	}

	// Check if this is a status-only update
	if isStatusOnlyUpdate(oldCluster.Spec, newCluster.Spec) {
		return nil, nil
	}

	// Deny spec changes
	return nil, fmt.Errorf("opensearch.opster.io/v1 API group is deprecated. Direct updates to old API group OpenSearchCluster resources are not allowed. Please update the opensearch.org/v1 resource instead")
}

// ValidateDelete implements webhook.Validator
func (v *OpenSearchClusterLegacyValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}
