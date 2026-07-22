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

	"github.com/go-logr/logr"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/builders"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/helpers"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/reconcilers"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/reconcilers/k8s"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/reconcilers/util"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	opensearchv1 "github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/api/opensearch.org/v1"
	opsterv1 "github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/api/v1"
)

// OpenSearchClusterReconciler reconciles a OpenSearchCluster object
// Now reconciles opensearch.org/v1 API group (new API) instead of opensearch.opster.io/v1 (old API)
type OpenSearchClusterReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
	Instance *opensearchv1.OpenSearchCluster
	logr.Logger
}

//+kubebuilder:rbac:groups=opensearch.org,resources=opensearchclusters,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=opensearch.org,resources=opensearchclusters/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=opensearch.org,resources=opensearchclusters/finalizers,verbs=update
//+kubebuilder:rbac:groups=opensearch.opster.io,resources=opensearchclusters,verbs=get;list;watch
//+kubebuilder:rbac:groups=opensearch.opster.io,resources=opensearchclusters/status,verbs=get
//+kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps,resources=statefulsets/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=events,verbs=create;update;patch
//+kubebuilder:rbac:groups=core,resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=policy,resources=poddisruptionbudgets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=monitoring.coreos.com,resources=servicemonitors,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the OpenSearchCluster object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.8.3/pkg/reconcile
func (r *OpenSearchClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.Logger = log.FromContext(ctx).WithValues("cluster", req.NamespacedName, "apiGroup", "opensearch.org/v1")
	r.Info("Reconciling OpenSearchCluster (opensearch.org/v1)")
	myFinalizerName := "Opensearch"

	// Try to get new API group resource first
	r.Instance = &opensearchv1.OpenSearchCluster{}
	err := r.Get(ctx, req.NamespacedName, r.Instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// If new API group resource not found, check if old one exists (for backward compatibility during migration)
			oldInstance := &opsterv1.OpenSearchCluster{}
			if err := r.Get(ctx, req.NamespacedName, oldInstance); err == nil {
				// Old instance exists but new one doesn't - migration controller should handle this
				// Just requeue to let migration controller create the new one
				r.Info("Old API group resource exists, waiting for migration", "name", req.Name)
				return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
			}
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	/// ------ check if CRD has been deleted ------ ///
	///	if ns deleted, delete the associated resources ///
	if r.Instance.DeletionTimestamp.IsZero() {
		if !helpers.ContainsString(r.Instance.GetFinalizers(), myFinalizerName) {
			// Use RetryOnConflict to update finalizer to handle any changes to resource
			err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
				if err := r.Get(ctx, req.NamespacedName, r.Instance); err != nil {
					return err
				}
				controllerutil.AddFinalizer(r.Instance, myFinalizerName)
				return r.Update(ctx, r.Instance)
			})
			if err != nil {
				return ctrl.Result{}, err
			}
		}
	} else {
		if helpers.ContainsString(r.Instance.GetFinalizers(), myFinalizerName) {
			// our finalizer is present, so lets handle any external dependency
			if result, err := r.deleteExternalResources(ctx); err != nil {
				// if fail to delete the external dependency here, return with error
				// so that it can be retried
				return result, err
			}

			// remove our finalizer from the list and update it.
			err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
				if err := r.Get(ctx, req.NamespacedName, r.Instance); err != nil {
					return err
				}
				controllerutil.RemoveFinalizer(r.Instance, myFinalizerName)
				return r.Update(ctx, r.Instance)
			})
			if err != nil {
				return ctrl.Result{}, err
			}

			helpers.DeleteClusterMetrics(req.Namespace, r.Instance.Name)
		}
		return ctrl.Result{}, nil
	}

	/// if crd not deleted started phase 1
	if r.Instance.Status.Phase == "" {
		r.Instance.Status.Phase = opensearchv1.PhasePending
	}

	switch r.Instance.Status.Phase {
	case opensearchv1.PhasePending:
		return r.reconcilePhasePending(ctx)
	case opensearchv1.PhaseRunning, opensearchv1.PhaseUpgrading:
		return r.reconcilePhaseRunning(ctx)
	default:
		// NOTHING WILL HAPPEN - DEFAULT
		return ctrl.Result{Requeue: true, RequeueAfter: 30 * time.Second}, nil
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *OpenSearchClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&opensearchv1.OpenSearchCluster{}). // Watch new API group
		Owns(&corev1.Pod{}).
		Owns(&corev1.Secret{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.Service{}).
		Owns(&appsv1.Deployment{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&corev1.PersistentVolumeClaim{}).
		Complete(r)
}

// delete associated cluster resources //
func (r *OpenSearchClusterReconciler) deleteExternalResources(ctx context.Context) (ctrl.Result, error) {
	r.Info("Deleting resources")
	// Run through all sub controllers to delete existing objects
	reconcilerContext := reconcilers.NewReconcilerContext(r.Recorder, r.Instance, r.Instance.Spec.NodePools)

	tls := reconcilers.NewTLSReconciler(
		r.Client,
		ctx,
		&reconcilerContext,
		r.Instance,
	)
	securityconfig := reconcilers.NewSecurityconfigReconciler(
		r.Client,
		ctx,
		r.Recorder,
		&reconcilerContext,
		r.Instance,
	)
	config := reconcilers.NewConfigurationReconciler(
		r.Client,
		ctx,
		r.Recorder,
		&reconcilerContext,
		r.Instance,
	)
	cluster := reconcilers.NewClusterReconciler(
		r.Client,
		ctx,
		r.Recorder,
		&reconcilerContext,
		r.Instance,
	)
	dashboards := reconcilers.NewDashboardsReconciler(
		r.Client,
		ctx,
		r.Recorder,
		&reconcilerContext,
		r.Instance,
	)

	componentReconcilers := []reconcilers.NamedComponentReconciler{
		{Name: tls.Name(), Func: tls.DeleteResources},
		{Name: securityconfig.Name(), Func: securityconfig.DeleteResources},
		{Name: config.Name(), Func: config.DeleteResources},
		{Name: cluster.Name(), Func: cluster.DeleteResources},
		{Name: dashboards.Name(), Func: dashboards.DeleteResources},
	}
	for _, rec := range componentReconcilers {
		result, err := rec.Func()
		if err != nil {
			helpers.ReconcileErrors.WithLabelValues(r.Instance.Namespace, r.Instance.Name, rec.Name).Inc()
			return result, err
		}
		if result.Requeue {
			return result, nil
		}
	}
	r.Info("Finished deleting resources")
	return ctrl.Result{}, nil
}

func (r *OpenSearchClusterReconciler) reconcilePhasePending(ctx context.Context) (ctrl.Result, error) {
	r.Info("Start reconcile - Phase: PENDING")
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		if err := r.Get(ctx, client.ObjectKeyFromObject(r.Instance), r.Instance); err != nil {
			return err
		}
		r.Instance.Status.Phase = opensearchv1.PhaseRunning
		r.Instance.Status.ComponentsStatus = make([]opensearchv1.ComponentStatus, 0)
		return r.Status().Update(ctx, r.Instance)
	})
	if err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{Requeue: true}, nil
}

func (r *OpenSearchClusterReconciler) reconcilePhaseRunning(ctx context.Context) (ctrl.Result, error) {
	// Update initialized status first. Only set Initialized when all master pods are ready
	// and the OpenSearch cluster API is reachable (cluster has formed). This prevents
	// deleting the bootstrap pod before the cluster has actually bootstrapped, which
	// can happen with parallel pod management when pods report ready before quorum is formed.
	if !r.Instance.Status.Initialized {
		if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			if err := r.Get(ctx, client.ObjectKeyFromObject(r.Instance), r.Instance); err != nil {
				return err
			}
			allMastersReady := builders.AllMastersReady(ctx, r.Client, r.Instance)
			k8sClient := k8s.NewK8sClient(r.Client, ctx)
			health, _ := util.GetClusterHealth(k8sClient, ctx, r.Instance, r.Logger)
			clusterReachable := health != opensearchv1.OpenSearchUnknownHealth
			r.Instance.Status.Initialized = allMastersReady && clusterReachable
			return r.Status().Update(ctx, r.Instance)
		}); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Run through all sub controllers to create or update all needed objects
	reconcilerContext := reconcilers.NewReconcilerContext(r.Recorder, r.Instance, r.Instance.Spec.NodePools)

	tls := reconcilers.NewTLSReconciler(
		r.Client,
		ctx,
		&reconcilerContext,
		r.Instance,
	)
	securityconfig := reconcilers.NewSecurityconfigReconciler(
		r.Client,
		ctx,
		r.Recorder,
		&reconcilerContext,
		r.Instance,
	)
	config := reconcilers.NewConfigurationReconciler(
		r.Client,
		ctx,
		r.Recorder,
		&reconcilerContext,
		r.Instance,
	)
	cluster := reconcilers.NewClusterReconciler(
		r.Client,
		ctx,
		r.Recorder,
		&reconcilerContext,
		r.Instance,
	)
	scaler := reconcilers.NewScalerReconciler(
		r.Client,
		ctx,
		r.Recorder,
		&reconcilerContext,
		r.Instance,
	)
	dashboards := reconcilers.NewDashboardsReconciler(
		r.Client,
		ctx,
		r.Recorder,
		&reconcilerContext,
		r.Instance,
	)
	upgrade := reconcilers.NewUpgradeReconciler(
		r.Client,
		ctx,
		r.Recorder,
		&reconcilerContext,
		r.Instance,
	)
	restart := reconcilers.NewRollingRestartReconciler(
		r.Client,
		ctx,
		r.Recorder,
		&reconcilerContext,
		r.Instance,
	)
	snapshotrepository := reconcilers.NewSnapshotRepositoryReconciler(
		r.Client,
		ctx,
		r.Recorder,
		r.Instance,
	)

	componentReconcilers := []reconcilers.NamedComponentReconciler{
		{Name: tls.Name(), Func: tls.Reconcile},
		{Name: securityconfig.Name(), Func: securityconfig.Reconcile},
		{Name: config.Name(), Func: config.Reconcile},
		{Name: cluster.Name(), Func: cluster.Reconcile},
		{Name: scaler.Name(), Func: scaler.Reconcile},
		{Name: dashboards.Name(), Func: dashboards.Reconcile},
		{Name: upgrade.Name(), Func: upgrade.Reconcile},
		{Name: restart.Name(), Func: restart.Reconcile},
		{Name: snapshotrepository.Name(), Func: snapshotrepository.Reconcile},
	}
	for _, rec := range componentReconcilers {
		result, err := rec.Func()
		if err != nil {
			helpers.ReconcileErrors.WithLabelValues(r.Instance.Namespace, r.Instance.Name, rec.Name).Inc()
			return result, err
		}
		if result.Requeue {
			return result, nil
		}
	}

	// -------- all resources has been created -----------
	return ctrl.Result{Requeue: true, RequeueAfter: 30 * time.Second}, nil
}
