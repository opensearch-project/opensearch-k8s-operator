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
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

//+kubebuilder:webhook:path=/validate-opensearch-opster-io-v1-opensearchcluster,mutating=false,failurePolicy=fail,sideEffects=None,groups=opensearch.opster.io,resources=opensearchclusters,verbs=create;update,versions=v1,name=vopensearchcluster.opensearch.opster.io,admissionReviewVersions=v1

type OpenSearchClusterValidator struct {
	Client  client.Client
	decoder admission.Decoder
}

func (v *OpenSearchClusterValidator) SetupWithManager(mgr ctrl.Manager) error {
	v.Client = mgr.GetClient()
	v.decoder = admission.NewDecoder(mgr.GetScheme())
	return ctrl.NewWebhookManagedBy(mgr).
		For(&opsterv1.OpenSearchCluster{}).
		WithValidator(v).
		Complete()
}

func (v *OpenSearchClusterValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	cluster := obj.(*opsterv1.OpenSearchCluster)
	return v.validateTlsConfig(cluster)
}

func (v *OpenSearchClusterValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	cluster := newObj.(*opsterv1.OpenSearchCluster)

	if !cluster.DeletionTimestamp.IsZero() {
		return nil, nil
	}

	return v.validateTlsConfig(cluster)
}

func (v *OpenSearchClusterValidator) validateTlsConfig(cluster *opsterv1.OpenSearchCluster) (admission.Warnings, error) {
	if cluster.Spec.Security == nil || cluster.Spec.Security.Tls == nil {
		return nil, nil
	}

	tlsConfig := cluster.Spec.Security.Tls

	// Validate transport TLS: if enabled=true, transport config must be provided
	if tlsConfig.Transport != nil && tlsConfig.Transport.Enabled != nil && *tlsConfig.Transport.Enabled {
		// Transport TLS is explicitly enabled, config is already provided (Transport != nil)
		// Validation: if enabled=true, we need either Generate=true or existing certs via Secret
		if !tlsConfig.Transport.Generate && tlsConfig.Transport.Secret.Name == "" {
			return nil, fmt.Errorf("transport TLS is enabled but neither generate nor secret is provided")
		}
	}

	// Validate HTTP TLS: if enabled=true, HTTP config must be provided
	if tlsConfig.Http != nil && tlsConfig.Http.Enabled != nil && *tlsConfig.Http.Enabled {
		// HTTP TLS is explicitly enabled, config is already provided (Http != nil)
		// Validation: if enabled=true, we need either Generate=true or existing certs via Secret
		if !tlsConfig.Http.Generate && tlsConfig.Http.Secret.Name == "" {
			return nil, fmt.Errorf("HTTP TLS is enabled but neither generate nor secret is provided")
		}
	}

	return nil, nil
}

func (v *OpenSearchClusterValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}
