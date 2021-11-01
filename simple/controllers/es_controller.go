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
	"opster.io/es/pkg/builders"
	"opster.io/es/pkg/helpers"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	opsterv1alpha1 "opster.io/es/api/v1alpha1"
)

// EsReconciler reconciles a Es object
type EsReconciler struct {
	client.Client
	//	Log    logr.Logger
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=opster.opster.io,resources=es,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=opster.opster.io,resources=es/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=opster.opster.io,resources=es/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Es object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.8.3/pkg/reconcile
func (r *EsReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	//_ = log.FromContext(ctx)

	//	reqLogger := r.Log.WithValues("es", req.NamespacedName)
	//	reqLogger.Info("=== Reconciling ES")
	instance := &opsterv1alpha1.Es{}
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
		instance.Status.Phase = opsterv1alpha1.PhasePending
	}
	fmt.Println("ENTER switch")
	switch instance.Status.Phase {
	case opsterv1alpha1.PhasePending:
		//	reqLogger.info("start reconcile - Phase: PENDING")
		fmt.Println("start reconcile - Phase: PENDING")
		instance.Status.Phase = opsterv1alpha1.PhaseRunning
	case opsterv1alpha1.PhaseRunning:
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

		myFinalizerName := "idan"


		 /// ------ check if CRD has been deleted ------
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
		service :=builders.NewServiceForCR(instance)
		stsm := builders.NewMasterSTSForCR(instance)
		ns := builders.NewNsForCR(instance)
		cm := builders.NewCmForCR(instance)
		headless_service := builders.NewHeadlessServiceForCR(instance)


		/// ------ Create NameSpace -------
		ns_query := &corev1.Namespace{}
		// try to see if the pod already exists
		err = r.Get(context.TODO(), req.NamespacedName, ns_query)
		if err != nil && errors.IsNotFound(err) {
			// does not exist, create a pod
			err = r.Create(context.TODO(), ns)
			if err != nil {
				return ctrl.Result{}, err
			}
			// Successfully created a Pod
			//		reqLogger.Info("Pod Created successfully", "name", pod.Name)
			fmt.Println("ns Created successfully", "name", ns.Name)
		} else if err != nil {
			// requeue with err
			//		reqLogger.Error(err, "cannot create pod")
			fmt.Println(err,"Cannot create namespace")
			return ctrl.Result{}, err
		}

		/// ------ Create ConfigMap -------
		cm_query := &corev1.ConfigMap{}
		// try to see if the pod already exists
		err = r.Get(context.TODO(), req.NamespacedName, cm_query)
		if err != nil && errors.IsNotFound(err) {
			// does not exist, create a pod
			err = r.Create(context.TODO(), cm)
			if err != nil {
				return ctrl.Result{}, err
			}
			// Successfully created a Pod
			//		reqLogger.Info("Pod Created successfully", "name", pod.Name)
			fmt.Println("Cm Created successfully", "name", cm.Name)
		} else if err != nil {
			// requeue with err
			//		reqLogger.Error(err, "cannot create pod")
			fmt.Println(err,"Cannot create Configmap "+ cm.Name)
			return ctrl.Result{}, err
		}

		/// ------ Create Headleass Service -------
		service_query := &corev1.Service{}
		// try to see if the pod already exists
		err = r.Get(context.TODO(), req.NamespacedName, service_query)
		if err != nil && errors.IsNotFound(err) {
			// does not exist, create a pod
			err = r.Create(context.TODO(), headless_service)
			if err != nil {
				return ctrl.Result{}, err
			}
			fmt.Println("service Created successfully", "name", headless_service.Name)
		} else if err != nil {
			// requeue with err
			fmt.Println(err,"Cannot create Headless Service")
			return ctrl.Result{}, err
		}


		/// ------ Create External Service -------
		service_query = &corev1.Service{}
		// try to see if the pod already exists
		err = r.Get(context.TODO(), req.NamespacedName, service_query)
		if err != nil && errors.IsNotFound(err) {
			// does not exist, create a pod
			err = r.Create(context.TODO(), service)
			if err != nil {
				return ctrl.Result{}, err
			}
			fmt.Println("service Created successfully", "name", service.Name)
		} else if err != nil {
			// requeue with err
			fmt.Println(err, "Cannot create service")
			return ctrl.Result{}, err
		}

			/// ------ Create Es Masters StatefulSet -------
			sts_query := &sts.StatefulSet{}
			// try to see if the pod already exists
			err = r.Get(context.TODO(), req.NamespacedName, sts_query)
			if err != nil && errors.IsNotFound(err) {
				// does not exist, create a pod
				err = r.Create(context.TODO(), stsm)
				if err != nil {
					return ctrl.Result{}, err
				}
				fmt.Println("StatefulSet Created successfully", "name", stsm.Name)
				return ctrl.Result{}, nil
			} else if err != nil {
				// requeue with err
				fmt.Println(err, "Cannot create STS")
				return ctrl.Result{}, err
			} else {
				// don't requeue, it will happen automatically when
				// pod status changes
				return ctrl.Result{}, nil
			}

	case opsterv1alpha1.PhaseDone:
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
func (r *EsReconciler) SetupWithManager(mgr ctrl.Manager) error {
	err := ctrl.NewControllerManagedBy(mgr).
		For(&opsterv1alpha1.Es{}).
		Owns(&corev1.Pod{}).
		Complete(r)
	fmt.Println("start - SetupWithManager")

	if err != nil {
		return err
	}

	return nil
}


func (r *EsReconciler) deleteExternalResources(es *opsterv1alpha1.Es) error {
	namespace := es.Spec.General.ClusterName

	nsToDel := builders.NewNsForCR(es)

	fmt.Println("Cluster", es.Name, "has been deleted, Delete namesapce ", namespace)
	err := r.Delete(context.TODO(), nsToDel)
	if err != nil {
		 return err
	}
	fmt.Println("NS" ,namespace ,"Deleted successfully")
	return  nil
}