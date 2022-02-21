package controllers

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/client-go/tools/record"
	opsterv1 "opensearch.opster.io/api/v1"
	"opensearch.opster.io/pkg/helpers"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ScalerReconciler struct {
	client.Client
	Recorder record.EventRecorder
	logr.Logger
	Instance *opsterv1.OpenSearchCluster
}

func (r *ScalerReconciler) Reconcile(controllerContext *ControllerContext) ([]opsterv1.ComponentStatus, error) {
	var statusList []opsterv1.ComponentStatus
	for _, nodePool := range r.Instance.Spec.NodePools {
		status, err := r.reconcileNodePool(&nodePool, controllerContext)
		if status != nil {
			statusList = append(statusList, *status)
		}
		if err != nil {
			return statusList, err
		}
	}
	return statusList, nil
}

func (r *ScalerReconciler) reconcileNodePool(nodePool *opsterv1.NodePool, controllerContext *ControllerContext) (*opsterv1.ComponentStatus, error) {

	namespace := r.Instance.Spec.General.ClusterName
	sts_name := r.Instance.Spec.General.ClusterName + "-" + nodePool.Component

	nodePoolSTS := appsv1.StatefulSet{}

	if err := r.Get(context.TODO(), client.ObjectKey{Name: sts_name, Namespace: namespace}, &nodePoolSTS); err != nil {
		return nil, err
	}

	if *nodePoolSTS.Spec.Replicas == nodePool.Replicas {
		return nil, nil
	}
	var found bool
	name := fmt.Sprintf("Scaler-%s", nodePool.Component)
	for _, existingStatus := range r.Instance.Status.ComponentsStatus {
		if existingStatus.Component == name {
			found = true
			if existingStatus.Status == "Running" {
				// --- Now check if scaling is done logic ----
				done := true
				if !done {
					r.Recorder.Event(r.Instance, "Normal", "one scale is already in progress on that group ", "one scale is already in progress on that group")
					return nil, nil
				} else {
					// if scale logic is done - remove componentStatus
					componentStatus := opsterv1.ComponentStatus{
						Component:   name,
						Status:      "Running",
						Description: "",
					}
					newStatus := helpers.RemoveIt(componentStatus, r.Instance.Status.ComponentsStatus)
					r.Instance.Status.ComponentsStatus = newStatus
					if err := r.Status().Update(context.TODO(), r.Instance); err != nil {
						err = errors.New("cannot update instance status")
						return nil, err
					}

					//r.Recorder.Event(r.Instance, "Normal", "done scaling", "done scaling")
					nodePoolSTS.Spec.Replicas = &nodePool.Replicas
					if err := r.Update(context.TODO(), &nodePoolSTS); err != nil {
						err = errors.New("cannot update instance status")

						return nil, err
					}
					return nil, nil
				}
			} else if existingStatus.Status == "Failed" {
				r.Recorder.Event(r.Instance, "Normal", "something want worng with scaling operation", "something went worng)")
				return nil, nil
			}
		}
	}
	// if not found componentStatus and replicas not equal
	if !found {
		// starting new componentStatus
		componentStatus := opsterv1.ComponentStatus{
			Component:   name,
			Status:      "Running",
			Description: "",
		}
		//r.Recorder.Event(r.Instance, "Normal", "add new status event about scale ", "add new status event about scale ")

		r.Instance.Status.ComponentsStatus = append(r.Instance.Status.ComponentsStatus, componentStatus)
		if err := r.Status().Update(context.TODO(), r.Instance); err != nil {
			err = errors.New("cannot update instance status")
			return nil, err
		}
		// -----  Now start scaling logic ------

	}

	return nil, nil
}
