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

	opsterv1 "github.com/Opster/opensearch-k8s-operator/opensearch-operator/api/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

//+kubebuilder:webhook:path=/validate-opensearch-opster-io-v1-opensearchtenant,mutating=false,failurePolicy=fail,sideEffects=None,groups=opensearch.opster.io,resources=opensearchtenants,verbs=create;update,versions=v1,name=vopensearchtenant.opensearch.opster.io,admissionReviewVersions=v1

type OpenSearchTenantValidator struct {
	Client  client.Client
	decoder admission.Decoder
}

// SetupWithManager sets up the webhook with the Manager.
func (v *OpenSearchTenantValidator) SetupWithManager(mgr ctrl.Manager) error {
	v.Client = mgr.GetClient()
	v.decoder = admission.NewDecoder(mgr.GetScheme())
	return ctrl.NewWebhookManagedBy(mgr).
		For(&opsterv1.OpensearchTenant{}).
		Complete()
}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (v *OpenSearchTenantValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	tenant := obj.(*opsterv1.OpensearchTenant)

	// Validate that the OpenSearch cluster reference exists
	if err := v.validateClusterReference(ctx, tenant); err != nil {
		return nil, err
	}

	return nil, nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (v *OpenSearchTenantValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	oldTenant := oldObj.(*opsterv1.OpensearchTenant)
	newTenant := newObj.(*opsterv1.OpensearchTenant)

	// Validate that the OpenSearch cluster reference hasn't changed
	if err := v.validateClusterReferenceUnchanged(oldTenant, newTenant); err != nil {
		return nil, err
	}

	return nil, nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (v *OpenSearchTenantValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	// No validation needed for deletion
	return nil, nil
}

// validateClusterReference validates that the referenced OpenSearch cluster exists
func (v *OpenSearchTenantValidator) validateClusterReference(ctx context.Context, tenant *opsterv1.OpensearchTenant) error {
	cluster := &opsterv1.OpenSearchCluster{}
	err := v.Client.Get(ctx, types.NamespacedName{
		Name:      tenant.Spec.OpensearchRef.Name,
		Namespace: tenant.Namespace,
	}, cluster)

	if err != nil {
		return fmt.Errorf("referenced OpenSearch cluster '%s' not found: %w", tenant.Spec.OpensearchRef.Name, err)
	}

	return nil
}

// validateClusterReferenceUnchanged validates that the cluster reference hasn't changed
func (v *OpenSearchTenantValidator) validateClusterReferenceUnchanged(old, new *opsterv1.OpensearchTenant) error {
	if old.Spec.OpensearchRef.Name != new.Spec.OpensearchRef.Name {
		return fmt.Errorf("cannot change the cluster a tenant refers to")
	}
	return nil
}
