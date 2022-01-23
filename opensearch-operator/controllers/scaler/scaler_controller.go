package scaler

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
	controllerName           = "scaler-controller"
	configHashAnnotationName = "opster.os-operator.opster.io/config-hash"
)

type State struct {
	Compenent string `json:"compenent,omitempty"`
	Status    string `json:"status,omitempty"`
	Err       error  `json:"err,omitempty"`
}

type ScalerReconciler struct {
	client.Client
	Scheme     *runtime.Scheme
	Recorder   record.EventRecorder
	State      State
	Instnce    *opsterv1.Os
	StsFromEnv sts.StatefulSet
	Group      int
}

//+kubebuilder:rbac:groups="opster.os-operator.opster.io",resources=events,verbs=create;patch
//+kubebuilder:rbac:groups=opster.os-operator.opster.io,resources=os,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=opster.os-operator.opster.io,resources=os/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=opster.os-operator.opster.io,resources=os/finalizers,verbs=update

func (r *ScalerReconciler) InternalReconcile(ctx context.Context) (ScalerReconciler, ctrl.Result, error) {
	var found bool
	if *r.StsFromEnv.Spec.Replicas == r.Instnce.Spec.NodePools[r.Group].Replicas {
		return ScalerReconciler{}, ctrl.Result{}, nil

	} else {
		group := fmt.Sprintf("Group-%d", r.Group)
		var componentStatus opsterv1.ComponenetsStatus
		comp := r.Instnce.Status.ComponenetsStatus
		for i := 0; i < len(comp); i++ {
			if comp[i].Component == "Scaler" {
				if comp[i].Description == group {
					found = true
					if comp[i].Status == "Running" {
						{
							// --- Now check if scaling is done logic ----

							done := true
							if !done {
								scaler_reconcile := setReconcilerStatus(&ScalerReconciler{}, "Running")
								r.Recorder.Event(r.Instnce, "Normal", "one scale is already in progress on that group ", fmt.Sprintf("one scale is already in progress on that group"))
								return scaler_reconcile, ctrl.Result{}, nil
							} else {
								// if scale logic is done - remove componentStatus
								componentStatus = opsterv1.ComponenetsStatus{
									Component:   "Scaler",
									Status:      "Running",
									Description: group,
								}
								newStatus := helpers.RemoveIt(componentStatus, comp)
								r.Instnce.Status.ComponenetsStatus = newStatus
								r.Status().Update(ctx, r.Instnce)
								r.Recorder.Event(r.Instnce, "Normal", "done scaling", fmt.Sprintf("done scaling"))
								r.StsFromEnv.Spec.Replicas = &r.Instnce.Spec.NodePools[r.Group].Replicas
								if err := r.Update(ctx, &r.StsFromEnv); err != nil {
									scaler_reconcile := setReconcilerStatus(&ScalerReconciler{}, "Failed")
									return scaler_reconcile, ctrl.Result{}, err
								}
								scaler_reconcile := setReconcilerStatus(&ScalerReconciler{}, "Done")
								return scaler_reconcile, ctrl.Result{}, nil
							}
						}
					} else if comp[i].Status == "Failed" {
						r.Recorder.Event(r.Instnce, "Normal", "something want worng with scaling operation", fmt.Sprintf("something went worng)"))
						scaler_reconcile := setReconcilerStatus(&ScalerReconciler{}, "Failed")
						return scaler_reconcile, ctrl.Result{}, nil
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
			r.Recorder.Event(r.Instnce, "Normal", "add new status event about scale ", fmt.Sprintf("add new status event about scale "))
			r.Instnce.Status.ComponenetsStatus = append(r.Instnce.Status.ComponenetsStatus, componentStatus)
			r.Status().Update(ctx, r.Instnce)

			// -----  Now start scaling logic ------

		}
	}
	scaler_reconcile := setReconcilerStatus(&ScalerReconciler{}, "Done")
	return scaler_reconcile, ctrl.Result{Requeue: true}, nil
}
func setReconcilerStatus(cluster *ScalerReconciler, stat string) ScalerReconciler {
	new := ScalerReconciler{
		Client:   cluster.Client,
		Scheme:   cluster.Scheme,
		Recorder: cluster.Recorder,
		State: State{
			Compenent: controllerName,
			Status:    "Running",
		},
		Instnce: cluster.Instnce,
	}
	return new
}
