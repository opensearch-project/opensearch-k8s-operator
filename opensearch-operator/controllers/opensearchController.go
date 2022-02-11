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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/record"
	"opensearch.opster.io/pkg/builders"
	"opensearch.opster.io/pkg/helpers"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

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
	logr.Logger
}

//+kubebuilder:rbac:groups="opensearch.opster.io",resources=events,verbs=create;patch
//+kubebuilder:rbac:groups=opensearch.opster.io,resources=opensearchcluster,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=opensearch.opster.io,resources=opensearchcluster/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=opensearch.opster.io,resources=opensearchcluster/finalizers,verbs=update

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
	r.Logger = log.Log.WithValues("cluster", req.NamespacedName)
	r.Logger.Info("Reconciling OpenSearchCluster")
	myFinalizerName := "Opster"

	r.Instance = &opsterv1.OpenSearchCluster{}
	err := r.Get(context.TODO(), req.NamespacedName, r.Instance)
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
			controllerutil.AddFinalizer(r.Instance, myFinalizerName)
			if err := r.Update(ctx, r.Instance); err != nil {
				return ctrl.Result{}, err
			}
		}

	} else {
		if helpers.ContainsString(r.Instance.GetFinalizers(), myFinalizerName) {
			// our finalizer is present, so lets handle any external dependency
			if err := r.deleteExternalResources(r.Instance); err != nil {
				// if fail to delete the external dependency here, return with error
				// so that it can be retried
				return ctrl.Result{}, err
			}

			// remove our finalizer from the list and update it.
			controllerutil.RemoveFinalizer(r.Instance, myFinalizerName)
			if err := r.Update(ctx, r.Instance); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}
	}

	/// if crd not deleted started phase 1
	if r.Instance.Status.Phase == "" {
		r.Instance.Status.Phase = opsterv1.PhasePending
	}

	switch r.Instance.Status.Phase {
	case opsterv1.PhasePending:
		return r.reconcilePhasePending()
	case opsterv1.PhaseRunning:
		return r.reconcilePhaseRunning()
	default:
		r.Logger.Info("NOTHING WILL HAPPEN - DEFAULT")
		return ctrl.Result{Requeue: true, RequeueAfter: 30 * time.Second}, nil
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *OpenSearchClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&opsterv1.OpenSearchCluster{}).
		Complete(r)
}

// delete associated cluster resources //
func (r *OpenSearchClusterReconciler) deleteExternalResources(cluster *opsterv1.OpenSearchCluster) error {
	namespace := cluster.Spec.General.ClusterName
	nsToDel := builders.NewNsForCR(cluster)

	r.Logger.Info("Cluster has been deleted. Deleting namespace")
	err := r.Delete(context.TODO(), nsToDel)
	if err != nil {
		return err
	}
	r.Logger.Info("Namespace deleted successfully", "namespace", namespace)
	return nil
}

func (r *OpenSearchClusterReconciler) reconcilePhasePending() (ctrl.Result, error) {
	r.Logger.Info("start reconcile - Phase: PENDING")
	r.Instance.Status.Phase = opsterv1.PhaseRunning
	componentStatus := opsterv1.ComponentStatus{
		Component:   "",
		Status:      "",
		Description: "",
	}
	r.Instance.Status.ComponentsStatus = append(r.Instance.Status.ComponentsStatus, componentStatus)
	err := r.Status().Update(context.TODO(), r.Instance)
	if err != nil {
		return ctrl.Result{}, err

	}
	return ctrl.Result{Requeue: true}, nil
}

func (r *OpenSearchClusterReconciler) reconcilePhaseRunning() (ctrl.Result, error) {

	/// ------ Create NameSpace -------
	ns_query := &corev1.Namespace{}
	// try to see if the ns already exists
	err := r.Get(context.TODO(), client.ObjectKey{Name: r.Instance.Spec.General.ClusterName}, ns_query)
	if err != nil {
		result, err := r.createNamespace(r.Instance)
		if err != nil {
			return result, err
		}
	}

	// Run through all sub controllers to create or update all needed objects
	controllerContext := NewControllerContext()

	// Reconcile TLS config
	tls := TlsReconciler{
		Client:   r.Client,
		Recorder: r.Recorder,
		Logger:   r.Logger.WithValues("controller", "tls"),
		Instance: r.Instance,
	}
	state, err := tls.Reconcile(&controllerContext)
	r.updateStatus(state)
	if err != nil {
		return ctrl.Result{Requeue: true, RequeueAfter: 30 * time.Second}, nil
	}

	// Reconcile opensearch configuration
	config := ConfigurationReconciler{
		Client:   r.Client,
		Recorder: r.Recorder,
		Logger:   r.Logger.WithValues("controller", "configuration"),
		Instance: r.Instance,
	}
	state, err = config.Reconcile(&controllerContext)
	r.updateStatus(state)
	if err != nil {
		return ctrl.Result{Requeue: true, RequeueAfter: 30 * time.Second}, nil
	}

	// Reconcile cluster
	cluster := ClusterReconciler{
		Client:   r.Client,
		Recorder: r.Recorder,
		Logger:   r.Logger.WithValues("controller", "cluster"),
		Instance: r.Instance,
	}
	state, err = cluster.Reconcile(&controllerContext)
	r.updateStatus(state)
	if err != nil {
		return ctrl.Result{Requeue: true, RequeueAfter: 30 * time.Second}, nil
	}

	// Scaler Reconcile
	scaler := ScalerReconciler{
		Client:   r.Client,
		Recorder: r.Recorder,
		Logger:   r.Logger.WithValues("controller", "scaler"),
		Instance: r.Instance,
	}
	status, err := scaler.Reconcile(&controllerContext)
	r.updateStatusList(status)
	if err != nil {
		return ctrl.Result{Requeue: true, RequeueAfter: 30 * time.Second}, nil
	}

	if r.Instance.Spec.Dashboards.Enable {
		dash := DashboardReconciler{
			Client:   r.Client,
			Recorder: r.Recorder,
			Logger:   r.Logger.WithValues("controller", "dashboards"),
			Instance: r.Instance,
		}
		state, err = dash.Reconcile(&controllerContext)
		r.updateStatus(state)
		if err != nil {
			return ctrl.Result{Requeue: true, RequeueAfter: 10 * time.Second}, nil
		}
	}

	// -------- all resources has been created -----------
	return ctrl.Result{Requeue: true, RequeueAfter: 30 * time.Second}, nil
}

func (r *OpenSearchClusterReconciler) createNamespace(instance *opsterv1.OpenSearchCluster) (ctrl.Result, error) {
	ns := builders.NewNsForCR(instance)
	err := r.Create(context.TODO(), ns)
	if err != nil {
		// if ns cannot create ,  inform it and done reconcile
		r.Logger.Error(err, "Failed to create namespace")
		r.Recorder.Event(r.Instance, "Warning", "Cannot create Namespace for cluster", "requeuing - fix the problem that you have with creating Namespace for cluster")
		return ctrl.Result{Requeue: true, RequeueAfter: 30 * time.Second}, nil
	}
	r.Logger.Info("Namespace created successfully", "namespace", ns.Name)
	return ctrl.Result{}, nil
}

func (r *OpenSearchClusterReconciler) updateStatus(status *opsterv1.ComponentStatus) {
	if status != nil {
		found := false
		for idx, value := range r.Instance.Status.ComponentsStatus {
			if value.Component == status.Component {
				r.Instance.Status.ComponentsStatus[idx] = *status
				found = true
				break
			}
		}
		if !found {
			r.Instance.Status.ComponentsStatus = append(r.Instance.Status.ComponentsStatus, *status)
		}
		r.Status().Update(context.TODO(), r.Instance)
	}
}

func (r *OpenSearchClusterReconciler) updateStatusList(statusList []opsterv1.ComponentStatus) {
	for _, status := range statusList {
		r.updateStatus(&status)
	}
}
