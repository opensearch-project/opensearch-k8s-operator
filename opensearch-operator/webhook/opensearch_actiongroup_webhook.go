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
	opsterv1 "github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/api/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

//+kubebuilder:webhook:path=/validate-opensearch-org-v1-opensearchactiongroup,mutating=false,failurePolicy=fail,sideEffects=None,groups=opensearch.org,resources=opensearchactiongroups,verbs=create;update,versions=v1,name=vopensearchactiongroup.opensearch.org,admissionReviewVersions=v1

type OpenSearchActionGroupValidator struct {
	Client  client.Client
	decoder admission.Decoder
}

// SetupWithManager sets up the webhook with the Manager.
func (v *OpenSearchActionGroupValidator) SetupWithManager(mgr ctrl.Manager) error {
	v.Client = mgr.GetClient()
	v.decoder = admission.NewDecoder(mgr.GetScheme())
	return ctrl.NewWebhookManagedBy(mgr).
		For(&opensearchv1.OpensearchActionGroup{}).
		Complete()
}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (v *OpenSearchActionGroupValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	actionGroup := obj.(*opensearchv1.OpensearchActionGroup)

	// Validate that the OpenSearch cluster reference exists
	if err := v.validateClusterReference(ctx, actionGroup); err != nil {
		return nil, err
	}

	// Validate that allowedActions is not empty
	if len(actionGroup.Spec.AllowedActions) == 0 {
		return nil, fmt.Errorf("allowedActions cannot be empty")
	}

	return nil, nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (v *OpenSearchActionGroupValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	oldActionGroup := oldObj.(*opensearchv1.OpensearchActionGroup)
	newActionGroup := newObj.(*opensearchv1.OpensearchActionGroup)

	// Validate that the OpenSearch cluster reference hasn't changed
	if err := v.validateClusterReferenceUnchanged(oldActionGroup, newActionGroup); err != nil {
		return nil, err
	}

	// Validate that allowedActions is not empty
	if len(newActionGroup.Spec.AllowedActions) == 0 {
		return nil, fmt.Errorf("allowedActions cannot be empty")
	}

	return nil, nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (v *OpenSearchActionGroupValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	// No validation needed for deletion
	return nil, nil
}

// validateClusterReference validates that the referenced OpenSearch cluster exists
func (v *OpenSearchActionGroupValidator) validateClusterReference(ctx context.Context, actionGroup *opensearchv1.OpensearchActionGroup) error {
	// Try new API group first
	cluster := &opensearchv1.OpenSearchCluster{}
	err := v.Client.Get(ctx, types.NamespacedName{
		Name:      actionGroup.Spec.OpensearchRef.Name,
		Namespace: actionGroup.Namespace,
	}, cluster)

	if err != nil {
		// Fall back to old API group for backward compatibility
		oldCluster := &opsterv1.OpenSearchCluster{}
		if err := v.Client.Get(ctx, types.NamespacedName{
			Name:      actionGroup.Spec.OpensearchRef.Name,
			Namespace: actionGroup.Namespace,
		}, oldCluster); err != nil {
			return fmt.Errorf("referenced OpenSearch cluster '%s' not found: %w", actionGroup.Spec.OpensearchRef.Name, err)
		}
	}

	return nil
}

// validateClusterReferenceUnchanged validates that the cluster reference hasn't changed
func (v *OpenSearchActionGroupValidator) validateClusterReferenceUnchanged(old, new *opensearchv1.OpensearchActionGroup) error {
	if old.Spec.OpensearchRef.Name != new.Spec.OpensearchRef.Name {
		return fmt.Errorf("cannot change the cluster an action group refers to")
	}
	return nil
}
