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
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

//+kubebuilder:webhook:path=/validate-opensearch-opster-io-v1-opensearchrole,mutating=false,failurePolicy=fail,sideEffects=None,groups=opensearch.opster.io,resources=opensearchroles,verbs=create;update,versions=v1,name=vopensearchrole.opensearch.opster.io,admissionReviewVersions=v1

type OpenSearchRoleValidator struct {
	Client  client.Client
	decoder admission.Decoder
}

// SetupWithManager sets up the webhook with the Manager.
func (v *OpenSearchRoleValidator) SetupWithManager(mgr ctrl.Manager) error {
	v.Client = mgr.GetClient()
	v.decoder = admission.NewDecoder(mgr.GetScheme())
	return ctrl.NewWebhookManagedBy(mgr).
		For(&opsterv1.OpensearchRole{}).
		Complete()
}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (v *OpenSearchRoleValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	role := obj.(*opsterv1.OpensearchRole)

	// Validate that the OpenSearch cluster reference exists
	if err := v.validateClusterReference(ctx, role); err != nil {
		return nil, err
	}

	// Validate that at least one permission is defined
	if err := v.validatePermissions(role); err != nil {
		return nil, err
	}

	return nil, nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (v *OpenSearchRoleValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	oldRole := oldObj.(*opsterv1.OpensearchRole)
	newRole := newObj.(*opsterv1.OpensearchRole)

	// Validate that the OpenSearch cluster reference hasn't changed
	if err := v.validateClusterReferenceUnchanged(oldRole, newRole); err != nil {
		return nil, err
	}

	// Validate that at least one permission is defined
	if err := v.validatePermissions(newRole); err != nil {
		return nil, err
	}

	return nil, nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (v *OpenSearchRoleValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	// No validation needed for deletion
	return nil, nil
}

// validateClusterReference validates that the referenced OpenSearch cluster exists
func (v *OpenSearchRoleValidator) validateClusterReference(ctx context.Context, role *opsterv1.OpensearchRole) error {
	cluster := &opsterv1.OpenSearchCluster{}
	err := v.Client.Get(ctx, types.NamespacedName{
		Name:      role.Spec.OpensearchRef.Name,
		Namespace: role.Namespace,
	}, cluster)

	if err != nil {
		return fmt.Errorf("referenced OpenSearch cluster '%s' not found: %w", role.Spec.OpensearchRef.Name, err)
	}

	return nil
}

// validateClusterReferenceUnchanged validates that the cluster reference hasn't changed
func (v *OpenSearchRoleValidator) validateClusterReferenceUnchanged(old, new *opsterv1.OpensearchRole) error {
	if old.Spec.OpensearchRef.Name != new.Spec.OpensearchRef.Name {
		return fmt.Errorf("cannot change the cluster a role refers to")
	}
	return nil
}

// validatePermissions validates that at least one permission is defined
func (v *OpenSearchRoleValidator) validatePermissions(role *opsterv1.OpensearchRole) error {
	hasPermissions := len(role.Spec.ClusterPermissions) > 0 ||
		len(role.Spec.IndexPermissions) > 0 ||
		len(role.Spec.TenantPermissions) > 0

	if !hasPermissions {
		return fmt.Errorf("at least one of clusterPermissions, indexPermissions, or tenantPermissions must be defined")
	}

	return nil
}
