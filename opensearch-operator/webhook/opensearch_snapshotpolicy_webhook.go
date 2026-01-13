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

//+kubebuilder:webhook:path=/validate-opensearch-opster-io-v1-opensearchsnapshotpolicy,mutating=false,failurePolicy=fail,sideEffects=None,groups=opensearch.opster.io,resources=opensearchsnapshotpolicies,verbs=create;update,versions=v1,name=vopensearchsnapshotpolicy.opensearch.opster.io,admissionReviewVersions=v1

type OpenSearchSnapshotPolicyValidator struct {
	Client  client.Client
	decoder admission.Decoder
}

func (v *OpenSearchSnapshotPolicyValidator) SetupWithManager(mgr ctrl.Manager) error {
	v.Client = mgr.GetClient()
	v.decoder = admission.NewDecoder(mgr.GetScheme())
	return ctrl.NewWebhookManagedBy(mgr).
		For(&opsterv1.OpensearchSnapshotPolicy{}).
		Complete()
}

func (v *OpenSearchSnapshotPolicyValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	policy := obj.(*opsterv1.OpensearchSnapshotPolicy)

	if err := v.validateClusterReference(ctx, policy); err != nil {
		return nil, err
	}

	if policy.Spec.PolicyName == "" {
		return nil, fmt.Errorf("policyName cannot be empty")
	}

	if policy.Spec.SnapshotConfig.Repository == "" {
		return nil, fmt.Errorf("snapshotConfig.repository cannot be empty")
	}

	if policy.Spec.Creation.Schedule.Cron.Expression == "" {
		return nil, fmt.Errorf("creation.schedule.cron.expression cannot be empty")
	}

	return nil, nil
}

func (v *OpenSearchSnapshotPolicyValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	oldPolicy := oldObj.(*opsterv1.OpensearchSnapshotPolicy)
	newPolicy := newObj.(*opsterv1.OpensearchSnapshotPolicy)

	if err := v.validateClusterReferenceUnchanged(oldPolicy, newPolicy); err != nil {
		return nil, err
	}

	if err := v.validatePolicyNameUnchanged(oldPolicy, newPolicy); err != nil {
		return nil, err
	}

	if newPolicy.Spec.PolicyName == "" {
		return nil, fmt.Errorf("policyName cannot be empty")
	}

	if newPolicy.Spec.SnapshotConfig.Repository == "" {
		return nil, fmt.Errorf("snapshotConfig.repository cannot be empty")
	}

	if newPolicy.Spec.Creation.Schedule.Cron.Expression == "" {
		return nil, fmt.Errorf("creation.schedule.cron.expression cannot be empty")
	}

	return nil, nil
}

func (v *OpenSearchSnapshotPolicyValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

func (v *OpenSearchSnapshotPolicyValidator) validateClusterReference(ctx context.Context, policy *opsterv1.OpensearchSnapshotPolicy) error {
	cluster := &opsterv1.OpenSearchCluster{}
	err := v.Client.Get(ctx, types.NamespacedName{
		Name:      policy.Spec.OpensearchRef.Name,
		Namespace: policy.Namespace,
	}, cluster)

	if err != nil {
		return fmt.Errorf("referenced OpenSearch cluster '%s' not found: %w", policy.Spec.OpensearchRef.Name, err)
	}
	return nil
}

func (v *OpenSearchSnapshotPolicyValidator) validateClusterReferenceUnchanged(old, new *opsterv1.OpensearchSnapshotPolicy) error {
	if old.Spec.OpensearchRef.Name != new.Spec.OpensearchRef.Name {
		return fmt.Errorf("cannot change the cluster a snapshot policy refers to")
	}
	return nil
}

func (v *OpenSearchSnapshotPolicyValidator) validatePolicyNameUnchanged(old, new *opsterv1.OpensearchSnapshotPolicy) error {
	if old.Status.SnapshotPolicyName != "" && old.Status.SnapshotPolicyName != new.Spec.PolicyName {
		return fmt.Errorf("cannot change the snapshot policy name")
	}
	return nil
}
