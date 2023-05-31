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
	"fmt"
	"k8s.io/api/extensions/v1beta1"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	"opensearch.opster.io/pkg/builders"
	"opensearch.opster.io/pkg/helpers"
	"opensearch.opster.io/pkg/reconcilers"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	opsterv1 "opensearch.opster.io/api/v1"
)

// OpenSearchClusterReconciler reconciles a OpenSearchCluster object
type OpenSearchClusterReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
	Instance *opsterv1.OpenSearchCluster
	Logger   logr.Logger
}

//+kubebuilder:rbac:groups="opensearch.opster.io",resources=events,verbs=create;patch
//+kubebuilder:rbac:groups=opensearch.opster.io,resources=opensearchclusters,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=opensearch.opster.io,resources=opensearchclusters/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=opensearch.opster.io,resources=opensearchclusters/finalizers,verbs=update
//+kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=namespaces,verbs=get;list;create;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=events,verbs=create;update;patch
//+kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete

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
	r.Logger = log.FromContext(ctx).WithValues("cluster", req.NamespacedName)
	r.Logger.Info("Reconciling OpenSearchCluster")
	myFinalizerName := "Opster"

	r.Instance = &opsterv1.OpenSearchCluster{}
	err := r.Get(ctx, req.NamespacedName, r.Instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// object not found, could have been deleted after
			// reconcile request, hence don't requeue
			return ctrl.Result{}, nil
		}
		// error reading the object, requeue the request
		return ctrl.Result{}, err
	}
	/// ------ check if CRD has been deleted ------ ///
	///	if ns deleted, delete the associated resources ///
	if r.Instance.ObjectMeta.DeletionTimestamp.IsZero() {
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
		}
		return ctrl.Result{}, nil
	}

	/// if crd not deleted started phase 1
	if r.Instance.Status.Phase == "" {
		r.Instance.Status.Phase = opsterv1.PhasePending
	}

	switch r.Instance.Status.Phase {
	case opsterv1.PhasePending:
		return r.reconcilePhasePending(ctx)
	case opsterv1.PhaseRunning:
		return r.reconcilePhaseRunning(ctx)
	default:
		r.Logger.Info("NOTHING WILL HAPPEN - DEFAULT")
		return ctrl.Result{Requeue: true, RequeueAfter: 30 * time.Second}, nil
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *OpenSearchClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&opsterv1.OpenSearchCluster{}).
		Owns(&corev1.Pod{}).
		Owns(&corev1.Secret{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.Service{}).
		Owns(&appsv1.Deployment{}).
		Owns(&appsv1.StatefulSet{}).
		Complete(r)
}

// delete associated cluster resources //
func (r *OpenSearchClusterReconciler) deleteExternalResources(ctx context.Context) (ctrl.Result, error) {
	r.Logger.Info("Deleting resources")
	// Run through all sub controllers to delete existing objects
	reconcilerContext := reconcilers.NewReconcilerContext(r.Instance.Spec.NodePools)

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

	componentReconcilers := []reconcilers.ComponentReconciler{
		tls.DeleteResources,
		securityconfig.DeleteResources,
		config.DeleteResources,
		cluster.DeleteResources,
		dashboards.DeleteResources,
	}
	for _, rec := range componentReconcilers {
		result, err := rec()
		if err != nil || result.Requeue {
			return result, err
		}
	}
	r.Logger.Info("Finished deleting resources")
	return ctrl.Result{}, nil
}

func (r *OpenSearchClusterReconciler) reconcilePhasePending(ctx context.Context) (ctrl.Result, error) {
	r.Logger.Info("start reconcile - Phase: PENDING")
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		if err := r.Get(ctx, client.ObjectKeyFromObject(r.Instance), r.Instance); err != nil {
			return err
		}
		r.Instance.Status.Phase = opsterv1.PhaseRunning
		r.Instance.Status.ComponentsStatus = make([]opsterv1.ComponentStatus, 0)
		return r.Status().Update(ctx, r.Instance)
	})
	if err != nil {
		return ctrl.Result{}, err

	}
	return ctrl.Result{Requeue: true}, nil
}

func (r *OpenSearchClusterReconciler) reconcilePhaseRunning(ctx context.Context) (ctrl.Result, error) {
	// Update initialized status first
	if !r.Instance.Status.Initialized {
		if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			if err := r.Get(ctx, client.ObjectKeyFromObject(r.Instance), r.Instance); err != nil {
				return err
			}
			r.Instance.Status.Initialized = builders.AllMastersReady(ctx, r.Client, r.Instance)
			return r.Status().Update(ctx, r.Instance)
		}); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Run through all sub controllers to create or update all needed objects
	reconcilerContext := reconcilers.NewReconcilerContext(r.Instance.Spec.NodePools)

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
	upgradeChecker := reconcilers.NewUpgradeCheckerReconciler(
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

	DeployList := v1beta1.DeploymentList{}
	componentReconcilers := []reconcilers.ComponentReconciler{}

	// check the UpgradeChecker env inside of the manager deployment is true
	if err := r.List(ctx, &DeployList); err != nil {
		fmt.Println("Cannot find UID secret")
		// Handle the error
	}
	//secretList := &v1core.SecretList{}
	for _, deploy := range DeployList.Items {
		if deploy.Name == "opensearch-operator-controller-manager" {
			envVars := deploy.Spec.Template.Spec.Containers[0].Env
			for _, envVar := range envVars {
				// check each envVar, which is of type corev1.EnvVar
				// thats logic is for already running users that will upgrade to that version, the controller

				if envVar.Name == "UpgradeChecker" { //check if upgrade Chcker is enable
					if envVar.Value == "true" { // if not ture, the upgrade checker  is disabled and we building the reconciler list without the UpgradeCheckerReconciler.

						//if err := r.List(ctx, secretList); err != nil {
						//	r.Logger.Error(err, "Cannot list secrets")
						//	break
						//}
						//
						//for _, secret := range secretList.Items {
						//	if secret.Name == "operator-uid" {
						//		value, ok := secret.Data["secretKey"]
						//		if !ok {
						//			r.Logger.Info("Cannot secretKey inside of UID secret", value)
						//		}
						//		break
						//	}
						//}
						//r.Logger.Info("UpgradeChecker is enabled but Operator cannot find UID secret,creating one")
						//uid, err := helpers.GenerateSecretString()
						//if err != nil {
						//	r.Logger.Info("UpgradeChecker Cannot create UID secret - aborting ")
						//	break
						//}
						//// Create the secret object
						//secret := &corev1.Secret{
						//	ObjectMeta: metav1.ObjectMeta{
						//		Name:      "operator-uid",
						//		Namespace: deploy.Namespace,
						//	},
						//	Data: map[string][]byte{
						//		"secretKey": []byte(uid),
						//	},
						//}
						//
						//createdSecret, err := client.crt(context.TODO(), secret, metav1.CreateOptions{})
						//if err != nil {
						//	r.Logger.Info("UpgradeChecker Failed create UID secret - aborting ")
						//	break
						//}

						componentReconcilers = []reconcilers.ComponentReconciler{
							upgradeChecker.Reconcile,
							tls.Reconcile,
							securityconfig.Reconcile,
							config.Reconcile,
							cluster.Reconcile,
							scaler.Reconcile,
							dashboards.Reconcile,
							upgrade.Reconcile,
							restart.Reconcile,
						}
					} else {
						componentReconcilers = []reconcilers.ComponentReconciler{
							tls.Reconcile,
							securityconfig.Reconcile,
							config.Reconcile,
							cluster.Reconcile,
							scaler.Reconcile,
							dashboards.Reconcile,
							upgrade.Reconcile,
							restart.Reconcile,
						}
					}
				}
			}
		}

	}
	// check if the componentReconcilers is nil cause of break in the last for
	//if componentReconcilers == nil {
	//	componentReconcilers = []reconcilers.ComponentReconciler{
	//		tls.Reconcile,
	//		securityconfig.Reconcile,
	//		config.Reconcile,
	//		cluster.Reconcile,
	//		scaler.Reconcile,
	//		dashboards.Reconcile,
	//		upgrade.Reconcile,
	//		restart.Reconcile,
	//	}
	//}

	for _, rec := range componentReconcilers {
		result, err := rec()
		if err != nil || result.Requeue {
			return result, err
		}
	}

	// -------- all resources has been created -----------
	return ctrl.Result{Requeue: true, RequeueAfter: 30 * time.Second}, nil
}
