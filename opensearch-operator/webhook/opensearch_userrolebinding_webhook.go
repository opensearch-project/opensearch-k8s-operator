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

//+kubebuilder:webhook:path=/validate-opensearch-opster-io-v1-opensearchuserrolebinding,mutating=false,failurePolicy=fail,sideEffects=None,groups=opensearch.opster.io,resources=opensearchuserrolebindings,verbs=create;update,versions=v1,name=vopensearchuserrolebinding.opensearch.opster.io,admissionReviewVersions=v1

type OpenSearchUserRoleBindingValidator struct {
	Client  client.Client
	decoder admission.Decoder
}

// SetupWithManager sets up the webhook with the Manager.
func (v *OpenSearchUserRoleBindingValidator) SetupWithManager(mgr ctrl.Manager) error {
	v.Client = mgr.GetClient()
	v.decoder = admission.NewDecoder(mgr.GetScheme())
	return ctrl.NewWebhookManagedBy(mgr).
		For(&opsterv1.OpensearchUserRoleBinding{}).
		Complete()
}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (v *OpenSearchUserRoleBindingValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	binding := obj.(*opsterv1.OpensearchUserRoleBinding)

	// Validate that the OpenSearch cluster reference exists
	if err := v.validateClusterReference(ctx, binding); err != nil {
		return nil, err
	}

	// Validate that roles is not empty
	if len(binding.Spec.Roles) == 0 {
		return nil, fmt.Errorf("roles cannot be empty")
	}

	// Validate that at least one of users or backendRoles is specified
	if err := v.validateSubjects(binding); err != nil {
		return nil, err
	}

	return nil, nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (v *OpenSearchUserRoleBindingValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	oldBinding := oldObj.(*opsterv1.OpensearchUserRoleBinding)
	newBinding := newObj.(*opsterv1.OpensearchUserRoleBinding)

	// Validate that the OpenSearch cluster reference hasn't changed
	if err := v.validateClusterReferenceUnchanged(oldBinding, newBinding); err != nil {
		return nil, err
	}

	// Validate that roles is not empty
	if len(newBinding.Spec.Roles) == 0 {
		return nil, fmt.Errorf("roles cannot be empty")
	}

	// Validate that at least one of users or backendRoles is specified
	if err := v.validateSubjects(newBinding); err != nil {
		return nil, err
	}

	return nil, nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (v *OpenSearchUserRoleBindingValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	// No validation needed for deletion
	return nil, nil
}

// validateClusterReference validates that the referenced OpenSearch cluster exists
func (v *OpenSearchUserRoleBindingValidator) validateClusterReference(ctx context.Context, binding *opsterv1.OpensearchUserRoleBinding) error {
	cluster := &opsterv1.OpenSearchCluster{}
	err := v.Client.Get(ctx, types.NamespacedName{
		Name:      binding.Spec.OpensearchRef.Name,
		Namespace: binding.Namespace,
	}, cluster)

	if err != nil {
		return fmt.Errorf("referenced OpenSearch cluster '%s' not found: %w", binding.Spec.OpensearchRef.Name, err)
	}

	return nil
}

// validateClusterReferenceUnchanged validates that the cluster reference hasn't changed
func (v *OpenSearchUserRoleBindingValidator) validateClusterReferenceUnchanged(old, new *opsterv1.OpensearchUserRoleBinding) error {
	if old.Spec.OpensearchRef.Name != new.Spec.OpensearchRef.Name {
		return fmt.Errorf("cannot change the cluster a user role binding refers to")
	}
	return nil
}

// validateSubjects validates that at least one of users or backendRoles is specified
func (v *OpenSearchUserRoleBindingValidator) validateSubjects(binding *opsterv1.OpensearchUserRoleBinding) error {
	if len(binding.Spec.Users) == 0 && len(binding.Spec.BackendRoles) == 0 {
		return fmt.Errorf("at least one of users or backendRoles must be specified")
	}
	return nil
}
