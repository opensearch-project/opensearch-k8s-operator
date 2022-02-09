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
	sts "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	opsterv1 "opensearch.opster.io/api/v1"
	"opensearch.opster.io/controllers/dashboard"
	"opensearch.opster.io/pkg/builders"
	"opensearch.opster.io/pkg/helpers"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/record"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// OpenSearchClusterReconciler reconciles a OpenSearchCluster object
type OpenSearchClusterReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
	Instance *opsterv1.OpenSearchCluster
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
	//_ = log.FromContext(ctx)
	//	reqLogger := r.Log.WithValues("es", req.NamespacedName)
	//	reqLogger.Info("=== Reconciling ES")
	myFinalizerName := "Opster"

	instance := &opsterv1.OpenSearchCluster{}
	err := r.Get(context.TODO(), req.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// object not found, could have been deleted after
			// reconcile request, hence don't requeue
			return ctrl.Result{}, nil
		}

		// error reading the object, requeue the request
		return ctrl.Result{}, err
	} else {

		/// ------ check if CRD has been deleted ------ ///
		///	if ns deleted, delete the associated resources ///
		if instance.ObjectMeta.DeletionTimestamp.IsZero() {
			if !helpers.ContainsString(instance.GetFinalizers(), myFinalizerName) {
				controllerutil.AddFinalizer(instance, myFinalizerName)
				if err := r.Update(ctx, instance); err != nil {
					return ctrl.Result{}, err
				}
			}

		} else {
			if helpers.ContainsString(instance.GetFinalizers(), myFinalizerName) {
				// our finalizer is present, so lets handle any external dependency
				if err := r.deleteExternalResources(instance); err != nil {
					// if fail to delete the external dependency here, return with error
					// so that it can be retried
					return ctrl.Result{}, err
				}

				// remove our finalizer from the list and update it.
				controllerutil.RemoveFinalizer(instance, myFinalizerName)
				if err := r.Update(ctx, instance); err != nil {
					return ctrl.Result{}, err
				}
				return ctrl.Result{}, nil
			}
		}
	}

	/// if crd not deleted started phase 1
	if instance.Status.Phase == "" {
		instance.Status.Phase = opsterv1.PhasePending

	}
	var errs error
	var res ctrl.Result

	fmt.Println("ENTER switch")
	switch instance.Status.Phase {
	case opsterv1.PhasePending:
		//	reqLogger.info("start reconcile - Phase: PENDING")

		instance.Status.Phase = opsterv1.PhaseRunning
		componentStatus := opsterv1.ComponentsStatus{
			Component:   "",
			Status:      "",
			Description: "",
		}
		instance.Status.ComponentsStatus = append(instance.Status.ComponentsStatus, componentStatus)
		err = r.Status().Update(context.TODO(), instance)
		if err != nil {
			return ctrl.Result{}, err

		}
		return ctrl.Result{Requeue: true}, errs

	case opsterv1.PhaseRunning:
		fmt.Println("start reconcile - Phase: RUNNING")

		/// ------ Create NameSpace -------

		ns_query := &corev1.Namespace{}
		// try to see if the ns already exists
		err := r.Get(context.TODO(), client.ObjectKey{Name: instance.Spec.General.ClusterName}, ns_query)
		if err == nil {
			namespace := instance.Spec.General.ClusterName
			nodeGroups := len(instance.Spec.NodePools)

			// if ns is already exist, check if all cluster and kibana are deployed properly - if a necessary resource ass been deleted - recreate it

			cluster := ClusterReconciler{
				Client:   r.Client,
				Scheme:   r.Scheme,
				Recorder: r.Recorder,
				State:    State{},
				Instance: instance,
			}

			res, errs = cluster.Reconcile(ctx, req)
			if cluster.State.Status == "Failed" {
				fmt.Println(res)
				return ctrl.Result{}, errs
			}

			fmt.Println("out of cluster controller------------ -")

			//r.Recorder.Event(instance, "Normal", "Cluster keeps his desired state", fmt.Sprintf("Cluster %s has been created - (please wait few minutes for fully operative cluster))", instance.Spec.General.ClusterName))

			fmt.Println("after recored cluster controller------------ -")

			for nodeGroup := 0; nodeGroup < nodeGroups; nodeGroup++ {
				// Get the existing StatefulSet from the cluster
				sts_from_env := sts.StatefulSet{}
				sts_name := instance.Spec.General.ClusterName + "-" + instance.Spec.NodePools[nodeGroup].Component

				if err := r.Get(context.TODO(), client.ObjectKey{Name: sts_name, Namespace: namespace}, &sts_from_env); err != nil {
					return ctrl.Result{}, err
				}
				scale := ScalerReconciler{
					Client:     r.Client,
					Scheme:     r.Scheme,
					Recorder:   r.Recorder,
					State:      State{},
					Instance:   instance,
					StsFromEnv: sts_from_env,
					Group:      nodeGroup,
				}
				res, errs = scale.Reconcile(ctx, req)
				if errs != nil {
					instance.Status.Phase = opsterv1.PhaseError
				}
			}
			if res.Requeue {
				return ctrl.Result{Requeue: true}, nil

			}
			if instance.Spec.Dashboards.Enable {

				dash := dashboard.DashboardReconciler{
					Client:   r.Client,
					Scheme:   r.Scheme,
					Recorder: r.Recorder,
					State:    dashboard.State{},
					Instance: instance,
				}
				res, errs = dash.Reconcile(ctx, req)
				if errs != nil {
					instance.Status.Phase = opsterv1.PhaseError
				}
				if res.Requeue {
					return ctrl.Result{Requeue: true}, nil

				}
				//r.Recorder.Event(instance, "Normal", "Kibana keeps his desired state", fmt.Sprintf("Kibana %s has been created - (please wait few minutes for fully operative cluster))", instance.Spec.General.ClusterName))
			}
		} else {

			// if ns not exist in cluster, try to create it

			ns := builders.NewNsForCR(instance)
			err = r.Create(context.TODO(), ns)
			if err != nil {
				// if ns cannot create ,  inform it and done reconcile
				fmt.Println(err, "Cannot create namespace")
				instance.Status.Phase = opsterv1.PhaseError
				r.Recorder.Event(instance, "ERROR", "Cluster cannot be created", "cannot create cluster")
				return ctrl.Result{}, nil
			}
			fmt.Println("ns Created successfully", "name", ns.Name)
			return ctrl.Result{Requeue: true}, nil

		}

		// -------- all resources has been created -----------

		return ctrl.Result{Requeue: true, RequeueAfter: 30 * time.Second}, nil

	case opsterv1.PhaseError:
		r.Recorder.Event(instance, "ERROR", "The operator faced some errors", "")

		return ctrl.Result{}, nil

	default:
		//	reqLogger.Info("NOTHING WILL HAPPEN - DEFAULT")
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
func (r *OpenSearchClusterReconciler) deleteExternalResources(es *opsterv1.OpenSearchCluster) error {
	namespace := es.Spec.General.ClusterName

	nsToDel := builders.NewNsForCR(es)

	fmt.Println("Cluster", es.Name, "has been deleted, Delete namesapce ", namespace)
	err := r.Delete(context.TODO(), nsToDel)
	if err != nil {
		return err
	}
	fmt.Println("NS", namespace, "Deleted successfully")
	return nil
}

//func getField(v *sts.StatefulSet, field string) int {
//	r := reflect.ValueOf(v)
//	f := reflect.Indirect(r).FieldByName(field)
//	return int(f.Int())
//}
