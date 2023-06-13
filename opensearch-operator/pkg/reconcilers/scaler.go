package reconcilers

import (
	"context"
	"fmt"
	"time"

	"k8s.io/utils/pointer"
	"opensearch.opster.io/opensearch-gateway/services"
	"opensearch.opster.io/pkg/builders"

	"github.com/cisco-open/operator-tools/pkg/reconciler"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/client-go/tools/record"
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
	requeue := false
	results := &reconciler.CombinedResult{}
	var err error
	for _, nodePool := range r.instance.Spec.NodePools {
		requeue, err = r.reconcileNodePool(&nodePool)
		if err != nil {
			results.Combine(&ctrl.Result{Requeue: requeue}, err)
		}
	}
	results.Combine(&ctrl.Result{Requeue: requeue}, nil)

	// Clean up old node pools
	r.cleanupStatefulSets(results)

	return results.Result, results.Err
}

func (r *ScalerReconciler) reconcileNodePool(nodePool *opsterv1.NodePool) (bool, error) {
	lg := log.FromContext(r.ctx)
	namespace := r.instance.Namespace
	sts_name := builders.StsName(r.instance, nodePool)
	currentSts := appsv1.StatefulSet{}
	annotations := map[string]string{"cluster-name": r.instance.GetName()}
	if err := r.Get(r.ctx, client.ObjectKey{Name: sts_name, Namespace: namespace}, &currentSts); err != nil {
		return false, err
	}

	componentStatus := opsterv1.ComponentStatus{
		Component:   "Scaler",
		Status:      "Running",
		Description: nodePool.Component,
	}
	comp := r.instance.Status.ComponentsStatus
	currentStatus, found := helpers.FindFirstPartial(comp, componentStatus, helpers.GetByDescriptionAndGroup)

	var desireReplicaDiff = *currentSts.Spec.Replicas - nodePool.Replicas
	if desireReplicaDiff == 0 {
		// If a scaling operation was started before for this nodePool
		if found {
			if currentSts.Status.ReadyReplicas != nodePool.Replicas {
				// Change the status to waiting while the pods are coming up or getting deleted
				componentStatus.Status = "Waiting"
				r.instance.Status.ComponentsStatus = helpers.Replace(currentStatus, componentStatus, r.instance.Status.ComponentsStatus)
			} else {
				// Scaling operation is completed, remove the status
				r.instance.Status.ComponentsStatus = helpers.RemoveIt(currentStatus, r.instance.Status.ComponentsStatus)
			}
			err := r.Status().Update(r.ctx, r.instance)
			if err != nil {
				lg.Error(err, "failed to update status")
				return false, err
			}
		}
		return false, nil
	}

	// Check for 'Running' status as we set it to indicate the scaling operation has begun
	// Also the status is set to 'Running' if it fails to exclude node for some reason
	if !found || currentStatus.Status == "Running" {
		// Change the status to running, to indicate that a scaling operation for this nodePool has started
		r.instance.Status.ComponentsStatus = helpers.Replace(currentStatus, componentStatus, r.instance.Status.ComponentsStatus)
		err := r.Status().Update(r.ctx, r.instance)
		if err != nil {
			r.recorder.AnnotatedEventf(r.instance, annotations, "Warning", "Scaler", "Failed to update status")
			lg.Error(err, "failed to update status")
			return true, err
		}

		if desireReplicaDiff > 0 {
			r.recorder.AnnotatedEventf(r.instance, annotations, "Normal", "Scaler", "Starting to scaling")
			if !r.instance.Spec.ConfMgmt.SmartScaler {
				requeue, err := r.decreaseOneNode(currentStatus, currentSts, nodePool.Component, r.instance.Spec.ConfMgmt.SmartScaler)
				r.recorder.AnnotatedEventf(r.instance, annotations, "Normal", "Scaler", "Notice - your SmartScaler is not enable")
				r.recorder.AnnotatedEventf(r.instance, annotations, "Normal", "Scaler", "Starting to decrease node")
				return requeue, err
			}
			err := r.excludeNode(currentStatus, currentSts, nodePool.Component)
			return true, err

		}
		if desireReplicaDiff < 0 {
			requeue, err := r.increaseOneNode(currentSts, nodePool.Component)
			return requeue, err
		}
	}
	if currentStatus.Status == "Excluded" {
		r.recorder.AnnotatedEventf(r.instance, annotations, "Normal", "Scaler", "Start to Exclude %s/%s", r.instance.Namespace, r.instance.Name)
		err := r.drainNode(currentStatus, currentSts, nodePool.Component)
		return true, err
	}
	if currentStatus.Status == "Drained" {
		r.recorder.AnnotatedEventf(r.instance, annotations, "Normal", "Scaler", "Start to Drain %s/%s", r.instance.Namespace, r.instance.Name)

		requeue, err := r.decreaseOneNode(currentStatus, currentSts, nodePool.Component, r.instance.Spec.ConfMgmt.SmartScaler)
		return requeue, err
	}
	return false, nil
}

func (r *ScalerReconciler) increaseOneNode(currentSts appsv1.StatefulSet, nodePoolGroupName string) (bool, error) {
	lg := log.FromContext(r.ctx)
	*currentSts.Spec.Replicas++
	annotations := map[string]string{"cluster-name": r.instance.GetName()}
	lastReplicaNodeName := builders.ReplicaHostName(currentSts, *currentSts.Spec.Replicas)
	r.recorder.AnnotatedEventf(r.instance, annotations, "Normal", "Scaler", "Start increaseing node %s on %s ", lastReplicaNodeName, nodePoolGroupName)
	_, err := r.ReconcileResource(&currentSts, reconciler.StatePresent)
	if err != nil {
		r.recorder.AnnotatedEventf(r.instance, annotations, "Normal", "Scaler", "Failed to add node %s/%s", r.instance.Namespace, r.instance.Name)
		return true, err
	}
	lg.Info(fmt.Sprintf("Group-%s . added node %s", nodePoolGroupName, lastReplicaNodeName))
	r.recorder.AnnotatedEventf(r.instance, annotations, "Normal", "Scaler", "Added new node %s", lastReplicaNodeName)
	return false, nil
}

func (r *ScalerReconciler) decreaseOneNode(currentStatus opsterv1.ComponentStatus, currentSts appsv1.StatefulSet, nodePoolGroupName string, smartDecrease bool) (bool, error) {
	lg := log.FromContext(r.ctx)
	*currentSts.Spec.Replicas--
	annotations := map[string]string{"cluster-name": r.instance.GetName()}
	lastReplicaNodeName := builders.ReplicaHostName(currentSts, *currentSts.Spec.Replicas)
	r.recorder.AnnotatedEventf(r.instance, annotations, "Normal", "Scaler", "Start to decreaseing node %s on %s ", lastReplicaNodeName, nodePoolGroupName)
	_, err := r.ReconcileResource(&currentSts, reconciler.StatePresent)
	if err != nil {
		r.recorder.AnnotatedEventf(r.instance, annotations, "Normal", "Scaler", "Failed to remove node - Group-%s . Failed to remove node %s", nodePoolGroupName, lastReplicaNodeName)
		lg.Error(err, fmt.Sprintf("failed to remove node %s", lastReplicaNodeName))
		return true, err
	}
	lg.Info(fmt.Sprintf("Group-%s . removed node %s", nodePoolGroupName, lastReplicaNodeName))
	r.instance.Status.ComponentsStatus = helpers.RemoveIt(currentStatus, r.instance.Status.ComponentsStatus)
	err = r.Status().Update(r.ctx, r.instance)
	if err != nil {
		lg.Error(err, "failed to update status")
		return false, err
	}

	if !smartDecrease {
		return false, err
	}
	username, password, err := helpers.UsernameAndPassword(r.ctx, r.Client, r.instance)
	if err != nil {
		return true, err
	}
	clusterClient, err := services.NewOsClusterClient(builders.URLForCluster(r.instance), username, password)
	if err != nil {
		lg.Error(err, "failed to create os client")
		r.recorder.AnnotatedEventf(r.instance, annotations, "WARN", "failed to remove node exclude", "Group-%s . failed to remove node exclude %s", nodePoolGroupName, lastReplicaNodeName)
		return true, err
	}

	success, err := services.RemoveExcludeNodeHost(clusterClient, lastReplicaNodeName)
	if !success || err != nil {
		lg.Error(err, fmt.Sprintf("failed to remove exclude node %s", lastReplicaNodeName))
		r.recorder.AnnotatedEventf(r.instance, annotations, "Warning", "Scaler", "Failed to remove node exclude - Group-%s , node  %s", nodePoolGroupName, lastReplicaNodeName)
	}

	return false, err
}

func (r *ScalerReconciler) excludeNode(currentStatus opsterv1.ComponentStatus, currentSts appsv1.StatefulSet, nodePoolGroupName string) error {
	lg := log.FromContext(r.ctx)
	username, password, err := helpers.UsernameAndPassword(r.ctx, r.Client, r.instance)
	annotations := map[string]string{"cluster-name": r.instance.GetName()}
	if err != nil {
		return err
	}

	clusterClient, err := services.NewOsClusterClient(builders.URLForCluster(r.instance), username, password)
	if err != nil {
		lg.Error(err, "failed to create os client")
		r.recorder.AnnotatedEventf(r.instance, annotations, "Warning", "Scaler", "Failed to create os client for scaling")
		return err
	}
	// -----  Now start remove node ------
	lastReplicaNodeName := builders.ReplicaHostName(currentSts, *currentSts.Spec.Replicas-1)

	excluded, err := services.AppendExcludeNodeHost(clusterClient, lastReplicaNodeName)
	if err != nil {
		lg.Error(err, fmt.Sprintf("failed to exclude node %s", lastReplicaNodeName))
		return err
	}
	if excluded {
		componentStatus := opsterv1.ComponentStatus{
			Component:   "Scaler",
			Status:      "Excluded",
			Description: nodePoolGroupName,
		}
		r.recorder.AnnotatedEventf(r.instance, annotations, "Normal", "Scaler", "Finished to Exclude %s/%s", r.instance.Namespace, r.instance.Name)
		lg.Info(fmt.Sprintf("Group-%s .excluded node %s", nodePoolGroupName, lastReplicaNodeName))
		r.instance.Status.ComponentsStatus = helpers.Replace(currentStatus, componentStatus, r.instance.Status.ComponentsStatus)
		err = r.Status().Update(r.ctx, r.instance)
		if err != nil {
			lg.Error(err, "failed to update status")
			r.recorder.AnnotatedEventf(r.instance, annotations, "Warning", "Scaler", "Failed to update operator status")
			return err
		}

		return err
	}

	componentStatus := opsterv1.ComponentStatus{
		Component:   "Scaler",
		Status:      "Running",
		Description: nodePoolGroupName,
	}
	r.recorder.AnnotatedEventf(r.instance, annotations, "Normal", "Scaler", "Start sacle %s/%s from %d to %d", r.instance.Namespace, r.instance.Name, *currentSts.Spec.Replicas, *currentSts.Spec.Replicas-1)
	lg.Info(fmt.Sprintf("Group-%s . Failed to exclude node %s", nodePoolGroupName, lastReplicaNodeName))
	r.instance.Status.ComponentsStatus = helpers.Replace(currentStatus, componentStatus, r.instance.Status.ComponentsStatus)
	err = r.Status().Update(r.ctx, r.instance)
	if err != nil {
		r.recorder.AnnotatedEventf(r.instance, annotations, "Warning", "Scaler", "Group-%s . failed to remove node exclude %s", nodePoolGroupName, lastReplicaNodeName)
		lg.Error(err, "failed to update status")
		return err
	}

	return err
}

func (r *ScalerReconciler) drainNode(currentStatus opsterv1.ComponentStatus, currentSts appsv1.StatefulSet, nodePoolGroupName string) error {
	lg := log.FromContext(r.ctx)
	annotations := map[string]string{"cluster-name": r.instance.GetName()}
	lastReplicaNodeName := builders.ReplicaHostName(currentSts, *currentSts.Spec.Replicas-1)
	username, password, err := helpers.UsernameAndPassword(r.ctx, r.Client, r.instance)
	if err != nil {
		return err
	}

	clusterClient, err := services.NewOsClusterClient(builders.URLForCluster(r.instance), username, password)
	if err != nil {
		return err
	}
	nodeNotEmpty, err := services.HasShardsOnNode(clusterClient, lastReplicaNodeName)
	if nodeNotEmpty {
		lg.Info(fmt.Sprintf("Group-%s . draining node %s", nodePoolGroupName, lastReplicaNodeName))
		return err
	}
	success, err := services.RemoveExcludeNodeHost(clusterClient, lastReplicaNodeName)
	if !success {
		r.recorder.AnnotatedEventf(r.instance, annotations, "Normal", "Scaler", "Group-%s . node %s node is empty but node is still excluded from allocation", nodePoolGroupName, lastReplicaNodeName)
		return err
	}

	componentStatus := opsterv1.ComponentStatus{
		Component:   "Scaler",
		Status:      "Drained",
		Description: nodePoolGroupName,
	}
	lg.Info(fmt.Sprintf("Group-%s .node %s node is drained", nodePoolGroupName, lastReplicaNodeName))
	r.instance.Status.ComponentsStatus = helpers.Replace(currentStatus, componentStatus, r.instance.Status.ComponentsStatus)
	err = r.Status().Update(r.ctx, r.instance)
	if err != nil {
		lg.Error(err, "failed to update status")
		r.recorder.AnnotatedEventf(r.instance, annotations, "Warning", "Scaler", "Failed to update operator status")
		return err
	}
	return err
}

func (r *ScalerReconciler) cleanupStatefulSets(result *reconciler.CombinedResult) {
	stsList := &appsv1.StatefulSetList{}
	if err := r.Client.List(
		r.ctx,
		stsList,
		client.InNamespace(r.instance.Name),
		client.MatchingLabels{helpers.ClusterLabel: r.instance.Name},
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

func (r *ScalerReconciler) removeStatefulSet(sts appsv1.StatefulSet) (*ctrl.Result, error) {
	if !r.instance.Spec.ConfMgmt.SmartScaler {
		return r.ReconcileResource(&sts, reconciler.StateAbsent)
	}

	// Gracefully remove nodes
	lg := log.FromContext(r.ctx)
	username, password, err := helpers.UsernameAndPassword(r.ctx, r.Client, r.instance)
	if err != nil {
		return nil, err
	}
	annotations := map[string]string{"cluster-name": r.instance.GetName()}
	clusterClient, err := services.NewOsClusterClient(builders.URLForCluster(r.instance), username, password)
	if err != nil {
		lg.Error(err, "failed to create os client")
		r.recorder.AnnotatedEventf(r.instance, annotations, "Warning", "Scaler", "Failed to create os client")
		return nil, err
	}
	r.recorder.AnnotatedEventf(r.instance, annotations, "Normal", "Scaler", "Finished os client for scaling ")

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
		r.recorder.AnnotatedEventf(r.instance, annotations, "Warning", "Scaler", "Failed to check shards on node")
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
	if err != nil {
		return result, err
	}

	_, err = services.RemoveExcludeNodeHost(clusterClient, lastReplicaNodeName)
	if err != nil {
		lg.Error(err, fmt.Sprintf("failed to remove node exclusion for %s", lastReplicaNodeName))
	}
	r.recorder.AnnotatedEventf(r.instance, annotations, "Noraml", "Scaler", "Finished scaling")
	return result, err
}
