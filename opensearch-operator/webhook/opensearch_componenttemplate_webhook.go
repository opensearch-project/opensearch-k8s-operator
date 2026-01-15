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
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/helpers"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

//+kubebuilder:webhook:path=/validate-opensearch-org-v1-opensearchcomponenttemplate,mutating=false,failurePolicy=fail,sideEffects=None,groups=opensearch.org,resources=opensearchcomponenttemplates,verbs=create;update,versions=v1,name=vopensearchcomponenttemplate.opensearch.org,admissionReviewVersions=v1

type OpenSearchComponentTemplateValidator struct {
	Client  client.Client
	decoder admission.Decoder
}

// SetupWithManager sets up the webhook with the Manager.
func (v *OpenSearchComponentTemplateValidator) SetupWithManager(mgr ctrl.Manager) error {
	v.Client = mgr.GetClient()
	v.decoder = admission.NewDecoder(mgr.GetScheme())
	return ctrl.NewWebhookManagedBy(mgr).
		For(&opensearchv1.OpensearchComponentTemplate{}).
		Complete()
}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (v *OpenSearchComponentTemplateValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	componentTemplate := obj.(*opensearchv1.OpensearchComponentTemplate)

	// Validate that the OpenSearch cluster reference exists
	if err := v.validateClusterReference(ctx, componentTemplate); err != nil {
		return nil, err
	}

	return nil, nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (v *OpenSearchComponentTemplateValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	oldComponentTemplate := oldObj.(*opensearchv1.OpensearchComponentTemplate)
	newComponentTemplate := newObj.(*opensearchv1.OpensearchComponentTemplate)

	// Validate that the OpenSearch cluster reference hasn't changed
	if err := v.validateClusterReferenceUnchanged(oldComponentTemplate, newComponentTemplate); err != nil {
		return nil, err
	}

	// Validate that the component template name hasn't changed (if it was previously set)
	if err := v.validateComponentTemplateNameUnchanged(oldComponentTemplate, newComponentTemplate); err != nil {
		return nil, err
	}

	return nil, nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (v *OpenSearchComponentTemplateValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	// No validation needed for deletion
	return nil, nil
}

// validateClusterReference validates that the referenced OpenSearch cluster exists
func (v *OpenSearchComponentTemplateValidator) validateClusterReference(ctx context.Context, componentTemplate *opensearchv1.OpensearchComponentTemplate) error {
	// Try new API group first
	cluster := &opensearchv1.OpenSearchCluster{}
	err := v.Client.Get(ctx, types.NamespacedName{
		Name:      componentTemplate.Spec.OpensearchRef.Name,
		Namespace: componentTemplate.Namespace,
	}, cluster)

	if err != nil {
		// Fall back to old API group for backward compatibility
		oldCluster := &opsterv1.OpenSearchCluster{}
		if err := v.Client.Get(ctx, types.NamespacedName{
			Name:      componentTemplate.Spec.OpensearchRef.Name,
			Namespace: componentTemplate.Namespace,
		}, oldCluster); err != nil {
			return fmt.Errorf("referenced OpenSearch cluster '%s' not found: %w", componentTemplate.Spec.OpensearchRef.Name, err)
		}
	}

	return nil
}

// validateClusterReferenceUnchanged validates that the cluster reference hasn't changed
func (v *OpenSearchComponentTemplateValidator) validateClusterReferenceUnchanged(old, new *opensearchv1.OpensearchComponentTemplate) error {
	if old.Spec.OpensearchRef.Name != new.Spec.OpensearchRef.Name {
		return fmt.Errorf("cannot change the cluster a component template refers to")
	}
	return nil
}

// validateComponentTemplateNameUnchanged validates that the component template name hasn't changed
func (v *OpenSearchComponentTemplateValidator) validateComponentTemplateNameUnchanged(old, new *opensearchv1.OpensearchComponentTemplate) error {
	// Only validate if the old template had a name set in status
	if old.Status.ComponentTemplateName != "" {
		newTemplateName := helpers.GenComponentTemplateName(new)
		if old.Status.ComponentTemplateName != newTemplateName {
			return fmt.Errorf("cannot change the component template name")
		}
	}
	return nil
}
