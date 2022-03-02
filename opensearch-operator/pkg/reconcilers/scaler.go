package reconcilers

import (
	"context"
	"errors"
	"fmt"

	"github.com/banzaicloud/operator-tools/pkg/reconciler"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	"k8s.io/utils/pointer"
	opsterv1 "opensearch.opster.io/api/v1"
	"opensearch.opster.io/pkg/helpers"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type ScalerReconciler struct {
	client.Client
	reconciler.ResourceReconciler
	ctx               context.Context
	recorder          record.EventRecorder
	reconcilerContext *ReconcilerContext
	instance          *opsterv1.OpenSearchCluster
}

func NewScalerReconciler(
	client client.Client,
	ctx context.Context,
	recorder record.EventRecorder,
	reconcilerContext *ReconcilerContext,
	instance *opsterv1.OpenSearchCluster,
	opts ...reconciler.ResourceReconcilerOption,
) *ScalerReconciler {
	return &ScalerReconciler{
		Client: client,
		ResourceReconciler: reconciler.NewReconcilerWith(client,
			append(opts, reconciler.WithLog(log.FromContext(ctx).WithValues("reconciler", "scaler")))...),
		ctx:               ctx,
		recorder:          recorder,
		reconcilerContext: reconcilerContext,
		instance:          instance,
	}
}

func (r *ScalerReconciler) Reconcile() (ctrl.Result, error) {
	for _, nodePool := range r.instance.Spec.NodePools {
		err := r.reconcileNodePool(&nodePool)
		if err != nil {
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil
}

func (r *ScalerReconciler) reconcileNodePool(nodePool *opsterv1.NodePool) error {
	nodePoolSTS := &appsv1.StatefulSet{}

	if err := r.Get(r.ctx, types.NamespacedName{
		Name:      fmt.Sprintf("%s-%s", r.instance.Spec.General.ClusterName, nodePool.Component),
		Namespace: r.instance.Spec.General.ClusterName,
	}, nodePoolSTS); err != nil {
		return err
	}

	if pointer.Int32Deref(nodePoolSTS.Spec.Replicas, 1) == nodePool.Replicas {
		return nil
	}

	var found bool
	name := fmt.Sprintf("Scaler-%s", nodePool.Component)
	for _, existingStatus := range r.instance.Status.ComponentsStatus {
		if existingStatus.Component == name {
			found = true
			if existingStatus.Status == "Running" {
				// --- Now check if scaling is done logic ----
				done := true
				if !done {
					r.recorder.Event(r.instance, "Normal", "one scale is already in progress on that group ", "one scale is already in progress on that group")
					return nil
				} else {
					// if scale logic is done - remove componentStatus
					componentStatus := opsterv1.ComponentStatus{
						Component:   name,
						Status:      "Running",
						Description: "",
					}
					newStatus := helpers.RemoveIt(componentStatus, r.instance.Status.ComponentsStatus)
					r.instance.Status.ComponentsStatus = newStatus

					err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
						if err := r.Get(r.ctx, client.ObjectKeyFromObject(r.instance), r.instance); err != nil {
							return err
						}
						r.instance.Status.ComponentsStatus = newStatus
						return r.Status().Update(r.ctx, r.instance)
					})
					if err != nil {
						return err
					}

					//r.Recorder.Event(r.Instance, "Normal", "done scaling", "done scaling")
					nodePoolSTS.Spec.Replicas = &nodePool.Replicas
					if err := r.Update(r.ctx, nodePoolSTS); err != nil {
						return errors.New("cannot update instance status")
					}
					return nil
				}
			} else if existingStatus.Status == "Failed" {
				r.recorder.Event(r.instance, "Normal", "something want worng with scaling operation", "something went worng)")
				return nil
			}
		}
	}

	// if not found componentStatus and replicas not equal
	if !found {
		// starting new componentStatus
		componentStatus := &opsterv1.ComponentStatus{
			Component:   name,
			Status:      "Running",
			Description: "",
		}
		//r.Recorder.Event(r.Instance, "Normal", "add new status event about scale ", "add new status event about scale ")

		return UpdateOpensearchStatus(r.ctx, r.Client, r.instance, componentStatus)
		// -----  Now start scaling logic ------

	}

	return nil
}
