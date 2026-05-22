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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	opensearchv1 "github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/api/opensearch.org/v1"
	opsterv1 "github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/api/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var _ = Describe("OpenSearchISMPolicyValidator", func() {
	var (
		validator  *OpenSearchISMPolicyValidator
		ctx        context.Context
		scheme     *runtime.Scheme
		fakeClient client.Client
		cluster    *opensearchv1.OpenSearchCluster
	)

	BeforeEach(func() {
		ctx = context.Background()
		scheme = runtime.NewScheme()
		_ = opensearchv1.AddToScheme(scheme)
		_ = opsterv1.AddToScheme(scheme)
		_ = corev1.AddToScheme(scheme)

		cluster = &opensearchv1.OpenSearchCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-cluster",
				Namespace: "default",
			},
			Spec: opensearchv1.ClusterSpec{
				General: opensearchv1.GeneralConfig{
					Version: "2.19.4",
				},
			},
		}

		fakeClient = fake.NewClientBuilder().WithScheme(scheme).WithObjects(cluster).Build()
		validator = &OpenSearchISMPolicyValidator{
			Client: fakeClient,
		}
		validator.decoder = admission.NewDecoder(scheme)
	})

	Describe("ValidateCreate", func() {
		It("should allow valid ISM policy creation", func() {
			policy := &opensearchv1.OpenSearchISMPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-policy",
					Namespace: "default",
				},
				Spec: opensearchv1.OpenSearchISMPolicySpec{
					OpensearchRef: corev1.LocalObjectReference{
						Name: "test-cluster",
					},
					DefaultState: "hot",
					States: []opensearchv1.State{
						{
							Name: "hot",
						},
						{
							Name: "warm",
						},
					},
				},
			}

			warnings, err := validator.ValidateCreate(ctx, policy)
			Expect(err).NotTo(HaveOccurred())
			Expect(warnings).To(BeEmpty())
		})

		It("should reject policy with missing cluster reference", func() {
			policy := &opensearchv1.OpenSearchISMPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-policy",
					Namespace: "default",
				},
				Spec: opensearchv1.OpenSearchISMPolicySpec{
					OpensearchRef: corev1.LocalObjectReference{
						Name: "non-existent-cluster",
					},
					DefaultState: "hot",
					States: []opensearchv1.State{
						{Name: "hot"},
					},
				},
			}

			warnings, err := validator.ValidateCreate(ctx, policy)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("referenced OpenSearch cluster 'non-existent-cluster' not found"))
			Expect(warnings).To(BeEmpty())
		})

		It("should reject policy with empty states", func() {
			policy := &opensearchv1.OpenSearchISMPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-policy",
					Namespace: "default",
				},
				Spec: opensearchv1.OpenSearchISMPolicySpec{
					OpensearchRef: corev1.LocalObjectReference{
						Name: "test-cluster",
					},
					DefaultState: "hot",
					States:       []opensearchv1.State{}, // Empty
				},
			}

			warnings, err := validator.ValidateCreate(ctx, policy)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("states cannot be empty"))
			Expect(warnings).To(BeEmpty())
		})

		It("should reject policy with empty defaultState", func() {
			policy := &opensearchv1.OpenSearchISMPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-policy",
					Namespace: "default",
				},
				Spec: opensearchv1.OpenSearchISMPolicySpec{
					OpensearchRef: corev1.LocalObjectReference{
						Name: "test-cluster",
					},
					DefaultState: "", // Empty
					States: []opensearchv1.State{
						{Name: "hot"},
					},
				},
			}

			warnings, err := validator.ValidateCreate(ctx, policy)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("defaultState cannot be empty"))
			Expect(warnings).To(BeEmpty())
		})

		It("should reject policy where defaultState does not exist in states", func() {
			policy := &opensearchv1.OpenSearchISMPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-policy",
					Namespace: "default",
				},
				Spec: opensearchv1.OpenSearchISMPolicySpec{
					OpensearchRef: corev1.LocalObjectReference{
						Name: "test-cluster",
					},
					DefaultState: "cold", // Not in states
					States: []opensearchv1.State{
						{Name: "hot"},
						{Name: "warm"},
					},
				},
			}

			warnings, err := validator.ValidateCreate(ctx, policy)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("defaultState 'cold' does not exist in states"))
			Expect(warnings).To(BeEmpty())
		})

		It("should allow policy with old API group cluster reference", func() {
			oldCluster := &opsterv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "old-cluster",
					Namespace: "default",
				},
				Spec: opsterv1.ClusterSpec{
					General: opsterv1.GeneralConfig{
						Version: "2.19.4",
					},
				},
			}
			clientWithOldCluster := fake.NewClientBuilder().WithScheme(scheme).WithObjects(oldCluster).Build()
			validator.Client = clientWithOldCluster

			policy := &opensearchv1.OpenSearchISMPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-policy",
					Namespace: "default",
				},
				Spec: opensearchv1.OpenSearchISMPolicySpec{
					OpensearchRef: corev1.LocalObjectReference{
						Name: "old-cluster",
					},
					DefaultState: "hot",
					States: []opensearchv1.State{
						{Name: "hot"},
					},
				},
			}

			warnings, err := validator.ValidateCreate(ctx, policy)
			Expect(err).NotTo(HaveOccurred())
			Expect(warnings).To(BeEmpty())
		})
	})

	Describe("ValidateUpdate", func() {
		It("should allow valid policy update", func() {
			oldPolicy := &opensearchv1.OpenSearchISMPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-policy",
					Namespace: "default",
				},
				Spec: opensearchv1.OpenSearchISMPolicySpec{
					OpensearchRef: corev1.LocalObjectReference{
						Name: "test-cluster",
					},
					DefaultState: "hot",
					States: []opensearchv1.State{
						{Name: "hot"},
					},
				},
			}
			newPolicy := &opensearchv1.OpenSearchISMPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-policy",
					Namespace: "default",
				},
				Spec: opensearchv1.OpenSearchISMPolicySpec{
					OpensearchRef: corev1.LocalObjectReference{
						Name: "test-cluster",
					},
					DefaultState: "hot",
					States: []opensearchv1.State{
						{Name: "hot"},
						{Name: "warm"},
					},
				},
			}

			warnings, err := validator.ValidateUpdate(ctx, oldPolicy, newPolicy)
			Expect(err).NotTo(HaveOccurred())
			Expect(warnings).To(BeEmpty())
		})

		It("should reject cluster reference change", func() {
			oldPolicy := &opensearchv1.OpenSearchISMPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-policy",
					Namespace: "default",
				},
				Spec: opensearchv1.OpenSearchISMPolicySpec{
					OpensearchRef: corev1.LocalObjectReference{
						Name: "test-cluster",
					},
					DefaultState: "hot",
					States: []opensearchv1.State{
						{Name: "hot"},
					},
				},
			}
			newPolicy := &opensearchv1.OpenSearchISMPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-policy",
					Namespace: "default",
				},
				Spec: opensearchv1.OpenSearchISMPolicySpec{
					OpensearchRef: corev1.LocalObjectReference{
						Name: "different-cluster",
					},
					DefaultState: "hot",
					States: []opensearchv1.State{
						{Name: "hot"},
					},
				},
			}

			warnings, err := validator.ValidateUpdate(ctx, oldPolicy, newPolicy)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("cannot change the cluster an ISM policy refers to"))
			Expect(warnings).To(BeEmpty())
		})

		It("should reject policy ID change when status has policy ID", func() {
			oldPolicy := &opensearchv1.OpenSearchISMPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-policy",
					Namespace: "default",
				},
				Spec: opensearchv1.OpenSearchISMPolicySpec{
					OpensearchRef: corev1.LocalObjectReference{
						Name: "test-cluster",
					},
					PolicyID:     "original-policy-id",
					DefaultState: "hot",
					States: []opensearchv1.State{
						{Name: "hot"},
					},
				},
				Status: opensearchv1.OpensearchISMPolicyStatus{
					PolicyId: "original-policy-id",
				},
			}
			newPolicy := &opensearchv1.OpenSearchISMPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-policy",
					Namespace: "default",
				},
				Spec: opensearchv1.OpenSearchISMPolicySpec{
					OpensearchRef: corev1.LocalObjectReference{
						Name: "test-cluster",
					},
					PolicyID:     "new-policy-id", // Changed
					DefaultState: "hot",
					States: []opensearchv1.State{
						{Name: "hot"},
					},
				},
			}

			warnings, err := validator.ValidateUpdate(ctx, oldPolicy, newPolicy)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("cannot change the ISM policy ID"))
			Expect(warnings).To(BeEmpty())
		})

		It("should reject update with empty states", func() {
			oldPolicy := &opensearchv1.OpenSearchISMPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-policy",
					Namespace: "default",
				},
				Spec: opensearchv1.OpenSearchISMPolicySpec{
					OpensearchRef: corev1.LocalObjectReference{
						Name: "test-cluster",
					},
					DefaultState: "hot",
					States: []opensearchv1.State{
						{Name: "hot"},
					},
				},
			}
			newPolicy := &opensearchv1.OpenSearchISMPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-policy",
					Namespace: "default",
				},
				Spec: opensearchv1.OpenSearchISMPolicySpec{
					OpensearchRef: corev1.LocalObjectReference{
						Name: "test-cluster",
					},
					DefaultState: "hot",
					States:       []opensearchv1.State{}, // Empty
				},
			}

			warnings, err := validator.ValidateUpdate(ctx, oldPolicy, newPolicy)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("states cannot be empty"))
			Expect(warnings).To(BeEmpty())
		})
	})

	Describe("ValidateDelete", func() {
		It("should always allow deletion", func() {
			policy := &opensearchv1.OpenSearchISMPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-policy",
					Namespace: "default",
				},
			}

			warnings, err := validator.ValidateDelete(ctx, policy)
			Expect(err).NotTo(HaveOccurred())
			Expect(warnings).To(BeEmpty())
		})
	})
})
