package reconcilers

import (
	"context"
	"fmt"
	"time"

	"github.com/banzaicloud/operator-tools/pkg/reconciler"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/pointer"
	opsterv1 "opensearch.opster.io/api/v1"
	"opensearch.opster.io/opensearch-gateway/services"
	"opensearch.opster.io/pkg/builders"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type ClusterReconciler struct {
	client.Client
	reconciler.ResourceReconciler
	ctx               context.Context
	recorder          record.EventRecorder
	reconcilerContext *ReconcilerContext
	instance          *opsterv1.OpenSearchCluster
}

func NewClusterReconciler(
	client client.Client,
	ctx context.Context,
	recorder record.EventRecorder,
	reconcilerContext *ReconcilerContext,
	instance *opsterv1.OpenSearchCluster,
	opts ...reconciler.ResourceReconcilerOption,
) *ClusterReconciler {
	return &ClusterReconciler{
		Client: client,
		ResourceReconciler: reconciler.NewReconcilerWith(client,
			append(opts, reconciler.WithLog(log.FromContext(ctx).WithValues("reconciler", "cluster")))...),
		ctx:               ctx,
		recorder:          recorder,
		reconcilerContext: reconcilerContext,
		instance:          instance,
	}
}

func (r *ClusterReconciler) Reconcile() (ctrl.Result, error) {
	//lg := log.FromContext(r.ctx)
	result := reconciler.CombinedResult{}

	clusterService := builders.NewServiceForCR(r.instance)
	result.CombineErr(ctrl.SetControllerReference(r.instance, clusterService, r.Client.Scheme()))
	result.Combine(r.ReconcileResource(clusterService, reconciler.StatePresent))

	for _, nodePool := range r.instance.Spec.NodePools {
		headlessService := builders.NewHeadlessServiceForNodePool(r.instance, &nodePool)
		result.CombineErr(ctrl.SetControllerReference(r.instance, headlessService, r.Client.Scheme()))
		result.Combine(r.ReconcileResource(headlessService, reconciler.StatePresent))

		result.Combine(r.reconcileNodeStatefulSet(nodePool))
	}

	// Clean up statefulsets that are no longer in the spec
	r.cleanupStatefulSets(&result)

	return result.Result, result.Err
}

func (r *ClusterReconciler) reconcileNodeStatefulSet(nodePool opsterv1.NodePool) (*ctrl.Result, error) {
	sts := builders.NewSTSForNodePool(r.instance, nodePool, r.reconcilerContext.Volumes, r.reconcilerContext.VolumeMounts)
	if err := ctrl.SetControllerReference(r.instance, sts, r.Client.Scheme()); err != nil {
		return &ctrl.Result{}, err
	}

	// First ensure that the statefulset exists
	result, err := r.ReconcileResource(sts, reconciler.StateCreated)
	if err != nil || result != nil {
		return result, err
	}

	// Next get the existing statefulset
	existing := &appsv1.StatefulSet{}
	err = r.Client.Get(r.ctx, client.ObjectKeyFromObject(sts), existing)
	if err != nil {
		return result, err
	}

	// Now set the desired replicas to be the existing replicas
	// This will allow the scaler reconciler to function correctly
	sts.Spec.Replicas = existing.Spec.Replicas

	// Finally we enforce the desired state
	return r.ReconcileResource(sts, reconciler.StatePresent)
}

func (r *ClusterReconciler) DeleteResources() (ctrl.Result, error) {
	result := reconciler.CombinedResult{}
	return result.Result, result.Err
}

func (r *ClusterReconciler) cleanupStatefulSets(result *reconciler.CombinedResult) {
	stsList := &appsv1.StatefulSetList{}
	if err := r.Client.List(
		r.ctx,
		stsList,
		client.InNamespace(r.instance.Name),
		client.MatchingLabels{builders.ClusterLabel: r.instance.Name},
	); err != nil {
		result.Combine(&ctrl.Result{}, err)
		return
	}

	for _, sts := range stsList.Items {
		if !builders.STSInNodePools(sts, r.instance.Spec.NodePools) {
			result.Combine(r.removeStatefulSet(sts))
		}
	}

}

func (r *ClusterReconciler) removeStatefulSet(sts appsv1.StatefulSet) (*ctrl.Result, error) {
	if !r.instance.Spec.ConfMgmt.SmartScaler {
		return r.ReconcileResource(&sts, reconciler.StateAbsent)
	}

	// Gracefully remove nodes
	lg := log.FromContext(r.ctx)
	username, password := builders.UsernameAndPassword(r.instance)

	clusterClient, err := services.NewOsClusterClient(fmt.Sprintf("https://%s.%s", r.instance.Spec.General.ServiceName, r.instance.Name), username, password)
	if err != nil {
		lg.Error(err, "failed to create os client")
		return nil, err
	}

	workingOrdinal := pointer.Int32Deref(sts.Spec.Replicas, 1) - 1
	lastReplicaNodeName := builders.ReplicaHostName(sts, workingOrdinal)
	_, err = services.AppendExcludeNodeHost(clusterClient, lastReplicaNodeName)
	if err != nil {
		lg.Error(err, fmt.Sprintf("failed to exclude node %s", lastReplicaNodeName))
		return nil, err
	}

	nodeNotEmpty, err := services.HasShardsOnNode(clusterClient, lastReplicaNodeName)
	if err != nil {
		lg.Error(err, "failed to check shards on node")
		return nil, err
	}

	if nodeNotEmpty {
		return &ctrl.Result{
			Requeue:      true,
			RequeueAfter: 15 * time.Second,
		}, nil
	}

	if workingOrdinal == 0 {
		result, err := r.ReconcileResource(&sts, reconciler.StateAbsent)
		if err != nil {
			return result, err
		}
		_, err = services.RemoveExcludeNodeHost(clusterClient, lastReplicaNodeName)
		if err != nil {
			lg.Error(err, fmt.Sprintf("failed to remove node exclusion for %s", lastReplicaNodeName))
		}
		return result, err
	}

	sts.Spec.Replicas = &workingOrdinal
	result, err := r.ReconcileResource(&sts, reconciler.StatePresent)

	_, err = services.RemoveExcludeNodeHost(clusterClient, lastReplicaNodeName)
	if err != nil {
		lg.Error(err, fmt.Sprintf("failed to remove node exclusion for %s", lastReplicaNodeName))
	}
	return result, err
}
