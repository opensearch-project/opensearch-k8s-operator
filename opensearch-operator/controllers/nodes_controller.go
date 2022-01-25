package controllers

import (
	"context"
	"fmt"
	sts "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	opsterv1 "os-operator.io/api/v1"
	"os-operator.io/pkg/helpers"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	controllerNamed           = "scaler-controller"
	configHashAnnotationNamed = "opster.os-operator.opster.io/config-hash"
)

type ScalerReconciler struct {
	client.Client
	Scheme     *runtime.Scheme
	Recorder   record.EventRecorder
	State      State
	Instance   *opsterv1.Os
	StsFromEnv sts.StatefulSet
	Group      int
}

//+kubebuilder:rbac:groups="opster.os-operator.opster.io",resources=events,verbs=create;patch
//+kubebuilder:rbac:groups=opster.os-operator.opster.io,resources=os,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=opster.os-operator.opster.io,resources=os/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=opster.os-operator.opster.io,resources=os/finalizers,verbs=update

func (r *ScalerReconciler) Reconcile(ctx context.Context, request ctrl.Request) (ctrl.Result, error) {
	var found bool
	if *r.StsFromEnv.Spec.Replicas == r.Instance.Spec.NodePools[r.Group].Replicas {
		return ctrl.Result{}, nil

	} else {
		group := fmt.Sprintf("Group-%d", r.Group)
		var componentStatus opsterv1.ComponenetsStatus
		comp := r.Instance.Status.ComponenetsStatus
		for i := 0; i < len(comp); i++ {
			if comp[i].Component == "Scaler" {
				if comp[i].Description == group {
					found = true
					if comp[i].Status == "Running" {
						{
							// --- Now check if scaling is done logic ----

							done := true
							if !done {
								r.Recorder.Event(r.Instance, "Normal", "one scale is already in progress on that group ", fmt.Sprintf("one scale is already in progress on that group"))
								return ctrl.Result{}, nil
							} else {
								// if scale logic is done - remove componentStatus
								componentStatus = opsterv1.ComponenetsStatus{
									Component:   "Scaler",
									Status:      "Running",
									Description: group,
								}
								newStatus := helpers.RemoveIt(componentStatus, comp)
								r.Instance.Status.ComponenetsStatus = newStatus
								r.Status().Update(ctx, r.Instance)
								r.Recorder.Event(r.Instance, "Normal", "done scaling", fmt.Sprintf("done scaling"))
								r.StsFromEnv.Spec.Replicas = &r.Instance.Spec.NodePools[r.Group].Replicas
								if err := r.Update(ctx, &r.StsFromEnv); err != nil {
									return ctrl.Result{}, err
								}
								return ctrl.Result{}, nil
							}
						}
					} else if comp[i].Status == "Failed" {
						r.Recorder.Event(r.Instance, "Normal", "something want worng with scaling operation", fmt.Sprintf("something went worng)"))
						return ctrl.Result{}, nil
					}
				}
			}
		}
		// if not found componentStatus and replcias not equals
		if !found {
			// starting new componentStatus
			componentStatus = opsterv1.ComponenetsStatus{
				Component:   "Scaler",
				Status:      "Running",
				Description: group,
			}
			r.Recorder.Event(r.Instance, "Normal", "add new status event about scale ", fmt.Sprintf("add new status event about scale "))
			r.Instance.Status.ComponenetsStatus = append(r.Instance.Status.ComponenetsStatus, componentStatus)
			r.Status().Update(ctx, r.Instance)

			// -----  Now start scaling logic ------

		}
	}
	return ctrl.Result{Requeue: true}, nil
}
