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
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/record"
	"os-operator.io/pkg/builders"
	"os-operator.io/pkg/helpers"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	opsterv1 "os-operator.io/api/v1"
)

// OsReconciler reconciles a Os object
type OsReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

//+kubebuilder:rbac:groups="opster.os-operator.opster.io",resources=events,verbs=create;patch
//+kubebuilder:rbac:groups=opster.os-operator.opster.io,resources=os,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=opster.os-operator.opster.io,resources=os/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=opster.os-operator.opster.io,resources=os/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Os object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.8.3/pkg/reconcile
func (r *OsReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	//_ = log.FromContext(ctx)

	//	reqLogger := r.Log.WithValues("es", req.NamespacedName)
	//	reqLogger.Info("=== Reconciling ES")

	myFinalizerName := "Opster"

	instance := &opsterv1.Os{}
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
	fmt.Println("ENTER switch")
	switch instance.Status.Phase {
	case opsterv1.PhasePending:
		//	reqLogger.info("start reconcile - Phase: PENDING")
		fmt.Println("start reconcile - Phase: PENDING")
		instance.Status.Phase = opsterv1.PhaseRunning
	case opsterv1.PhaseRunning:
		fmt.Println("start reconcile - Phase: RUNNING")

		/// ------ Create NameSpace -------
		ns := builders.NewNsForCR(instance)

		ns_query := &corev1.Namespace{}
		// try to see if the ns already exists
		scaled := false
		new_spec := sts.StatefulSetSpec{}
		err := r.Get(context.TODO(), client.ObjectKey{Name: instance.Spec.General.ClusterName}, ns_query)
		if err == nil {
			// if ns is already exist, check if there is changes in cluster
			// in that function the operator will check if the cluster reached to the desired state
			nodeGroups := len(instance.Spec.OsNodes)
			var scaled_one bool
			for nodeGroup := 0; nodeGroup < nodeGroups; nodeGroup++ {
				// checking if every node group is equal to what configured in the crd
				// Build the StatefulSet has hw should be from the drd
				sts_from_crd := builders.NewSTSForCR(instance, instance.Spec.OsNodes[nodeGroup])
				// Get the existing StatefulSet from the cluster
				sts_from_env := sts.StatefulSet{}
				if err := r.Get(context.TODO(), client.ObjectKey{Name: instance.Spec.General.ClusterName + "-" + instance.Spec.OsNodes[nodeGroup].Compenent, Namespace: instance.Spec.General.ClusterName}, &sts_from_env); err != nil {
					return ctrl.Result{}, err
				}
				//sts_from_crd.Spec.Template.Spec.Containers[1].EnvFrom[1].
				new_spec, scaled_one, err = helpers.CheckUpdates(sts_from_env.Spec, sts_from_crd.Spec, instance, nodeGroup)
				if err != nil {
					r.Recorder.Event(instance, "Warning", "something went worng ", fmt.Sprintf("something went worng)"))
					return ctrl.Result{}, nil
				}
				if scaled_one {
					scaled = true
					r.Recorder.Event(instance, "Normal", "Operator scaled resource", fmt.Sprintf("Scaled %s Replicas - from %s to %s )", instance.Spec.General.ClusterName+"-"+instance.Spec.OsNodes[nodeGroup].Compenent, sts_from_env.Spec.Replicas, sts_from_crd.Spec.Replicas))
					sts_from_env.Spec = new_spec
					if err := r.Update(ctx, &sts_from_env); err != nil {
						return ctrl.Result{}, err
					}
				}

			}
			// if scale = true, so one or more resources has updated - return nil to operator - done reconcile
			if scaled {
				fmt.Println("Scaled - done reconcile")
				r.Recorder.Event(instance, "Normal", "Operator applied new configuration", fmt.Sprintf("Done reconcile"))
				return ctrl.Result{}, nil
			}
		} else {
			// if ns not exist in cluster, try to create it
			err = r.Create(context.TODO(), ns)
			if err != nil {
				// if ns cannot create ,  inform it and done reconcile
				fmt.Println(err, "Cannot create namespace")
				instance.Status.Phase = opsterv1.PhaseError
				r.Recorder.Event(instance, "Warning", "Cluster cannot be created", fmt.Sprintf("cannot create cluster %s )", instance.Spec.General.ClusterName))
				return ctrl.Result{}, nil
			}
			fmt.Println("ns Created successfully", "name", ns.Name)
		}
		// if ns created successfully -  start to create other resources
		//// ------------- start do build other resources ---------------

		/// ------ Create ConfigMap -------
		cm := builders.NewCmForCR(instance)

		err = r.Create(context.TODO(), cm)
		if err != nil {
			fmt.Println(err, "Cannot create Configmap "+cm.Name)
			return ctrl.Result{}, err
		}
		fmt.Println("Cm Created successfully", "name", cm.Name)

		/// ------ Create Headleass Service -------
		headless_service := builders.NewHeadlessServiceForCR(instance)

		err = r.Create(context.TODO(), headless_service)
		if err != nil {
			fmt.Println(err, "Cannot create Headless Service")
			return ctrl.Result{}, err
		}
		fmt.Println("service Created successfully", "name", headless_service.Name)

		/// ------ Create External Service -------
		service := builders.NewServiceForCR(instance)

		err = r.Create(context.TODO(), service)
		if err != nil {
			fmt.Println(err, "Cannot create service")
			return ctrl.Result{}, err
		}
		fmt.Println("service Created successfully", "name", service.Name)

		///// ------ Create Es Nodes StatefulSet -------
		NodesCount := len(instance.Spec.OsNodes)

		for x := 0; x < NodesCount; x++ {
			sts_for_build := builders.NewSTSForCR(instance, instance.Spec.OsNodes[x])
			/// ------ Create Es StatefulSet -------
			fmt.Println("Starting create ", instance.Spec.OsNodes[x].Compenent, " Sts")
			//	r.StsCreate(ctx, &sts_for_build)
			err = r.Create(context.TODO(), sts_for_build)
			if err != nil {
				fmt.Println(err, "Cannot create ", instance.Spec.OsNodes[x].Compenent, " Sts")
				return ctrl.Result{}, err
			}
			fmt.Println(instance.Spec.OsNodes[x].Compenent, " StatefulSet has Created successfully")
		}

		if instance.Spec.OsConfMgmt.Kibana {
			/// ------ create opensearch dashboard cm ------- ///
			os_dash_cm := builders.NewCm_OS_Dashboard_ForCR(instance)

			err = r.Create(context.TODO(), os_dash_cm)
			if err != nil {
				fmt.Println(err, "Cannot create Opensearch-Dashboard Configmap "+cm.Name)
				return ctrl.Result{}, err
			}
			fmt.Println("Opensearch-Dashboard Cm Created successfully", "name", cm.Name)

			/// -------- create Opensearch-Dashboard service ------- ///
			os_dash_service := builders.New_OS_Dashboard_SvcForCr(instance)

			err = r.Create(context.TODO(), os_dash_service)
			if err != nil {
				fmt.Println(err, "Cannot create Opensearch-Dashboard service "+cm.Name)
				return ctrl.Result{}, err
			}
			fmt.Println("Opensearch-Dashboard service Created successfully", "name", cm.Name)

			/// ------- create Opensearch-Dashboard sts ------- ///
			os_dash := builders.New_OS_Dashboard_ForCR(instance)

			err = r.Create(context.TODO(), os_dash)
			if err != nil {
				fmt.Println(err, "Cannot create Opensearch-Dashboard STS "+cm.Name)
				return ctrl.Result{}, err
			}
			fmt.Println("Opensearch-Dashboard STS Created successfully - ", "name : ", cm.Name)
		}

		// -------- all resources has been created -----------
		fmt.Println("Finshed reconcilng (please wait few minutes for fully operative cluster) - Phase: DONE")
		r.Recorder.Event(instance, "Normal", "Cluster has created", fmt.Sprintf("Cluster %s has been created - (please wait few minutes for fully operative cluster) )", instance.Spec.General.ClusterName))

		// if finished to build all resource -  done reconcile
		return ctrl.Result{}, nil

		// needs to implement errors handling, when error appears delete
		// all related resources and return error without crushing operator
		//}
	case opsterv1.PhaseDone:
		//		reqLogger.Info("start reconcile: DONE")
		// reconcile without requeuing
		fmt.Sprint("enter to DONE Phase")
		return ctrl.Result{}, nil

	default:
		//	reqLogger.Info("NOTHING WILL HAPPEN - DEFAULT")
		return ctrl.Result{}, nil

	}
	err = r.Status().Update(context.TODO(), instance)
	if err != nil {
		return ctrl.Result{}, err

	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *OsReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&opsterv1.Os{}).
		Complete(r)
}

// delete associated cluster resources //
func (r *OsReconciler) deleteExternalResources(es *opsterv1.Os) error {
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

//func (r *OsReconciler) StsCreate(ctx context.Context, log logr.Logger, sts *sts.StatefulSet) error {
//
//	if err := r.Create(ctx, sts); err != nil {
//		if !errors.IsConflict(err) {
//			log.V(2).Error(err, "unable to create Statefulset")
//			return err
//		}
//	}
//	return nil
//}

func getField(v *sts.StatefulSet, field string) int {
	r := reflect.ValueOf(v)
	f := reflect.Indirect(r).FieldByName(field)
	return int(f.Int())
}
