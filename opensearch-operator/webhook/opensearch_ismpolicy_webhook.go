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

//+kubebuilder:webhook:path=/validate-opensearch-org-v1-opensearchismpolicy,mutating=false,failurePolicy=fail,sideEffects=None,groups=opensearch.org,resources=opensearchismpolicies,verbs=create;update,versions=v1,name=vopensearchismpolicy.opensearch.org,admissionReviewVersions=v1

type OpenSearchISMPolicyValidator struct {
	Client  client.Client
	decoder admission.Decoder
}

func (v *OpenSearchISMPolicyValidator) SetupWithManager(mgr ctrl.Manager) error {
	v.Client = mgr.GetClient()
	v.decoder = admission.NewDecoder(mgr.GetScheme())
	return ctrl.NewWebhookManagedBy(mgr).
		For(&opensearchv1.OpenSearchISMPolicy{}).
		WithValidator(v).
		Complete()
}

func (v *OpenSearchISMPolicyValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	policy := obj.(*opensearchv1.OpenSearchISMPolicy)

	if err := v.validateClusterReference(ctx, policy); err != nil {
		return nil, err
	}

	if len(policy.Spec.States) == 0 {
		return nil, fmt.Errorf("states cannot be empty")
	}

	if policy.Spec.DefaultState == "" {
		return nil, fmt.Errorf("defaultState cannot be empty")
	}

	if err := v.validateDefaultStateExists(policy); err != nil {
		return nil, err
	}

	return nil, nil
}

func (v *OpenSearchISMPolicyValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	oldPolicy := oldObj.(*opensearchv1.OpenSearchISMPolicy)
	newPolicy := newObj.(*opensearchv1.OpenSearchISMPolicy)

	if err := v.validateClusterReferenceUnchanged(oldPolicy, newPolicy); err != nil {
		return nil, err
	}

	if err := v.validatePolicyIDUnchanged(oldPolicy, newPolicy); err != nil {
		return nil, err
	}

	if len(newPolicy.Spec.States) == 0 {
		return nil, fmt.Errorf("states cannot be empty")
	}

	if newPolicy.Spec.DefaultState == "" {
		return nil, fmt.Errorf("defaultState cannot be empty")
	}

	if err := v.validateDefaultStateExists(newPolicy); err != nil {
		return nil, err
	}

	return nil, nil
}

func (v *OpenSearchISMPolicyValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

func (v *OpenSearchISMPolicyValidator) validateClusterReference(ctx context.Context, policy *opensearchv1.OpenSearchISMPolicy) error {
	// Try new API group first
	cluster := &opensearchv1.OpenSearchCluster{}
	err := v.Client.Get(ctx, types.NamespacedName{
		Name:      policy.Spec.OpensearchRef.Name,
		Namespace: policy.Namespace,
	}, cluster)

	if err != nil {
		// Fall back to old API group for backward compatibility
		oldCluster := &opsterv1.OpenSearchCluster{}
		if err := v.Client.Get(ctx, types.NamespacedName{
			Name:      policy.Spec.OpensearchRef.Name,
			Namespace: policy.Namespace,
		}, oldCluster); err != nil {
			return fmt.Errorf("referenced OpenSearch cluster '%s' not found: %w", policy.Spec.OpensearchRef.Name, err)
		}
	}
	return nil
}

func (v *OpenSearchISMPolicyValidator) validateClusterReferenceUnchanged(old, new *opensearchv1.OpenSearchISMPolicy) error {
	if old.Spec.OpensearchRef.Name != new.Spec.OpensearchRef.Name {
		return fmt.Errorf("cannot change the cluster an ISM policy refers to")
	}
	return nil
}

func (v *OpenSearchISMPolicyValidator) validatePolicyIDUnchanged(old, new *opensearchv1.OpenSearchISMPolicy) error {
	if old.Status.PolicyId != "" {
		newPolicyID := new.Spec.PolicyID
		if newPolicyID == "" {
			newPolicyID = new.Name
		}
		if old.Status.PolicyId != newPolicyID {
			return fmt.Errorf("cannot change the ISM policy ID")
		}
	}
	return nil
}

func (v *OpenSearchISMPolicyValidator) validateDefaultStateExists(policy *opensearchv1.OpenSearchISMPolicy) error {
	for _, state := range policy.Spec.States {
		if state.Name == policy.Spec.DefaultState {
			return nil
		}
	}
	return fmt.Errorf("defaultState '%s' does not exist in states", policy.Spec.DefaultState)
}
