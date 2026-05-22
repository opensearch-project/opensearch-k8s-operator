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

package controllers

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	opensearchv1 "github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/api/opensearch.org/v1"
	opsterv1 "github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/api/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("ClusterMigrationReconciler", func() {
	var (
		reconciler *ClusterMigrationReconciler
		ctx        context.Context
		scheme     *runtime.Scheme
		fakeClient client.Client
		req        ctrl.Request
	)

	BeforeEach(func() {
		ctx = context.Background()
		scheme = runtime.NewScheme()
		_ = opensearchv1.AddToScheme(scheme)
		_ = opsterv1.AddToScheme(scheme)
		fakeClient = fake.NewClientBuilder().WithScheme(scheme).Build()
		reconciler = &ClusterMigrationReconciler{
			Client: fakeClient,
			Scheme: scheme,
		}
		req = ctrl.Request{
			NamespacedName: types.NamespacedName{
				Name:      "test-cluster",
				Namespace: "default",
			},
		}
	})

	Describe("Reconcile - New Cluster Deletion", func() {
		It("should add migration finalizer to new cluster", func() {
			newCluster := &opensearchv1.OpenSearchCluster{
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
			Expect(fakeClient.Create(ctx, newCluster)).To(Succeed())

			result, err := reconciler.Reconcile(ctx, req)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Requeue).To(BeTrue())

			// Verify finalizer was added
			updatedCluster := &opensearchv1.OpenSearchCluster{}
			Expect(fakeClient.Get(ctx, req.NamespacedName, updatedCluster)).To(Succeed())
			Expect(containsString(updatedCluster.Finalizers, MigrationFinalizer)).To(BeTrue())
		})
	})

	Describe("Reconcile - Old Cluster Migration", func() {
		It("should create new cluster from old cluster when ready", func() {
			oldCluster := &opsterv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "default",
				},
				Spec: opsterv1.ClusterSpec{
					General: opsterv1.GeneralConfig{
						Version: "2.19.4",
					},
				},
				Status: opsterv1.ClusterStatus{
					Phase: opsterv1.PhaseRunning, // Ready for migration
				},
			}
			Expect(fakeClient.Create(ctx, oldCluster)).To(Succeed())

			result, err := reconciler.Reconcile(ctx, req)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(BeNumerically(">", 0))

			// Verify new cluster was created
			newCluster := &opensearchv1.OpenSearchCluster{}
			Expect(fakeClient.Get(ctx, req.NamespacedName, newCluster)).To(Succeed())
			Expect(newCluster.Spec.General.Version).To(Equal("2.19.4"))
		})

		It("should not migrate old cluster that is not ready", func() {
			oldCluster := &opsterv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "default",
				},
				Spec: opsterv1.ClusterSpec{
					General: opsterv1.GeneralConfig{
						Version: "2.19.4",
					},
				},
				Status: opsterv1.ClusterStatus{
					Phase: opsterv1.PhasePending, // Not ready
				},
			}
			Expect(fakeClient.Create(ctx, oldCluster)).To(Succeed())

			result, err := reconciler.Reconcile(ctx, req)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(Equal(30 * time.Second))

			// Verify new cluster was NOT created
			newCluster := &opensearchv1.OpenSearchCluster{}
			err = fakeClient.Get(ctx, req.NamespacedName, newCluster)
			Expect(err).To(HaveOccurred()) // Should not exist
		})

		It("should add migration finalizer to old cluster", func() {
			oldCluster := &opsterv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "default",
				},
				Spec: opsterv1.ClusterSpec{
					General: opsterv1.GeneralConfig{
						Version: "2.19.4",
					},
				},
				Status: opsterv1.ClusterStatus{
					Phase: opsterv1.PhaseRunning,
				},
			}
			Expect(fakeClient.Create(ctx, oldCluster)).To(Succeed())

			_, err := reconciler.Reconcile(ctx, req)
			Expect(err).NotTo(HaveOccurred())

			// Verify finalizer was added
			updatedCluster := &opsterv1.OpenSearchCluster{}
			Expect(fakeClient.Get(ctx, req.NamespacedName, updatedCluster)).To(Succeed())
			Expect(containsString(updatedCluster.Finalizers, MigrationFinalizer)).To(BeTrue())
		})
	})

	Describe("handleOldClusterDeletion", func() {
		It("should allow deletion when new cluster exists", func() {
			now := metav1.Now()
			oldCluster := &opsterv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "test-cluster",
					Namespace:         "default",
					DeletionTimestamp: &now,
					Finalizers:        []string{MigrationFinalizer},
				},
			}
			Expect(fakeClient.Create(ctx, oldCluster)).To(Succeed())

			// Create corresponding new cluster
			newCluster := &opensearchv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "default",
				},
			}
			Expect(fakeClient.Create(ctx, newCluster)).To(Succeed())

			result, err := reconciler.handleOldClusterDeletion(ctx, oldCluster)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Requeue).To(BeFalse())

			// Verify finalizer was removed
			updatedCluster := &opsterv1.OpenSearchCluster{}
			Expect(fakeClient.Get(ctx, req.NamespacedName, updatedCluster)).To(Succeed())
			Expect(containsString(updatedCluster.Finalizers, MigrationFinalizer)).To(BeFalse())
		})

		It("should allow deletion when annotation indicates new cluster deletion", func() {
			now := metav1.Now()
			oldCluster := &opsterv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "test-cluster",
					Namespace:         "default",
					DeletionTimestamp: &now,
					Finalizers:        []string{MigrationFinalizer},
					Annotations: map[string]string{
						DeletedByNewResourceAnnotation: "true",
					},
				},
			}
			Expect(fakeClient.Create(ctx, oldCluster)).To(Succeed())

			result, err := reconciler.handleOldClusterDeletion(ctx, oldCluster)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Requeue).To(BeFalse())

			// Verify finalizer was removed
			updatedCluster := &opsterv1.OpenSearchCluster{}
			Expect(fakeClient.Get(ctx, req.NamespacedName, updatedCluster)).To(Succeed())
			Expect(containsString(updatedCluster.Finalizers, MigrationFinalizer)).To(BeFalse())
		})

		It("should prevent deletion when new cluster does not exist and no annotation", func() {
			now := metav1.Now()
			oldCluster := &opsterv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "test-cluster",
					Namespace:         "default",
					DeletionTimestamp: &now,
					Finalizers:        []string{MigrationFinalizer},
					// No annotation
				},
			}
			Expect(fakeClient.Create(ctx, oldCluster)).To(Succeed())

			result, err := reconciler.handleOldClusterDeletion(ctx, oldCluster)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(Equal(5 * time.Second))

			// Verify finalizer was NOT removed
			updatedCluster := &opsterv1.OpenSearchCluster{}
			Expect(fakeClient.Get(ctx, req.NamespacedName, updatedCluster)).To(Succeed())
			Expect(containsString(updatedCluster.Finalizers, MigrationFinalizer)).To(BeTrue())
		})
	})

	Describe("handleNewClusterDeletion", func() {
		It("should delete old cluster when new cluster is deleted", func() {
			now := metav1.Now()
			newCluster := &opensearchv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "test-cluster",
					Namespace:         "default",
					DeletionTimestamp: &now,
				},
			}
			Expect(fakeClient.Create(ctx, newCluster)).To(Succeed())

			oldCluster := &opsterv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "default",
				},
			}
			Expect(fakeClient.Create(ctx, oldCluster)).To(Succeed())

			result, err := reconciler.handleNewClusterDeletion(ctx, newCluster)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Requeue).To(BeFalse())

			// Verify old cluster was deleted (handleNewClusterDeletion deletes it after annotating)
			updatedOldCluster := &opsterv1.OpenSearchCluster{}
			err = fakeClient.Get(ctx, req.NamespacedName, updatedOldCluster)
			Expect(errors.IsNotFound(err)).To(BeTrue())
		})

		It("should handle case when old cluster does not exist", func() {
			now := metav1.Now()
			newCluster := &opensearchv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "test-cluster",
					Namespace:         "default",
					DeletionTimestamp: &now,
				},
			}
			Expect(fakeClient.Create(ctx, newCluster)).To(Succeed())

			result, err := reconciler.handleNewClusterDeletion(ctx, newCluster)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Requeue).To(BeFalse())
		})
	})

	Describe("isClusterReady", func() {
		It("should return true for RUNNING phase", func() {
			cluster := &opsterv1.OpenSearchCluster{
				Status: opsterv1.ClusterStatus{
					Phase: opsterv1.PhaseRunning,
				},
			}
			Expect(isClusterReady(cluster)).To(BeTrue())
		})

		It("should return false for PENDING phase", func() {
			cluster := &opsterv1.OpenSearchCluster{
				Status: opsterv1.ClusterStatus{
					Phase: opsterv1.PhasePending,
				},
			}
			Expect(isClusterReady(cluster)).To(BeFalse())
		})

		It("should return false for empty phase", func() {
			cluster := &opsterv1.OpenSearchCluster{
				Status: opsterv1.ClusterStatus{},
			}
			Expect(isClusterReady(cluster)).To(BeFalse())
		})
	})

	Describe("containsString and removeString helpers", func() {
		It("should correctly identify string in slice", func() {
			slice := []string{"a", "b", "c"}
			Expect(containsString(slice, "b")).To(BeTrue())
			Expect(containsString(slice, "d")).To(BeFalse())
		})

		It("should correctly remove string from slice", func() {
			slice := []string{"a", "b", "c"}
			result := removeString(slice, "b")
			Expect(result).To(Equal([]string{"a", "c"}))
			Expect(containsString(result, "b")).To(BeFalse())
		})

		It("should handle removing non-existent string", func() {
			slice := []string{"a", "b", "c"}
			result := removeString(slice, "d")
			Expect(result).To(Equal(slice))
		})
	})
})
