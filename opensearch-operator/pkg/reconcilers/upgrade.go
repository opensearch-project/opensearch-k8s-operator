package reconcilers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Masterminds/semver"
	opsterv1 "github.com/Opster/opensearch-k8s-operator/opensearch-operator/api/v1"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/opensearch-gateway/services"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/pkg/builders"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/pkg/helpers"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/pkg/reconciler"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/pkg/reconcilers/k8s"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/pkg/reconcilers/util"
	"github.com/go-logr/logr"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
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
	replicaRestoreAnnotation = "opster.io/reduced-replicas"
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
		// If phase is UPGRADING but versions are in sync, set it back to RUNNING
		if r.instance.Status.Phase == opsterv1.PhaseUpgrading {
			err := r.client.UpdateOpenSearchClusterStatus(client.ObjectKeyFromObject(r.instance), func(instance *opsterv1.OpenSearchCluster) {
				instance.Status.Phase = opsterv1.PhaseRunning
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

	annotations := map[string]string{"cluster-name": r.instance.GetName()}

	// If version validation fails log a warning and do nothing
	if err := r.validateUpgrade(); err != nil {
		r.logger.V(1).Error(err, "version validation failed", "currentVersion", r.instance.Status.Version, "requestedVersion", r.instance.Spec.General.Version)
		r.recorder.AnnotatedEventf(r.instance, annotations, "Normal", "Upgrade", "Failed to validation version, currentVersion: %s , requestedVersion: %s", r.instance.Status.Version, r.instance.Spec.General.Version)
		return ctrl.Result{}, err
	}

	// Set phase to UPGRADING if not already set
	if r.instance.Status.Phase != opsterv1.PhaseUpgrading {
		err := r.client.UpdateOpenSearchClusterStatus(client.ObjectKeyFromObject(r.instance), func(instance *opsterv1.OpenSearchCluster) {
			instance.Status.Phase = opsterv1.PhaseUpgrading
		})
		if err != nil {
			return ctrl.Result{}, err
		}
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
	case upgradeStatusPending:
		// Set it to upgrading and requeue
		err := r.client.UpdateOpenSearchClusterStatus(client.ObjectKeyFromObject(r.instance), func(instance *opsterv1.OpenSearchCluster) {
			currentStatus.Status = upgradeStatusInProgress
			instance.Status.ComponentsStatus = append(instance.Status.ComponentsStatus, currentStatus)
		})
		r.recorder.AnnotatedEventf(r.instance, annotations, "Normal", "Upgrade", "Starting upgrade of node pool '%s'", currentStatus.Description)
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
	case "Finished":
		// Restore replicas before cleanup
		if err := r.restoreReducedReplicas(); err != nil {
			r.logger.Error(err, "Failed to restore reduced replicas after upgrade")
			// Continue with upgrade completion even if replica restoration fails
		}

		// Cleanup status after successful upgrade
		err := r.client.UpdateOpenSearchClusterStatus(client.ObjectKeyFromObject(r.instance), func(instance *opsterv1.OpenSearchCluster) {
			instance.Status.Version = instance.Spec.General.Version
			instance.Status.Phase = opsterv1.PhaseRunning
			for _, pool := range instance.Spec.NodePools {
				componentStatus := opsterv1.ComponentStatus{
					Component:   componentNameUpgrader,
					Description: pool.Component,
				}
				currentStatus, found := helpers.FindFirstPartial(instance.Status.ComponentsStatus, componentStatus, helpers.GetByDescriptionAndComponent)
				if found {
					instance.Status.ComponentsStatus = helpers.RemoveIt(currentStatus, instance.Status.ComponentsStatus)
				}
			}
		})
		r.recorder.AnnotatedEventf(r.instance, annotations, "Normal", "Upgrade", "Finished upgrade - NewVersion: %s", r.instance.Spec.General.Version)
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
		r.recorder.AnnotatedEventf(r.instance, annotations, "Error", "Upgrade", "Invalid version: specified version is a downgrade")
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
			Component:   componentNameUpgrader,
			Description: pool.Component,
			Status:      upgradeStatusInProgress,
		}
	}
	// Pick the first unworked on node next
	pool, found = r.findNextPool(dataNodes)
	if found {
		return pool, opsterv1.ComponentStatus{
			Component:   componentNameUpgrader,
			Description: pool.Component,
			Status:      upgradeStatusPending,
		}
	}
	// Next do the same for any nodes that are data and master
	pool, found = r.findInProgress(dataAndMasterNodes)
	if found {
		return pool, opsterv1.ComponentStatus{
			Component:   componentNameUpgrader,
			Description: pool.Component,
			Status:      upgradeStatusInProgress,
		}
	}
	pool, found = r.findNextPool(dataAndMasterNodes)
	if found {
		return pool, opsterv1.ComponentStatus{
			Component:   componentNameUpgrader,
			Description: pool.Component,
			Status:      upgradeStatusPending,
		}
	}

	// Finally do the non data nodes
	pool, found = r.findInProgress(otherNodes)
	if found {
		return pool, opsterv1.ComponentStatus{
			Component:   componentNameUpgrader,
			Description: pool.Component,
			Status:      upgradeStatusInProgress,
		}
	}
	pool, found = r.findNextPool(otherNodes)
	if found {
		return pool, opsterv1.ComponentStatus{
			Component:   componentNameUpgrader,
			Description: pool.Component,
			Status:      upgradeStatusPending,
		}
	}

	// If we get here all nodes should be upgraded
	return opsterv1.NodePool{}, opsterv1.ComponentStatus{
		Component: componentNameUpgrader,
		Status:    "Finished",
	}
}

func (r *UpgradeReconciler) findInProgress(pools []opsterv1.NodePool) (opsterv1.NodePool, bool) {
	for _, nodePool := range pools {
		componentStatus := opsterv1.ComponentStatus{
			Component:   componentNameUpgrader,
			Description: nodePool.Component,
		}
		currentStatus, found := helpers.FindFirstPartial(r.instance.Status.ComponentsStatus, componentStatus, helpers.GetByDescriptionAndComponent)
		if found && currentStatus.Status == upgradeStatusInProgress {
			return nodePool, true
		}
	}
	return opsterv1.NodePool{}, false
}

func (r *UpgradeReconciler) findNextPool(pools []opsterv1.NodePool) (opsterv1.NodePool, bool) {
	for _, nodePool := range pools {
		componentStatus := opsterv1.ComponentStatus{
			Component:   componentNameUpgrader,
			Description: nodePool.Component,
		}
		_, found := helpers.FindFirstPartial(r.instance.Status.ComponentsStatus, componentStatus, helpers.GetByDescriptionAndComponent)
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
				Component:   componentNameUpgrader,
				Status:      upgradeStatusInProgress,
				Description: pool.Component,
			}
			componentStatus := opsterv1.ComponentStatus{
				Component:   componentNameUpgrader,
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

	// Use PreparePodForDeleteWithReplicas to get replica information
	prepareResult, err := services.PreparePodForDeleteWithReplicas(r.osClient, r.logger, workingPod, r.instance.Spec.General.DrainDataNodes, dataCount)
	if err != nil {
		conditions = append(conditions, "Could not prepare pod for delete")
		r.setComponentConditions(conditions, pool.Component)
		return err
	}
	if !prepareResult.Ready {
		conditions = append(conditions, "Waiting for node to drain")
		r.setComponentConditions(conditions, pool.Component)
		return nil
	}

	// Store original replica counts if any were reduced
	if len(prepareResult.OriginalReplicas) > 0 {
		err = r.storeReducedReplicas(prepareResult.OriginalReplicas)
		if err != nil {
			r.logger.Error(err, "Failed to store reduced replica information")
			// Don't fail the upgrade if we can't store this, but log it
		}
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
			Component:   componentNameUpgrader,
			Status:      upgradeStatusInProgress,
			Description: component,
		}
		componentStatus, found := helpers.FindFirstPartial(instance.Status.ComponentsStatus, currentStatus, helpers.GetByDescriptionAndComponent)
		newStatus := opsterv1.ComponentStatus{
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

// storeReducedReplicas stores the original replica counts in cluster annotations
func (r *UpgradeReconciler) storeReducedReplicas(newReplicas map[string]int) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		// Get fresh instance
		instance, err := r.client.GetOpenSearchCluster(r.instance.Name, r.instance.Namespace)
		if err != nil {
			return err
		}

		// Get existing replica map from annotations
		existingReplicas := make(map[string]int)
		if instance.Annotations != nil {
			if replicaData, ok := instance.Annotations[replicaRestoreAnnotation]; ok {
				if err := json.Unmarshal([]byte(replicaData), &existingReplicas); err != nil {
					r.logger.V(1).Info(fmt.Sprintf("Could not parse existing replica annotation: %v", err))
					existingReplicas = make(map[string]int)
				}
			}
		}

		// Merge with new replicas (new takes precedence)
		for k, v := range newReplicas {
			existingReplicas[k] = v
		}

		// Store back to annotations
		replicaData, err := json.Marshal(existingReplicas)
		if err != nil {
			return err
		}

		if instance.Annotations == nil {
			instance.Annotations = make(map[string]string)
		}
		instance.Annotations[replicaRestoreAnnotation] = string(replicaData)

		// Update the cluster object using the underlying client
		// We need to use the embedded client.Client from K8sClientImpl
		if clientImpl, ok := r.client.(k8s.K8sClientImpl); ok {
			return clientImpl.Update(r.ctx, &instance)
		}
		return fmt.Errorf("unable to access underlying client for update")
	})
}

// restoreReducedReplicas restores replica counts from annotations and clears the annotation
func (r *UpgradeReconciler) restoreReducedReplicas() error {
	if r.osClient == nil {
		var err error
		r.osClient, err = util.CreateClientForCluster(r.client, r.ctx, r.instance, nil)
		if err != nil {
			return fmt.Errorf("failed to create OpenSearch client: %w", err)
		}
	}

	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		// Get fresh instance
		instance, err := r.client.GetOpenSearchCluster(r.instance.Name, r.instance.Namespace)
		if err != nil {
			return err
		}

		// Check if we have replica data to restore
		if instance.Annotations == nil {
			return nil
		}

		replicaData, ok := instance.Annotations[replicaRestoreAnnotation]
		if !ok || replicaData == "" {
			return nil
		}

		// Parse replica map
		originalReplicas := make(map[string]int)
		if err := json.Unmarshal([]byte(replicaData), &originalReplicas); err != nil {
			r.logger.Error(err, "Failed to parse replica restore annotation")
			// Clear the annotation even if parsing fails
			delete(instance.Annotations, replicaRestoreAnnotation)
			if clientImpl, ok := r.client.(k8s.K8sClientImpl); ok {
				return clientImpl.Update(r.ctx, &instance)
			}
			return fmt.Errorf("unable to access underlying client for update")
		}

		if len(originalReplicas) == 0 {
			// No replicas to restore, just clear the annotation
			delete(instance.Annotations, replicaRestoreAnnotation)
			if clientImpl, ok := r.client.(k8s.K8sClientImpl); ok {
				return clientImpl.Update(r.ctx, &instance)
			}
			return fmt.Errorf("unable to access underlying client for update")
		}

		// Restore replicas
		r.logger.Info(fmt.Sprintf("Restoring replicas for %d indices after upgrade", len(originalReplicas)))
		if err := services.RestoreIndexReplicas(r.osClient, originalReplicas, r.logger); err != nil {
			r.logger.Error(err, "Failed to restore some replicas, will retry on next reconciliation")
			return err
		}

		// Clear the annotation after successful restoration
		delete(instance.Annotations, replicaRestoreAnnotation)
		if clientImpl, ok := r.client.(k8s.K8sClientImpl); ok {
			return clientImpl.Update(r.ctx, &instance)
		}
		return fmt.Errorf("unable to access underlying client for update")
	})
}
