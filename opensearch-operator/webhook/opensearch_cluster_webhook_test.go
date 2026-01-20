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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var _ = Describe("OpenSearchClusterValidator", func() {
	var (
		validator  *OpenSearchClusterValidator
		ctx        context.Context
		scheme     *runtime.Scheme
		fakeClient client.Client
	)

	BeforeEach(func() {
		ctx = context.Background()
		scheme = runtime.NewScheme()
		_ = opensearchv1.AddToScheme(scheme)
		_ = corev1.AddToScheme(scheme)
		fakeClient = fake.NewClientBuilder().WithScheme(scheme).Build()
		validator = &OpenSearchClusterValidator{
			Client: fakeClient,
		}
		validator.decoder = admission.NewDecoder(scheme)
	})

	Describe("ValidateCreate", func() {
		It("should allow valid cluster creation", func() {
			cluster := &opensearchv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "default",
				},
				Spec: opensearchv1.ClusterSpec{
					General: opensearchv1.GeneralConfig{
						Version: "2.19.4",
					},
					NodePools: []opensearchv1.NodePool{
						{
							Component: "masters",
							Replicas:  3,
						},
					},
				},
			}

			warnings, err := validator.ValidateCreate(ctx, cluster)
			Expect(err).NotTo(HaveOccurred())
			Expect(warnings).To(BeEmpty())
		})

		It("should reject transport TLS enabled without generate or secret", func() {
			enabled := true
			cluster := &opensearchv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "default",
				},
				Spec: opensearchv1.ClusterSpec{
					General: opensearchv1.GeneralConfig{
						Version: "2.19.4",
					},
					Security: &opensearchv1.Security{
						Tls: &opensearchv1.TlsConfig{
							Transport: &opensearchv1.TlsConfigTransport{
								Enabled:  &enabled,
								Generate: false,
								TlsCertificateConfig: opensearchv1.TlsCertificateConfig{
									Secret: corev1.LocalObjectReference{Name: ""}, // Empty secret name
								},
							},
						},
					},
				},
			}

			warnings, err := validator.ValidateCreate(ctx, cluster)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("transport TLS is enabled but neither generate nor secret is provided"))
			Expect(warnings).To(BeEmpty())
		})

		It("should reject HTTP TLS enabled without generate or secret", func() {
			enabled := true
			cluster := &opensearchv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "default",
				},
				Spec: opensearchv1.ClusterSpec{
					General: opensearchv1.GeneralConfig{
						Version: "2.19.4",
					},
					Security: &opensearchv1.Security{
						Tls: &opensearchv1.TlsConfig{
							Http: &opensearchv1.TlsConfigHttp{
								Enabled:  &enabled,
								Generate: false,
								TlsCertificateConfig: opensearchv1.TlsCertificateConfig{
									Secret: corev1.LocalObjectReference{Name: ""}, // Empty secret name
								},
							},
						},
					},
				},
			}

			warnings, err := validator.ValidateCreate(ctx, cluster)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("HTTP TLS is enabled but neither generate nor secret is provided"))
			Expect(warnings).To(BeEmpty())
		})

		It("should allow transport TLS with generate enabled", func() {
			enabled := true
			cluster := &opensearchv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "default",
				},
				Spec: opensearchv1.ClusterSpec{
					General: opensearchv1.GeneralConfig{
						Version: "2.19.4",
					},
					Security: &opensearchv1.Security{
						Tls: &opensearchv1.TlsConfig{
							Transport: &opensearchv1.TlsConfigTransport{
								Enabled:  &enabled,
								Generate: true,
							},
						},
					},
				},
			}

			warnings, err := validator.ValidateCreate(ctx, cluster)
			Expect(err).NotTo(HaveOccurred())
			Expect(warnings).To(BeEmpty())
		})

		It("should allow transport TLS with secret provided", func() {
			enabled := true
			cluster := &opensearchv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "default",
				},
				Spec: opensearchv1.ClusterSpec{
					General: opensearchv1.GeneralConfig{
						Version: "2.19.4",
					},
					Security: &opensearchv1.Security{
						Tls: &opensearchv1.TlsConfig{
							Transport: &opensearchv1.TlsConfigTransport{
								Enabled:  &enabled,
								Generate: false,
								TlsCertificateConfig: opensearchv1.TlsCertificateConfig{
									Secret: corev1.LocalObjectReference{Name: "my-tls-secret"},
								},
							},
						},
					},
				},
			}

			warnings, err := validator.ValidateCreate(ctx, cluster)
			Expect(err).NotTo(HaveOccurred())
			Expect(warnings).To(BeEmpty())
		})
	})

	Describe("ValidateUpdate", func() {
		It("should allow deletion", func() {
			now := metav1.Now()
			oldCluster := &opensearchv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
			}
			newCluster := &opensearchv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "test-cluster",
					DeletionTimestamp: &now,
				},
			}

			warnings, err := validator.ValidateUpdate(ctx, oldCluster, newCluster)
			Expect(err).NotTo(HaveOccurred())
			Expect(warnings).To(BeEmpty())
		})

		It("should reject storage class changes", func() {
			oldStorageClass := "old-storage-class"
			newStorageClass := "new-storage-class"
			oldCluster := &opensearchv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: opensearchv1.ClusterSpec{
					NodePools: []opensearchv1.NodePool{
						{
							Component: "masters",
							Persistence: &opensearchv1.PersistenceConfig{
								PersistenceSource: opensearchv1.PersistenceSource{
									PVC: &opensearchv1.PVCSource{
										StorageClassName: &oldStorageClass,
									},
								},
							},
						},
					},
				},
			}
			newCluster := &opensearchv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: opensearchv1.ClusterSpec{
					NodePools: []opensearchv1.NodePool{
						{
							Component: "masters",
							Persistence: &opensearchv1.PersistenceConfig{
								PersistenceSource: opensearchv1.PersistenceSource{
									PVC: &opensearchv1.PVCSource{
										StorageClassName: &newStorageClass,
									},
								},
							},
						},
					},
				},
			}

			warnings, err := validator.ValidateUpdate(ctx, oldCluster, newCluster)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("storage class cannot be changed"))
			Expect(warnings).To(BeEmpty())
		})

		It("should allow storage class to remain the same", func() {
			storageClass := "my-storage-class"
			oldCluster := &opensearchv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: opensearchv1.ClusterSpec{
					NodePools: []opensearchv1.NodePool{
						{
							Component: "masters",
							Persistence: &opensearchv1.PersistenceConfig{
								PersistenceSource: opensearchv1.PersistenceSource{
									PVC: &opensearchv1.PVCSource{
										StorageClassName: &storageClass,
									},
								},
							},
						},
					},
				},
			}
			newCluster := &opensearchv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: opensearchv1.ClusterSpec{
					NodePools: []opensearchv1.NodePool{
						{
							Component: "masters",
							Persistence: &opensearchv1.PersistenceConfig{
								PersistenceSource: opensearchv1.PersistenceSource{
									PVC: &opensearchv1.PVCSource{
										StorageClassName: &storageClass,
									},
								},
							},
						},
					},
				},
			}

			warnings, err := validator.ValidateUpdate(ctx, oldCluster, newCluster)
			Expect(err).NotTo(HaveOccurred())
			Expect(warnings).To(BeEmpty())
		})

		It("should allow adding new node pools", func() {
			oldCluster := &opensearchv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: opensearchv1.ClusterSpec{
					NodePools: []opensearchv1.NodePool{
						{
							Component: "masters",
						},
					},
				},
			}
			newCluster := &opensearchv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: opensearchv1.ClusterSpec{
					NodePools: []opensearchv1.NodePool{
						{
							Component: "masters",
						},
						{
							Component: "data",
						},
					},
				},
			}

			warnings, err := validator.ValidateUpdate(ctx, oldCluster, newCluster)
			Expect(err).NotTo(HaveOccurred())
			Expect(warnings).To(BeEmpty())
		})
	})

	Describe("ValidateDelete", func() {
		It("should always allow deletion", func() {
			cluster := &opensearchv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
			}

			warnings, err := validator.ValidateDelete(ctx, cluster)
			Expect(err).NotTo(HaveOccurred())
			Expect(warnings).To(BeEmpty())
		})
	})
})
