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
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// ClusterReferenceValidator is a generic interface for resources that reference OpenSearch clusters
type ClusterReferenceValidator interface {
	GetOpensearchRef() opsterv1.OpensearchRef
	GetNamespace() string
}

// validateClusterReference validates that the referenced OpenSearch cluster exists
func validateClusterReference(ctx context.Context, client client.Client, resource ClusterReferenceValidator) error {
	cluster := &opsterv1.OpenSearchCluster{}
	err := client.Get(ctx, client.ObjectKey{
		Name:      resource.GetOpensearchRef().Name,
		Namespace: resource.GetNamespace(),
	}, cluster)
	
	if err != nil {
		return fmt.Errorf("referenced OpenSearch cluster '%s' not found: %w", resource.GetOpensearchRef().Name, err)
	}
	
	return nil
}

// validateClusterReferenceUnchanged validates that the cluster reference hasn't changed
func validateClusterReferenceUnchanged(old, new ClusterReferenceValidator, resourceType string) error {
	if old.GetOpensearchRef().Name != new.GetOpensearchRef().Name {
		return fmt.Errorf("cannot change the cluster a %s refers to", resourceType)
	}
	return nil
}

//+kubebuilder:webhook:path=/validate-opensearch-opster-io-v1-opensearchuser,mutating=false,failurePolicy=fail,sideEffects=None,groups=opensearch.opster.io,resources=opensearchusers,verbs=create;update,versions=v1,name=vopensearchuser.kb.io,admissionReviewVersions=v1

type OpenSearchUserValidator struct {
	Client  client.Client
	decoder *admission.Decoder
}

// SetupWithManager sets up the webhook with the Manager.
func (v *OpenSearchUserValidator) SetupWithManager(mgr ctrl.Manager) error {
	v.Client = mgr.GetClient()
	v.decoder = admission.NewDecoder(mgr.GetScheme())
	return ctrl.NewWebhookManagedBy(mgr).
		For(&opsterv1.OpensearchUser{}).
		Complete()
}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (v *OpenSearchUserValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	user := obj.(*opsterv1.OpensearchUser)
	
	// Validate that the OpenSearch cluster reference exists
	if err := validateClusterReference(ctx, v.Client, user); err != nil {
		return nil, err
	}
	
	return nil, nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (v *OpenSearchUserValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	oldUser := oldObj.(*opsterv1.OpensearchUser)
	newUser := newObj.(*opsterv1.OpensearchUser)
	
	// Validate that the OpenSearch cluster reference hasn't changed
	if err := validateClusterReferenceUnchanged(oldUser, newUser, "user"); err != nil {
		return nil, err
	}
	
	return nil, nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (v *OpenSearchUserValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	// No validation needed for deletion
	return nil, nil
}

//+kubebuilder:webhook:path=/validate-opensearch-opster-io-v1-opensearchrole,mutating=false,failurePolicy=fail,sideEffects=None,groups=opensearch.opster.io,resources=opensearchroles,verbs=create;update,versions=v1,name=vopensearchrole.kb.io,admissionReviewVersions=v1

type OpenSearchRoleValidator struct {
	Client  client.Client
	decoder *admission.Decoder
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
	if err := validateClusterReference(ctx, v.Client, role); err != nil {
		return nil, err
	}
	
	return nil, nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (v *OpenSearchRoleValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	oldRole := oldObj.(*opsterv1.OpensearchRole)
	newRole := newObj.(*opsterv1.OpensearchRole)
	
	// Validate that the OpenSearch cluster reference hasn't changed
	if err := validateClusterReferenceUnchanged(oldRole, newRole, "role"); err != nil {
		return nil, err
	}
	
	return nil, nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (v *OpenSearchRoleValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	// No validation needed for deletion
	return nil, nil
}

//+kubebuilder:webhook:path=/validate-opensearch-opster-io-v1-opensearchtenant,mutating=false,failurePolicy=fail,sideEffects=None,groups=opensearch.opster.io,resources=opensearchtenants,verbs=create;update,versions=v1,name=vopensearchtenant.kb.io,admissionReviewVersions=v1

type OpenSearchTenantValidator struct {
	Client  client.Client
	decoder *admission.Decoder
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
	if err := validateClusterReference(ctx, v.Client, tenant); err != nil {
		return nil, err
	}
	
	return nil, nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (v *OpenSearchTenantValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	oldTenant := oldObj.(*opsterv1.OpensearchTenant)
	newTenant := newObj.(*opsterv1.OpensearchTenant)
	
	// Validate that the OpenSearch cluster reference hasn't changed
	if err := validateClusterReferenceUnchanged(oldTenant, newTenant, "tenant"); err != nil {
		return nil, err
	}
	
	return nil, nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (v *OpenSearchTenantValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	// No validation needed for deletion
	return nil, nil
}

//+kubebuilder:webhook:path=/validate-opensearch-opster-io-v1-opensearchuserrolebinding,mutating=false,failurePolicy=fail,sideEffects=None,groups=opensearch.opster.io,resources=opensearchuserrolebindings,verbs=create;update,versions=v1,name=vopensearchuserrolebinding.kb.io,admissionReviewVersions=v1

type OpenSearchUserRoleBindingValidator struct {
	Client  client.Client
	decoder *admission.Decoder
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
	userRoleBinding := obj.(*opsterv1.OpensearchUserRoleBinding)
	
	// Validate that the OpenSearch cluster reference exists
	if err := validateClusterReference(ctx, v.Client, userRoleBinding); err != nil {
		return nil, err
	}
	
	return nil, nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (v *OpenSearchUserRoleBindingValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	oldUserRoleBinding := oldObj.(*opsterv1.OpensearchUserRoleBinding)
	newUserRoleBinding := newObj.(*opsterv1.OpensearchUserRoleBinding)
	
	// Validate that the OpenSearch cluster reference hasn't changed
	if err := validateClusterReferenceUnchanged(oldUserRoleBinding, newUserRoleBinding, "userrolebinding"); err != nil {
		return nil, err
	}
	
	return nil, nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (v *OpenSearchUserRoleBindingValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	// No validation needed for deletion
	return nil, nil
}

//+kubebuilder:webhook:path=/validate-opensearch-opster-io-v1-opensearchactiongroup,mutating=false,failurePolicy=fail,sideEffects=None,groups=opensearch.opster.io,resources=opensearchactiongroups,verbs=create;update,versions=v1,name=vopensearchactiongroup.kb.io,admissionReviewVersions=v1

type OpenSearchActionGroupValidator struct {
	Client  client.Client
	decoder *admission.Decoder
}

// SetupWithManager sets up the webhook with the Manager.
func (v *OpenSearchActionGroupValidator) SetupWithManager(mgr ctrl.Manager) error {
	v.Client = mgr.GetClient()
	v.decoder = admission.NewDecoder(mgr.GetScheme())
	return ctrl.NewWebhookManagedBy(mgr).
		For(&opsterv1.OpensearchActionGroup{}).
		Complete()
}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (v *OpenSearchActionGroupValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	actionGroup := obj.(*opsterv1.OpensearchActionGroup)
	
	// Validate that the OpenSearch cluster reference exists
	if err := validateClusterReference(ctx, v.Client, actionGroup); err != nil {
		return nil, err
	}
	
	return nil, nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (v *OpenSearchActionGroupValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	oldActionGroup := oldObj.(*opsterv1.OpensearchActionGroup)
	newActionGroup := newObj.(*opsterv1.OpensearchActionGroup)
	
	// Validate that the OpenSearch cluster reference hasn't changed
	if err := validateClusterReferenceUnchanged(oldActionGroup, newActionGroup, "actiongroup"); err != nil {
		return nil, err
	}
	
	return nil, nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (v *OpenSearchActionGroupValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	// No validation needed for deletion
	return nil, nil
}

//+kubebuilder:webhook:path=/validate-opensearch-opster-io-v1-opensearchismpolicy,mutating=false,failurePolicy=fail,sideEffects=None,groups=opensearch.opster.io,resources=opensearchismpolicies,verbs=create;update,versions=v1,name=vopensearchismpolicy.kb.io,admissionReviewVersions=v1

type OpenSearchISMPolicyValidator struct {
	Client  client.Client
	decoder *admission.Decoder
}

// SetupWithManager sets up the webhook with the Manager.
func (v *OpenSearchISMPolicyValidator) SetupWithManager(mgr ctrl.Manager) error {
	v.Client = mgr.GetClient()
	v.decoder = admission.NewDecoder(mgr.GetScheme())
	return ctrl.NewWebhookManagedBy(mgr).
		For(&opsterv1.OpensearchISMPolicy{}).
		Complete()
}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (v *OpenSearchISMPolicyValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	ismPolicy := obj.(*opsterv1.OpensearchISMPolicy)
	
	// Validate that the OpenSearch cluster reference exists
	if err := validateClusterReference(ctx, v.Client, ismPolicy); err != nil {
		return nil, err
	}
	
	return nil, nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (v *OpenSearchISMPolicyValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	oldISMPolicy := oldObj.(*opsterv1.OpensearchISMPolicy)
	newISMPolicy := newObj.(*opsterv1.OpensearchISMPolicy)
	
	// Validate that the OpenSearch cluster reference hasn't changed
	if err := validateClusterReferenceUnchanged(oldISMPolicy, newISMPolicy, "resource"); err != nil {
		return nil, err
	}
	
	return nil, nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (v *OpenSearchISMPolicyValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	// No validation needed for deletion
	return nil, nil
}

//+kubebuilder:webhook:path=/validate-opensearch-opster-io-v1-opensearchsnapshotpolicy,mutating=false,failurePolicy=fail,sideEffects=None,groups=opensearch.opster.io,resources=opensearchsnapshotpolicies,verbs=create;update,versions=v1,name=vopensearchsnapshotpolicy.kb.io,admissionReviewVersions=v1

type OpenSearchSnapshotPolicyValidator struct {
	Client  client.Client
	decoder *admission.Decoder
}

// SetupWithManager sets up the webhook with the Manager.
func (v *OpenSearchSnapshotPolicyValidator) SetupWithManager(mgr ctrl.Manager) error {
	v.Client = mgr.GetClient()
	v.decoder = admission.NewDecoder(mgr.GetScheme())
	return ctrl.NewWebhookManagedBy(mgr).
		For(&opsterv1.OpensearchSnapshotPolicy{}).
		Complete()
}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (v *OpenSearchSnapshotPolicyValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	snapshotPolicy := obj.(*opsterv1.OpensearchSnapshotPolicy)
	
	// Validate that the OpenSearch cluster reference exists
	if err := validateClusterReference(ctx, v.Client, snapshotPolicy); err != nil {
		return nil, err
	}
	
	return nil, nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (v *OpenSearchSnapshotPolicyValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	oldSnapshotPolicy := oldObj.(*opsterv1.OpensearchSnapshotPolicy)
	newSnapshotPolicy := newObj.(*opsterv1.OpensearchSnapshotPolicy)
	
	// Validate that the OpenSearch cluster reference hasn't changed
	if err := validateClusterReferenceUnchanged(oldSnapshotPolicy, newSnapshotPolicy, "resource"); err != nil {
		return nil, err
	}
	
	return nil, nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (v *OpenSearchSnapshotPolicyValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	// No validation needed for deletion
	return nil, nil
}
