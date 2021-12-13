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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"os-operator.io/pkg/builders"
	"os-operator.io/pkg/helpers"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	opsterv1 "os-operator.io/api/v1"

)

// OsReconciler reconciles a Os object
type OsReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

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
	}

	// if no phase set, default to Pending
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
		//	reqLogger.info("start reconcile - Phase: RUNNING")

		//err = ctrl.SetControllerReference(instance, ns, r.Scheme)
		//if err != nil {
		//	// requeue with error
		//	return ctrl.Result{}, err
		//}
		//err := ctrl.SetControllerReference(instance, sts, r.Scheme)
		//if err != nil {
		//	// requeue with error
		//	return ctrl.Result{}, err
		//}
		//err = ctrl.SetControllerReference(instance, service, r.Scheme)
		//if err != nil {
		//	// requeue with error
		//	return ctrl.Result{}, err
		//}

		myFinalizerName := "Opster"

		/// ------ check if CRD has been deleted ------
		///	if ns deleted, delete the associated resources///

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

		/// ----- build resources ------
		service := builders.NewServiceForCR(instance)
		stsm := builders.NewMasterSTSForCR(instance)
		ns := builders.NewNsForCR(instance)
		cm := builders.NewCmForCR(instance)
		headless_service := builders.NewHeadlessServiceForCR(instance)
		stsn := builders.NewNodeSTSForCR(instance)


		kibana := builders.NewKibanaForCR(instance)
		kibana_cm := builders.NewCmKibanaForCR(instance)
		kibana_service := builders.NewKibanaSvcForCr(instance)

		/// ------ Create NameSpace -------
		ns_query := &corev1.Namespace{}
		// try to see if the ns already exists
		err = r.Get(context.TODO(), req.NamespacedName, ns_query)
		if err != nil && errors.IsNotFound(err) {
			// does not exist, create a ns
			err = r.Create(context.TODO(), ns)
			if err != nil {
				return ctrl.Result{}, err
			}
			// Successfully created a ns
			fmt.Println("ns Created successfully", "name", ns.Name)
		} else if err != nil {
			// requeue with err
			//		reqLogger.Error(err, "cannot create ns")
			fmt.Println(err, "Cannot create namespace")
			return ctrl.Result{}, err
		}

		/// if ns not exist, assuming that cluster not exist ////

		/// ------ Create ConfigMap -------
		err = r.Create(context.TODO(), cm)
		if err != nil {
			fmt.Println(err, "Cannot create Configmap "+cm.Name)
			return ctrl.Result{}, err
		}
		fmt.Println("Cm Created successfully", "name", cm.Name)

		/// ------ Create Headleass Service -------
		err = r.Create(context.TODO(), headless_service)
		if err != nil {
			fmt.Println(err, "Cannot create Headless Service")
			return ctrl.Result{}, err
		}
		fmt.Println("service Created successfully", "name", headless_service.Name)


		/// ------ Create External Service -------
		err = r.Create(context.TODO(), service)
		if err != nil {
			fmt.Println(err, "Cannot create service")
			return ctrl.Result{}, err
		}
		fmt.Println("service Created successfully", "name", service.Name)



		/// ------ Create Es Masters StatefulSet -------
		err = r.Create(context.TODO(), stsm)
		if err != nil {
			fmt.Println(err, "Cannot create STS")
			return ctrl.Result{}, err
		}
		fmt.Println("StatefulSet Created successfully", "name", stsm.Name)


		/// ------ Create Es Nodes StatefulSet -------
		err = r.Create(context.TODO(), stsn)
		if err != nil {
			fmt.Println(err, "Cannot create STS")
			return ctrl.Result{}, err
		}
		fmt.Println("StatefulSet Created successfully", "name", stsn.Name)


		/// ------ create opensearch dashboard cm ------- ///
		err = r.Create(context.TODO(), kibana_cm)
		if err != nil {
			fmt.Println(err, "Cannot create Kibana Configmap "+cm.Name)
			return ctrl.Result{}, err
		}
		fmt.Println("KIbana Cm Created successfully", "name", cm.Name)

		/// -------- create kibana service ------- ///
		err = r.Create(context.TODO(), kibana_service)
		if err != nil {
			fmt.Println(err, "Cannot create Kibana service "+cm.Name)
			return ctrl.Result{}, err
		}
		fmt.Println("Kibana service Created successfully", "name", cm.Name)

		/// ------- create kibana sts ------- ///
		err = r.Create(context.TODO(), kibana)
		if err != nil {
			fmt.Println(err, "Cannot create Kibana STS "+cm.Name)
			return ctrl.Result{}, err
		}
		fmt.Println("KIbana STS Created successfully - ", "name : ", cm.Name)

		fmt.Println("Finshed reconcilng (please wait few minutes for fully operative cluster) - Phase: DONE")
		//instance.Status.Phase = opsterv1alpha1.PhaseDone
		return ctrl.Result{}, nil

		// needs to implement errors handling, when error appears delete
		// all related resources and return error without crushing operator

	case opsterv1.PhaseDone:
		//		reqLogger.Info("start reconcile: DONE")
		// reconcile without requeuing
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
