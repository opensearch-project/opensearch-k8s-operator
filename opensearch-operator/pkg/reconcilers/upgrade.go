package reconcilers

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Masterminds/semver"
	opsterv1 "github.com/Opster/opensearch-k8s-operator/opensearch-operator/api/v1"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/opensearch-gateway/services"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/pkg/builders"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/pkg/helpers"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/pkg/reconcilers/k8s"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/pkg/reconcilers/util"
	"github.com/cisco-open/operator-tools/pkg/reconciler"
	"github.com/go-logr/logr"
	"github.com/samber/lo"
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

type UpgradeReconciler struct {
	client            k8s.K8sClient
	ctx               context.Context
	osClient          *services.OsClusterClient
	recorder          record.EventRecorder
	reconcilerContext *ReconcilerContext
	instance          *opsterv1.OpenSearchCluster
	logger            logr.Logger
}

func NewUpgradeReconciler(
	client client.Client,
	ctx context.Context,
	recorder record.EventRecorder,
	reconcilerContext *ReconcilerContext,
	instance *opsterv1.OpenSearchCluster,
	opts ...reconciler.ResourceReconcilerOption,
) *UpgradeReconciler {
	return &UpgradeReconciler{
		client:            k8s.NewK8sClient(client, ctx, append(opts, reconciler.WithLog(log.FromContext(ctx).WithValues("reconciler", "upgrade")))...),
		ctx:               ctx,
		recorder:          recorder,
		reconcilerContext: reconcilerContext,
		instance:          instance,
		logger:            log.FromContext(ctx).WithValues("reconciler", "upgrade"),
	}
}

func (r *UpgradeReconciler) Reconcile() (ctrl.Result, error) {
	// If versions are in sync do nothing
	if r.instance.Spec.General.Version == r.instance.Status.Version {
		return ctrl.Result{}, nil
	}

	// Skip an upgrade if the cluster hasn't finished initializing
	if !r.instance.Status.Initialized {
		return ctrl.Result{
			Requeue:      true,
			RequeueAfter: 10 * time.Second,
		}, nil
	}

	annotations := map[string]string{"cluster-name": r.instance.GetName()}

	// If version validation fails log a warning and do nothing
	if err := r.validateUpgrade(); err != nil {
		r.logger.V(1).Error(err, "version validation failed", "currentVersion", r.instance.Status.Version, "requestedVersion", r.instance.Spec.General.Version)
		r.recorder.AnnotatedEventf(r.instance, annotations, "Normal", "Upgrade", "Failed to validation version, currentVersion: %s , requestedVersion: %s", r.instance.Status.Version, r.instance.Spec.General.Version)
		return ctrl.Result{}, err
	}

	var err error

	r.osClient, err = util.CreateClientForCluster(r.client, r.ctx, r.instance, nil)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Fetch the working nodepool
	nodePool, currentStatus := r.findNextNodePoolForUpgrade()

	// Work on the current nodepool as appropriate
	switch currentStatus.Status {
	case "Pending":
		// Set it to upgrading and requeue
		err := r.client.UpdateOpenSearchClusterStatus(client.ObjectKeyFromObject(r.instance), func(instance *opsterv1.OpenSearchCluster) {
			currentStatus.Status = "Upgrading"
			instance.Status.ComponentsStatus = append(instance.Status.ComponentsStatus, currentStatus)
		})
		r.recorder.AnnotatedEventf(r.instance, annotations, "Normal", "Upgrade", "Starting upgrade of node pool '%s'", currentStatus.Description)
		return ctrl.Result{
			Requeue:      true,
			RequeueAfter: 15 * time.Second,
		}, err
	case "Upgrading":
		err := r.doNodePoolUpgrade(nodePool)
		return ctrl.Result{
			Requeue:      true,
			RequeueAfter: 30 * time.Second,
		}, err
	case "Finished":
		// Cleanup status after successful upgrade
		err := r.client.UpdateOpenSearchClusterStatus(client.ObjectKeyFromObject(r.instance), func(instance *opsterv1.OpenSearchCluster) {
			instance.Status.Version = instance.Spec.General.Version
			for _, pool := range instance.Spec.NodePools {
				componentStatus := opsterv1.ComponentStatus{
					Component:   "Upgrader",
					Description: pool.Component,
				}
				currentStatus, found := helpers.FindFirstPartial(instance.Status.ComponentsStatus, componentStatus, helpers.GetByDescriptionAndGroup)
				if found {
					instance.Status.ComponentsStatus = helpers.RemoveIt(currentStatus, instance.Status.ComponentsStatus)
				}
			}
		})
		r.recorder.AnnotatedEventf(r.instance, annotations, "Normal", "Upgrade", "Finished upgrade - NewVersion: %s", r.instance.Status.Version)
		return ctrl.Result{}, err
	default:
		// We should never get here so return an error
		return ctrl.Result{}, ErrUnexpectedStatus
	}
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
		r.recorder.AnnotatedEventf(r.instance, annotations, "Error", "Upgrade", "Invalid version: specified version is more than 1 major version greater than existing")
		return ErrVersionDowngrade
	}

	// Don't allow more than one major version upgrade
	nextMajor := existing.IncMajor().IncMajor()
	upgradeConstraint, err := semver.NewConstraint(fmt.Sprintf("< %s", nextMajor.String()))
	if err != nil {
		return err
	}

	if !upgradeConstraint.Check(new) {
		r.recorder.AnnotatedEventf(r.instance, annotations, "Error", "Upgrade", "Invalid version: specified version is more than 1 major version greater than existing")
		return ErrMajorVersionJump
	}

	return nil
}

// Find which nodepool to work on
func (r *UpgradeReconciler) findNextNodePoolForUpgrade() (opsterv1.NodePool, opsterv1.ComponentStatus) {
	// First sort node pools
	var dataNodes, dataAndMasterNodes, otherNodes []opsterv1.NodePool
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
		return pool, opsterv1.ComponentStatus{
			Component:   "Upgrader",
			Description: pool.Component,
			Status:      "Upgrading",
		}
	}
	// Pick the first unworked on node next
	pool, found = r.findNextPool(dataNodes)
	if found {
		return pool, opsterv1.ComponentStatus{
			Component:   "Upgrader",
			Description: pool.Component,
			Status:      "Pending",
		}
	}
	// Next do the same for any nodes that are data and master
	pool, found = r.findInProgress(dataAndMasterNodes)
	if found {
		return pool, opsterv1.ComponentStatus{
			Component:   "Upgrader",
			Description: pool.Component,
			Status:      "Upgrading",
		}
	}
	pool, found = r.findNextPool(dataAndMasterNodes)
	if found {
		return pool, opsterv1.ComponentStatus{
			Component:   "Upgrader",
			Description: pool.Component,
			Status:      "Pending",
		}
	}

	// Finally do the non data nodes
	pool, found = r.findInProgress(otherNodes)
	if found {
		return pool, opsterv1.ComponentStatus{
			Component:   "Upgrader",
			Description: pool.Component,
			Status:      "Upgrading",
		}
	}
	pool, found = r.findNextPool(otherNodes)
	if found {
		return pool, opsterv1.ComponentStatus{
			Component:   "Upgrader",
			Description: pool.Component,
			Status:      "Pending",
		}
	}

	// If we get here all nodes should be upgraded
	return opsterv1.NodePool{}, opsterv1.ComponentStatus{
		Component: "Upgrade",
		Status:    "Finished",
	}
}

func (r *UpgradeReconciler) findInProgress(pools []opsterv1.NodePool) (opsterv1.NodePool, bool) {
	for _, nodePool := range pools {
		componentStatus := opsterv1.ComponentStatus{
			Component:   "Upgrader",
			Description: nodePool.Component,
		}
		currentStatus, found := helpers.FindFirstPartial(r.instance.Status.ComponentsStatus, componentStatus, helpers.GetByDescriptionAndGroup)
		if found && currentStatus.Status == "Upgrading" {
			return nodePool, true
		}
	}
	return opsterv1.NodePool{}, false
}

func (r *UpgradeReconciler) findNextPool(pools []opsterv1.NodePool) (opsterv1.NodePool, bool) {
	for _, nodePool := range pools {
		componentStatus := opsterv1.ComponentStatus{
			Component:   "Upgrader",
			Description: nodePool.Component,
		}
		_, found := helpers.FindFirstPartial(r.instance.Status.ComponentsStatus, componentStatus, helpers.GetByDescriptionAndGroup)
		if !found {
			return nodePool, true
		}
	}
	return opsterv1.NodePool{}, false
}

func (r *UpgradeReconciler) doNodePoolUpgrade(pool opsterv1.NodePool) error {
	var conditions []string
	annotations := map[string]string{"cluster-name": r.instance.GetName()}
	// Fetch the STS
	stsName := builders.StsName(r.instance, &pool)
	sts, err := r.client.GetStatefulSet(stsName, r.instance.Namespace)
	if err != nil {
		return err
	}

	dataCount := util.DataNodesCount(r.client, r.instance)
	if dataCount == 2 && r.instance.Spec.General.DrainDataNodes {
		r.logger.Info("Only 2 data nodes and drain is set, some shards may not drain")
	}

	if sts.Status.ReadyReplicas < lo.FromPtrOr(sts.Spec.Replicas, 1) {
		r.logger.Info("Waiting for all pods to be ready")
		conditions = append(conditions, "Waiting for all pods to be ready")
		r.setComponentConditions(conditions, pool.Component)
		return nil
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
			return err
		}
		r.recorder.AnnotatedEventf(r.instance, annotations, "Normal", "Upgrade", "Finished upgrade of node pool '%s'", pool.Component)

		return r.client.UpdateOpenSearchClusterStatus(client.ObjectKeyFromObject(r.instance), func(instance *opsterv1.OpenSearchCluster) {
			currentStatus := opsterv1.ComponentStatus{
				Component:   "Upgrader",
				Status:      "Upgrading",
				Description: pool.Component,
			}
			componentStatus := opsterv1.ComponentStatus{
				Component:   "Upgrader",
				Status:      "Upgraded",
				Description: pool.Component,
			}
			instance.Status.ComponentsStatus = helpers.Replace(currentStatus, componentStatus, instance.Status.ComponentsStatus)
		})
	}

	workingPod, err := helpers.WorkingPodForRollingRestart(r.client, &sts)
	if err != nil {
		conditions = append(conditions, "Could not find working pod")
		r.setComponentConditions(conditions, pool.Component)
		return err
	}

	ready, err = services.PreparePodForDelete(r.osClient, r.logger, workingPod, r.instance.Spec.General.DrainDataNodes, dataCount)
	if err != nil {
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
		conditions = append(conditions, "Could not delete pod")
		r.setComponentConditions(conditions, pool.Component)
		return err
	}

	conditions = append(conditions, fmt.Sprintf("Deleted pod %s", workingPod))
	r.setComponentConditions(conditions, pool.Component)

	// If we are draining nodes remove the exclusion after the pod is deleted
	if r.instance.Spec.General.DrainDataNodes {
		_, err = services.RemoveExcludeNodeHost(r.osClient, workingPod)
		return err
	}

	return nil
}

func (r *UpgradeReconciler) setComponentConditions(conditions []string, component string) {

	err := r.client.UpdateOpenSearchClusterStatus(client.ObjectKeyFromObject(r.instance), func(instance *opsterv1.OpenSearchCluster) {
		currentStatus := opsterv1.ComponentStatus{
			Component:   "Upgrader",
			Status:      "Upgrading",
			Description: component,
		}
		componentStatus, found := helpers.FindFirstPartial(instance.Status.ComponentsStatus, currentStatus, helpers.GetByDescriptionAndGroup)
		newStatus := opsterv1.ComponentStatus{
			Component:   "Upgrader",
			Status:      "Upgrading",
			Description: component,
			Conditions:  conditions,
		}
		if found {
			conditions = append(componentStatus.Conditions, conditions...)
		}

		instance.Status.ComponentsStatus = helpers.Replace(currentStatus, newStatus, instance.Status.ComponentsStatus)
	})
	if err != nil {
		r.logger.Error(err, "Could not update status")
	}
}
