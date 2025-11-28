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
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/pkg/helpers"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

//+kubebuilder:webhook:path=/validate-opensearch-opster-io-v1-opensearchindextemplate,mutating=false,failurePolicy=fail,sideEffects=None,groups=opensearch.opster.io,resources=opensearchindextemplates,verbs=create;update,versions=v1,name=vopensearchindextemplate.opensearch.opster.io,admissionReviewVersions=v1

type OpenSearchIndexTemplateValidator struct {
	Client  client.Client
	decoder admission.Decoder
}

// SetupWithManager sets up the webhook with the Manager.
func (v *OpenSearchIndexTemplateValidator) SetupWithManager(mgr ctrl.Manager) error {
	v.Client = mgr.GetClient()
	v.decoder = admission.NewDecoder(mgr.GetScheme())
	return ctrl.NewWebhookManagedBy(mgr).
		For(&opsterv1.OpensearchIndexTemplate{}).
		WithValidator(v).
		Complete()
}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (v *OpenSearchIndexTemplateValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	indexTemplate := obj.(*opsterv1.OpensearchIndexTemplate)

	// Validate that the OpenSearch cluster reference exists
	if err := v.validateClusterReference(ctx, indexTemplate); err != nil {
		return nil, err
	}

	return nil, nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (v *OpenSearchIndexTemplateValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	oldIndexTemplate := oldObj.(*opsterv1.OpensearchIndexTemplate)
	newIndexTemplate := newObj.(*opsterv1.OpensearchIndexTemplate)

	// Skip validation for resources being deleted (allow finalizer removal)
	if !newIndexTemplate.DeletionTimestamp.IsZero() {
		return nil, nil
	}

	// Validate that the OpenSearch cluster reference hasn't changed
	if err := v.validateClusterReferenceUnchanged(oldIndexTemplate, newIndexTemplate); err != nil {
		return nil, err
	}

	// Validate that the index template name hasn't changed (if it was previously set)
	if err := v.validateIndexTemplateNameUnchanged(oldIndexTemplate, newIndexTemplate); err != nil {
		return nil, err
	}

	return nil, nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (v *OpenSearchIndexTemplateValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	// No validation needed for deletion
	return nil, nil
}

// validateClusterReference validates that the referenced OpenSearch cluster exists
func (v *OpenSearchIndexTemplateValidator) validateClusterReference(ctx context.Context, indexTemplate *opsterv1.OpensearchIndexTemplate) error {
	cluster := &opsterv1.OpenSearchCluster{}
	err := v.Client.Get(ctx, types.NamespacedName{
		Name:      indexTemplate.Spec.OpensearchRef.Name,
		Namespace: indexTemplate.Namespace,
	}, cluster)

	if err != nil {
		return fmt.Errorf("referenced OpenSearch cluster '%s' not found: %w", indexTemplate.Spec.OpensearchRef.Name, err)
	}

	return nil
}

// validateClusterReferenceUnchanged validates that the cluster reference hasn't changed
func (v *OpenSearchIndexTemplateValidator) validateClusterReferenceUnchanged(old, new *opsterv1.OpensearchIndexTemplate) error {
	if old.Spec.OpensearchRef.Name != new.Spec.OpensearchRef.Name {
		return fmt.Errorf("cannot change the cluster an index template refers to")
	}
	return nil
}

// validateIndexTemplateNameUnchanged validates that the index template name hasn't changed
func (v *OpenSearchIndexTemplateValidator) validateIndexTemplateNameUnchanged(old, new *opsterv1.OpensearchIndexTemplate) error {
	// Only validate if the old template had a name set in status
	if old.Status.IndexTemplateName != "" {
		newTemplateName := helpers.GenIndexTemplateName(new)
		if old.Status.IndexTemplateName != newTemplateName {
			return fmt.Errorf("cannot change the index template name")
		}
	}
	return nil
}
