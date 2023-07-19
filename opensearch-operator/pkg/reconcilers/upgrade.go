package reconcilers

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Masterminds/semver"
	"github.com/cisco-open/operator-tools/pkg/reconciler"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	opsterv1 "opensearch.opster.io/api/v1"
	"opensearch.opster.io/opensearch-gateway/services"
	"opensearch.opster.io/pkg/builders"
	"opensearch.opster.io/pkg/helpers"
	"opensearch.opster.io/pkg/reconcilers/util"
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
	client.Client
	reconciler.ResourceReconciler
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
		Client: client,
		ResourceReconciler: reconciler.NewReconcilerWith(client,
			append(opts, reconciler.WithLog(log.FromContext(ctx).WithValues("reconciler", "upgrade")))...),
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
	annotations := map[string]string{"cluster-name": r.instance.GetName()}

	// If version validation fails log a warning and do nothing
	if err := r.validateUpgrade(); err != nil {
		r.logger.V(1).Error(err, "version validation failed", "currentVersion", r.instance.Status.Version, "requestedVersion", r.instance.Spec.General.Version)
		r.recorder.AnnotatedEventf(r.instance, annotations, "Normal", "Upgrade", "Failed to validation version, currentVersion: %s , requestedVersion: %s", r.instance.Status.Version, r.instance.Spec.General.Version)
		return ctrl.Result{}, err
	}

	var err error

	r.osClient, err = util.CreateClientForCluster(r.ctx, r.Client, r.instance, nil)
	if err != nil {
		return ctrl.Result{}, err
	}

	//Fetch the working nodepool
	nodePool, currentStatus := r.findNextNodePoolForUpgrade()

	// Work on the current nodepool as appropriate
	switch currentStatus.Status {
	case "Pending":
		// Set it to upgrading and requeue
		err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			if err := r.Get(r.ctx, client.ObjectKeyFromObject(r.instance), r.instance); err != nil {
				return err
			}
			currentStatus.Status = "Upgrading"
			r.instance.Status.ComponentsStatus = append(r.instance.Status.ComponentsStatus, currentStatus)
			return r.Status().Update(r.ctx, r.instance)
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
		err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			if err := r.Get(r.ctx, client.ObjectKeyFromObject(r.instance), r.instance); err != nil {
				return err
			}
			r.instance.Status.Version = r.instance.Spec.General.Version
			for _, pool := range r.instance.Spec.NodePools {
				componentStatus := opsterv1.ComponentStatus{
					Component:   "Upgrader",
					Description: pool.Component,
				}
				currentStatus, found := helpers.FindFirstPartial(r.instance.Status.ComponentsStatus, componentStatus, helpers.GetByDescriptionAndGroup)
				if found {
					r.instance.Status.ComponentsStatus = helpers.RemoveIt(currentStatus, r.instance.Status.ComponentsStatus)
				}
			}
			return r.Status().Update(r.ctx, r.instance)
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
	// Fetch the STS
	stsName := builders.StsName(r.instance, &pool)
	sts := &appsv1.StatefulSet{}
	annotations := map[string]string{"cluster-name": r.instance.GetName()}
	if err := r.Get(r.ctx, types.NamespacedName{
		Name:      stsName,
		Namespace: r.instance.Namespace,
	}, sts); err != nil {
		return err
	}

	dataCount := builders.DataNodesCount(r.ctx, r.Client, r.instance)
	if dataCount == 2 && r.instance.Spec.General.DrainDataNodes {
		r.logger.Info("only 2 data nodes and drain is set, some shards may not drain")
	}

	ready, err := services.CheckClusterStatusForRestart(r.osClient, r.instance.Spec.General.DrainDataNodes)
	if err != nil {
		r.logger.Error(err, "Could not check opensearch cluster status")
		return err
	}
	if !ready {
		r.logger.Info("Cluster is not ready for next pod to restart")
		return nil
	}

	// Work around for https://github.com/kubernetes/kubernetes/issues/73492
	// If upgrade on this node pool is complete update status and return
	if sts.Status.UpdatedReplicas == sts.Status.Replicas {
		if err = services.ReactivateShardAllocation(r.osClient); err != nil {
			return err
		}
		r.recorder.AnnotatedEventf(r.instance, annotations, "Normal", "Upgrade", "Finished upgrade of node pool '%s'", pool.Component)

		return retry.RetryOnConflict(retry.DefaultRetry, func() error {
			if err := r.Get(r.ctx, client.ObjectKeyFromObject(r.instance), r.instance); err != nil {
				return err
			}
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
			r.instance.Status.ComponentsStatus = helpers.Replace(currentStatus, componentStatus, r.instance.Status.ComponentsStatus)

			return r.Status().Update(r.ctx, r.instance)
		})
	}

	workingPod := builders.WorkingPodForRollingRestart(sts)

	ready, err = services.PreparePodForDelete(r.osClient, workingPod, r.instance.Spec.General.DrainDataNodes, dataCount)
	if err != nil {
		return err
	}
	if !ready {
		return nil
	}

	err = r.Delete(r.ctx, &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      workingPod,
			Namespace: sts.Namespace,
		},
	})
	if err != nil {
		return err
	}

	// If we are draining nodes remove the exclusion after the pod is deleted
	if r.instance.Spec.General.DrainDataNodes {
		_, err = services.RemoveExcludeNodeHost(r.osClient, workingPod)
		return err
	}

	return nil
}
