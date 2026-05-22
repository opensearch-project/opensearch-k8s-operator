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

var _ = Describe("OpenSearchUserValidator", func() {
	var (
		validator  *OpenSearchUserValidator
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
		validator = &OpenSearchUserValidator{
			Client: fakeClient,
		}
		validator.decoder = admission.NewDecoder(scheme)
	})

	Describe("ValidateCreate", func() {
		It("should allow valid user creation", func() {
			user := &opensearchv1.OpensearchUser{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-user",
					Namespace: "default",
				},
				Spec: opensearchv1.OpensearchUserSpec{
					OpensearchRef: corev1.LocalObjectReference{
						Name: "test-cluster",
					},
					PasswordFrom: corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "user-password",
						},
						Key: "password",
					},
				},
			}

			warnings, err := validator.ValidateCreate(ctx, user)
			Expect(err).NotTo(HaveOccurred())
			Expect(warnings).To(BeEmpty())
		})

		It("should reject user with missing cluster reference", func() {
			user := &opensearchv1.OpensearchUser{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-user",
					Namespace: "default",
				},
				Spec: opensearchv1.OpensearchUserSpec{
					OpensearchRef: corev1.LocalObjectReference{
						Name: "non-existent-cluster",
					},
					PasswordFrom: corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "user-password",
						},
						Key: "password",
					},
				},
			}

			warnings, err := validator.ValidateCreate(ctx, user)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("referenced OpenSearch cluster 'non-existent-cluster' not found"))
			Expect(warnings).To(BeEmpty())
		})

		It("should allow user with old API group cluster reference", func() {
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

			user := &opensearchv1.OpensearchUser{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-user",
					Namespace: "default",
				},
				Spec: opensearchv1.OpensearchUserSpec{
					OpensearchRef: corev1.LocalObjectReference{
						Name: "old-cluster",
					},
					PasswordFrom: corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "user-password",
						},
						Key: "password",
					},
				},
			}

			warnings, err := validator.ValidateCreate(ctx, user)
			Expect(err).NotTo(HaveOccurred())
			Expect(warnings).To(BeEmpty())
		})

		It("should reject user with missing passwordFrom.name", func() {
			user := &opensearchv1.OpensearchUser{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-user",
					Namespace: "default",
				},
				Spec: opensearchv1.OpensearchUserSpec{
					OpensearchRef: corev1.LocalObjectReference{
						Name: "test-cluster",
					},
					PasswordFrom: corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "", // Empty name
						},
						Key: "password",
					},
				},
			}

			warnings, err := validator.ValidateCreate(ctx, user)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("passwordFrom.name must be specified"))
			Expect(warnings).To(BeEmpty())
		})

		It("should reject user with missing passwordFrom.key", func() {
			user := &opensearchv1.OpensearchUser{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-user",
					Namespace: "default",
				},
				Spec: opensearchv1.OpensearchUserSpec{
					OpensearchRef: corev1.LocalObjectReference{
						Name: "test-cluster",
					},
					PasswordFrom: corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "user-password",
						},
						Key: "", // Empty key
					},
				},
			}

			warnings, err := validator.ValidateCreate(ctx, user)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("passwordFrom.key must be specified"))
			Expect(warnings).To(BeEmpty())
		})
	})

	Describe("ValidateUpdate", func() {
		It("should allow valid user update", func() {
			oldUser := &opensearchv1.OpensearchUser{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-user",
					Namespace: "default",
				},
				Spec: opensearchv1.OpensearchUserSpec{
					OpensearchRef: corev1.LocalObjectReference{
						Name: "test-cluster",
					},
					PasswordFrom: corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "user-password",
						},
						Key: "password",
					},
				},
			}
			newUser := &opensearchv1.OpensearchUser{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-user",
					Namespace: "default",
				},
				Spec: opensearchv1.OpensearchUserSpec{
					OpensearchRef: corev1.LocalObjectReference{
						Name: "test-cluster",
					},
					PasswordFrom: corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "user-password",
						},
						Key: "password",
					},
				},
			}

			warnings, err := validator.ValidateUpdate(ctx, oldUser, newUser)
			Expect(err).NotTo(HaveOccurred())
			Expect(warnings).To(BeEmpty())
		})

		It("should reject cluster reference change", func() {
			oldUser := &opensearchv1.OpensearchUser{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-user",
					Namespace: "default",
				},
				Spec: opensearchv1.OpensearchUserSpec{
					OpensearchRef: corev1.LocalObjectReference{
						Name: "test-cluster",
					},
					PasswordFrom: corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "user-password",
						},
						Key: "password",
					},
				},
			}
			newUser := &opensearchv1.OpensearchUser{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-user",
					Namespace: "default",
				},
				Spec: opensearchv1.OpensearchUserSpec{
					OpensearchRef: corev1.LocalObjectReference{
						Name: "different-cluster",
					},
					PasswordFrom: corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "user-password",
						},
						Key: "password",
					},
				},
			}

			warnings, err := validator.ValidateUpdate(ctx, oldUser, newUser)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("cannot change the cluster a user refers to"))
			Expect(warnings).To(BeEmpty())
		})

		It("should reject update with missing passwordFrom.name", func() {
			oldUser := &opensearchv1.OpensearchUser{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-user",
					Namespace: "default",
				},
				Spec: opensearchv1.OpensearchUserSpec{
					OpensearchRef: corev1.LocalObjectReference{
						Name: "test-cluster",
					},
					PasswordFrom: corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "user-password",
						},
						Key: "password",
					},
				},
			}
			newUser := &opensearchv1.OpensearchUser{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-user",
					Namespace: "default",
				},
				Spec: opensearchv1.OpensearchUserSpec{
					OpensearchRef: corev1.LocalObjectReference{
						Name: "test-cluster",
					},
					PasswordFrom: corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "", // Empty name
						},
						Key: "password",
					},
				},
			}

			warnings, err := validator.ValidateUpdate(ctx, oldUser, newUser)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("passwordFrom.name must be specified"))
			Expect(warnings).To(BeEmpty())
		})
	})

	Describe("ValidateDelete", func() {
		It("should always allow deletion", func() {
			user := &opensearchv1.OpensearchUser{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-user",
					Namespace: "default",
				},
			}

			warnings, err := validator.ValidateDelete(ctx, user)
			Expect(err).NotTo(HaveOccurred())
			Expect(warnings).To(BeEmpty())
		})
	})
})
