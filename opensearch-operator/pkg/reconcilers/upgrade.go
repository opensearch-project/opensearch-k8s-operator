package reconcilers

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Masterminds/semver"
	"github.com/banzaicloud/operator-tools/pkg/reconciler"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	"k8s.io/utils/pointer"
	opsterv1 "opensearch.opster.io/api/v1"
	"opensearch.opster.io/opensearch-gateway/services"
	"opensearch.opster.io/pkg/builders"
	"opensearch.opster.io/pkg/helpers"
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
			append(opts, reconciler.WithLog(log.FromContext(ctx).WithValues("reconciler", "scaler")))...),
		ctx:               ctx,
		recorder:          recorder,
		reconcilerContext: reconcilerContext,
		instance:          instance,
	}
}

func (r *UpgradeReconciler) Reconcile() (ctrl.Result, error) {
	// If versions are in sync do nothing
	if r.instance.Spec.General.Version == r.instance.Status.Version {
		return ctrl.Result{}, nil
	}

	lg := log.FromContext(r.ctx)

	// If version validation fails log a warning and do nothing
	if err := r.validateUpgrade(); err != nil {
		lg.V(1).Error(err, "version validation vailed", "currentVersion", r.instance.Status.Version, "requestedVersion", r.instance.Spec.General.Version)
		return ctrl.Result{}, err
	}

	// If there is work to do create an Opensearch Client
	username, password, err := helpers.UsernameAndPassword(r.ctx, r.Client, r.instance)
	if err != nil {
		return ctrl.Result{}, err
	}

	clusterClient, err := services.NewOsClusterClient(fmt.Sprintf("https://%s.%s:9200", r.instance.Spec.General.ServiceName, r.instance.Name), username, password)
	if err != nil {
		return ctrl.Result{}, err
	}
	r.osClient = clusterClient

	//Fetch the working nodepool
	nodePool, currentStatus := r.findWorkingNodePool()

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
		return ctrl.Result{
			Requeue:      true,
			RequeueAfter: 15 * time.Second,
		}, err
	case "NonDataPending":
		// Set it to upgrading and requeue
		err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			if err := r.Get(r.ctx, client.ObjectKeyFromObject(r.instance), r.instance); err != nil {
				return err
			}
			currentStatus.Status = "UntrackedUpgrade"
			r.instance.Status.ComponentsStatus = append(r.instance.Status.ComponentsStatus, currentStatus)
			return r.Status().Update(r.ctx, r.instance)
		})
		return ctrl.Result{
			Requeue:      true,
			RequeueAfter: 15 * time.Second,
		}, err
	case "Upgrading":
		err := r.doDataNodeUpgrade(nodePool)
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

	// Don't allow version downgrades as they might cause unexpected issues
	if new.LessThan(existing) {
		return ErrVersionDowngrade
	}

	// Don't allow more than one major version upgrade
	nextMajor := existing.IncMajor()
	upgradeConstraint, err := semver.NewConstraint(fmt.Sprintf("^%s", nextMajor.String()))
	if err != nil {
		return err
	}

	if !upgradeConstraint.Check(new) {
		return ErrMajorVersionJump
	}

	return nil
}

// Find which nodepool to work on
func (r *UpgradeReconciler) findWorkingNodePool() (opsterv1.NodePool, opsterv1.ComponentStatus) {
	// First sort node pools
	var dataNodes, dataAndMasterNodes, otherNodes []opsterv1.NodePool
	for _, nodePool := range r.instance.Spec.NodePools {
		if helpers.ContainsString(nodePool.Roles, "data") {
			if helpers.ContainsString(nodePool.Roles, "master") {
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

	// Finally do the non data nodes.  We can let the kubernetes rollout logic do the work here
	pool, found = r.findNextPool(otherNodes)
	if found {
		return pool, opsterv1.ComponentStatus{
			Component:   "Upgrader",
			Description: pool.Component,
			Status:      "NonDataPending",
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

func (r *UpgradeReconciler) doDataNodeUpgrade(pool opsterv1.NodePool) error {
	// Fetch the STS
	stsName := builders.StsName(r.instance, &pool)
	sts := &appsv1.StatefulSet{}
	if err := r.Get(r.ctx, types.NamespacedName{
		Name:      stsName,
		Namespace: r.instance.Namespace,
	}, sts); err != nil {
		return err
	}

	// If not all nodes are ready requeue
	if sts.Status.ReadyReplicas != sts.Status.Replicas {
		return nil
	}

	// Check cluster health
	health, err := r.osClient.CatHealth()
	if err != nil {
		return err
	}

	if health.Status != "green" {
		// Check cluster shard routing
		flatSettings, err := r.osClient.GetFlatClusterSettings()
		if err != nil {
			return err
		}

		// If shard routing is all return and requeue
		if flatSettings.Transient.ClusterRoutingAllocationEnable == string(services.ClusterSettingsAllocationAll) {
			return nil
		}

		// Set shard routing to all
		if err := services.SetClusterShardAllocation(r.osClient, services.ClusterSettingsAllocationAll); err != nil {
			return err
		}

		// return and requeue
		return nil
	}

	// Work around for https://github.com/kubernetes/kubernetes/issues/73492
	// If upgrade on this node pool is complete update status and return
	if sts.Status.UpdatedReplicas == sts.Status.Replicas {
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

	// Update cluster routing then delete appropriate ordinal pod
	if err := services.SetClusterShardAllocation(r.osClient, services.ClusterSettingsAllocationPrimaries); err != nil {
		return err
	}
	deleteOrdinal := pointer.Int32Deref(sts.Spec.Replicas, 1) - 1 - sts.Status.UpdatedReplicas

	return r.Delete(r.ctx, &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%d", stsName, deleteOrdinal),
			Namespace: r.instance.Namespace,
		},
	})
}
