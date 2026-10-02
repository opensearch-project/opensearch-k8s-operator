package reconcilers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"k8s.io/utils/ptr"

	opensearchv1 "github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/api/opensearch.org/v1"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/opensearch-gateway/services"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/builders"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/helpers"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/reconciler"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/reconcilers/k8s"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/reconcilers/util"
	appsv1 "k8s.io/api/apps/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	scalerReconcilerName        = "scaler"
	drainStartedConditionPrefix = "drainStarted:"
	drainStalledCondition       = "DrainStalled"
	// drainStallWarningAfter is how long a scale-down drain may wait with no
	// observed emptiness before a Warning event and DrainStalled condition are
	// recorded. The drain itself is not aborted.
	drainStallWarningAfter = 15 * time.Minute
	// drainPollInterval is the fixed cadence for waiting on a scale-down drain
	drainPollInterval = 15 * time.Second
)

type ScalerReconciler struct {
	client            k8s.K8sClient
	ctx               context.Context
	recorder          record.EventRecorder
	reconcilerContext *ReconcilerContext
	instance          *opensearchv1.OpenSearchCluster
	ReconcilerOptions
}

func NewScalerReconciler(
	client client.Client,
	ctx context.Context,
	recorder record.EventRecorder,
	reconcilerContext *ReconcilerContext,
	instance *opensearchv1.OpenSearchCluster,
	opts ...ReconcilerOption,
) *ScalerReconciler {
	options := ReconcilerOptions{}
	options.apply(opts...)
	return &ScalerReconciler{
		client:            k8s.NewK8sClient(client, ctx, reconciler.WithLog(log.FromContext(ctx).WithValues("reconciler", scalerReconcilerName))),
		ctx:               ctx,
		recorder:          recorder,
		reconcilerContext: reconcilerContext,
		instance:          instance,
		ReconcilerOptions: options,
	}
}

func (r *ScalerReconciler) Name() string { return scalerReconcilerName }

func (r *ScalerReconciler) Reconcile() (ctrl.Result, error) {
	results := &reconciler.CombinedResult{}
	lg := log.FromContext(r.ctx)

	upgradeInProgress := r.instance.Status.Version != "" && r.instance.Status.Version != r.instance.Spec.General.Version

	// Voting-config exclusions are cluster-wide and survive a failed reconcile;
	// clear the ones no in-flight master removal owns any more.
	r.sweepVotingConfigExclusions()

	// Skip replica scaling while an upgrade is changing versions — mirrors the
	// rolling-restart guard so scaler and upgrader do not delete pods at once.
	// Cleanup of entirely removed node pools still runs below.
	if !upgradeInProgress {
		// Clean stale allocation exclusions (e.g. from a failed RemoveExcludeNodeHost after scale-down or upgrade).
		// CleanStaleExclusionList itself skips when a scale-down drain is in progress.
		if r.instance.Spec.ConfMgmt.SmartScaler {
			clusterClient, clientErr := util.CreateClientForCluster(r.client, r.ctx, r.instance, r.osClientTransport)
			if clientErr == nil {
				if res, cleanupErr := util.CleanStaleExclusionList(r.client, r.instance, clusterClient, lg); cleanupErr != nil {
					return ctrl.Result{}, cleanupErr
				} else if res.Requeue {
					return res, nil
				}
			}
		}

		// Accumulate requeue across all pools — previously only the last pool's
		// value was kept, so a drain on a non-last pool failed to short-circuit
		// the main reconcile chain before upgrade/restart.
		for _, nodePool := range r.instance.Spec.NodePools {
			requeue, poolErr := r.reconcileNodePool(&nodePool)
			res := ctrl.Result{Requeue: requeue}
			if requeue {
				// Same cadence as removeStatefulSet's drain wait. RequeueAfter is
				// dropped by controller-runtime whenever poolErr != nil, so
				// actual error retries still back off as before.
				res.RequeueAfter = drainPollInterval
			}
			results.Combine(&res, poolErr)
		}
	} else {
		lg.V(1).Info("Upgrade in progress, skipping replica scaling")
	}

	// Check readiness of current NodePools before cleaning up old node pools.
	// Cleanup of removed pools must happen even during upgrades so that
	// deleted node pools do not linger.
	ready, err := r.nodePoolsReady()
	if err != nil {
		results.Combine(&ctrl.Result{}, err)
	} else if !ready {
		// Not all node pools are ready yet: skip cleanup and retry later. Deliberately
		// RequeueAfter-only (no Requeue=true) so the main chain still runs the upgrade
		// and rolling-restart reconcilers, which own recovery of stuck pods (issue #1531).
		results.Combine(&ctrl.Result{RequeueAfter: drainPollInterval}, nil)
	} else {
		// Clean up old node pools (all current nodePools are ready)
		r.cleanupStatefulSets(results)
	}

	return results.Result, results.Err
}

func (r *ScalerReconciler) reconcileNodePool(nodePool *opensearchv1.NodePool) (bool, error) {
	lg := log.FromContext(r.ctx)
	namespace := r.instance.Namespace
	sts_name := builders.StsName(r.instance, nodePool)
	annotations := map[string]string{"cluster-name": r.instance.GetName()}
	currentSts, err := r.client.GetStatefulSet(sts_name, namespace)
	if err != nil {
		return false, err
	}

	readyReplicas, err := helpers.ReadyReplicasForNodePool(r.client, r.instance, nodePool)
	if err != nil {
		return false, err
	}
	currentSts.Status.ReadyReplicas = readyReplicas

	componentStatus := opensearchv1.ComponentStatus{
		Component:   "Scaler",
		Status:      "Running",
		Description: nodePool.Component,
	}
	comp := r.instance.Status.ComponentsStatus
	currentStatus, found := helpers.FindFirstPartial(comp, componentStatus, helpers.GetByDescriptionAndComponent)

	desireReplicaDiff := *currentSts.Spec.Replicas - nodePool.Replicas
	if desireReplicaDiff == 0 {
		// If a scaling operation was started before for this nodePool
		if found {
			if currentStatus.Status == "Excluded" || currentStatus.Status == "Drained" {
				target := scalerTargetNodeName(currentStatus.Conditions)
				isMaster := helpers.HasManagerRole(nodePool)
				// The target is the first ordinal past the StatefulSet: decreaseOneNode already
				// shrank it, only the OpenSearch cleanup (allocation / voting-config exclusions)
				// failed and is still pending.
				removedName := helpers.ReplicaHostName(currentSts, *currentSts.Spec.Replicas)
				pendingCleanup := currentStatus.Status == "Drained" && target == removedName
				// A Drained status written by an older operator has no target. The removed
				// node is that same past-the-end ordinal. If it is still excluded, the shrink
				// already happened and a no-wait clear would put a leaving voter back into
				// the voting configuration. Otherwise the scale-down was reverted.
				if !pendingCleanup && currentStatus.Status == "Drained" && target == "" && isMaster {
					var excludeErr error
					pendingCleanup, excludeErr = r.votingConfigExcludes(removedName)
					if excludeErr != nil {
						return true, excludeErr
					}
					if pendingCleanup {
						target = removedName
					}
				}
				if pendingCleanup {
					return r.finishDecreaseCleanup(currentStatus, currentSts, nodePool.Component, nil, r.instance.Spec.ConfMgmt.SmartScaler, isMaster, target)
				}
				// Otherwise the scale-down was reverted before the target was removed.
				return r.cancelDecrease(currentStatus, nodePool.Component, target, isMaster)
			}
			err := r.client.UpdateOpenSearchClusterStatus(client.ObjectKeyFromObject(r.instance), func(instance *opensearchv1.OpenSearchCluster) {
				if currentSts.Status.ReadyReplicas != nodePool.Replicas {
					// Change the status to waiting while the pods are coming up or getting deleted
					componentStatus.Status = "Waiting"
					instance.Status.ComponentsStatus = helpers.Replace(currentStatus, componentStatus, instance.Status.ComponentsStatus)
				} else {
					// Scaling operation is completed, remove the status
					instance.Status.ComponentsStatus = helpers.RemoveIt(currentStatus, instance.Status.ComponentsStatus)
				}
			})
			if err != nil {
				lg.Error(err, "failed to update status")
				return false, err
			}
		}
		return false, nil
	}

	// A scale-down must not take a member out while any node pool already has a
	// pod down, whatever took it down: a rolling restart or upgrade mid-cycle, a
	// node failure, or a pod the scaler itself just removed that is still
	// terminating. This is the reverse direction of #1572, which stops the
	// rolling restart from deleting a pod while a scaled-down one is leaving; the
	// scaler runs before the restart in the chain and had no gate of its own.
	// The gate covers starting a scale-down, continuing a drain and shrinking a
	// drained pool. The drain is held too: drainNode reactivates shard allocation,
	// which undoes the primaries-only freeze a restart in flight relies on, and its
	// Requeue=true would starve the restart reconciler that has to bring the pod
	// back. Scale-up stays unguarded because adding capacity never removes a
	// member and is a way to recover a degraded cluster.
	// Deliberately return without Requeue: the upgrade and rolling-restart
	// reconcilers run after the scaler and own the recovery of stuck pods (issue
	// #1531), so Requeue=true would wait forever on a pool only they can make
	// ready again. Reconcile() still polls via the nodePoolsReady() check.
	scalingDown := desireReplicaDiff > 0 || currentStatus.Status == "Excluded" || currentStatus.Status == "Drained"
	if scalingDown {
		unreadyPool, err := r.unreadyNodePool()
		if err != nil {
			return false, err
		}
		if unreadyPool != "" {
			lg.Info(fmt.Sprintf("Group: %s, holding scale-down while node pool %s has a pod that is not ready", nodePool.Component, unreadyPool))
			return false, nil
		}
	}

	// Check for 'Running' or 'Waiting' status so we process scaling when replicas change.
	// 'Running' indicates a scaling operation has begun; 'Waiting' means we were waiting for
	// pods to become ready—if the user changed replicas in that state, we must handle it.
	if !found || currentStatus.Status == "Running" || currentStatus.Status == "Waiting" {
		// Change the status to running, to indicate that a scaling operation for this nodePool has started
		err := r.client.UpdateOpenSearchClusterStatus(client.ObjectKeyFromObject(r.instance), func(instance *opensearchv1.OpenSearchCluster) {
			instance.Status.ComponentsStatus = helpers.Replace(currentStatus, componentStatus, instance.Status.ComponentsStatus)
		})
		if err != nil {
			r.recorder.AnnotatedEventf(r.instance, annotations, "Warning", "Scaler", "Failed to update status")
			lg.Error(err, "failed to update status")
			return true, err
		}

		if desireReplicaDiff > 0 {
			r.recorder.AnnotatedEventf(r.instance, annotations, "Normal", "Scaler", "Starting to scaling")
			isMaster := helpers.HasManagerRole(nodePool)
			// Master-eligible nodes always go through the exclude/drain path so we can
			// apply voting-config exclusions before permanently removing a voter.
			// Data-only nodes without SmartScaler are removed immediately.
			if !r.instance.Spec.ConfMgmt.SmartScaler && !isMaster {
				lg.Info(fmt.Sprintf("SmartScaler is disabled, removing nodes from nodegroup %s without draining", nodePool.Component))
				requeue, err := r.decreaseOneNode(currentStatus, currentSts, nodePool.Component, false, false)
				r.recorder.AnnotatedEventf(r.instance, annotations, "Warning", "Scaler", "SmartScaler is disabled: removing node from %s without draining", nodePool.Component)
				r.recorder.AnnotatedEventf(r.instance, annotations, "Normal", "Scaler", "Starting to decrease node")
				return requeue, err
			}
			if !r.instance.Spec.ConfMgmt.SmartScaler && isMaster {
				r.recorder.AnnotatedEventf(r.instance, annotations, "Normal", "Scaler", "SmartScaler disabled but master node requires voting exclusion before removal")
			}
			err := r.excludeNode(currentStatus, currentSts, nodePool.Component, isMaster)
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

		requeue, err := r.decreaseOneNode(currentStatus, currentSts, nodePool.Component, r.instance.Spec.ConfMgmt.SmartScaler, helpers.HasManagerRole(nodePool))
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

func (r *ScalerReconciler) decreaseOneNode(currentStatus opensearchv1.ComponentStatus, currentSts appsv1.StatefulSet, nodePoolGroupName string, smartDecrease bool, isMaster bool) (bool, error) {
	lg := log.FromContext(r.ctx)
	*currentSts.Spec.Replicas--
	annotations := map[string]string{"cluster-name": r.instance.GetName()}
	lastReplicaNodeName := helpers.ReplicaHostName(currentSts, *currentSts.Spec.Replicas)

	// Verify that the node being removed is the same one that was excluded/drained
	if targetNodeName := scalerTargetNodeName(currentStatus.Conditions); targetNodeName != "" {
		if lastReplicaNodeName != targetNodeName {
			lg.Info(fmt.Sprintf("Group: %s, Target node %s does not match last replica %s, resetting to Running", nodePoolGroupName, targetNodeName, lastReplicaNodeName))
			*currentSts.Spec.Replicas++
			componentStatus := opensearchv1.ComponentStatus{
				Component:   "Scaler",
				Status:      "Running",
				Description: nodePoolGroupName,
			}
			err := r.client.UpdateOpenSearchClusterStatus(client.ObjectKeyFromObject(r.instance), func(instance *opensearchv1.OpenSearchCluster) {
				instance.Status.ComponentsStatus = helpers.Replace(currentStatus, componentStatus, instance.Status.ComponentsStatus)
			})
			if err != nil {
				lg.Error(err, "failed to update status")
			}
			return true, fmt.Errorf("target node mismatch during decrease: excluded/drained %s but would remove %s, reset to Running", targetNodeName, lastReplicaNodeName)
		}
	}

	var clusterClient *services.OsClusterClient
	if smartDecrease || isMaster {
		var err error
		clusterClient, err = util.CreateClientForCluster(r.client, r.ctx, r.instance, r.osClientTransport)
		if err != nil {
			*currentSts.Spec.Replicas++
			lg.Error(err, "failed to create os client")
			r.recorder.AnnotatedEventf(r.instance, annotations, "Warning", "Scaler", "Failed to create os client before removing node %s", lastReplicaNodeName)
			return true, err
		}
	}

	if isMaster {
		// Voting exclusions are cluster-wide and can be cleared underneath us (e.g. by
		// cancelDecrease on another pool); re-apply right before shrinking so a voter is
		// never removed. Idempotent for an already-excluded node.
		if err := services.AddVotingConfigExclusion(clusterClient, lg, lastReplicaNodeName); err != nil {
			*currentSts.Spec.Replicas++
			r.recorder.AnnotatedEventf(r.instance, annotations, "Warning", "Scaler", "Failed to add voting config exclusion for %s", lastReplicaNodeName)
			return true, err
		}
	}

	if smartDecrease {
		// Drained is persisted on the CR; allocation exclusions are only transient
		// cluster settings. Re-check emptiness before shrinking so a lost exclusion
		// cannot delete a node that still holds data.
		nodeNotEmpty, err := services.HasShardsOnNode(clusterClient, lastReplicaNodeName)
		if err != nil {
			*currentSts.Spec.Replicas++
			lg.Error(err, "failed to verify node is empty before scale-down")
			r.recorder.AnnotatedEventf(r.instance, annotations, "Warning", "Scaler", "Failed to verify node %s is empty before scale-down", lastReplicaNodeName)
			return true, err
		}
		if nodeNotEmpty {
			*currentSts.Spec.Replicas++
			lg.Info(fmt.Sprintf("Group: %s, Node %s still has shards after drain, resetting to Excluded", nodePoolGroupName, lastReplicaNodeName))
			r.recorder.AnnotatedEventf(r.instance, annotations, "Warning", "Scaler", "Node %s still has shards after being marked drained; re-excluding and waiting", lastReplicaNodeName)
			if _, excludeErr := services.AppendExcludeNodeHost(clusterClient, lg, lastReplicaNodeName); excludeErr != nil {
				lg.Error(excludeErr, fmt.Sprintf("failed to re-apply exclusion for node %s", lastReplicaNodeName))
			}
			startedAt, ok := drainStartedAt(currentStatus.Conditions)
			if !ok {
				startedAt = time.Now().UTC()
			}
			stalled := drainHasStalled(startedAt, time.Now().UTC(), drainStallWarningAfter)
			componentStatus := opensearchv1.ComponentStatus{
				Component:   "Scaler",
				Status:      "Excluded",
				Description: nodePoolGroupName,
				Conditions:  drainConditions(lastReplicaNodeName, startedAt, stalled),
			}
			if err := r.client.UpdateOpenSearchClusterStatus(client.ObjectKeyFromObject(r.instance), func(instance *opensearchv1.OpenSearchCluster) {
				instance.Status.ComponentsStatus = helpers.Replace(currentStatus, componentStatus, instance.Status.ComponentsStatus)
			}); err != nil {
				lg.Error(err, "failed to update status")
				return true, err
			}
			return true, nil
		}
	}

	r.recorder.AnnotatedEventf(r.instance, annotations, "Normal", "Scaler", "Start to decreaseing node %s on %s ", lastReplicaNodeName, nodePoolGroupName)
	_, err := r.client.ReconcileResource(&currentSts, reconciler.StatePresent)
	if err != nil {
		r.recorder.AnnotatedEventf(r.instance, annotations, "Normal", "Scaler", "Failed to remove node - Group-%s . Failed to remove node %s", nodePoolGroupName, lastReplicaNodeName)
		lg.Error(err, fmt.Sprintf("failed to remove node %s", lastReplicaNodeName))
		return true, err
	}
	lg.Info(fmt.Sprintf("Group: %s, Removed node %s", nodePoolGroupName, lastReplicaNodeName))

	if !smartDecrease && !isMaster {
		err = r.client.UpdateOpenSearchClusterStatus(client.ObjectKeyFromObject(r.instance), func(instance *opensearchv1.OpenSearchCluster) {
			instance.Status.ComponentsStatus = helpers.RemoveIt(currentStatus, instance.Status.ComponentsStatus)
		})
		if err != nil {
			lg.Error(err, "failed to update status")
			return false, err
		}
		return false, nil
	}

	return r.finishDecreaseCleanup(currentStatus, currentSts, nodePoolGroupName, clusterClient, smartDecrease, isMaster, lastReplicaNodeName)
}

// finishDecreaseCleanup removes allocation and voting-config exclusions after the
// StatefulSet has already been shrunk. The scaler component status is dropped
// only after a successful voting-config clear (for masters) so a failed clear
// is retried on the next reconcile instead of accumulating exclusions.
func (r *ScalerReconciler) finishDecreaseCleanup(currentStatus opensearchv1.ComponentStatus, currentSts appsv1.StatefulSet, nodePoolGroupName string, clusterClient *services.OsClusterClient, smartDecrease bool, isMaster bool, lastReplicaNodeName string) (bool, error) {
	lg := log.FromContext(r.ctx)
	annotations := map[string]string{"cluster-name": r.instance.GetName()}

	if lastReplicaNodeName == "" {
		lastReplicaNodeName = helpers.ReplicaHostName(currentSts, *currentSts.Spec.Replicas)
	}

	if clusterClient == nil && (smartDecrease || isMaster) {
		var err error
		clusterClient, err = util.CreateClientForCluster(r.client, r.ctx, r.instance, r.osClientTransport)
		if err != nil {
			lg.Error(err, "failed to create os client")
			r.recorder.AnnotatedEventf(r.instance, annotations, "Warning", "Scaler", "Failed to create os client after removing node %s", lastReplicaNodeName)
			if isMaster {
				return true, err
			}
		}
	}

	if isMaster && clusterClient != nil {
		// The StatefulSet was just shrunk, so the pod is normally still terminating.
		// A waiting clear issued now would sit on OpenSearch's 30s wait_for_removal
		// deadline (the client deadline as well) and fail; keep the Drained status
		// and come back once the node has left the cluster.
		cleared, clearErr := services.ClearVotingConfigExclusionsIfNodeGone(clusterClient, lg, lastReplicaNodeName)
		if clearErr != nil {
			lg.Error(clearErr, fmt.Sprintf("failed to clear voting config exclusions after removing %s", lastReplicaNodeName))
			r.recorder.AnnotatedEventf(r.instance, annotations, "Warning", "Scaler", "Failed to clear voting config exclusions - Group-%s , node  %s", nodePoolGroupName, lastReplicaNodeName)
			return true, clearErr
		}
		if !cleared {
			lg.Info(fmt.Sprintf("Group: %s, deferring voting config exclusion clear for %s until every excluded node has left the cluster", nodePoolGroupName, lastReplicaNodeName))
			return true, nil
		}
	}

	if clusterClient != nil && smartDecrease {
		success, removeErr := services.RemoveExcludeNodeHost(clusterClient, lg, lastReplicaNodeName)
		if !success || removeErr != nil {
			lg.Error(removeErr, fmt.Sprintf("failed to remove exclude node %s", lastReplicaNodeName))
			r.recorder.AnnotatedEventf(r.instance, annotations, "Warning", "Scaler", "Failed to remove node exclude - Group-%s , node  %s", nodePoolGroupName, lastReplicaNodeName)
		}
	}

	err := r.client.UpdateOpenSearchClusterStatus(client.ObjectKeyFromObject(r.instance), func(instance *opensearchv1.OpenSearchCluster) {
		instance.Status.ComponentsStatus = helpers.RemoveIt(currentStatus, instance.Status.ComponentsStatus)
	})
	if err != nil {
		lg.Error(err, "failed to update status")
		return true, err
	}
	return false, nil
}

// cancelDecrease undoes a scale-down that was reverted before its target node was
// removed: the node stays, so it must hold shards and vote again.
func (r *ScalerReconciler) cancelDecrease(currentStatus opensearchv1.ComponentStatus, nodePoolGroupName, targetNodeName string, isMaster bool) (bool, error) {
	lg := log.FromContext(r.ctx)
	clusterClient, err := util.CreateClientForCluster(r.client, r.ctx, r.instance, r.osClientTransport)
	if err != nil {
		lg.Error(err, "failed to create os client")
		return true, err
	}
	if targetNodeName != "" {
		if _, err := services.RemoveExcludeNodeHost(clusterClient, lg, targetNodeName); err != nil {
			return true, err
		}
	}
	if isMaster {
		// wait_for_removal=false: the node is staying, so a waiting clear would never
		// finish. The DELETE drops every exclusion, including voters that are already
		// past their StatefulSet shrink and will not be posted again by decreaseOneNode.
		// Read those names first and re-add them after the clear.
		excluded, err := services.GetVotingConfigExclusions(clusterClient)
		if err != nil {
			lg.Error(err, "failed to read voting config exclusions before cancelled scale-down clear")
			return true, err
		}
		keep, err := r.votingExclusionsToKeep(targetNodeName, excluded)
		if err != nil {
			return true, err
		}
		if err := clusterClient.ClearVotingConfigExclusions(r.ctx, false); err != nil {
			lg.Error(err, fmt.Sprintf("failed to clear voting config exclusions after cancelled scale-down of %s", targetNodeName))
			return true, err
		}
		if err := r.reapplyLeavingVotingExclusions(clusterClient, keep); err != nil {
			lg.Error(err, "failed to re-apply voting config exclusions for nodes still being removed")
			return true, err
		}
	}
	lg.Info(fmt.Sprintf("Group: %s, scale-down reverted, keeping node %s", nodePoolGroupName, targetNodeName))
	err = r.client.UpdateOpenSearchClusterStatus(client.ObjectKeyFromObject(r.instance), func(instance *opensearchv1.OpenSearchCluster) {
		instance.Status.ComponentsStatus = helpers.RemoveIt(currentStatus, instance.Status.ComponentsStatus)
	})
	return false, err
}

// votingConfigExcludes reports whether nodeName is on the voting-config exclusion list.
func (r *ScalerReconciler) votingConfigExcludes(nodeName string) (bool, error) {
	clusterClient, err := util.CreateClientForCluster(r.client, r.ctx, r.instance, r.osClientTransport)
	if err != nil {
		return false, err
	}
	excluded, err := services.GetVotingConfigExclusions(clusterClient)
	if err != nil {
		return false, err
	}
	for _, name := range excluded {
		if name == nodeName {
			return true, nil
		}
	}
	return false, nil
}

// votingExclusionsToKeep is the set of exclusion names a cluster-wide no-wait clear
// must restore: names already on the list, scaler targets still marked Excluded or
// Drained, and terminating master or bootstrap pods. stayingNode is omitted because
// that scale-down was reverted and the node has to vote again.
func (r *ScalerReconciler) votingExclusionsToKeep(stayingNode string, alreadyExcluded []string) ([]string, error) {
	seen := make(map[string]bool, len(alreadyExcluded))
	var names []string
	add := func(name string) {
		if name == "" || name == stayingNode || seen[name] {
			return
		}
		seen[name] = true
		names = append(names, name)
	}
	for _, name := range alreadyExcluded {
		add(name)
	}
	for _, cs := range r.instance.Status.ComponentsStatus {
		if cs.Component == "Scaler" && (cs.Status == "Excluded" || cs.Status == "Drained") {
			add(scalerTargetNodeName(cs.Conditions))
		}
	}
	pods, err := r.client.ListPods(&client.ListOptions{
		Namespace:     r.instance.Namespace,
		LabelSelector: labels.SelectorFromSet(map[string]string{helpers.ClusterLabel: r.instance.Name}),
	})
	if err != nil {
		return nil, err
	}
	for _, pod := range pods.Items {
		if pod.DeletionTimestamp == nil {
			continue
		}
		role := pod.Labels["opensearch.role"]
		if role == "master" || role == "cluster_manager" || pod.Name == builders.BootstrapPodName(r.instance) {
			add(pod.Name)
		}
	}
	return names, nil
}

// reapplyLeavingVotingExclusions posts exclusions again for nodes that are still
// being removed. A live member that nothing is removing is left off the list so
// the no-wait clear can put it back into the voting configuration.
func (r *ScalerReconciler) reapplyLeavingVotingExclusions(clusterClient *services.OsClusterClient, names []string) error {
	lg := log.FromContext(r.ctx)
	for _, name := range names {
		dangling, err := r.isDanglingVotingExclusion(name)
		if err != nil {
			return err
		}
		if dangling {
			continue
		}
		if err := services.AddVotingConfigExclusion(clusterClient, lg, name); err != nil {
			return err
		}
	}
	return nil
}

func (r *ScalerReconciler) excludeNode(currentStatus opensearchv1.ComponentStatus, currentSts appsv1.StatefulSet, nodePoolGroupName string, isMaster bool) error {
	lg := log.FromContext(r.ctx)
	annotations := map[string]string{"cluster-name": r.instance.GetName()}

	clusterClient, err := util.CreateClientForCluster(r.client, r.ctx, r.instance, r.osClientTransport)
	if err != nil {
		lg.Error(err, "failed to create os client")
		r.recorder.AnnotatedEventf(r.instance, annotations, "Warning", "Scaler", "Failed to create os client for scaling")
		return err
	}
	// -----  Now start remove node ------
	lastReplicaNodeName := helpers.ReplicaHostName(currentSts, *currentSts.Spec.Replicas-1)

	// Master-eligible nodes must leave the voting configuration before they are stopped.
	if isMaster {
		if err := services.AddVotingConfigExclusion(clusterClient, lg, lastReplicaNodeName); err != nil {
			lg.Error(err, fmt.Sprintf("failed to add voting config exclusion for node %s", lastReplicaNodeName))
			r.recorder.AnnotatedEventf(r.instance, annotations, "Warning", "Scaler", "Failed to add voting config exclusion for %s", lastReplicaNodeName)
			return err
		}
	}

	// With SmartScaler off a master-eligible node is only removed from the voting
	// configuration; the user opted out of moving its shards first.
	excluded := true
	if r.instance.Spec.ConfMgmt.SmartScaler {
		excluded, err = services.AppendExcludeNodeHost(clusterClient, lg, lastReplicaNodeName)
		if err != nil {
			lg.Error(err, fmt.Sprintf("failed to exclude node %s", lastReplicaNodeName))
			return err
		}
	}
	if excluded {
		componentStatus := opensearchv1.ComponentStatus{
			Component:   "Scaler",
			Status:      "Excluded",
			Description: nodePoolGroupName,
			Conditions:  drainConditions(lastReplicaNodeName, time.Now().UTC(), false),
		}
		r.recorder.AnnotatedEventf(r.instance, annotations, "Normal", "Scaler", "Finished to Exclude %s/%s", r.instance.Namespace, r.instance.Name)
		lg.Info(fmt.Sprintf("Group: %s, Excluded node: %s", nodePoolGroupName, lastReplicaNodeName))
		err = r.client.UpdateOpenSearchClusterStatus(client.ObjectKeyFromObject(r.instance), func(instance *opensearchv1.OpenSearchCluster) {
			instance.Status.ComponentsStatus = helpers.Replace(currentStatus, componentStatus, instance.Status.ComponentsStatus)
		})
		if err != nil {
			lg.Error(err, "failed to update status")
			r.recorder.AnnotatedEventf(r.instance, annotations, "Warning", "Scaler", "Failed to update operator status")
			return err
		}

		return err
	}

	componentStatus := opensearchv1.ComponentStatus{
		Component:   "Scaler",
		Status:      "Running",
		Description: nodePoolGroupName,
	}
	r.recorder.AnnotatedEventf(r.instance, annotations, "Normal", "Scaler", "Start sacle %s/%s from %d to %d", r.instance.Namespace, r.instance.Name, *currentSts.Spec.Replicas, *currentSts.Spec.Replicas-1)
	lg.Info(fmt.Sprintf("Group: %s, Failed to exclude node: %s", nodePoolGroupName, lastReplicaNodeName))
	err = r.client.UpdateOpenSearchClusterStatus(client.ObjectKeyFromObject(r.instance), func(instance *opensearchv1.OpenSearchCluster) {
		instance.Status.ComponentsStatus = helpers.Replace(currentStatus, componentStatus, instance.Status.ComponentsStatus)
	})
	if err != nil {
		r.recorder.AnnotatedEventf(r.instance, annotations, "Warning", "Scaler", "Group-%s . failed to remove node exclude %s", nodePoolGroupName, lastReplicaNodeName)
		lg.Error(err, "failed to update status")
		return err
	}

	return err
}

func (r *ScalerReconciler) drainNode(currentStatus opensearchv1.ComponentStatus, currentSts appsv1.StatefulSet, nodePoolGroupName string) error {
	lg := log.FromContext(r.ctx)
	annotations := map[string]string{"cluster-name": r.instance.GetName()}

	// Retrieve the target node name from the status conditions set during exclude phase
	var lastReplicaNodeName string
	if target := scalerTargetNodeName(currentStatus.Conditions); target != "" {
		lastReplicaNodeName = target
	} else {
		// Fallback for backwards compatibility
		lastReplicaNodeName = helpers.ReplicaHostName(currentSts, *currentSts.Spec.Replicas-1)
	}

	// Verify the target node is still the last replica
	expectedNodeName := helpers.ReplicaHostName(currentSts, *currentSts.Spec.Replicas-1)
	if lastReplicaNodeName != expectedNodeName {
		lg.Info(fmt.Sprintf("Group: %s, Target node %s no longer matches last replica %s, resetting to Running", nodePoolGroupName, lastReplicaNodeName, expectedNodeName))
		componentStatus := opensearchv1.ComponentStatus{
			Component:   "Scaler",
			Status:      "Running",
			Description: nodePoolGroupName,
		}
		err := r.client.UpdateOpenSearchClusterStatus(client.ObjectKeyFromObject(r.instance), func(instance *opensearchv1.OpenSearchCluster) {
			instance.Status.ComponentsStatus = helpers.Replace(currentStatus, componentStatus, instance.Status.ComponentsStatus)
		})
		if err != nil {
			lg.Error(err, "failed to update status")
		}
		return fmt.Errorf("target node mismatch during drain: excluded %s but last replica is %s, reset to Running", lastReplicaNodeName, expectedNodeName)
	}

	if !r.instance.Spec.ConfMgmt.SmartScaler {
		// Only master-eligible pools reach the drain with SmartScaler off, and
		// they are here for the voting-config exclusion alone: mark them Drained
		// without waiting for shards to move.
		lg.Info(fmt.Sprintf("Group: %s, SmartScaler is disabled, skipping shard drain for node %s", nodePoolGroupName, lastReplicaNodeName))
		return r.markDrained(currentStatus, lastReplicaNodeName, nodePoolGroupName, annotations)
	}

	clusterClient, err := util.CreateClientForCluster(r.client, r.ctx, r.instance, r.osClientTransport)
	if err != nil {
		return err
	}
	// Same reason as removeStatefulSet: the drain needs replica movement, which
	// allocation.enable=primaries blocks. Idempotent.
	if err := services.ReactivateShardAllocation(clusterClient); err != nil {
		lg.Error(err, "failed to reactivate shard allocation before draining")
		return err
	}
	nodeNotEmpty, err := services.HasShardsOnNode(clusterClient, lastReplicaNodeName)
	if err != nil {
		lg.Error(err, "failed to check shards on node")
		r.recorder.AnnotatedEventf(r.instance, annotations, "Warning", "Scaler", "Failed to check shards on node %s; not marking drained", lastReplicaNodeName)
		return err
	}
	if nodeNotEmpty {
		lg.Info(fmt.Sprintf("Group: %s, Waiting for node %s to drain", nodePoolGroupName, lastReplicaNodeName))
		if _, excludeErr := services.AppendExcludeNodeHost(clusterClient, lg, lastReplicaNodeName); excludeErr != nil {
			lg.Error(excludeErr, fmt.Sprintf("failed to re-apply exclusion for node %s during drain", lastReplicaNodeName))
		}
		return r.recordDrainWait(currentStatus, lastReplicaNodeName, nodePoolGroupName, annotations)
	}

	return r.markDrained(currentStatus, lastReplicaNodeName, nodePoolGroupName, annotations)
}

// markDrained records that the target node may now be removed.
func (r *ScalerReconciler) markDrained(currentStatus opensearchv1.ComponentStatus, lastReplicaNodeName, nodePoolGroupName string, annotations map[string]string) error {
	lg := log.FromContext(r.ctx)
	componentStatus := opensearchv1.ComponentStatus{
		Component:   "Scaler",
		Status:      "Drained",
		Description: nodePoolGroupName,
		Conditions:  []string{lastReplicaNodeName},
	}
	lg.Info(fmt.Sprintf("Group: %s, Node %s is drained", nodePoolGroupName, lastReplicaNodeName))
	err := r.client.UpdateOpenSearchClusterStatus(client.ObjectKeyFromObject(r.instance), func(instance *opensearchv1.OpenSearchCluster) {
		instance.Status.ComponentsStatus = helpers.Replace(currentStatus, componentStatus, instance.Status.ComponentsStatus)
	})
	if err != nil {
		lg.Error(err, "failed to update status")
		r.recorder.AnnotatedEventf(r.instance, annotations, "Warning", "Scaler", "Failed to update operator status")
		return err
	}
	return nil
}

// unreadyNodePool returns the first node pool that has fewer ready pods than its
// StatefulSet asks for, or "" when every pool is whole. Readiness comes from the
// pods rather than the StatefulSet status so that a terminating pod still counts
// as a member on its way out (see helpers.CountRunningPodsForNodePool).
func (r *ScalerReconciler) unreadyNodePool() (string, error) {
	for i := range r.instance.Spec.NodePools {
		nodePool := &r.instance.Spec.NodePools[i]
		currentSts, err := r.client.GetStatefulSet(builders.StsName(r.instance, nodePool), r.instance.Namespace)
		if err != nil {
			return "", err
		}
		readyReplicas, err := helpers.ReadyReplicasForNodePool(r.client, r.instance, nodePool)
		if err != nil {
			return "", err
		}
		if readyReplicas != ptr.Deref(currentSts.Spec.Replicas, 1) {
			return nodePool.Component, nil
		}
	}
	return "", nil
}

// nodePoolsReady checks that all StatefulSets for current NodePools are fully available.
func (r *ScalerReconciler) nodePoolsReady() (bool, error) {
	lg := log.FromContext(r.ctx)
	for _, nodePool := range r.instance.Spec.NodePools {
		stsName := builders.StsName(r.instance, &nodePool)
		currentSts, err := r.client.GetStatefulSet(stsName, r.instance.Namespace)
		if err != nil {
			return false, err
		}
		if currentSts.Status.AvailableReplicas != *currentSts.Spec.Replicas {
			lg.Info(fmt.Sprintf("Waiting for statefulset to become ready: %s", stsName))
			return false, nil
		}
	}
	return true, nil
}

func (r *ScalerReconciler) cleanupStatefulSets(result *reconciler.CombinedResult) {
	stsList, err := r.client.ListStatefulSets(client.InNamespace(r.instance.Namespace),
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

	annotations := map[string]string{"cluster-name": r.instance.GetName()}
	isMaster := helpers.IsMasterStatefulSet(sts)
	if !r.instance.Spec.ConfMgmt.SmartScaler && !isMaster {
		r.recorder.AnnotatedEventf(r.instance, annotations, "Warning", "Scaler", "SmartScaler is disabled: removing statefulset %s without draining", sts.Name)
		return r.client.ReconcileResource(&sts, reconciler.StateAbsent)
	}

	// Gracefully remove nodes
	clusterClient, err := util.CreateClientForCluster(r.client, r.ctx, r.instance, r.osClientTransport)
	if err != nil {
		lg.Error(err, "failed to create os client")
		r.recorder.AnnotatedEventf(r.instance, annotations, "Warning", "Scaler", "Failed to create os client")
		return nil, err
	}
	r.recorder.AnnotatedEventf(r.instance, annotations, "Normal", "Scaler", "Finished os client for scaling ")

	workingOrdinal := ptr.Deref(sts.Spec.Replicas, 1) - 1
	lastReplicaNodeName := helpers.ReplicaHostName(sts, workingOrdinal)

	if isMaster {
		if err := services.AddVotingConfigExclusion(clusterClient, lg, lastReplicaNodeName); err != nil {
			lg.Error(err, fmt.Sprintf("failed to add voting config exclusion for node %s", lastReplicaNodeName))
			r.recorder.AnnotatedEventf(r.instance, annotations, "Warning", "Scaler", "Failed to add voting config exclusion for %s", lastReplicaNodeName)
			return nil, err
		}
	}

	// This drain only finishes once replicas move off the excluded node, which
	// allocation.enable=primaries prevents. RollingRestart and Upgrade set that
	// while restarting a pod and clear it when their cycle ends, but a drain
	// starting mid-cycle would wait forever: returning Requeue below
	// short-circuits the reconciler chain (see opensearchController.go), so
	// neither of them runs again to clear it. Reactivating here is idempotent -
	// it no-ops when allocation is already unrestricted.
	if r.instance.Spec.ConfMgmt.SmartScaler {
		if err := services.ReactivateShardAllocation(clusterClient); err != nil {
			lg.Error(err, "failed to reactivate shard allocation before draining")
			return nil, err
		}

		_, err = services.AppendExcludeNodeHost(clusterClient, lg, lastReplicaNodeName)
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
				RequeueAfter: drainPollInterval,
			}, nil
		}
	}

	if workingOrdinal == 0 {
		result, err := r.client.ReconcileResource(&sts, reconciler.StateAbsent)
		if err != nil {
			return result, err
		}
		if r.instance.Spec.ConfMgmt.SmartScaler {
			_, err = services.RemoveExcludeNodeHost(clusterClient, lg, lastReplicaNodeName)
			if err != nil {
				lg.Error(err, fmt.Sprintf("failed to remove node exclusion for %s", lastReplicaNodeName))
			}
		}
		if isMaster {
			r.clearVotingExclusionsAfterRemoval(clusterClient, lastReplicaNodeName)
		}
		return result, err
	}

	sts.Spec.Replicas = &workingOrdinal
	result, err := r.client.ReconcileResource(&sts, reconciler.StatePresent)
	if err != nil {
		return result, err
	}

	if r.instance.Spec.ConfMgmt.SmartScaler {
		_, err = services.RemoveExcludeNodeHost(clusterClient, lg, lastReplicaNodeName)
		if err != nil {
			lg.Error(err, fmt.Sprintf("failed to remove node exclusion for %s", lastReplicaNodeName))
		}
	}
	if isMaster {
		r.clearVotingExclusionsAfterRemoval(clusterClient, lastReplicaNodeName)
	}
	r.recorder.AnnotatedEventf(r.instance, annotations, "Normal", "Scaler", "Finished scaling")
	return result, err
}

func (r *ScalerReconciler) recordDrainWait(currentStatus opensearchv1.ComponentStatus, nodeName, nodePoolGroupName string, annotations map[string]string) error {
	lg := log.FromContext(r.ctx)
	startedAt, ok := drainStartedAt(currentStatus.Conditions)
	if !ok {
		startedAt = time.Now().UTC()
	}
	stalled := drainHasStalled(startedAt, time.Now().UTC(), drainStallWarningAfter)
	alreadyStalled := hasDrainStalledCondition(currentStatus.Conditions)
	if stalled && !alreadyStalled {
		r.recorder.AnnotatedEventf(r.instance, annotations, "Warning", "Scaler",
			"Drain of node %s in group %s has made no progress for %s; check remaining capacity, disk watermarks, or replica settings",
			nodeName, nodePoolGroupName, drainStallWarningAfter)
	}
	newConditions := drainConditions(nodeName, startedAt, stalled || alreadyStalled)
	if drainConditionsEqual(currentStatus.Conditions, newConditions) {
		return nil
	}
	componentStatus := opensearchv1.ComponentStatus{
		Component:   "Scaler",
		Status:      "Excluded",
		Description: nodePoolGroupName,
		Conditions:  newConditions,
	}
	err := r.client.UpdateOpenSearchClusterStatus(client.ObjectKeyFromObject(r.instance), func(instance *opensearchv1.OpenSearchCluster) {
		instance.Status.ComponentsStatus = helpers.Replace(currentStatus, componentStatus, instance.Status.ComponentsStatus)
	})
	if err != nil {
		lg.Error(err, "failed to update drain wait status")
		return err
	}
	return nil
}

func scalerTargetNodeName(conditions []string) string {
	if len(conditions) == 0 {
		return ""
	}
	return conditions[0]
}

func drainStartedAt(conditions []string) (time.Time, bool) {
	for _, condition := range conditions {
		if strings.HasPrefix(condition, drainStartedConditionPrefix) {
			startedAt, err := time.Parse(time.RFC3339, strings.TrimPrefix(condition, drainStartedConditionPrefix))
			if err == nil {
				return startedAt, true
			}
		}
	}
	return time.Time{}, false
}

func hasDrainStalledCondition(conditions []string) bool {
	for _, condition := range conditions {
		if condition == drainStalledCondition {
			return true
		}
	}
	return false
}

func drainConditions(nodeName string, startedAt time.Time, stalled bool) []string {
	conditions := []string{nodeName}
	if !startedAt.IsZero() {
		conditions = append(conditions, drainStartedConditionPrefix+startedAt.UTC().Format(time.RFC3339))
	}
	if stalled {
		conditions = append(conditions, drainStalledCondition)
	}
	return conditions
}

func drainHasStalled(startedAt, now time.Time, threshold time.Duration) bool {
	if startedAt.IsZero() || threshold <= 0 {
		return false
	}
	return !now.Before(startedAt.Add(threshold))
}

func drainConditionsEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// clearVotingExclusionsAfterRemoval is the best-effort clear used where no
// component status tracks the removal (removeStatefulSet): it only succeeds
// once the node has left the cluster, and otherwise leaves the exclusion to
// sweepVotingConfigExclusions on a later pass.
func (r *ScalerReconciler) clearVotingExclusionsAfterRemoval(clusterClient *services.OsClusterClient, nodeName string) {
	lg := log.FromContext(r.ctx)
	cleared, err := services.ClearVotingConfigExclusionsIfNodeGone(clusterClient, lg, nodeName)
	if err != nil {
		lg.Error(err, fmt.Sprintf("failed to clear voting config exclusions after removing %s; will retry", nodeName))
		return
	}
	if !cleared {
		lg.Info(fmt.Sprintf("Voting config exclusion for %s is cleared once the node has left the cluster", nodeName))
	}
}

// sweepVotingConfigExclusions clears voting-config exclusions that no in-flight
// master removal owns any more. Two cases are distinguished:
//
//   - every excluded node has left the cluster: the removal completed but its
//     clear was never issued or failed (removeStatefulSet, the bootstrap pod, an
//     operator restart). Clear with wait_for_removal=true, which returns at once.
//   - every excluded node is a live member of a pool in the spec that nothing is
//     removing (a failure after the POST, a pool re-added mid-removal). Such a
//     node is silently out of the voting configuration and a later restart of any
//     other master can lose quorum. Clear with wait_for_removal=false so it votes
//     again, and record a Warning.
//
// Anything in between (some excluded node still leaving) is left alone: clearing
// without wait while a voter is terminating can put it back into the voting
// configuration right before it dies. Errors are logged, never returned, so a
// failed sweep cannot block the reconcile chain.
func (r *ScalerReconciler) sweepVotingConfigExclusions() {
	if !r.instance.Status.Initialized {
		return
	}
	lg := log.FromContext(r.ctx)
	clusterClient, err := util.CreateClientForCluster(r.client, r.ctx, r.instance, r.osClientTransport)
	if err != nil {
		lg.V(1).Info("skipping voting config exclusion sweep: cannot create OpenSearch client", "error", err.Error())
		return
	}
	excluded, err := services.GetVotingConfigExclusions(clusterClient)
	if err != nil {
		lg.Error(err, "failed to read voting config exclusions")
		return
	}
	if len(excluded) == 0 {
		return
	}
	nodes, err := clusterClient.CatNodes()
	if err != nil {
		lg.Error(err, "failed to list cluster nodes for voting config exclusion sweep")
		return
	}
	members := make(map[string]bool, len(nodes))
	for _, n := range nodes {
		members[n.Name] = true
	}

	var present, dangling []string
	for _, name := range excluded {
		if !members[name] {
			continue
		}
		present = append(present, name)
		isDangling, err := r.isDanglingVotingExclusion(name)
		if err != nil {
			lg.Error(err, "failed to inspect excluded node for voting config exclusion sweep", "node", name)
			return
		}
		if isDangling {
			dangling = append(dangling, name)
		}
	}

	if len(present) == 0 {
		lg.Info("Clearing voting config exclusions left by a completed master removal", "nodes", strings.Join(excluded, ","))
		if err := services.ClearVotingConfigExclusions(clusterClient, lg); err != nil {
			lg.Error(err, "failed to clear leftover voting config exclusions; will retry")
		}
		return
	}
	if len(dangling) != len(present) {
		// A removal is still in flight; its own path (or a later sweep) clears.
		return
	}
	annotations := map[string]string{"cluster-name": r.instance.GetName()}
	r.recorder.AnnotatedEventf(r.instance, annotations, "Warning", "Scaler",
		"Clearing stale voting config exclusion for live node(s) %s so they can vote again", strings.Join(dangling, ","))
	lg.Info("Clearing stale voting config exclusions on live master-eligible nodes", "nodes", strings.Join(dangling, ","))
	if err := clusterClient.ClearVotingConfigExclusions(r.ctx, false); err != nil {
		lg.Error(err, "failed to clear stale voting config exclusions; will retry")
	}
}

// isDanglingVotingExclusion reports whether an excluded node that is still a
// cluster member is one nothing is removing: its pod exists, is not
// terminating, belongs to a node pool in the spec within that pool's current
// replica count, and no Scaler status targets it. The bootstrap pod is owned by
// the cluster reconciler while it exists.
func (r *ScalerReconciler) isDanglingVotingExclusion(nodeName string) (bool, error) {
	if nodeName == builders.BootstrapPodName(r.instance) {
		return false, nil
	}
	for _, cs := range r.instance.Status.ComponentsStatus {
		if cs.Component == "Scaler" && (cs.Status == "Excluded" || cs.Status == "Drained") && scalerTargetNodeName(cs.Conditions) == nodeName {
			return false, nil
		}
	}
	pod, err := r.client.GetPod(nodeName, r.instance.Namespace)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	if pod.DeletionTimestamp != nil {
		return false, nil
	}
	for i := range r.instance.Spec.NodePools {
		nodePool := r.instance.Spec.NodePools[i]
		if !helpers.HasManagerRole(&nodePool) {
			continue
		}
		sts, err := r.client.GetStatefulSet(builders.StsName(r.instance, &nodePool), r.instance.Namespace)
		if err != nil {
			if k8serrors.IsNotFound(err) {
				continue
			}
			return false, err
		}
		for ord := int32(0); ord < ptr.Deref(sts.Spec.Replicas, 1); ord++ {
			if helpers.ReplicaHostName(sts, ord) == nodeName {
				return true, nil
			}
		}
	}
	return false, nil
}
