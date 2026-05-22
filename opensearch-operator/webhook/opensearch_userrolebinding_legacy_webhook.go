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

//+kubebuilder:webhook:path=/validate-opensearch-opster-io-v1-opensearchuserrolebinding,mutating=false,failurePolicy=fail,sideEffects=None,groups=opensearch.opster.io,resources=opensearchuserrolebindings,verbs=create;update,versions=v1,name=vopensearchuserrolebinding.opensearch.opster.io,admissionReviewVersions=v1

// OpenSearchUserRoleBindingLegacyValidator validates old API group resources (opensearch.opster.io/v1)
// It denies all user updates to support revert functionality - only the operator can update old CRs
type OpenSearchUserRoleBindingLegacyValidator struct {
	Client  client.Client
	decoder admission.Decoder
}

func (v *OpenSearchUserRoleBindingLegacyValidator) SetupWithManager(mgr ctrl.Manager) error {
	v.Client = mgr.GetClient()
	v.decoder = admission.NewDecoder(mgr.GetScheme())
	return ctrl.NewWebhookManagedBy(mgr).
		For(&opsterv1.OpensearchUserRoleBinding{}).
		WithValidator(v).
		Complete()
}

// ValidateCreate implements webhook.Validator
func (v *OpenSearchUserRoleBindingLegacyValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return nil, fmt.Errorf("opensearch.opster.io/v1 API group is deprecated. Please use opensearch.org/v1 instead. Creation of old API group OpensearchUserRoleBinding resources is not allowed")
}

// ValidateUpdate implements webhook.Validator
func (v *OpenSearchUserRoleBindingLegacyValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	oldBinding := oldObj.(*opsterv1.OpensearchUserRoleBinding)
	newBinding := newObj.(*opsterv1.OpensearchUserRoleBinding)

	// Allow deletion
	if !newBinding.DeletionTimestamp.IsZero() {
		return nil, nil
	}

	// Check if this is a status-only update
	if isStatusOnlyUpdate(oldBinding.Spec, newBinding.Spec) {
		return nil, nil
	}

	// Deny spec changes
	return nil, fmt.Errorf("opensearch.opster.io/v1 API group is deprecated. Direct updates to old API group OpensearchUserRoleBinding resources are not allowed. Please update the opensearch.org/v1 resource instead")
}

// ValidateDelete implements webhook.Validator
func (v *OpenSearchUserRoleBindingLegacyValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}
