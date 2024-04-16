package reconcilers

import (
	"context"
	"fmt"
	"time"

	opsterv1 "github.com/Opster/opensearch-k8s-operator/opensearch-operator/api/v1"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/opensearch-gateway/services"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/pkg/builders"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/pkg/helpers"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/pkg/reconcilers/k8s"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/pkg/reconcilers/util"
	"github.com/cisco-open/operator-tools/pkg/reconciler"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	statusInProgress = "InProgress"
	statusFinished   = "Finished"
	componentName    = "Restarter"
)

type RollingRestartReconciler struct {
	client            k8s.K8sClient
	ctx               context.Context
	osClient          *services.OsClusterClient
	recorder          record.EventRecorder
	reconcilerContext *ReconcilerContext
	instance          *opsterv1.OpenSearchCluster
}

func NewRollingRestartReconciler(
	client client.Client,
	ctx context.Context,
	recorder record.EventRecorder,
	reconcilerContext *ReconcilerContext,
	instance *opsterv1.OpenSearchCluster,
	opts ...reconciler.ResourceReconcilerOption,
) *RollingRestartReconciler {
	return &RollingRestartReconciler{
		client:            k8s.NewK8sClient(client, ctx, append(opts, reconciler.WithLog(log.FromContext(ctx).WithValues("reconciler", "restart")))...),
		ctx:               ctx,
		recorder:          recorder,
		reconcilerContext: reconcilerContext,
		instance:          instance,
	}
}

func (r *RollingRestartReconciler) Reconcile() (ctrl.Result, error) {
	lg := log.FromContext(r.ctx).WithValues("reconciler", "restart")

	// We should never get to this while an upgrade is in progress
	// but put a defensive check in
	if r.instance.Status.Version != "" && r.instance.Status.Version != r.instance.Spec.General.Version {
		lg.V(1).Info("Upgrade in progress, skipping rolling restart")
		return ctrl.Result{}, nil
	}

	status := r.findStatus()
	var pendingUpdate bool

	// Check that all nodes are ready before doing work
	// Also check if there are pending updates for all nodes.
	for _, nodePool := range r.instance.Spec.NodePools {
		sts, err := r.client.GetStatefulSet(builders.StsName(r.instance, &nodePool), r.instance.Namespace)
		if err != nil {
			return ctrl.Result{}, err
		}
		if sts.Status.UpdateRevision != "" &&
			sts.Status.UpdatedReplicas != pointer.Int32Deref(sts.Spec.Replicas, 1) {
			pendingUpdate = true
			break
		} else if sts.Status.UpdateRevision != "" &&
			sts.Status.CurrentRevision != sts.Status.UpdateRevision {
			// If all pods in sts are updated to spec.replicas but current version is not updated.
			err := r.client.UdateObjectStatus(&sts, func(object client.Object) {
				instance := object.(*appsv1.StatefulSet)
				instance.Status.CurrentRevision = sts.Status.UpdateRevision
			})
			if err != nil {
				lg.Error(err, "failed to update status")
				return ctrl.Result{}, err
			}

		}
		if sts.Status.ReadyReplicas != pointer.Int32Deref(sts.Spec.Replicas, 1) {
			return ctrl.Result{
				Requeue:      true,
				RequeueAfter: 10 * time.Second,
			}, nil
		}
	}

	if !pendingUpdate {
		// Check if we had a restart running that is finished so that we can reactivate shard allocation
		if status != nil && status.Status == statusInProgress {
			osClient, err := util.CreateClientForCluster(r.client, r.ctx, r.instance, nil)
			if err != nil {
				return ctrl.Result{Requeue: true}, err
			}
			if err = services.ReactivateShardAllocation(osClient); err != nil {
				lg.V(1).Info("Restart complete. Reactivating shard allocation")
				return ctrl.Result{Requeue: true}, err
			}
			if err = r.updateStatus(statusFinished); err != nil {
				return ctrl.Result{Requeue: true}, err
			}
		}
		lg.V(1).Info("No pods pending restart")
		return ctrl.Result{}, nil
	}

	// Skip a rolling restart if the cluster hasn't finished initializing
	if !r.instance.Status.Initialized {
		return ctrl.Result{
			Requeue:      true,
			RequeueAfter: 10 * time.Second,
		}, nil
	}

	if err := r.updateStatus(statusInProgress); err != nil {
		return ctrl.Result{Requeue: true}, err
	}
	r.recorder.AnnotatedEventf(r.instance, map[string]string{"cluster-name": r.instance.GetName()}, "Normal", "RollingRestart", "Starting to rolling restart")

	// If there is work to do create an Opensearch Client
	var err error

	r.osClient, err = util.CreateClientForCluster(r.client, r.ctx, r.instance, nil)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Restart StatefulSet pod.  Order is not important So we just pick the first we find

	for _, nodePool := range r.instance.Spec.NodePools {
		sts, err := r.client.GetStatefulSet(builders.StsName(r.instance, &nodePool), r.instance.Namespace)
		if err != nil {
			return ctrl.Result{}, err
		}
		if sts.Status.UpdateRevision != "" &&
			sts.Status.UpdatedReplicas != pointer.Int32Deref(sts.Spec.Replicas, 1) {
			// Only restart pods if not all pods are updated and the sts is healthy with no pods terminating
			if sts.Status.ReadyReplicas == pointer.Int32Deref(sts.Spec.Replicas, 1) {
				if numReadyPods, err := helpers.CountRunningPodsForNodePool(r.client, r.instance, &nodePool); err == nil && numReadyPods == int(pointer.Int32Deref(sts.Spec.Replicas, 1)) {
					lg.Info(fmt.Sprintf("Starting rolling restart of the StatefulSet %s", sts.Name))
					return r.restartStatefulSetPod(&sts)
				}
			} else { // Check if there is any crashed pod. Delete it if there is any update in sts.
				err = helpers.DeleteStuckPodWithOlderRevision(r.client, &sts)
				if err != nil {
					return ctrl.Result{}, err
				}
			}
		}
	}

	return ctrl.Result{}, nil
}

func (r *RollingRestartReconciler) restartStatefulSetPod(sts *appsv1.StatefulSet) (ctrl.Result, error) {
	lg := log.FromContext(r.ctx).WithValues("reconciler", "restart")
	dataCount := util.DataNodesCount(r.client, r.instance)
	if dataCount == 2 && r.instance.Spec.General.DrainDataNodes {
		lg.Info("Only 2 data nodes and drain is set, some shards may not drain")
	}

	ready, message, err := services.CheckClusterStatusForRestart(r.osClient, r.instance.Spec.General.DrainDataNodes)
	if err != nil {
		return ctrl.Result{}, err
	}
	if !ready {
		lg.Info(fmt.Sprintf("Couldn't proceed with rolling restart for Statefulset %s because %s", sts.Name, message))
		return ctrl.Result{
			Requeue:      true,
			RequeueAfter: 10 * time.Second,
		}, nil
	}

	workingPod, err := helpers.WorkingPodForRollingRestart(r.client, sts)
	if err != nil {
		return ctrl.Result{}, err
	}

	lg.Info(fmt.Sprintf("Preparing to restart pod %s", workingPod))
	ready, err = services.PreparePodForDelete(r.osClient, lg, workingPod, r.instance.Spec.General.DrainDataNodes, dataCount)
	if err != nil {
		return ctrl.Result{}, err
	}
	if !ready {
		return ctrl.Result{
			Requeue:      true,
			RequeueAfter: 10 * time.Second,
		}, nil
	}

	err = r.client.DeletePod(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      workingPod,
			Namespace: sts.Namespace,
		},
	})
	if err != nil {
		return ctrl.Result{}, err
	}

	// If we are draining nodes remove the exclusion after the pod is deleted
	if r.instance.Spec.General.DrainDataNodes {
		_, err = services.RemoveExcludeNodeHost(r.osClient, workingPod)
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *RollingRestartReconciler) updateStatus(status string) error {
	return UpdateComponentStatus(r.client, r.instance, &opsterv1.ComponentStatus{
		Component:   componentName,
		Status:      status,
		Description: "",
	})
}

func (r *RollingRestartReconciler) findStatus() *opsterv1.ComponentStatus {
	for _, component := range r.instance.Status.ComponentsStatus {
		if component.Component == componentName {
			return &component
		}
	}
	return nil
}
