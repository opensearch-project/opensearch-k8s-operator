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
	corev1 "k8s.io/api/core/v1"
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

		It("should not add migration finalizer to a new cluster that is being deleted", func() {
			// Reproduces #1417: the migration controller must not attempt to add a
			// finalizer to an object that is already being deleted, otherwise the API
			// server rejects the update with "no new finalizers can be added if the
			// object is being deleted" and the controller loops forever.
			newCluster := &opensearchv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "default",
					// Some other finalizer keeps the object around (so deletion sets a
					// DeletionTimestamp instead of removing it) but the migration
					// finalizer is intentionally absent.
					Finalizers: []string{"Opensearch"},
				},
			}
			Expect(fakeClient.Create(ctx, newCluster)).To(Succeed())
			Expect(fakeClient.Delete(ctx, newCluster)).To(Succeed())

			result, err := reconciler.Reconcile(ctx, req)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Requeue).To(BeFalse())

			// Verify the migration finalizer was NOT added to the deleting object
			updatedCluster := &opensearchv1.OpenSearchCluster{}
			Expect(fakeClient.Get(ctx, req.NamespacedName, updatedCluster)).To(Succeed())
			Expect(updatedCluster.DeletionTimestamp.IsZero()).To(BeFalse())
			Expect(containsString(updatedCluster.Finalizers, MigrationFinalizer)).To(BeFalse())
		})

		It("should wait while the main reconciler is still cleaning up", func() {
			// When the new cluster is being deleted but the main "Opensearch"
			// finalizer is still present, the migration controller must wait for the
			// main reconciler to finish external cleanup before removing its own
			// finalizer.
			newCluster := &opensearchv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test-cluster",
					Namespace:  "default",
					Finalizers: []string{"Opensearch", MigrationFinalizer},
				},
			}
			Expect(fakeClient.Create(ctx, newCluster)).To(Succeed())
			Expect(fakeClient.Delete(ctx, newCluster)).To(Succeed())

			result, err := reconciler.Reconcile(ctx, req)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(Equal(5 * time.Second))

			// Both finalizers must still be present (cleanup not finished)
			updatedCluster := &opensearchv1.OpenSearchCluster{}
			Expect(fakeClient.Get(ctx, req.NamespacedName, updatedCluster)).To(Succeed())
			Expect(containsString(updatedCluster.Finalizers, MigrationFinalizer)).To(BeTrue())
			Expect(containsString(updatedCluster.Finalizers, "Opensearch")).To(BeTrue())
		})

		It("should remove the migration finalizer once the main reconciler is done", func() {
			// When the new cluster is being deleted and only the migration finalizer
			// remains, the migration controller deletes the old CR and removes its
			// finalizer, allowing the new CR to be garbage collected.
			newCluster := &opensearchv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test-cluster",
					Namespace:  "default",
					Finalizers: []string{MigrationFinalizer},
				},
			}
			Expect(fakeClient.Create(ctx, newCluster)).To(Succeed())
			Expect(fakeClient.Delete(ctx, newCluster)).To(Succeed())

			oldCluster := &opsterv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "default",
				},
			}
			Expect(fakeClient.Create(ctx, oldCluster)).To(Succeed())

			result, err := reconciler.Reconcile(ctx, req)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Requeue).To(BeFalse())
			Expect(result.RequeueAfter).To(BeZero())

			// The migration finalizer was the only one left, so removing it allows the
			// fake client to garbage collect the new cluster.
			updatedCluster := &opensearchv1.OpenSearchCluster{}
			err = fakeClient.Get(ctx, req.NamespacedName, updatedCluster)
			Expect(errors.IsNotFound(err)).To(BeTrue())

			// The corresponding old cluster should have been deleted as well.
			updatedOld := &opsterv1.OpenSearchCluster{}
			err = fakeClient.Get(ctx, req.NamespacedName, updatedOld)
			Expect(errors.IsNotFound(err)).To(BeTrue())
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

	Describe("backfillPVCLegacyLabels", func() {
		It("should backfill new PVC labels from legacy labels", func() {
			oldCluster := &opsterv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "default",
				},
			}
			Expect(fakeClient.Create(ctx, oldCluster)).To(Succeed())

			pvc := &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "data-test-cluster-master-0",
					Namespace: "default",
					Labels: map[string]string{
						oldClusterLabel:  "test-cluster",
						oldNodePoolLabel: "master",
					},
				},
			}
			Expect(fakeClient.Create(ctx, pvc)).To(Succeed())

			Expect(reconciler.backfillPVCLegacyLabels(ctx, oldCluster)).To(Succeed())

			updatedPVC := &corev1.PersistentVolumeClaim{}
			Expect(fakeClient.Get(ctx, types.NamespacedName{Name: pvc.Name, Namespace: pvc.Namespace}, updatedPVC)).To(Succeed())
			Expect(updatedPVC.Labels[newClusterLabel]).To(Equal("test-cluster"))
			Expect(updatedPVC.Labels[newNodePoolLabel]).To(Equal("master"))
		})

		It("should not overwrite existing new labels", func() {
			oldCluster := &opsterv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "default",
				},
			}
			Expect(fakeClient.Create(ctx, oldCluster)).To(Succeed())

			pvc := &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "data-test-cluster-master-1",
					Namespace: "default",
					Labels: map[string]string{
						oldClusterLabel:  "test-cluster",
						oldNodePoolLabel: "master",
						newClusterLabel:  "already-set-cluster",
						newNodePoolLabel: "already-set-nodepool",
					},
				},
			}
			Expect(fakeClient.Create(ctx, pvc)).To(Succeed())

			Expect(reconciler.backfillPVCLegacyLabels(ctx, oldCluster)).To(Succeed())

			updatedPVC := &corev1.PersistentVolumeClaim{}
			Expect(fakeClient.Get(ctx, types.NamespacedName{Name: pvc.Name, Namespace: pvc.Namespace}, updatedPVC)).To(Succeed())
			Expect(updatedPVC.Labels[newClusterLabel]).To(Equal("already-set-cluster"))
			Expect(updatedPVC.Labels[newNodePoolLabel]).To(Equal("already-set-nodepool"))
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

	Describe("Generic Migration Reconciler - New Resource Deletion", func() {
		It("should not add migration finalizer to a new resource that is being deleted", func() {
			// Reproduces #1417 for the generic migration reconciler (used by
			// usermigration / userrolebindingmigration / etc.): adding a finalizer
			// to an object that is already being deleted is rejected by the API
			// server and causes a reconcile error loop.
			newUser := &opensearchv1.OpensearchUser{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test-cluster",
					Namespace:  "default",
					Finalizers: []string{OpensearchFinalizer},
				},
			}
			Expect(fakeClient.Create(ctx, newUser)).To(Succeed())
			Expect(fakeClient.Delete(ctx, newUser)).To(Succeed())

			userMigration := &UserMigrationReconciler{Client: fakeClient, Scheme: scheme}
			result, err := userMigration.Reconcile(ctx, req)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Requeue).To(BeFalse())

			updatedUser := &opensearchv1.OpensearchUser{}
			Expect(fakeClient.Get(ctx, req.NamespacedName, updatedUser)).To(Succeed())
			Expect(updatedUser.DeletionTimestamp.IsZero()).To(BeFalse())
			Expect(containsString(updatedUser.Finalizers, MigrationFinalizer)).To(BeFalse())
		})

		It("should add migration finalizer to a new resource that is not being deleted", func() {
			newUser := &opensearchv1.OpensearchUser{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "default",
				},
			}
			Expect(fakeClient.Create(ctx, newUser)).To(Succeed())

			userMigration := &UserMigrationReconciler{Client: fakeClient, Scheme: scheme}
			result, err := userMigration.Reconcile(ctx, req)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Requeue).To(BeTrue())

			updatedUser := &opensearchv1.OpensearchUser{}
			Expect(fakeClient.Get(ctx, req.NamespacedName, updatedUser)).To(Succeed())
			Expect(containsString(updatedUser.Finalizers, MigrationFinalizer)).To(BeTrue())
		})

		It("should wait while other finalizers are still present on a deleting resource", func() {
			// The migration controller must not remove its finalizer (and delete the
			// old resource) until the main reconciler has removed its own finalizer.
			newUser := &opensearchv1.OpensearchUser{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test-cluster",
					Namespace:  "default",
					Finalizers: []string{OpensearchFinalizer, MigrationFinalizer},
				},
			}
			Expect(fakeClient.Create(ctx, newUser)).To(Succeed())
			Expect(fakeClient.Delete(ctx, newUser)).To(Succeed())

			userMigration := &UserMigrationReconciler{Client: fakeClient, Scheme: scheme}
			result, err := userMigration.Reconcile(ctx, req)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(Equal(5 * time.Second))

			updatedUser := &opensearchv1.OpensearchUser{}
			Expect(fakeClient.Get(ctx, req.NamespacedName, updatedUser)).To(Succeed())
			Expect(containsString(updatedUser.Finalizers, MigrationFinalizer)).To(BeTrue())
			Expect(containsString(updatedUser.Finalizers, OpensearchFinalizer)).To(BeTrue())
		})

		It("should remove the migration finalizer once only it remains on a deleting resource", func() {
			newUser := &opensearchv1.OpensearchUser{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test-cluster",
					Namespace:  "default",
					Finalizers: []string{MigrationFinalizer},
				},
			}
			Expect(fakeClient.Create(ctx, newUser)).To(Succeed())
			Expect(fakeClient.Delete(ctx, newUser)).To(Succeed())

			oldUser := &opsterv1.OpensearchUser{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "default",
				},
			}
			Expect(fakeClient.Create(ctx, oldUser)).To(Succeed())

			userMigration := &UserMigrationReconciler{Client: fakeClient, Scheme: scheme}
			result, err := userMigration.Reconcile(ctx, req)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Requeue).To(BeFalse())
			Expect(result.RequeueAfter).To(BeZero())

			// Migration finalizer was the only one left, so the new resource is removed.
			updatedUser := &opensearchv1.OpensearchUser{}
			err = fakeClient.Get(ctx, req.NamespacedName, updatedUser)
			Expect(errors.IsNotFound(err)).To(BeTrue())

			// The corresponding old resource should have been deleted as well.
			updatedOld := &opsterv1.OpensearchUser{}
			err = fakeClient.Get(ctx, req.NamespacedName, updatedOld)
			Expect(errors.IsNotFound(err)).To(BeTrue())
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
