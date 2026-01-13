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
	"sigs.k8s.io/controller-runtime/pkg/log"

	opsterv1 "github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/api/v1"
	opensearchv1 "github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/api/opensearch.org/v1"
)

const (
	// Annotation to mark resources created via reverse migration
	ReverseMigratedFromAnnotation = "opensearch.org/reverse-migrated-from"
)

// ClusterReverseMigrationReconciler watches NEW API (opensearch.org) and creates OLD API resources
// This allows existing controllers to continue working while supporting the new API group
type ClusterReverseMigrationReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=opensearch.org,resources=opensearchclusters,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=opensearch.org,resources=opensearchclusters/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=opensearch.org,resources=opensearchclusters/finalizers,verbs=update
//+kubebuilder:rbac:groups=opensearch.opster.io,resources=opensearchclusters,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=opensearch.opster.io,resources=opensearchclusters/status,verbs=get;update;patch

func (r *ClusterReverseMigrationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Get the new API group resource
	newCluster := &opensearchv1.OpenSearchCluster{}
	err := r.Get(ctx, req.NamespacedName, newCluster)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Handle deletion
	if !newCluster.DeletionTimestamp.IsZero() {
		return r.handleNewClusterDeletion(ctx, newCluster)
	}

	// Add finalizer if not present
	if !containsString(newCluster.Finalizers, MigrationFinalizer) {
		newCluster.Finalizers = append(newCluster.Finalizers, MigrationFinalizer)
		if err := r.Update(ctx, newCluster); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Check if old API group resource exists
	oldCluster := &opsterv1.OpenSearchCluster{}
	err = r.Get(ctx, req.NamespacedName, oldCluster)
	if err != nil {
		if errors.IsNotFound(err) {
			// Check if this was originally a forward migration (old -> new)
			// If so, don't create an old resource (the old one should already exist)
			if _, wasMigrated := newCluster.Annotations[MigratedFromAnnotation]; wasMigrated {
				// This was created by forward migration, old resource should exist
				// Just sync status
				return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
			}

			// Create old API group resource for new resources
			logger.Info("Creating old API group resource from new", "name", newCluster.Name, "namespace", newCluster.Namespace)
			return r.createOldFromNew(ctx, newCluster)
		}
		return ctrl.Result{}, err
	}

	// Sync status from old to new
	return r.syncStatusOldToNew(ctx, oldCluster, newCluster)
}

func (r *ClusterReverseMigrationReconciler) createOldFromNew(ctx context.Context, newCluster *opensearchv1.OpenSearchCluster) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Convert new cluster spec to old cluster spec
	oldCluster := &opsterv1.OpenSearchCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      newCluster.Name,
			Namespace: newCluster.Namespace,
			Labels:    newCluster.Labels,
			Annotations: map[string]string{
				ReverseMigratedFromAnnotation:    "opensearch.org/v1",
				MigrationTimestampAnnotation: time.Now().UTC().Format(time.RFC3339),
				SourceUIDAnnotation:          string(newCluster.UID),
			},
		},
	}

	// Copy existing annotations
	if newCluster.Annotations != nil {
		for k, v := range newCluster.Annotations {
			if k != MigratedFromAnnotation && k != MigrationTimestampAnnotation && k != SourceUIDAnnotation {
				oldCluster.Annotations[k] = v
			}
		}
	}

	// Convert spec using JSON marshaling for deep copy
	specBytes, err := json.Marshal(newCluster.Spec)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to marshal new cluster spec: %w", err)
	}
	if err := json.Unmarshal(specBytes, &oldCluster.Spec); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to unmarshal to old cluster spec: %w", err)
	}

	// Create the old resource
	if err := r.Create(ctx, oldCluster); err != nil {
		if errors.IsAlreadyExists(err) {
			logger.Info("Old API group resource already exists", "name", oldCluster.Name)
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	logger.Info("Created old API group resource", "name", oldCluster.Name, "namespace", oldCluster.Namespace)
	return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
}

func (r *ClusterReverseMigrationReconciler) syncStatusOldToNew(ctx context.Context, oldCluster *opsterv1.OpenSearchCluster, newCluster *opensearchv1.OpenSearchCluster) (ctrl.Result, error) {
	// Convert status using JSON marshaling
	statusBytes, err := json.Marshal(oldCluster.Status)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to marshal old cluster status: %w", err)
	}

	var newStatus opensearchv1.ClusterStatus
	if err := json.Unmarshal(statusBytes, &newStatus); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to unmarshal to new cluster status: %w", err)
	}

	// Check if status is different
	oldStatusBytes, _ := json.Marshal(newCluster.Status)
	if string(statusBytes) != string(oldStatusBytes) {
		newCluster.Status = newStatus
		if err := r.Status().Update(ctx, newCluster); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

func (r *ClusterReverseMigrationReconciler) handleNewClusterDeletion(ctx context.Context, newCluster *opensearchv1.OpenSearchCluster) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	if containsString(newCluster.Finalizers, MigrationFinalizer) {
		// Only delete old resource if it was created by reverse migration
		oldCluster := &opsterv1.OpenSearchCluster{}
		err := r.Get(ctx, types.NamespacedName{Name: newCluster.Name, Namespace: newCluster.Namespace}, oldCluster)
		if err == nil {
			// Check if old resource was created by reverse migration
			if _, wasReverseMigrated := oldCluster.Annotations[ReverseMigratedFromAnnotation]; wasReverseMigrated {
				logger.Info("Deleting old API group resource due to new resource deletion", "name", newCluster.Name)
				if err := r.Delete(ctx, oldCluster); err != nil && !errors.IsNotFound(err) {
					return ctrl.Result{}, err
				}
			}
		}

		// Remove finalizer
		newCluster.Finalizers = removeString(newCluster.Finalizers, MigrationFinalizer)
		if err := r.Update(ctx, newCluster); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *ClusterReverseMigrationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&opensearchv1.OpenSearchCluster{}).
		Complete(r)
}

// UserReverseMigrationReconciler watches NEW API and creates OLD API resources
type UserReverseMigrationReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=opensearch.org,resources=opensearchusers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=opensearch.org,resources=opensearchusers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=opensearch.org,resources=opensearchusers/finalizers,verbs=update

func (r *UserReverseMigrationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	return reconcileGenericReverseMigration[opensearchv1.OpensearchUser, opsterv1.OpensearchUser](ctx, r.Client, req, "OpensearchUser")
}

func (r *UserReverseMigrationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&opensearchv1.OpensearchUser{}).
		Complete(r)
}

// RoleReverseMigrationReconciler watches NEW API and creates OLD API resources
type RoleReverseMigrationReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=opensearch.org,resources=opensearchroles,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=opensearch.org,resources=opensearchroles/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=opensearch.org,resources=opensearchroles/finalizers,verbs=update

func (r *RoleReverseMigrationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	return reconcileGenericReverseMigration[opensearchv1.OpensearchRole, opsterv1.OpensearchRole](ctx, r.Client, req, "OpensearchRole")
}

func (r *RoleReverseMigrationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&opensearchv1.OpensearchRole{}).
		Complete(r)
}

// UserRoleBindingReverseMigrationReconciler watches NEW API and creates OLD API resources
type UserRoleBindingReverseMigrationReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=opensearch.org,resources=opensearchuserrolebindings,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=opensearch.org,resources=opensearchuserrolebindings/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=opensearch.org,resources=opensearchuserrolebindings/finalizers,verbs=update

func (r *UserRoleBindingReverseMigrationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	return reconcileGenericReverseMigration[opensearchv1.OpensearchUserRoleBinding, opsterv1.OpensearchUserRoleBinding](ctx, r.Client, req, "OpensearchUserRoleBinding")
}

func (r *UserRoleBindingReverseMigrationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&opensearchv1.OpensearchUserRoleBinding{}).
		Complete(r)
}

// TenantReverseMigrationReconciler watches NEW API and creates OLD API resources
type TenantReverseMigrationReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=opensearch.org,resources=opensearchtenants,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=opensearch.org,resources=opensearchtenants/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=opensearch.org,resources=opensearchtenants/finalizers,verbs=update

func (r *TenantReverseMigrationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	return reconcileGenericReverseMigration[opensearchv1.OpensearchTenant, opsterv1.OpensearchTenant](ctx, r.Client, req, "OpensearchTenant")
}

func (r *TenantReverseMigrationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&opensearchv1.OpensearchTenant{}).
		Complete(r)
}

// ActionGroupReverseMigrationReconciler watches NEW API and creates OLD API resources
type ActionGroupReverseMigrationReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=opensearch.org,resources=opensearchactiongroups,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=opensearch.org,resources=opensearchactiongroups/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=opensearch.org,resources=opensearchactiongroups/finalizers,verbs=update

func (r *ActionGroupReverseMigrationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	return reconcileGenericReverseMigration[opensearchv1.OpensearchActionGroup, opsterv1.OpensearchActionGroup](ctx, r.Client, req, "OpensearchActionGroup")
}

func (r *ActionGroupReverseMigrationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&opensearchv1.OpensearchActionGroup{}).
		Complete(r)
}

// ISMPolicyReverseMigrationReconciler watches NEW API and creates OLD API resources
type ISMPolicyReverseMigrationReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=opensearch.org,resources=opensearchismpolicies,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=opensearch.org,resources=opensearchismpolicies/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=opensearch.org,resources=opensearchismpolicies/finalizers,verbs=update

func (r *ISMPolicyReverseMigrationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	return reconcileGenericReverseMigration[opensearchv1.OpenSearchISMPolicy, opsterv1.OpenSearchISMPolicy](ctx, r.Client, req, "OpenSearchISMPolicy")
}

func (r *ISMPolicyReverseMigrationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&opensearchv1.OpenSearchISMPolicy{}).
		Complete(r)
}

// SnapshotPolicyReverseMigrationReconciler watches NEW API and creates OLD API resources
type SnapshotPolicyReverseMigrationReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=opensearch.org,resources=opensearchsnapshotpolicies,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=opensearch.org,resources=opensearchsnapshotpolicies/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=opensearch.org,resources=opensearchsnapshotpolicies/finalizers,verbs=update

func (r *SnapshotPolicyReverseMigrationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	return reconcileGenericReverseMigration[opensearchv1.OpensearchSnapshotPolicy, opsterv1.OpensearchSnapshotPolicy](ctx, r.Client, req, "OpensearchSnapshotPolicy")
}

func (r *SnapshotPolicyReverseMigrationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&opensearchv1.OpensearchSnapshotPolicy{}).
		Complete(r)
}

// IndexTemplateReverseMigrationReconciler watches NEW API and creates OLD API resources
type IndexTemplateReverseMigrationReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=opensearch.org,resources=opensearchindextemplates,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=opensearch.org,resources=opensearchindextemplates/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=opensearch.org,resources=opensearchindextemplates/finalizers,verbs=update

func (r *IndexTemplateReverseMigrationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	return reconcileGenericReverseMigration[opensearchv1.OpensearchIndexTemplate, opsterv1.OpensearchIndexTemplate](ctx, r.Client, req, "OpensearchIndexTemplate")
}

func (r *IndexTemplateReverseMigrationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&opensearchv1.OpensearchIndexTemplate{}).
		Complete(r)
}

// ComponentTemplateReverseMigrationReconciler watches NEW API and creates OLD API resources
type ComponentTemplateReverseMigrationReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=opensearch.org,resources=opensearchcomponenttemplates,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=opensearch.org,resources=opensearchcomponenttemplates/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=opensearch.org,resources=opensearchcomponenttemplates/finalizers,verbs=update

func (r *ComponentTemplateReverseMigrationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	return reconcileGenericReverseMigration[opensearchv1.OpensearchComponentTemplate, opsterv1.OpensearchComponentTemplate](ctx, r.Client, req, "OpensearchComponentTemplate")
}

func (r *ComponentTemplateReverseMigrationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&opensearchv1.OpensearchComponentTemplate{}).
		Complete(r)
}

// Generic reverse migration reconciler
func reconcileGenericReverseMigration[NewType, OldType any, NewPtr interface {
	*NewType
	client.Object
}, OldPtr interface {
	*OldType
	client.Object
}](ctx context.Context, c client.Client, req ctrl.Request, resourceKind string) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Get new resource
	newResource := NewPtr(new(NewType))
	err := c.Get(ctx, req.NamespacedName, newResource)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Handle deletion
	if !newResource.GetDeletionTimestamp().IsZero() {
		return handleGenericReverseDeletion[NewType, OldType, NewPtr, OldPtr](ctx, c, newResource, req)
	}

	// Add finalizer if not present
	if !containsString(newResource.GetFinalizers(), MigrationFinalizer) {
		newResource.SetFinalizers(append(newResource.GetFinalizers(), MigrationFinalizer))
		if err := c.Update(ctx, newResource); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Check if this was forward migrated
	annotations := newResource.GetAnnotations()
	if annotations != nil {
		if _, wasMigrated := annotations[MigratedFromAnnotation]; wasMigrated {
			// This was created by forward migration, just sync status
			return syncGenericStatusOldToNew[NewType, OldType, NewPtr, OldPtr](ctx, c, newResource, req)
		}
	}

	// Check if old resource exists
	oldResource := OldPtr(new(OldType))
	err = c.Get(ctx, req.NamespacedName, oldResource)
	if err != nil {
		if errors.IsNotFound(err) {
			// Create old resource
			logger.Info("Creating old API group resource from new", "kind", resourceKind, "name", req.Name)
			return createGenericOldFromNew[NewType, OldType, NewPtr, OldPtr](ctx, c, newResource, req)
		}
		return ctrl.Result{}, err
	}

	// Sync status from old to new
	return syncGenericStatusOldToNew[NewType, OldType, NewPtr, OldPtr](ctx, c, newResource, req)
}

func createGenericOldFromNew[NewType, OldType any, NewPtr interface {
	*NewType
	client.Object
}, OldPtr interface {
	*OldType
	client.Object
}](ctx context.Context, c client.Client, newResource NewPtr, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Create old resource with same name/namespace
	oldResource := OldPtr(new(OldType))

	// Convert using JSON
	newBytes, err := json.Marshal(newResource)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to marshal new resource: %w", err)
	}

	if err := json.Unmarshal(newBytes, oldResource); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to unmarshal to old resource: %w", err)
	}

	// Set metadata
	oldResource.SetName(newResource.GetName())
	oldResource.SetNamespace(newResource.GetNamespace())
	oldResource.SetLabels(newResource.GetLabels())
	oldResource.SetResourceVersion("")
	oldResource.SetUID("")

	// Set annotations
	annotations := make(map[string]string)
	for k, v := range newResource.GetAnnotations() {
		if k != MigratedFromAnnotation && k != MigrationTimestampAnnotation && k != SourceUIDAnnotation {
			annotations[k] = v
		}
	}
	annotations[ReverseMigratedFromAnnotation] = "opensearch.org/v1"
	annotations[MigrationTimestampAnnotation] = time.Now().UTC().Format(time.RFC3339)
	annotations[SourceUIDAnnotation] = string(newResource.GetUID())
	oldResource.SetAnnotations(annotations)

	// Clear finalizers on old resource
	oldResource.SetFinalizers(nil)

	if err := c.Create(ctx, oldResource); err != nil {
		if errors.IsAlreadyExists(err) {
			logger.Info("Old API group resource already exists", "name", req.Name)
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	logger.Info("Created old API group resource", "name", req.Name, "namespace", req.Namespace)
	return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
}

func syncGenericStatusOldToNew[NewType, OldType any, NewPtr interface {
	*NewType
	client.Object
}, OldPtr interface {
	*OldType
	client.Object
}](ctx context.Context, c client.Client, newResource NewPtr, req ctrl.Request) (ctrl.Result, error) {
	// Get old resource to read status
	oldResource := OldPtr(new(OldType))
	err := c.Get(ctx, req.NamespacedName, oldResource)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
		}
		return ctrl.Result{}, err
	}

	// For now, just requeue periodically
	// A full implementation would sync status fields
	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

func handleGenericReverseDeletion[NewType, OldType any, NewPtr interface {
	*NewType
	client.Object
}, OldPtr interface {
	*OldType
	client.Object
}](ctx context.Context, c client.Client, newResource NewPtr, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	if containsString(newResource.GetFinalizers(), MigrationFinalizer) {
		// Only delete old resource if it was created by reverse migration
		oldResource := OldPtr(new(OldType))
		err := c.Get(ctx, types.NamespacedName{Name: req.Name, Namespace: req.Namespace}, oldResource)
		if err == nil {
			annotations := oldResource.GetAnnotations()
			if annotations != nil {
				if _, wasReverseMigrated := annotations[ReverseMigratedFromAnnotation]; wasReverseMigrated {
					logger.Info("Deleting old API group resource due to new resource deletion", "name", req.Name)
					if err := c.Delete(ctx, oldResource); err != nil && !errors.IsNotFound(err) {
						return ctrl.Result{}, err
					}
				}
			}
		}

		// Remove finalizer
		newResource.SetFinalizers(removeString(newResource.GetFinalizers(), MigrationFinalizer))
		if err := c.Update(ctx, newResource); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}
