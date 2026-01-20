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

var _ = Describe("OpenSearchRoleValidator", func() {
	var (
		validator  *OpenSearchRoleValidator
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
		validator = &OpenSearchRoleValidator{
			Client: fakeClient,
		}
		validator.decoder = admission.NewDecoder(scheme)
	})

	Describe("ValidateCreate", func() {
		It("should allow valid role creation with cluster permissions", func() {
			role := &opensearchv1.OpensearchRole{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-role",
					Namespace: "default",
				},
				Spec: opensearchv1.OpensearchRoleSpec{
					OpensearchRef: corev1.LocalObjectReference{
						Name: "test-cluster",
					},
					ClusterPermissions: []string{"cluster_composite_ops"},
				},
			}

			warnings, err := validator.ValidateCreate(ctx, role)
			Expect(err).NotTo(HaveOccurred())
			Expect(warnings).To(BeEmpty())
		})

		It("should allow valid role creation with index permissions", func() {
			role := &opensearchv1.OpensearchRole{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-role",
					Namespace: "default",
				},
				Spec: opensearchv1.OpensearchRoleSpec{
					OpensearchRef: corev1.LocalObjectReference{
						Name: "test-cluster",
					},
					IndexPermissions: []opensearchv1.IndexPermissionSpec{
						{
							IndexPatterns: []string{"logs-*"},
							AllowedActions: []string{
								"read",
							},
						},
					},
				},
			}

			warnings, err := validator.ValidateCreate(ctx, role)
			Expect(err).NotTo(HaveOccurred())
			Expect(warnings).To(BeEmpty())
		})

		It("should allow valid role creation with tenant permissions", func() {
			role := &opensearchv1.OpensearchRole{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-role",
					Namespace: "default",
				},
				Spec: opensearchv1.OpensearchRoleSpec{
					OpensearchRef: corev1.LocalObjectReference{
						Name: "test-cluster",
					},
					TenantPermissions: []opensearchv1.TenantPermissionsSpec{
						{
							TenantPatterns: []string{"tenant-*"},
							AllowedActions: []string{"kibana_all_write"},
						},
					},
				},
			}

			warnings, err := validator.ValidateCreate(ctx, role)
			Expect(err).NotTo(HaveOccurred())
			Expect(warnings).To(BeEmpty())
		})

		It("should reject role with missing cluster reference", func() {
			role := &opensearchv1.OpensearchRole{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-role",
					Namespace: "default",
				},
				Spec: opensearchv1.OpensearchRoleSpec{
					OpensearchRef: corev1.LocalObjectReference{
						Name: "non-existent-cluster",
					},
					ClusterPermissions: []string{"cluster_composite_ops"},
				},
			}

			warnings, err := validator.ValidateCreate(ctx, role)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("referenced OpenSearch cluster 'non-existent-cluster' not found"))
			Expect(warnings).To(BeEmpty())
		})

		It("should reject role with no permissions", func() {
			role := &opensearchv1.OpensearchRole{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-role",
					Namespace: "default",
				},
				Spec: opensearchv1.OpensearchRoleSpec{
					OpensearchRef: corev1.LocalObjectReference{
						Name: "test-cluster",
					},
					// No permissions specified
				},
			}

			warnings, err := validator.ValidateCreate(ctx, role)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("at least one of clusterPermissions, indexPermissions, or tenantPermissions must be defined"))
			Expect(warnings).To(BeEmpty())
		})

		It("should allow role with old API group cluster reference", func() {
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

			role := &opensearchv1.OpensearchRole{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-role",
					Namespace: "default",
				},
				Spec: opensearchv1.OpensearchRoleSpec{
					OpensearchRef: corev1.LocalObjectReference{
						Name: "old-cluster",
					},
					ClusterPermissions: []string{"cluster_composite_ops"},
				},
			}

			warnings, err := validator.ValidateCreate(ctx, role)
			Expect(err).NotTo(HaveOccurred())
			Expect(warnings).To(BeEmpty())
		})
	})

	Describe("ValidateUpdate", func() {
		It("should allow valid role update", func() {
			oldRole := &opensearchv1.OpensearchRole{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-role",
					Namespace: "default",
				},
				Spec: opensearchv1.OpensearchRoleSpec{
					OpensearchRef: corev1.LocalObjectReference{
						Name: "test-cluster",
					},
					ClusterPermissions: []string{"cluster_composite_ops"},
				},
			}
			newRole := &opensearchv1.OpensearchRole{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-role",
					Namespace: "default",
				},
				Spec: opensearchv1.OpensearchRoleSpec{
					OpensearchRef: corev1.LocalObjectReference{
						Name: "test-cluster",
					},
					ClusterPermissions: []string{"cluster_composite_ops", "cluster_monitor"},
				},
			}

			warnings, err := validator.ValidateUpdate(ctx, oldRole, newRole)
			Expect(err).NotTo(HaveOccurred())
			Expect(warnings).To(BeEmpty())
		})

		It("should reject cluster reference change", func() {
			oldRole := &opensearchv1.OpensearchRole{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-role",
					Namespace: "default",
				},
				Spec: opensearchv1.OpensearchRoleSpec{
					OpensearchRef: corev1.LocalObjectReference{
						Name: "test-cluster",
					},
					ClusterPermissions: []string{"cluster_composite_ops"},
				},
			}
			newRole := &opensearchv1.OpensearchRole{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-role",
					Namespace: "default",
				},
				Spec: opensearchv1.OpensearchRoleSpec{
					OpensearchRef: corev1.LocalObjectReference{
						Name: "different-cluster",
					},
					ClusterPermissions: []string{"cluster_composite_ops"},
				},
			}

			warnings, err := validator.ValidateUpdate(ctx, oldRole, newRole)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("cannot change the cluster a role refers to"))
			Expect(warnings).To(BeEmpty())
		})

		It("should reject update with no permissions", func() {
			oldRole := &opensearchv1.OpensearchRole{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-role",
					Namespace: "default",
				},
				Spec: opensearchv1.OpensearchRoleSpec{
					OpensearchRef: corev1.LocalObjectReference{
						Name: "test-cluster",
					},
					ClusterPermissions: []string{"cluster_composite_ops"},
				},
			}
			newRole := &opensearchv1.OpensearchRole{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-role",
					Namespace: "default",
				},
				Spec: opensearchv1.OpensearchRoleSpec{
					OpensearchRef: corev1.LocalObjectReference{
						Name: "test-cluster",
					},
					// No permissions
				},
			}

			warnings, err := validator.ValidateUpdate(ctx, oldRole, newRole)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("at least one of clusterPermissions, indexPermissions, or tenantPermissions must be defined"))
			Expect(warnings).To(BeEmpty())
		})
	})

	Describe("ValidateDelete", func() {
		It("should always allow deletion", func() {
			role := &opensearchv1.OpensearchRole{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-role",
					Namespace: "default",
				},
			}

			warnings, err := validator.ValidateDelete(ctx, role)
			Expect(err).NotTo(HaveOccurred())
			Expect(warnings).To(BeEmpty())
		})
	})
})
