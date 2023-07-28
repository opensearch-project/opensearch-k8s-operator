package reconcilers

import (
	"context"
	"time"

	"github.com/cisco-open/operator-tools/pkg/reconciler"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/pointer"
	opsterv1 "opensearch.opster.io/api/v1"
	"opensearch.opster.io/opensearch-gateway/services"
	"opensearch.opster.io/pkg/builders"
	"opensearch.opster.io/pkg/helpers"
	"opensearch.opster.io/pkg/reconcilers/util"
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
	client.Client
	reconciler.ResourceReconciler
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
		Client: client,
		ResourceReconciler: reconciler.NewReconcilerWith(client,
			append(opts, reconciler.WithLog(log.FromContext(ctx).WithValues("reconciler", "restart")))...),
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
	// Check that all data nodes are ready before doing work
	// Also check if there are pending updates
	for _, nodePool := range r.instance.Spec.NodePools {
		if helpers.HasDataRole(&nodePool) {
			sts := &appsv1.StatefulSet{}
			if err := r.Get(r.ctx, types.NamespacedName{
				Name:      builders.StsName(r.instance, &nodePool),
				Namespace: r.instance.Namespace,
			}, sts); err != nil {
				return ctrl.Result{}, err
			}
			if sts.Status.ReadyReplicas != pointer.Int32Deref(sts.Spec.Replicas, 1) {
				return ctrl.Result{
					Requeue:      true,
					RequeueAfter: 10 * time.Second,
				}, nil
			}

			if sts.Status.UpdateRevision != "" &&
				sts.Status.UpdatedReplicas != pointer.Int32Deref(sts.Spec.Replicas, 1) {
				pendingUpdate = true
			}
		}
	}

	if !pendingUpdate {
		// Check if we had a restart running that is finished so that we can reactivate shard allocation
		if status != nil && status.Status == statusInProgress {
			osClient, err := util.CreateClientForCluster(r.ctx, r.Client, r.instance, nil)
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

	r.osClient, err = util.CreateClientForCluster(r.ctx, r.Client, r.instance, nil)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Restart statefulset pod.  Order is not important so we just pick the first we find
	for _, nodePool := range r.instance.Spec.NodePools {
		sts := &appsv1.StatefulSet{}
		if err := r.Get(r.ctx, types.NamespacedName{
			Name:      builders.StsName(r.instance, &nodePool),
			Namespace: r.instance.Namespace,
		}, sts); err != nil {
			return ctrl.Result{}, err
		}
		// Only restart pods if not all pods are updated and the sts is healthy with no pods terminating
		if sts.Status.ReadyReplicas == pointer.Int32Deref(sts.Spec.Replicas, 1) &&
			sts.Status.UpdateRevision != "" &&
			sts.Status.UpdatedReplicas != pointer.Int32Deref(sts.Spec.Replicas, 1) {
			if numReadyPods, err := helpers.CountRunningPodsForNodePool(r.ctx, r.Client, r.instance, &nodePool); err == nil {
				if numReadyPods == int(pointer.Int32Deref(sts.Spec.Replicas, 1)) {
					return r.restartStatefulSetPod(sts)
				}
			}
		}
	}

	return ctrl.Result{}, nil
}

func (r *RollingRestartReconciler) restartStatefulSetPod(sts *appsv1.StatefulSet) (ctrl.Result, error) {
	lg := log.FromContext(r.ctx).WithValues("reconciler", "restart")
	dataCount := builders.DataNodesCount(r.ctx, r.Client, r.instance)
	if dataCount == 2 && r.instance.Spec.General.DrainDataNodes {
		lg.Info("only 2 data nodes and drain is set, some shards may not drain")
	}

	ready, _, err := services.CheckClusterStatusForRestart(r.osClient, r.instance.Spec.General.DrainDataNodes)
	if err != nil {
		return ctrl.Result{}, err
	}
	if !ready {
		return ctrl.Result{
			Requeue:      true,
			RequeueAfter: 10 * time.Second,
		}, nil
	}

	workingPod, err := helpers.WorkingPodForRollingRestart(r.ctx, r.Client, sts)
	if err != nil {
		return ctrl.Result{}, err
	}

	ready, err = services.PreparePodForDelete(r.osClient, workingPod, r.instance.Spec.General.DrainDataNodes, dataCount)
	if err != nil {
		return ctrl.Result{}, err
	}
	if !ready {
		return ctrl.Result{
			Requeue:      true,
			RequeueAfter: 10 * time.Second,
		}, nil
	}

	err = r.Delete(r.ctx, &corev1.Pod{
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
	return UpdateComponentStatus(r.ctx, r.Client, r.instance, &opsterv1.ComponentStatus{
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
