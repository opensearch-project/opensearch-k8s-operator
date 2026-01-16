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
	"encoding/json"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"

	opensearchv1 "github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/api/opensearch.org/v1"
	opsterv1 "github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/api/v1"
)

const (
	// Migration annotations
	MigratedFromAnnotation         = "opensearch.org/migrated-from"
	MigrationTimestampAnnotation   = "opensearch.org/migration-timestamp"
	SourceUIDAnnotation            = "opensearch.org/source-uid"
	MigrationSyncAnnotation        = "opensearch.org/migration-sync"
	DeletedByNewResourceAnnotation = "opensearch.org/deleted-by-new-resource"

	// Finalizer for migration
	MigrationFinalizer = "opensearch.org/migration"

	// Old finalizers that need to be removed during deletion
	OldClusterFinalizer  = "Opster"                    // Old cluster finalizer corresponding myFinalizerName
	OldResourceFinalizer = "opster.io/opensearch-data" // Old resource finalizer (User, Role, etc.) coresponding to OpensearchFinalizer)
)

// ClusterMigrationReconciler reconciles OpenSearchCluster resources between old and new API groups
type ClusterMigrationReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=opensearch.opster.io,resources=opensearchclusters,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=opensearch.opster.io,resources=opensearchclusters/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=opensearch.opster.io,resources=opensearchclusters/finalizers,verbs=update
//+kubebuilder:rbac:groups=opensearch.org,resources=opensearchclusters,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=opensearch.org,resources=opensearchclusters/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=opensearch.org,resources=opensearchclusters/finalizers,verbs=update

func (r *ClusterMigrationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// First check if this is a new cluster event
	newCluster := &opensearchv1.OpenSearchCluster{}
	err := r.Get(ctx, req.NamespacedName, newCluster)
	if err == nil {
		// Add migration finalizer to new cluster if not present
		// This ensures we can handle deletion even after main reconciler removes its finalizer
		if !containsString(newCluster.Finalizers, MigrationFinalizer) {
			newCluster.Finalizers = append(newCluster.Finalizers, MigrationFinalizer)
			if err := r.Update(ctx, newCluster); err != nil {
				return ctrl.Result{}, err
			}
			// Requeue to process deletion if needed
			return ctrl.Result{Requeue: true}, nil
		}

		// This is a new cluster - check if it's being deleted
		if !newCluster.DeletionTimestamp.IsZero() {
			// Check if the main reconciler has finished cleanup (finalizer removed)
			// The main reconciler removes the "Opensearch" finalizer after cleanup
			hasMainFinalizer := false
			for _, finalizer := range newCluster.Finalizers {
				if finalizer == "Opensearch" {
					hasMainFinalizer = true
					break
				}
			}
			// Only delete old CR if new CR is fully deleted (no main finalizer)
			// This ensures the main reconciler has finished cleaning up external resources
			if !hasMainFinalizer {
				result, err := r.handleNewClusterDeletion(ctx, newCluster)
				if err != nil {
					return result, err
				}
				// Remove migration finalizer to allow new CR to be deleted
				newCluster.Finalizers = removeString(newCluster.Finalizers, MigrationFinalizer)
				if err := r.Update(ctx, newCluster); err != nil {
					return ctrl.Result{}, err
				}
				return ctrl.Result{}, nil
			}
			// New CR still has finalizer - main reconciler is still cleaning up
			// Requeue to wait for cleanup to complete
			logger.Info("New cluster deletion in progress, waiting for main reconciler to finish cleanup", "name", newCluster.Name)
			return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
		}
		// If new cluster exists and is not being deleted, continue to check old cluster
	}

	// Try to get the old API group resource
	oldCluster := &opsterv1.OpenSearchCluster{}
	err = r.Get(ctx, req.NamespacedName, oldCluster)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Log deprecation warning for old API group usage
	logger.Info("DEPRECATION WARNING: opensearch.opster.io API group is deprecated. Please migrate to opensearch.org/v1.")

	// Handle deletion
	if !oldCluster.DeletionTimestamp.IsZero() {
		return r.handleOldClusterDeletion(ctx, oldCluster)
	}

	// Add finalizer if not present
	if !containsString(oldCluster.Finalizers, MigrationFinalizer) {
		oldCluster.Finalizers = append(oldCluster.Finalizers, MigrationFinalizer)
		if err := r.Update(ctx, oldCluster); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Re-check if new API group resource exists (in case it was just created)
	err = r.Get(ctx, req.NamespacedName, newCluster)
	if err != nil {
		if errors.IsNotFound(err) {
			// Check if old cluster is in ready status before migrating
			if !isClusterReady(oldCluster) {
				logger.Info("Old cluster is not ready, skipping migration", "name", oldCluster.Name, "phase", oldCluster.Status.Phase)
				return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
			}
			// Create new API group resource
			logger.Info("Creating new API group resource from old", "name", oldCluster.Name, "namespace", oldCluster.Namespace)
			return r.createNewFromOld(ctx, oldCluster)
		}
		return ctrl.Result{}, err
	}

	// Sync changes from old to new
	return r.syncOldToNew(ctx, oldCluster, newCluster)
}

func (r *ClusterMigrationReconciler) createNewFromOld(ctx context.Context, oldCluster *opsterv1.OpenSearchCluster) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Convert old cluster spec to new cluster spec
	newCluster := &opensearchv1.OpenSearchCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      oldCluster.Name,
			Namespace: oldCluster.Namespace,
			Labels:    oldCluster.Labels,
			Annotations: map[string]string{
				MigratedFromAnnotation:       "opensearch.opster.io/v1",
				MigrationTimestampAnnotation: time.Now().UTC().Format(time.RFC3339),
				SourceUIDAnnotation:          string(oldCluster.UID),
			},
		},
	}

	// Copy existing annotations
	if oldCluster.Annotations != nil {
		for k, v := range oldCluster.Annotations {
			newCluster.Annotations[k] = v
		}
	}

	// Convert spec using JSON marshaling for deep copy
	specBytes, err := json.Marshal(oldCluster.Spec)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to marshal old cluster spec: %w", err)
	}
	if err := json.Unmarshal(specBytes, &newCluster.Spec); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to unmarshal to new cluster spec: %w", err)
	}

	// Create the new resource
	if err := r.Create(ctx, newCluster); err != nil {
		if errors.IsAlreadyExists(err) {
			logger.Info("New API group resource already exists", "name", newCluster.Name)
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	logger.Info("Created new API group resource", "name", newCluster.Name, "namespace", newCluster.Namespace)
	return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
}

func (r *ClusterMigrationReconciler) syncOldToNew(ctx context.Context, oldCluster *opsterv1.OpenSearchCluster, newCluster *opensearchv1.OpenSearchCluster) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Check if spec has changed
	oldSpecBytes, _ := json.Marshal(oldCluster.Spec)
	newSpecBytes, _ := json.Marshal(newCluster.Spec)

	if string(oldSpecBytes) != string(newSpecBytes) {
		logger.Info("Syncing spec changes from old to new API group", "name", oldCluster.Name)

		// Update new cluster spec
		if err := json.Unmarshal(oldSpecBytes, &newCluster.Spec); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to unmarshal spec: %w", err)
		}

		// Update annotations
		if newCluster.Annotations == nil {
			newCluster.Annotations = make(map[string]string)
		}
		newCluster.Annotations[MigrationSyncAnnotation] = time.Now().UTC().Format(time.RFC3339)

		if err := r.Update(ctx, newCluster); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Sync status from new back to old
	return r.syncStatusNewToOld(ctx, oldCluster, newCluster)
}

func (r *ClusterMigrationReconciler) syncStatusNewToOld(ctx context.Context, oldCluster *opsterv1.OpenSearchCluster, newCluster *opensearchv1.OpenSearchCluster) (ctrl.Result, error) {
	// Convert status using JSON marshaling
	statusBytes, err := json.Marshal(newCluster.Status)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to marshal new cluster status: %w", err)
	}

	var newStatus opsterv1.ClusterStatus
	if err := json.Unmarshal(statusBytes, &newStatus); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to unmarshal to old cluster status: %w", err)
	}

	// Check if status is different
	oldStatusBytes, _ := json.Marshal(oldCluster.Status)
	if string(statusBytes) != string(oldStatusBytes) {
		oldCluster.Status = newStatus
		if err := r.Status().Update(ctx, oldCluster); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

func (r *ClusterMigrationReconciler) handleNewClusterDeletion(ctx context.Context, newCluster *opensearchv1.OpenSearchCluster) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// When new cluster is deleted, delete the corresponding old cluster
	oldCluster := &opsterv1.OpenSearchCluster{}
	err := r.Get(ctx, types.NamespacedName{Name: newCluster.Name, Namespace: newCluster.Namespace}, oldCluster)
	if err != nil {
		if errors.IsNotFound(err) {
			// Old cluster doesn't exist, nothing to do
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Add annotation to mark that this deletion was triggered by new cluster deletion
	// This allows handleOldClusterDeletion to distinguish between:
	// 1. Old CR manually deleted before migration (should wait for migration)
	// 2. Old CR deleted because new CR was deleted (should allow deletion)
	if oldCluster.Annotations == nil {
		oldCluster.Annotations = make(map[string]string)
	}
	oldCluster.Annotations[DeletedByNewResourceAnnotation] = "true"
	if err := r.Update(ctx, oldCluster); err != nil {
		return ctrl.Result{}, err
	}

	logger.Info("Deleting old cluster due to new cluster deletion", "name", oldCluster.Name)
	if err := r.Delete(ctx, oldCluster); err != nil && !errors.IsNotFound(err) {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *ClusterMigrationReconciler) handleOldClusterDeletion(ctx context.Context, oldCluster *opsterv1.OpenSearchCluster) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	if containsString(oldCluster.Finalizers, MigrationFinalizer) {
		// Check if corresponding new cluster exists
		newCluster := &opensearchv1.OpenSearchCluster{}
		err := r.Get(ctx, types.NamespacedName{Name: oldCluster.Name, Namespace: oldCluster.Namespace}, newCluster)
		if err != nil {
			if errors.IsNotFound(err) {
				// New cluster doesn't exist
				// Check if this deletion was triggered by new cluster deletion (has annotation)
				// vs manually deleted before migration (no annotation)
				if oldCluster.Annotations != nil && oldCluster.Annotations[DeletedByNewResourceAnnotation] == "true" {
					// Old cluster deletion was triggered by new cluster deletion - safe to allow
					logger.Info("Old cluster deletion triggered by new cluster deletion, allowing deletion", "name", oldCluster.Name)
					// Remove all finalizers (migration finalizer and old finalizers)
					oldCluster.Finalizers = removeString(oldCluster.Finalizers, MigrationFinalizer)
					oldCluster.Finalizers = removeString(oldCluster.Finalizers, OldClusterFinalizer)
					if err := r.Update(ctx, oldCluster); err != nil {
						return ctrl.Result{}, err
					}
					return ctrl.Result{}, nil
				}
				// Old cluster was manually deleted before migration - prevent deletion
				logger.Info("Cannot delete old cluster: corresponding new cluster does not exist (migration not completed)", "name", oldCluster.Name)
				// Requeue to retry after new cluster is created
				return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
			}
			return ctrl.Result{}, err
		}

		// New cluster exists, safe to remove finalizers and allow deletion
		logger.Info("Removing finalizers from old cluster", "name", oldCluster.Name)
		// Remove all finalizers (migration finalizer and old finalizers)
		oldCluster.Finalizers = removeString(oldCluster.Finalizers, MigrationFinalizer)
		oldCluster.Finalizers = removeString(oldCluster.Finalizers, OldClusterFinalizer)
		if err := r.Update(ctx, oldCluster); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *ClusterMigrationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named("clustermigration").
		For(&opsterv1.OpenSearchCluster{}).
		Watches(&opensearchv1.OpenSearchCluster{}, &handler.EnqueueRequestForObject{}).
		Complete(r)
}

// UserMigrationReconciler reconciles OpensearchUser resources between old and new API groups
type UserMigrationReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=opensearch.opster.io,resources=opensearchusers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=opensearch.opster.io,resources=opensearchusers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=opensearch.opster.io,resources=opensearchusers/finalizers,verbs=update
//+kubebuilder:rbac:groups=opensearch.org,resources=opensearchusers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=opensearch.org,resources=opensearchusers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=opensearch.org,resources=opensearchusers/finalizers,verbs=update

func (r *UserMigrationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	return reconcileGenericMigration[opsterv1.OpensearchUser, opensearchv1.OpensearchUser](ctx, r.Client, req, "OpensearchUser")
}

func (r *UserMigrationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named("usermigration").
		For(&opsterv1.OpensearchUser{}).
		Watches(&opensearchv1.OpensearchUser{}, &handler.EnqueueRequestForObject{}).
		Complete(r)
}

// RoleMigrationReconciler reconciles OpensearchRole resources between old and new API groups
type RoleMigrationReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=opensearch.opster.io,resources=opensearchroles,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=opensearch.opster.io,resources=opensearchroles/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=opensearch.opster.io,resources=opensearchroles/finalizers,verbs=update
//+kubebuilder:rbac:groups=opensearch.org,resources=opensearchroles,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=opensearch.org,resources=opensearchroles/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=opensearch.org,resources=opensearchroles/finalizers,verbs=update

func (r *RoleMigrationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	return reconcileGenericMigration[opsterv1.OpensearchRole, opensearchv1.OpensearchRole](ctx, r.Client, req, "OpensearchRole")
}

func (r *RoleMigrationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named("rolemigration").
		For(&opsterv1.OpensearchRole{}).
		Watches(&opensearchv1.OpensearchRole{}, &handler.EnqueueRequestForObject{}).
		Complete(r)
}

// UserRoleBindingMigrationReconciler reconciles OpensearchUserRoleBinding resources
type UserRoleBindingMigrationReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=opensearch.opster.io,resources=opensearchuserrolebindings,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=opensearch.opster.io,resources=opensearchuserrolebindings/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=opensearch.opster.io,resources=opensearchuserrolebindings/finalizers,verbs=update
//+kubebuilder:rbac:groups=opensearch.org,resources=opensearchuserrolebindings,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=opensearch.org,resources=opensearchuserrolebindings/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=opensearch.org,resources=opensearchuserrolebindings/finalizers,verbs=update

func (r *UserRoleBindingMigrationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	return reconcileGenericMigration[opsterv1.OpensearchUserRoleBinding, opensearchv1.OpensearchUserRoleBinding](ctx, r.Client, req, "OpensearchUserRoleBinding")
}

func (r *UserRoleBindingMigrationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named("userrolebindingmigration").
		For(&opsterv1.OpensearchUserRoleBinding{}).
		Watches(&opensearchv1.OpensearchUserRoleBinding{}, &handler.EnqueueRequestForObject{}).
		Complete(r)
}

// TenantMigrationReconciler reconciles OpensearchTenant resources
type TenantMigrationReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=opensearch.opster.io,resources=opensearchtenants,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=opensearch.opster.io,resources=opensearchtenants/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=opensearch.opster.io,resources=opensearchtenants/finalizers,verbs=update
//+kubebuilder:rbac:groups=opensearch.org,resources=opensearchtenants,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=opensearch.org,resources=opensearchtenants/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=opensearch.org,resources=opensearchtenants/finalizers,verbs=update

func (r *TenantMigrationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	return reconcileGenericMigration[opsterv1.OpensearchTenant, opensearchv1.OpensearchTenant](ctx, r.Client, req, "OpensearchTenant")
}

func (r *TenantMigrationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named("tenantmigration").
		For(&opsterv1.OpensearchTenant{}).
		Watches(&opensearchv1.OpensearchTenant{}, &handler.EnqueueRequestForObject{}).
		Complete(r)
}

// ActionGroupMigrationReconciler reconciles OpensearchActionGroup resources
type ActionGroupMigrationReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=opensearch.opster.io,resources=opensearchactiongroups,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=opensearch.opster.io,resources=opensearchactiongroups/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=opensearch.opster.io,resources=opensearchactiongroups/finalizers,verbs=update
//+kubebuilder:rbac:groups=opensearch.org,resources=opensearchactiongroups,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=opensearch.org,resources=opensearchactiongroups/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=opensearch.org,resources=opensearchactiongroups/finalizers,verbs=update

func (r *ActionGroupMigrationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	return reconcileGenericMigration[opsterv1.OpensearchActionGroup, opensearchv1.OpensearchActionGroup](ctx, r.Client, req, "OpensearchActionGroup")
}

func (r *ActionGroupMigrationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named("actiongroupmigration").
		For(&opsterv1.OpensearchActionGroup{}).
		Watches(&opensearchv1.OpensearchActionGroup{}, &handler.EnqueueRequestForObject{}).
		Complete(r)
}

// ISMPolicyMigrationReconciler reconciles OpenSearchISMPolicy resources
type ISMPolicyMigrationReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=opensearch.opster.io,resources=opensearchismpolicies,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=opensearch.opster.io,resources=opensearchismpolicies/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=opensearch.opster.io,resources=opensearchismpolicies/finalizers,verbs=update
//+kubebuilder:rbac:groups=opensearch.org,resources=opensearchismpolicies,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=opensearch.org,resources=opensearchismpolicies/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=opensearch.org,resources=opensearchismpolicies/finalizers,verbs=update

func (r *ISMPolicyMigrationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	return reconcileGenericMigration[opsterv1.OpenSearchISMPolicy, opensearchv1.OpenSearchISMPolicy](ctx, r.Client, req, "OpenSearchISMPolicy")
}

func (r *ISMPolicyMigrationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named("ismpolicymigration").
		For(&opsterv1.OpenSearchISMPolicy{}).
		Watches(&opensearchv1.OpenSearchISMPolicy{}, &handler.EnqueueRequestForObject{}).
		Complete(r)
}

// SnapshotPolicyMigrationReconciler reconciles OpensearchSnapshotPolicy resources
type SnapshotPolicyMigrationReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=opensearch.opster.io,resources=opensearchsnapshotpolicies,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=opensearch.opster.io,resources=opensearchsnapshotpolicies/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=opensearch.opster.io,resources=opensearchsnapshotpolicies/finalizers,verbs=update
//+kubebuilder:rbac:groups=opensearch.org,resources=opensearchsnapshotpolicies,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=opensearch.org,resources=opensearchsnapshotpolicies/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=opensearch.org,resources=opensearchsnapshotpolicies/finalizers,verbs=update

func (r *SnapshotPolicyMigrationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	return reconcileGenericMigration[opsterv1.OpensearchSnapshotPolicy, opensearchv1.OpensearchSnapshotPolicy](ctx, r.Client, req, "OpensearchSnapshotPolicy")
}

func (r *SnapshotPolicyMigrationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named("snapshotpolicymigration").
		For(&opsterv1.OpensearchSnapshotPolicy{}).
		Watches(&opensearchv1.OpensearchSnapshotPolicy{}, &handler.EnqueueRequestForObject{}).
		Complete(r)
}

// IndexTemplateMigrationReconciler reconciles OpensearchIndexTemplate resources
type IndexTemplateMigrationReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=opensearch.opster.io,resources=opensearchindextemplates,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=opensearch.opster.io,resources=opensearchindextemplates/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=opensearch.opster.io,resources=opensearchindextemplates/finalizers,verbs=update
//+kubebuilder:rbac:groups=opensearch.org,resources=opensearchindextemplates,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=opensearch.org,resources=opensearchindextemplates/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=opensearch.org,resources=opensearchindextemplates/finalizers,verbs=update

func (r *IndexTemplateMigrationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	return reconcileGenericMigration[opsterv1.OpensearchIndexTemplate, opensearchv1.OpensearchIndexTemplate](ctx, r.Client, req, "OpensearchIndexTemplate")
}

func (r *IndexTemplateMigrationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named("indextemplatemigration").
		For(&opsterv1.OpensearchIndexTemplate{}).
		Watches(&opensearchv1.OpensearchIndexTemplate{}, &handler.EnqueueRequestForObject{}).
		Complete(r)
}

// ComponentTemplateMigrationReconciler reconciles OpensearchComponentTemplate resources
type ComponentTemplateMigrationReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=opensearch.opster.io,resources=opensearchcomponenttemplates,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=opensearch.opster.io,resources=opensearchcomponenttemplates/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=opensearch.opster.io,resources=opensearchcomponenttemplates/finalizers,verbs=update
//+kubebuilder:rbac:groups=opensearch.org,resources=opensearchcomponenttemplates,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=opensearch.org,resources=opensearchcomponenttemplates/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=opensearch.org,resources=opensearchcomponenttemplates/finalizers,verbs=update

func (r *ComponentTemplateMigrationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	return reconcileGenericMigration[opsterv1.OpensearchComponentTemplate, opensearchv1.OpensearchComponentTemplate](ctx, r.Client, req, "OpensearchComponentTemplate")
}

func (r *ComponentTemplateMigrationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named("componenttemplatemigration").
		For(&opsterv1.OpensearchComponentTemplate{}).
		Watches(&opensearchv1.OpensearchComponentTemplate{}, &handler.EnqueueRequestForObject{}).
		Complete(r)
}

// Generic migration reconciler using generics
func reconcileGenericMigration[OldType, NewType any, OldPtr interface {
	*OldType
	client.Object
}, NewPtr interface {
	*NewType
	client.Object
}](ctx context.Context, c client.Client, req ctrl.Request, resourceKind string) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// First check if this is a new resource event
	newResource := NewPtr(new(NewType))
	err := c.Get(ctx, req.NamespacedName, newResource)
	if err == nil {
		// Add migration finalizer to new resource if not present
		// This ensures we can handle deletion even after main reconciler removes its finalizer
		if !containsString(newResource.GetFinalizers(), MigrationFinalizer) {
			newResource.SetFinalizers(append(newResource.GetFinalizers(), MigrationFinalizer))
			if err := c.Update(ctx, newResource); err != nil {
				return ctrl.Result{}, err
			}
			// Requeue to process deletion if needed
			return ctrl.Result{Requeue: true}, nil
		}

		// This is a new resource - check if it's being deleted
		if !newResource.GetDeletionTimestamp().IsZero() {
			// Check if the resource still has other finalizers (cleanup in progress)
			// Only delete old resource if new resource has no other finalizers (only migration finalizer remains)
			otherFinalizers := false
			for _, finalizer := range newResource.GetFinalizers() {
				if finalizer != MigrationFinalizer {
					otherFinalizers = true
					break
				}
			}
			if otherFinalizers {
				// New resource still has other finalizers - main reconciler is still cleaning up
				// Requeue to wait for cleanup to complete
				logger.Info("New resource deletion in progress, waiting for other finalizers to be removed", "kind", resourceKind, "name", req.Name)
				return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
			}
			// New resource only has migration finalizer - main reconciler finished cleanup
			// Safe to delete old resource
			result, err := handleGenericNewDeletion[OldType, NewType, OldPtr, NewPtr](ctx, c, newResource, req, resourceKind)
			if err != nil {
				return result, err
			}
			// Remove migration finalizer to allow new resource to be deleted
			newResource.SetFinalizers(removeString(newResource.GetFinalizers(), MigrationFinalizer))
			if err := c.Update(ctx, newResource); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}
		// If new resource exists and is not being deleted, continue to check old resource
	}

	// Get old resource
	oldResource := OldPtr(new(OldType))
	err = c.Get(ctx, req.NamespacedName, oldResource)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	logger.Info("DEPRECATION WARNING: opensearch.opster.io API group is deprecated", "kind", resourceKind)

	// Handle deletion
	if !oldResource.GetDeletionTimestamp().IsZero() {
		return handleGenericDeletion[OldType, NewType, OldPtr, NewPtr](ctx, c, oldResource, req, resourceKind)
	}

	// Add finalizer if not present
	if !containsString(oldResource.GetFinalizers(), MigrationFinalizer) {
		oldResource.SetFinalizers(append(oldResource.GetFinalizers(), MigrationFinalizer))
		if err := c.Update(ctx, oldResource); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Re-check if new resource exists (in case it was just created)
	err = c.Get(ctx, req.NamespacedName, newResource)
	if err != nil {
		if errors.IsNotFound(err) {
			// Check if old resource is in ready status before migrating
			if !isGenericResourceReady(oldResource, resourceKind) {
				logger.Info("Old resource is not ready, skipping migration", "kind", resourceKind, "name", req.Name)
				return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
			}
			// Create new resource
			logger.Info("Creating new API group resource from old", "kind", resourceKind, "name", req.Name)
			return createGenericNewFromOld[OldType, NewType, OldPtr, NewPtr](ctx, c, oldResource, req)
		}
		return ctrl.Result{}, err
	}

	// Sync old to new
	return syncGenericOldToNew[OldType, NewType, OldPtr, NewPtr](ctx, c, oldResource, newResource)
}

func createGenericNewFromOld[OldType, NewType any, OldPtr interface {
	*OldType
	client.Object
}, NewPtr interface {
	*NewType
	client.Object
}](ctx context.Context, c client.Client, oldResource OldPtr, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Create new resource with same name/namespace
	newResource := NewPtr(new(NewType))

	// Convert using JSON
	oldBytes, err := json.Marshal(oldResource)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to marshal old resource: %w", err)
	}

	if err := json.Unmarshal(oldBytes, newResource); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to unmarshal to new resource: %w", err)
	}

	// Set metadata
	newResource.SetName(oldResource.GetName())
	newResource.SetNamespace(oldResource.GetNamespace())
	newResource.SetLabels(oldResource.GetLabels())
	newResource.SetResourceVersion("")
	newResource.SetUID("")

	// Set annotations
	annotations := make(map[string]string)
	for k, v := range oldResource.GetAnnotations() {
		annotations[k] = v
	}
	annotations[MigratedFromAnnotation] = "opensearch.opster.io/v1"
	annotations[MigrationTimestampAnnotation] = time.Now().UTC().Format(time.RFC3339)
	annotations[SourceUIDAnnotation] = string(oldResource.GetUID())
	newResource.SetAnnotations(annotations)

	// Clear finalizers on new resource
	newResource.SetFinalizers(nil)

	if err := c.Create(ctx, newResource); err != nil {
		if errors.IsAlreadyExists(err) {
			logger.Info("New API group resource already exists", "name", req.Name)
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	logger.Info("Created new API group resource", "name", req.Name, "namespace", req.Namespace)
	return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
}

func syncGenericOldToNew[OldType, NewType any, OldPtr interface {
	*OldType
	client.Object
}, NewPtr interface {
	*NewType
	client.Object
}](ctx context.Context, c client.Client, oldResource OldPtr, newResource NewPtr) (ctrl.Result, error) {
	// For simplicity, we just requeue periodically
	// In a full implementation, you'd compare specs and sync changes
	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

func handleGenericNewDeletion[OldType, NewType any, OldPtr interface {
	*OldType
	client.Object
}, NewPtr interface {
	*NewType
	client.Object
}](ctx context.Context, c client.Client, newResource NewPtr, req ctrl.Request, resourceKind string) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// When new resource is deleted, delete the corresponding old resource
	oldResource := OldPtr(new(OldType))
	err := c.Get(ctx, types.NamespacedName{Name: req.Name, Namespace: req.Namespace}, oldResource)
	if err != nil {
		if errors.IsNotFound(err) {
			// Old resource doesn't exist, nothing to do
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Add annotation to mark that this deletion was triggered by new resource deletion
	// This allows handleGenericDeletion to distinguish between:
	// 1. Old resource manually deleted before migration (should wait for migration)
	// 2. Old resource deleted because new resource was deleted (should allow deletion)
	if oldResource.GetAnnotations() == nil {
		oldResource.SetAnnotations(make(map[string]string))
	}
	annotations := oldResource.GetAnnotations()
	annotations[DeletedByNewResourceAnnotation] = "true"
	oldResource.SetAnnotations(annotations)
	if err := c.Update(ctx, oldResource); err != nil {
		return ctrl.Result{}, err
	}

	logger.Info("Deleting old resource due to new resource deletion", "kind", resourceKind, "name", req.Name)
	if err := c.Delete(ctx, oldResource); err != nil && !errors.IsNotFound(err) {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func handleGenericDeletion[OldType, NewType any, OldPtr interface {
	*OldType
	client.Object
}, NewPtr interface {
	*NewType
	client.Object
}](ctx context.Context, c client.Client, oldResource OldPtr, req ctrl.Request, resourceKind string) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	if containsString(oldResource.GetFinalizers(), MigrationFinalizer) {
		// Check if corresponding new resource exists
		newResource := NewPtr(new(NewType))
		err := c.Get(ctx, types.NamespacedName{Name: req.Name, Namespace: req.Namespace}, newResource)
		if err != nil {
			if errors.IsNotFound(err) {
				// New resource doesn't exist
				// Check if this deletion was triggered by new resource deletion (has annotation)
				// vs manually deleted before migration (no annotation)
				annotations := oldResource.GetAnnotations()
				if annotations != nil && annotations[DeletedByNewResourceAnnotation] == "true" {
					// Old resource deletion was triggered by new resource deletion - safe to allow
					logger.Info("Old resource deletion triggered by new resource deletion, allowing deletion", "kind", resourceKind, "name", req.Name)
					// Remove all finalizers (migration finalizer and old finalizers)
					finalizers := removeString(oldResource.GetFinalizers(), MigrationFinalizer)
					finalizers = removeString(finalizers, OldResourceFinalizer)
					oldResource.SetFinalizers(finalizers)
					if err := c.Update(ctx, oldResource); err != nil {
						return ctrl.Result{}, err
					}
					return ctrl.Result{}, nil
				}
				// Old resource was manually deleted before migration - prevent deletion
				logger.Info("Cannot delete old resource: corresponding new resource does not exist (migration not completed)", "kind", resourceKind, "name", req.Name)
				// Requeue to retry after new resource is created
				return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
			}
			return ctrl.Result{}, err
		}

		// New resource exists, safe to remove finalizers and allow deletion
		logger.Info("Removing finalizers from old resource", "kind", resourceKind, "name", req.Name)
		// Remove all finalizers (migration finalizer and old finalizers)
		finalizers := removeString(oldResource.GetFinalizers(), MigrationFinalizer)
		finalizers = removeString(finalizers, OldResourceFinalizer)
		oldResource.SetFinalizers(finalizers)
		if err := c.Update(ctx, oldResource); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// Helper functions
func isClusterReady(cluster *opsterv1.OpenSearchCluster) bool {
	// Only migrate when cluster is in RUNNING phase
	return cluster.Status.Phase == opsterv1.PhaseRunning
}

func isGenericResourceReady(resource client.Object, resourceKind string) bool {
	// Use type assertion to check status based on resource type
	switch r := resource.(type) {
	case *opsterv1.OpensearchUser:
		return r.Status.State == opsterv1.OpensearchUserStateCreated
	case *opsterv1.OpensearchRole:
		return r.Status.State == opsterv1.OpensearchRoleStateCreated
	case *opsterv1.OpensearchUserRoleBinding:
		return r.Status.State == opsterv1.OpensearchUserRoleBindingStateCreated
	case *opsterv1.OpensearchTenant:
		return r.Status.State == opsterv1.OpensearchTenantCreated
	case *opsterv1.OpensearchActionGroup:
		return r.Status.State == opsterv1.OpensearchActionGroupCreated
	case *opsterv1.OpenSearchISMPolicy:
		return r.Status.State == opsterv1.OpensearchISMPolicyCreated
	case *opsterv1.OpensearchSnapshotPolicy:
		return r.Status.State == opsterv1.OpensearchSnapshotPolicyCreated
	case *opsterv1.OpensearchIndexTemplate:
		return r.Status.State == opsterv1.OpensearchIndexTemplateCreated
	case *opsterv1.OpensearchComponentTemplate:
		return r.Status.State == opsterv1.OpensearchComponentTemplateCreated
	default:
		// If we can't determine the type, assume not ready to be safe
		return false
	}
}

func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func removeString(slice []string, s string) []string {
	var result []string
	for _, item := range slice {
		if item != s {
			result = append(result, item)
		}
	}
	return result
}
