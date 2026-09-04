package reconcilers

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Masterminds/semver"
	"github.com/go-logr/logr"
	opensearchv1 "github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/api/opensearch.org/v1"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/opensearch-gateway/services"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/builders"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/helpers"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/reconciler"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/reconcilers/k8s"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/reconcilers/util"
	"github.com/samber/lo"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var (
	ErrVersionDowngrade = errors.New("version requested is downgrade")
	ErrMajorVersionJump = errors.New("version request is more than 1 major version ahead")
	ErrUnexpectedStatus = errors.New("unexpected upgrade status")
)

const (
	componentNameUpgrader    = "Upgrader"
	upgradeStatusPending     = "Pending"
	upgradeStatusInProgress  = "Upgrading"
	upgradeStatusFinished    = "Finished"
	upgradeTargetDescription = "__upgrade_target__"
)

const upgradeReconcilerName = "upgrade"

type UpgradeReconciler struct {
	client            k8s.K8sClient
	ctx               context.Context
	osClient          *services.OsClusterClient
	recorder          record.EventRecorder
	reconcilerContext *ReconcilerContext
	instance          *opensearchv1.OpenSearchCluster
	logger            logr.Logger
}

func NewUpgradeReconciler(
	client client.Client,
	ctx context.Context,
	recorder record.EventRecorder,
	reconcilerContext *ReconcilerContext,
	instance *opensearchv1.OpenSearchCluster,
	opts ...reconciler.ResourceReconcilerOption,
) *UpgradeReconciler {
	return &UpgradeReconciler{
		client:            k8s.NewK8sClient(client, ctx, append(opts, reconciler.WithLog(log.FromContext(ctx).WithValues("reconciler", upgradeReconcilerName)))...),
		ctx:               ctx,
		recorder:          recorder,
		reconcilerContext: reconcilerContext,
		instance:          instance,
		logger:            log.FromContext(ctx).WithValues("reconciler", upgradeReconcilerName),
	}
}

func (r *UpgradeReconciler) Name() string { return upgradeReconcilerName }

func (r *UpgradeReconciler) Reconcile() (ctrl.Result, error) {
	annotations := map[string]string{"cluster-name": r.instance.GetName()}

	// If versions are in sync do nothing ΓÇö but always clear leftover Upgrader bookkeeping so an
	// aborted/reverted upgrade cannot skip pools on the next upgrade (issue #1453).
	if r.instance.Spec.General.Version == r.instance.Status.Version {
		if r.instance.Status.Phase == opensearchv1.PhaseUpgrading || r.hasUpgraderStatuses() {
			err := r.client.UpdateOpenSearchClusterStatus(client.ObjectKeyFromObject(r.instance), func(instance *opensearchv1.OpenSearchCluster) {
				instance.Status.Phase = opensearchv1.PhaseRunning
				instance.Status.ComponentsStatus = helpers.ClearUpgraderComponentStatuses(instance.Status.ComponentsStatus)
			})
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// Skip an upgrade if the cluster hasn't finished initializing
	if !r.instance.Status.Initialized {
		return ctrl.Result{
			Requeue:      true,
			RequeueAfter: 10 * time.Second,
		}, nil
	}

	// A pinned custom image ignores spec.general.version for the pod template. Bumping version
	// alone would otherwise look like an instant successful upgrade with no pods restarted.
	if helpers.HasPinnedCustomImage(r.instance) {
		r.logger.Info("Skipping version upgrade because a custom image is pinned",
			"image", helpers.PinnedCustomImage(r.instance),
			"requestedVersion", r.instance.Spec.General.Version,
			"statusVersion", r.instance.Status.Version)
		r.recorder.AnnotatedEventf(r.instance, annotations, "Warning", "Upgrade",
			"spec.general.version changed to %s but a custom image is pinned (%s); pods are not upgraded by version changes. Update imageSpec.image (or remove it) to change the running image",
			r.instance.Spec.General.Version, helpers.PinnedCustomImage(r.instance))
		err := r.client.UpdateOpenSearchClusterStatus(client.ObjectKeyFromObject(r.instance), func(instance *opensearchv1.OpenSearchCluster) {
			instance.Status.Version = instance.Spec.General.Version
			instance.Status.Phase = opensearchv1.PhaseRunning
			instance.Status.ComponentsStatus = helpers.ClearUpgraderComponentStatuses(instance.Status.ComponentsStatus)
		})
		return ctrl.Result{}, err
	}

	// If version validation fails log a warning and do nothing. Return a
	// terminal error so the main chain can continue (restart, snapshots, etc.)
	// instead of freezing all maintenance on a permanent spec mistake.
	if err := r.validateUpgrade(); err != nil {
		r.logger.V(1).Error(err, "version validation failed", "currentVersion", r.instance.Status.Version, "requestedVersion", r.instance.Spec.General.Version)
		r.recorder.AnnotatedEventf(r.instance, annotations, "Warning", "Upgrade", "Failed to validate version, currentVersion: %s , requestedVersion: %s", r.instance.Status.Version, r.instance.Spec.General.Version)
		return ctrl.Result{}, AsTerminal(err)
	}

	// Reset per-pool progress when the upgrade target changes mid-flight (or after an abort that
	// left stale Upgraded entries while versions still differ).
	if err := r.ensureUpgradeTarget(); err != nil {
		r.logger.Error(err, "Could not update upgrade target status")
		return ctrl.Result{}, err
	}

	// Drop Upgrader entries for pools that were removed from the spec so they cannot permanently
	// leave IsUpgradeInProgress stuck true.
	if err := r.cleanOrphanedUpgraderStatuses(); err != nil {
		r.logger.Error(err, "Could not clean orphaned upgrader statuses")
		return ctrl.Result{}, err
	}

	// Set phase to UPGRADING if not already set
	if r.instance.Status.Phase != opensearchv1.PhaseUpgrading {
		err := r.client.UpdateOpenSearchClusterStatus(client.ObjectKeyFromObject(r.instance), func(instance *opensearchv1.OpenSearchCluster) {
			instance.Status.Phase = opensearchv1.PhaseUpgrading
		})
		if err != nil {
			r.logger.Error(err, "Could not update status")
			return ctrl.Result{}, err
		}
	}

	var err error

	r.osClient, err = util.CreateClientForCluster(r.client, r.ctx, r.instance, nil)
	if err != nil {
		r.logger.Error(err, "Could not create client for cluster")
		return ctrl.Result{}, err
	}

	// Clean stale allocation exclusions so a previously failed RemoveExcludeNodeHost (e.g. after DeletePod) gets retried.
	if r.instance.Spec.General.DrainDataNodes {
		if res, err := util.CleanStaleExclusionList(r.client, r.instance, r.osClient, r.logger); err != nil || res.Requeue {
			if err != nil {
				return ctrl.Result{}, err
			}
			return res, nil
		}
	}

	// Start the nodepool upgrade loop

	// Fetch the working nodepool
	nodePool, componentStatus := r.findNextNodePoolForUpgrade()

	// Work on the current nodepool as appropriate
	switch componentStatus.Status {
	case upgradeStatusPending:
		// Set it to upgrading and requeue
		err := r.client.UpdateOpenSearchClusterStatus(client.ObjectKeyFromObject(r.instance), func(instance *opensearchv1.OpenSearchCluster) {
			componentStatus.Status = upgradeStatusInProgress
			instance.Status.ComponentsStatus = append(instance.Status.ComponentsStatus, componentStatus)
		})
		r.recorder.AnnotatedEventf(r.instance, annotations, "Normal", "Upgrade", "Starting upgrade of node pool '%s'", componentStatus.Description)
		return ctrl.Result{
			Requeue:      true,
			RequeueAfter: 15 * time.Second,
		}, err
	case upgradeStatusInProgress:
		err := r.doNodePoolUpgrade(nodePool)
		return ctrl.Result{
			Requeue:      true,
			RequeueAfter: 30 * time.Second,
		}, err
	case upgradeStatusFinished:
		// Cleanup status after successful upgrade
		err := r.client.UpdateOpenSearchClusterStatus(client.ObjectKeyFromObject(r.instance), func(instance *opensearchv1.OpenSearchCluster) {
			instance.Status.Version = instance.Spec.General.Version
			instance.Status.Phase = opensearchv1.PhaseRunning
			instance.Status.ComponentsStatus = helpers.ClearUpgraderComponentStatuses(instance.Status.ComponentsStatus)
		})
		r.recorder.AnnotatedEventf(r.instance, annotations, "Normal", "Upgrade", "Finished upgrade - NewVersion: %s", r.instance.Spec.General.Version)
		return ctrl.Result{}, err
	default:
		// We should never get here so return an error
		return ctrl.Result{}, ErrUnexpectedStatus
	}
}

func (r *UpgradeReconciler) hasUpgraderStatuses() bool {
	for _, status := range r.instance.Status.ComponentsStatus {
		if status.Component == componentNameUpgrader {
			return true
		}
	}
	return false
}

// ensureUpgradeTarget records the version currently being upgraded to. If the target changes
// (or no target is recorded yet while Upgrader entries exist), clear per-pool progress so pools
// are not skipped.
func (r *UpgradeReconciler) ensureUpgradeTarget() error {
	target := opensearchv1.ComponentStatus{
		Component:   componentNameUpgrader,
		Description: upgradeTargetDescription,
	}
	current, found := helpers.FindFirstPartial(r.instance.Status.ComponentsStatus, target, helpers.GetByDescriptionAndComponent)
	desiredVersion := r.instance.Spec.General.Version

	if found && current.Status == desiredVersion {
		return nil
	}

	r.logger.Info("Resetting upgrade progress for new target version",
		"previousTarget", current.Status,
		"newTarget", desiredVersion,
		"hadPreviousTarget", found)

	targetStatus := opensearchv1.ComponentStatus{
		Component:   componentNameUpgrader,
		Description: upgradeTargetDescription,
		Status:      desiredVersion,
	}
	err := r.client.UpdateOpenSearchClusterStatus(client.ObjectKeyFromObject(r.instance), func(instance *opensearchv1.OpenSearchCluster) {
		instance.Status.ComponentsStatus = helpers.ClearUpgraderComponentStatuses(instance.Status.ComponentsStatus)
		instance.Status.ComponentsStatus = append(instance.Status.ComponentsStatus, targetStatus)
	})
	if err != nil {
		return err
	}
	// Keep the local copy in sync so pool selection in this reconcile does not use stale entries.
	r.instance.Status.ComponentsStatus = helpers.ClearUpgraderComponentStatuses(r.instance.Status.ComponentsStatus)
	r.instance.Status.ComponentsStatus = append(r.instance.Status.ComponentsStatus, targetStatus)
	return nil
}

func (r *UpgradeReconciler) cleanOrphanedUpgraderStatuses() error {
	validPools := make(map[string]struct{}, len(r.instance.Spec.NodePools)+1)
	for _, pool := range r.instance.Spec.NodePools {
		validPools[pool.Component] = struct{}{}
	}
	validPools[upgradeTargetDescription] = struct{}{}

	filtered := make([]opensearchv1.ComponentStatus, 0, len(r.instance.Status.ComponentsStatus))
	removed := false
	for _, status := range r.instance.Status.ComponentsStatus {
		if status.Component == componentNameUpgrader {
			if _, ok := validPools[status.Description]; !ok {
				removed = true
				continue
			}
		}
		filtered = append(filtered, status)
	}
	if !removed {
		return nil
	}

	err := r.client.UpdateOpenSearchClusterStatus(client.ObjectKeyFromObject(r.instance), func(instance *opensearchv1.OpenSearchCluster) {
		kept := make([]opensearchv1.ComponentStatus, 0, len(instance.Status.ComponentsStatus))
		for _, status := range instance.Status.ComponentsStatus {
			if status.Component == componentNameUpgrader {
				if _, ok := validPools[status.Description]; !ok {
					continue
				}
			}
			kept = append(kept, status)
		}
		instance.Status.ComponentsStatus = kept
	})
	if err != nil {
		return err
	}
	r.instance.Status.ComponentsStatus = filtered
	return nil
}

// Currently provides basic validation on versions.
// TODO Improve the validation (maybe allow patch version downgrades)
func (r *UpgradeReconciler) validateUpgrade() error {
	// Parse versions
	existing, err := semver.NewVersion(r.instance.Status.Version)
	if err != nil {
		return err
	}

	new, err := semver.NewVersion(r.instance.Spec.General.Version)
	if err != nil {
		return err
	}
	annotations := map[string]string{"cluster-name": r.instance.GetName()}

	// Don't allow version downgrades as they might cause unexpected issues
	if new.LessThan(existing) {
		r.recorder.AnnotatedEventf(r.instance, annotations, "Warning", "Upgrade", "Invalid version: specified version is a downgrade")
		return ErrVersionDowngrade
	}

	// Don't allow more than one major version upgrade
	nextMajor := existing.IncMajor().IncMajor()
	upgradeConstraint, err := semver.NewConstraint(fmt.Sprintf("< %s", nextMajor.String()))
	if err != nil {
		return err
	}

	if !upgradeConstraint.Check(new) {
		r.recorder.AnnotatedEventf(r.instance, annotations, "Warning", "Upgrade", "Invalid version: specified version is more than 1 major version greater than existing")
		return ErrMajorVersionJump
	}

	return nil
}

// Find which nodepool to work on
func (r *UpgradeReconciler) findNextNodePoolForUpgrade() (opensearchv1.NodePool, opensearchv1.ComponentStatus) {
	// First sort node pools
	var dataNodes, dataAndMasterNodes, otherNodes []opensearchv1.NodePool
	for _, nodePool := range r.instance.Spec.NodePools {
		if helpers.HasDataRole(&nodePool) {
			if helpers.HasManagerRole(&nodePool) {
				dataAndMasterNodes = append(dataAndMasterNodes, nodePool)
			} else {
				dataNodes = append(dataNodes, nodePool)
			}
		} else {
			otherNodes = append(otherNodes, nodePool)
		}
	}

	// First work on data only nodes
	// Complete the in progress node first
	pool, found := r.findInProgress(dataNodes)
	if found {
		return pool, opensearchv1.ComponentStatus{
			Component:   componentNameUpgrader,
			Description: pool.Component,
			Status:      upgradeStatusInProgress,
		}
	}
	// Pick the first unworked on node next
	pool, found = r.findNextPool(dataNodes)
	if found {
		return pool, opensearchv1.ComponentStatus{
			Component:   componentNameUpgrader,
			Description: pool.Component,
			Status:      upgradeStatusPending,
		}
	}
	// Next do the same for any nodes that are data and master
	pool, found = r.findInProgress(dataAndMasterNodes)
	if found {
		return pool, opensearchv1.ComponentStatus{
			Component:   componentNameUpgrader,
			Description: pool.Component,
			Status:      upgradeStatusInProgress,
		}
	}
	pool, found = r.findNextPool(dataAndMasterNodes)
	if found {
		return pool, opensearchv1.ComponentStatus{
			Component:   componentNameUpgrader,
			Description: pool.Component,
			Status:      upgradeStatusPending,
		}
	}

	// Finally do the non data nodes
	pool, found = r.findInProgress(otherNodes)
	if found {
		return pool, opensearchv1.ComponentStatus{
			Component:   componentNameUpgrader,
			Description: pool.Component,
			Status:      upgradeStatusInProgress,
		}
	}
	pool, found = r.findNextPool(otherNodes)
	if found {
		return pool, opensearchv1.ComponentStatus{
			Component:   componentNameUpgrader,
			Description: pool.Component,
			Status:      upgradeStatusPending,
		}
	}

	// If we get here all nodes should be upgraded
	return opensearchv1.NodePool{}, opensearchv1.ComponentStatus{
		Component: componentNameUpgrader,
		Status:    upgradeStatusFinished,
	}
}

func (r *UpgradeReconciler) findInProgress(pools []opensearchv1.NodePool) (opensearchv1.NodePool, bool) {
	for _, nodePool := range pools {
		componentStatus := opensearchv1.ComponentStatus{
			Component:   componentNameUpgrader,
			Description: nodePool.Component,
		}
		currentStatus, found := helpers.FindFirstPartial(r.instance.Status.ComponentsStatus, componentStatus, helpers.GetByDescriptionAndComponent)
		if found && currentStatus.Status == upgradeStatusInProgress {
			return nodePool, true
		}
	}
	return opensearchv1.NodePool{}, false
}

func (r *UpgradeReconciler) findNextPool(pools []opensearchv1.NodePool) (opensearchv1.NodePool, bool) {
	for _, nodePool := range pools {
		componentStatus := opensearchv1.ComponentStatus{
			Component:   componentNameUpgrader,
			Description: nodePool.Component,
		}
		_, found := helpers.FindFirstPartial(r.instance.Status.ComponentsStatus, componentStatus, helpers.GetByDescriptionAndComponent)
		if !found {
			return nodePool, true
		}
	}
	return opensearchv1.NodePool{}, false
}

func (r *UpgradeReconciler) doNodePoolUpgrade(pool opensearchv1.NodePool) error {
	var conditions []string
	annotations := map[string]string{"cluster-name": r.instance.GetName()}
	// Fetch the STS
	stsName := builders.StsName(r.instance, &pool)
	sts, err := r.client.GetStatefulSet(stsName, r.instance.Namespace)
	if err != nil {
		return err
	}

	readyReplicas, err := helpers.ReadyReplicasForNodePool(r.client, r.instance, &pool)
	if err != nil {
		return err
	}
	sts.Status.ReadyReplicas = readyReplicas

	dataCount := util.DataNodesCount(r.client, r.instance)
	if dataCount == 2 && r.instance.Spec.General.DrainDataNodes {
		r.logger.Info("Only 2 data nodes and drain is set, some shards may not drain")
	}

	if sts.Status.ReadyReplicas < lo.FromPtrOr(sts.Spec.Replicas, 1) {
		r.logger.Info("Waiting for all pods to be ready")
		conditions = append(conditions, "Waiting for all pods to be ready")
		r.handleUnreadyPods(pool, &sts, annotations)
		r.setComponentConditions(conditions, pool.Component)
		return nil
	}

	// Delete deprecated settings that have been archived in the updated version
	// NOTE: This needs to be called before each pod delete, since some settings are being re-applied automatically during node restart.
	// NOTE: This can be removed if OpenSearch 2.x stops erroring on archived settings
	// See https://github.com/opensearch-project/OpenSearch/issues/18515
	err = services.DeleteUnsupportedClusterSettings(r.osClient, r.instance.Spec.General.Version)
	if err != nil {
		r.logger.Error(err, "Could not delete unsupported cluster settings")
		conditions = append(conditions, "Could not delete unsupported cluster settings")
		r.setComponentConditions(conditions, pool.Component)
		return err
	}

	ready, condition, err := services.CheckClusterStatusForRestart(r.osClient, r.instance.Spec.General.DrainDataNodes)
	if err != nil {
		r.logger.Error(err, "Could not check opensearch cluster status")
		conditions = append(conditions, "Could not check opensearch cluster status")
		r.setComponentConditions(conditions, pool.Component)
		return err
	}
	if !ready {
		r.logger.Info(fmt.Sprintf("Cluster is not ready for next pod to restart because %s", condition))
		conditions = append(conditions, condition)
		r.setComponentConditions(conditions, pool.Component)
		return nil
	}

	conditions = append(conditions, "preparing for pod delete")

	// Work around for https://github.com/kubernetes/kubernetes/issues/73492
	// If upgrade on this node pool is complete update status and return
	if sts.Status.UpdatedReplicas == lo.FromPtrOr(sts.Spec.Replicas, 1) {
		if err = services.ReactivateShardAllocation(r.osClient); err != nil {
			r.logger.Error(err, "Could not reactivate shard allocation")
			return err
		}
		r.recorder.AnnotatedEventf(r.instance, annotations, "Normal", "Upgrade", "Finished upgrade of node pool '%s'", pool.Component)

		return r.client.UpdateOpenSearchClusterStatus(client.ObjectKeyFromObject(r.instance), func(instance *opensearchv1.OpenSearchCluster) {
			currentStatus := opensearchv1.ComponentStatus{
				Component:   componentNameUpgrader,
				Status:      upgradeStatusInProgress,
				Description: pool.Component,
			}
			componentStatus := opensearchv1.ComponentStatus{
				Component:   componentNameUpgrader,
				Status:      "Upgraded",
				Description: pool.Component,
			}
			instance.Status.ComponentsStatus = helpers.Replace(currentStatus, componentStatus, instance.Status.ComponentsStatus)
		})
	}

	workingPod, err := helpers.WorkingPodForRollingRestart(r.client, &sts)
	if err != nil {
		r.logger.Error(err, "Could not find working pod")
		conditions = append(conditions, "Could not find working pod")
		r.setComponentConditions(conditions, pool.Component)
		return err
	}

	ready, err = services.PreparePodForDelete(r.osClient, r.logger, workingPod, r.instance.Spec.General.DrainDataNodes, dataCount)
	if err != nil {
		r.logger.Error(err, "Could not prepare pod for delete")
		conditions = append(conditions, "Could not prepare pod for delete")
		r.setComponentConditions(conditions, pool.Component)
		return err
	}
	if !ready {
		conditions = append(conditions, "Waiting for node to drain")
		r.setComponentConditions(conditions, pool.Component)
		return nil
	}

	err = r.client.DeletePod(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      workingPod,
			Namespace: sts.Namespace,
		},
	})
	if err != nil {
		r.logger.Error(err, "Could not delete pod")
		conditions = append(conditions, "Could not delete pod")
		r.setComponentConditions(conditions, pool.Component)
		return err
	}

	conditions = append(conditions, fmt.Sprintf("Deleted pod %s", workingPod))
	r.setComponentConditions(conditions, pool.Component)

	// If we are draining nodes remove the exclusion after the pod is deleted
	if r.instance.Spec.General.DrainDataNodes {
		_, err = services.RemoveExcludeNodeHost(r.osClient, r.logger, workingPod)
		return err
	}

	return nil
}

// handleUnreadyPods tries to unblock upgrades stalled by stuck (CrashLoopBackOff/ImagePullBackOff/...) pods and emits Warning events.
func (r *UpgradeReconciler) handleUnreadyPods(pool opensearchv1.NodePool, sts *appsv1.StatefulSet, annotations map[string]string) {
	deletedPod, err := helpers.DeleteStuckPodWithOlderRevision(r.client, sts)
	if err != nil {
		r.logger.Error(err, "Could not delete stuck pod with older revision", "pool", pool.Component)
	} else if deletedPod != "" {
		r.logger.Info("Deleted stuck pod with older revision", "pod", deletedPod, "pool", pool.Component)
		r.recorder.AnnotatedEventf(r.instance, annotations, "Warning", "Upgrade",
			"Deleted stuck pod '%s' in node pool '%s' to allow the upgrade to proceed", deletedPod, pool.Component)
	}

	stuckPods, err := helpers.StuckPods(r.client, sts)
	if err != nil {
		r.logger.Error(err, "Could not list stuck pods", "pool", pool.Component)
		return
	}
	for podName, reason := range stuckPods {
		if podName == deletedPod {
			continue
		}
		r.logger.Info("Upgrade stalled by stuck pod", "pod", podName, "reason", reason, "pool", pool.Component)
		r.recorder.AnnotatedEventf(r.instance, annotations, "Warning", "Upgrade",
			"Pod '%s' in node pool '%s' is in %s; upgrade is stalled until the pod becomes ready", podName, pool.Component, reason)
	}
}

func (r *UpgradeReconciler) setComponentConditions(conditions []string, component string) {

	err := r.client.UpdateOpenSearchClusterStatus(client.ObjectKeyFromObject(r.instance), func(instance *opensearchv1.OpenSearchCluster) {
		currentStatus := opensearchv1.ComponentStatus{
			Component:   componentNameUpgrader,
			Status:      upgradeStatusInProgress,
			Description: component,
		}
		componentStatus, found := helpers.FindFirstPartial(instance.Status.ComponentsStatus, currentStatus, helpers.GetByDescriptionAndComponent)
		newStatus := opensearchv1.ComponentStatus{
			Component:   componentNameUpgrader,
			Status:      upgradeStatusInProgress,
			Description: component,
			Conditions:  conditions,
		}
		if found {
			conditions = append(componentStatus.Conditions, conditions...)
		}

		instance.Status.ComponentsStatus = helpers.Replace(componentStatus, newStatus, instance.Status.ComponentsStatus)
	})
	if err != nil {
		r.logger.Error(err, "Could not update status")
	}
}
