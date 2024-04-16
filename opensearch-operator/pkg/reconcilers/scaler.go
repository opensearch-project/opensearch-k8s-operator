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
	"github.com/cisco-open/operator-tools/pkg/reconciler"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type ScalerReconciler struct {
	client            k8s.K8sClient
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
		client:            k8s.NewK8sClient(client, ctx, append(opts, reconciler.WithLog(log.FromContext(ctx).WithValues("reconciler", "scaler")))...),
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
	annotations := map[string]string{"cluster-name": r.instance.GetName()}
	currentSts, err := r.client.GetStatefulSet(sts_name, namespace)
	if err != nil {
		return false, err
	}

	componentStatus := opsterv1.ComponentStatus{
		Component:   "Scaler",
		Status:      "Running",
		Description: nodePool.Component,
	}
	comp := r.instance.Status.ComponentsStatus
	currentStatus, found := helpers.FindFirstPartial(comp, componentStatus, helpers.GetByDescriptionAndGroup)

	desireReplicaDiff := *currentSts.Spec.Replicas - nodePool.Replicas
	if desireReplicaDiff == 0 {
		// If a scaling operation was started before for this nodePool
		if found {
			err := r.client.UpdateOpenSearchClusterStatus(client.ObjectKeyFromObject(r.instance), func(instance *opsterv1.OpenSearchCluster) {
				if currentSts.Status.ReadyReplicas != nodePool.Replicas {
					// Change the status to waiting while the pods are coming up or getting deleted
					componentStatus.Status = "Waiting"
					instance.Status.ComponentsStatus = helpers.Replace(currentStatus, componentStatus, r.instance.Status.ComponentsStatus)
				} else {
					// Scaling operation is completed, remove the status
					instance.Status.ComponentsStatus = helpers.RemoveIt(currentStatus, r.instance.Status.ComponentsStatus)
				}
			})
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
		err := r.client.UpdateOpenSearchClusterStatus(client.ObjectKeyFromObject(r.instance), func(instance *opsterv1.OpenSearchCluster) {
			instance.Status.ComponentsStatus = helpers.Replace(currentStatus, componentStatus, instance.Status.ComponentsStatus)
		})
		if err != nil {
			r.recorder.AnnotatedEventf(r.instance, annotations, "Warning", "Scaler", "Failed to update status")
			lg.Error(err, "failed to update status")
			return true, err
		}

		if desireReplicaDiff > 0 {
			r.recorder.AnnotatedEventf(r.instance, annotations, "Normal", "Scaler", "Starting to scaling")
			if !r.instance.Spec.ConfMgmt.SmartScaler {
				lg.Info(fmt.Sprintf("SmartScaler is disabled, removing nodes from nodegroup %s without draining", nodePool.Component))
				requeue, err := r.decreaseOneNode(currentStatus, currentSts, nodePool.Component, r.instance.Spec.ConfMgmt.SmartScaler)
				r.recorder.AnnotatedEventf(r.instance, annotations, "Normal", "Scaler", "Notice - your SmartScaler is not enabled")
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
	lastReplicaNodeName := helpers.ReplicaHostName(currentSts, *currentSts.Spec.Replicas)
	r.recorder.AnnotatedEventf(r.instance, annotations, "Normal", "Scaler", "Start increaseing node %s on %s ", lastReplicaNodeName, nodePoolGroupName)
	_, err := r.client.ReconcileResource(&currentSts, reconciler.StatePresent)
	if err != nil {
		r.recorder.AnnotatedEventf(r.instance, annotations, "Normal", "Scaler", "Failed to add node %s/%s", r.instance.Namespace, r.instance.Name)
		return true, err
	}
	lg.Info(fmt.Sprintf("Group: %s, Added node %s", nodePoolGroupName, lastReplicaNodeName))
	r.recorder.AnnotatedEventf(r.instance, annotations, "Normal", "Scaler", "Added new node %s", lastReplicaNodeName)
	return false, nil
}

func (r *ScalerReconciler) decreaseOneNode(currentStatus opsterv1.ComponentStatus, currentSts appsv1.StatefulSet, nodePoolGroupName string, smartDecrease bool) (bool, error) {
	lg := log.FromContext(r.ctx)
	*currentSts.Spec.Replicas--
	annotations := map[string]string{"cluster-name": r.instance.GetName()}
	lastReplicaNodeName := helpers.ReplicaHostName(currentSts, *currentSts.Spec.Replicas)
	r.recorder.AnnotatedEventf(r.instance, annotations, "Normal", "Scaler", "Start to decreaseing node %s on %s ", lastReplicaNodeName, nodePoolGroupName)
	_, err := r.client.ReconcileResource(&currentSts, reconciler.StatePresent)
	if err != nil {
		r.recorder.AnnotatedEventf(r.instance, annotations, "Normal", "Scaler", "Failed to remove node - Group-%s . Failed to remove node %s", nodePoolGroupName, lastReplicaNodeName)
		lg.Error(err, fmt.Sprintf("failed to remove node %s", lastReplicaNodeName))
		return true, err
	}
	lg.Info(fmt.Sprintf("Group: %s, Removed node %s", nodePoolGroupName, lastReplicaNodeName))
	err = r.client.UpdateOpenSearchClusterStatus(client.ObjectKeyFromObject(r.instance), func(instance *opsterv1.OpenSearchCluster) {
		instance.Status.ComponentsStatus = helpers.RemoveIt(currentStatus, instance.Status.ComponentsStatus)
	})
	if err != nil {
		lg.Error(err, "failed to update status")
		return false, err
	}

	if !smartDecrease {
		return false, err
	}
	username, password, err := helpers.UsernameAndPassword(r.client, r.instance)
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
	username, password, err := helpers.UsernameAndPassword(r.client, r.instance)
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
	lastReplicaNodeName := helpers.ReplicaHostName(currentSts, *currentSts.Spec.Replicas-1)

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
		lg.Info(fmt.Sprintf("Group: %s, Excluded node: %s", nodePoolGroupName, lastReplicaNodeName))
		err = r.client.UpdateOpenSearchClusterStatus(client.ObjectKeyFromObject(r.instance), func(instance *opsterv1.OpenSearchCluster) {
			instance.Status.ComponentsStatus = helpers.Replace(currentStatus, componentStatus, instance.Status.ComponentsStatus)
		})
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
	lg.Info(fmt.Sprintf("Group: %s, Failed to exclude node: %s", nodePoolGroupName, lastReplicaNodeName))
	err = r.client.UpdateOpenSearchClusterStatus(client.ObjectKeyFromObject(r.instance), func(instance *opsterv1.OpenSearchCluster) {
		instance.Status.ComponentsStatus = helpers.Replace(currentStatus, componentStatus, instance.Status.ComponentsStatus)
	})
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
	lastReplicaNodeName := helpers.ReplicaHostName(currentSts, *currentSts.Spec.Replicas-1)
	username, password, err := helpers.UsernameAndPassword(r.client, r.instance)
	if err != nil {
		return err
	}

	clusterClient, err := services.NewOsClusterClient(builders.URLForCluster(r.instance), username, password)
	if err != nil {
		return err
	}
	nodeNotEmpty, err := services.HasShardsOnNode(clusterClient, lastReplicaNodeName)
	if nodeNotEmpty {
		lg.Info(fmt.Sprintf("Group: %s, Waiting for node %s to drain", nodePoolGroupName, lastReplicaNodeName))
		return err
	}

	componentStatus := opsterv1.ComponentStatus{
		Component:   "Scaler",
		Status:      "Drained",
		Description: nodePoolGroupName,
	}
	lg.Info(fmt.Sprintf("Group: %s, Node %s is drained", nodePoolGroupName, lastReplicaNodeName))
	err = r.client.UpdateOpenSearchClusterStatus(client.ObjectKeyFromObject(r.instance), func(instance *opsterv1.OpenSearchCluster) {
		instance.Status.ComponentsStatus = helpers.Replace(currentStatus, componentStatus, instance.Status.ComponentsStatus)
	})
	if err != nil {
		lg.Error(err, "failed to update status")
		r.recorder.AnnotatedEventf(r.instance, annotations, "Warning", "Scaler", "Failed to update operator status")
		return err
	}
	return err
}

func (r *ScalerReconciler) cleanupStatefulSets(result *reconciler.CombinedResult) {
	stsList, err := r.client.ListStatefulSets(client.InNamespace(r.instance.Name),
		client.MatchingLabels{helpers.ClusterLabel: r.instance.Name})
	if err != nil {
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
	lg := log.FromContext(r.ctx)
	lg.Info(fmt.Sprintf("Removing statefulset: %s", sts.Name))

	if !r.instance.Spec.ConfMgmt.SmartScaler {
		return r.client.ReconcileResource(&sts, reconciler.StateAbsent)
	}

	// Gracefully remove nodes
	username, password, err := helpers.UsernameAndPassword(r.client, r.instance)
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
	lastReplicaNodeName := helpers.ReplicaHostName(sts, workingOrdinal)
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
		lg.Info(fmt.Sprintf("Waiting for shards to drain from node %s", lastReplicaNodeName))
		return &ctrl.Result{
			Requeue:      true,
			RequeueAfter: 15 * time.Second,
		}, nil
	}

	if workingOrdinal == 0 {
		result, err := r.client.ReconcileResource(&sts, reconciler.StateAbsent)
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
	result, err := r.client.ReconcileResource(&sts, reconciler.StatePresent)
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
