package reconcilers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	batchv1 "k8s.io/api/batch/v1"
	policyv1 "k8s.io/api/policy/v1"
	"opensearch.opster.io/pkg/reconcilers/util"

	"github.com/cisco-open/k8s-objectmatcher/patch"
	"github.com/cisco-open/operator-tools/pkg/reconciler"
	"github.com/go-logr/logr"
	"github.com/samber/lo"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	opsterv1 "opensearch.opster.io/api/v1"
	"opensearch.opster.io/pkg/builders"
	"opensearch.opster.io/pkg/helpers"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	snapshotRepoConfigChecksumAnnotation = "snapshotrepoconfig/checksum"
)

type ClusterReconciler struct {
	client.Client
	reconciler.ResourceReconciler
	ctx               context.Context
	recorder          record.EventRecorder
	reconcilerContext *ReconcilerContext
	instance          *opsterv1.OpenSearchCluster
	logger            logr.Logger
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
			append(
				opts,
				reconciler.WithPatchCalculateOptions(patch.IgnoreVolumeClaimTemplateTypeMetaAndStatus(), patch.IgnoreStatusFields()),
				reconciler.WithLog(log.FromContext(ctx).WithValues("reconciler", "cluster")),
			)...),
		ctx:               ctx,
		recorder:          recorder,
		reconcilerContext: reconcilerContext,
		instance:          instance,
		logger:            log.FromContext(ctx),
	}
}

func (r *ClusterReconciler) Reconcile() (ctrl.Result, error) {
	//lg := log.FromContext(r.ctx)
	result := reconciler.CombinedResult{}
	username, password, err := helpers.UsernameAndPassword(r.ctx, r.Client, r.instance)
	if err != nil {
		return ctrl.Result{}, err
	}

	if r.instance.Spec.General.Monitoring.Enable {
		serviceMonitor := builders.NewServiceMonitor(r.instance)
		result.CombineErr(ctrl.SetControllerReference(r.instance, serviceMonitor, r.Client.Scheme()))
		result.Combine(r.ReconcileResource(serviceMonitor, reconciler.StatePresent))

	} else {
		serviceMonitor := builders.NewServiceMonitor(r.instance)
		res, err := r.ReconcileResource(serviceMonitor, reconciler.StateAbsent)
		if err != nil {
			if strings.Contains(err.Error(), "unable to retrieve the complete list of server APIs: monitoring.coreos.com/v1") {
				r.logger.Info("ServiceMonitor crd not found, skipping deletion")
			} else {
				result.Combine(res, err)
			}
		} else {
			result.Combine(res, err)
		}

	}
	clusterService := builders.NewServiceForCR(r.instance)
	result.CombineErr(ctrl.SetControllerReference(r.instance, clusterService, r.Client.Scheme()))
	result.Combine(r.ReconcileResource(clusterService, reconciler.StatePresent))

	discoveryService := builders.NewDiscoveryServiceForCR(r.instance)
	result.CombineErr(ctrl.SetControllerReference(r.instance, discoveryService, r.Scheme()))
	result.Combine(r.ReconcileResource(discoveryService, reconciler.StatePresent))

	passwordSecret := builders.PasswordSecret(r.instance, username, password)
	result.CombineErr(ctrl.SetControllerReference(r.instance, passwordSecret, r.Scheme()))
	result.Combine(r.ReconcileResource(passwordSecret, reconciler.StatePresent))

	bootstrapPod := builders.NewBootstrapPod(r.instance, r.reconcilerContext.Volumes, r.reconcilerContext.VolumeMounts)
	result.CombineErr(ctrl.SetControllerReference(r.instance, bootstrapPod, r.Scheme()))
	if r.instance.Status.Initialized {
		result.Combine(r.ReconcileResource(bootstrapPod, reconciler.StateAbsent))
	} else {
		result.Combine(r.ReconcileResource(bootstrapPod, reconciler.StatePresent))
	}

	for _, nodePool := range r.instance.Spec.NodePools {
		headlessService := builders.NewHeadlessServiceForNodePool(r.instance, &nodePool)
		result.CombineErr(ctrl.SetControllerReference(r.instance, headlessService, r.Client.Scheme()))
		result.Combine(r.ReconcileResource(headlessService, reconciler.StatePresent))

		result.Combine(r.reconcileNodeStatefulSet(nodePool, username))
	}

	// if Version isn't set we set it now to check for upgrades later.
	if r.instance.Status.Version == "" {
		err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			if err := r.Get(r.ctx, client.ObjectKeyFromObject(r.instance), r.instance); err != nil {
				return err
			}
			r.instance.Status.Version = r.instance.Spec.General.Version
			return r.Status().Update(r.ctx, r.instance)
		})
		result.CombineErr(err)
	}
	if r.instance.Spec.General.SnapshotRepositories != nil && len(r.instance.Spec.General.SnapshotRepositories) > 0 {
		// Calculate checksum and check for changes
		result.Combine(r.ReconcileSnapshotRepoConfig(username))
	}

	// If the cluster is using only emptyDir, then check for failure and recreate if necessary
	if r.isEmptyDirCluster() {
		result.Combine(r.checkForEmptyDirRecovery())
	}

	return result.Result, result.Err
}

func (r *ClusterReconciler) ReconcileSnapshotRepoConfig(username string) (*ctrl.Result, error) {
	annotations := map[string]string{"cluster-name": r.instance.GetName()}
	var checksumerr error
	var checksumval string
	snapshotRepodata, _ := json.Marshal(&r.instance.Spec.General.SnapshotRepositories)
	checksumval, checksumerr = util.GetSha1Sum(snapshotRepodata)
	if checksumerr != nil {
		return &ctrl.Result{}, checksumerr
	}
	clusterName := r.instance.Name
	jobName := clusterName + "-snapshotrepoconfig-update"
	job := batchv1.Job{}
	if err := r.Get(r.ctx, client.ObjectKey{Name: jobName, Namespace: r.instance.Namespace}, &job); err == nil {
		value, exists := job.ObjectMeta.Annotations[snapshotRepoConfigChecksumAnnotation]
		if exists && value == checksumval {
			// Nothing to do, current snapshotconfig already applied
			return &ctrl.Result{}, nil
		}
		// Delete old job
		r.logger.Info("Deleting old snapshotconfig job")
		opts := client.DeleteOptions{}
		// Add this so pods of the job are deleted as well, otherwise they would remain as orphaned pods
		client.PropagationPolicy(metav1.DeletePropagationForeground).ApplyToDelete(&opts)
		err = r.Delete(r.ctx, &job, &opts)
		if err != nil {
			return &ctrl.Result{}, err
		}
		// Make sure job is completely deleted (when r.Delete returns deletion sometimes is not yet complete)
		_, err = r.ReconcileResource(&job, reconciler.StateAbsent)
		if err != nil {
			return &ctrl.Result{}, err
		}
	}
	r.logger.Info("Starting snapshotconfig update job")
	r.recorder.AnnotatedEventf(r.instance, annotations, "Normal", "Snapshot", "Starting to snapshotconfig update job")
	snapshotRepoJob := builders.NewSnapshotRepoconfigUpdateJob(
		r.instance,
		jobName,
		r.instance.Namespace,
		checksumval,
		r.reconcilerContext.Volumes,
		r.reconcilerContext.VolumeMounts,
	)
	if err := ctrl.SetControllerReference(r.instance, &snapshotRepoJob, r.Client.Scheme()); err != nil {
		return &ctrl.Result{}, err
	}
	return r.ReconcileResource(&snapshotRepoJob, reconciler.StatePresent)
}

func (r *ClusterReconciler) reconcileNodeStatefulSet(nodePool opsterv1.NodePool, username string) (*ctrl.Result, error) {
	found, nodePoolConfig := r.reconcilerContext.fetchNodePoolHash(nodePool.Component)

	// If config hasn't been set up for the node pool requeue
	if !found {
		return &ctrl.Result{
			Requeue: true,
		}, nil
	}

	extraConfig := helpers.MergeConfigs(r.instance.Spec.General.AdditionalConfig, nodePool.AdditionalConfig)

	sts := builders.NewSTSForNodePool(
		username,
		r.instance,
		nodePool,
		nodePoolConfig.ConfigHash,
		r.reconcilerContext.Volumes,
		r.reconcilerContext.VolumeMounts,
		extraConfig,
	)
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

	// Fix selector.matchLabels (issue #311), need to recreate the STS for it as spec.selector is immutable
	if _, exists := existing.Spec.Selector.MatchLabels["opensearch.role"]; exists {
		r.logger.Info("deleting statefulset while orphaning pods to fix labels " + existing.Name)
		if err := helpers.WaitForSTSDelete(r.ctx, r.Client, existing); err != nil {
			r.logger.Error(err, "Failed to delete Statefulset for nodePool "+nodePool.Component)
			return result, err
		}
		result, err := r.ReconcileResource(sts, reconciler.StateCreated)
		if err != nil || result != nil {
			return result, err
		}
	}

	// Detect cluster failure and initiate parallel recovery
	if helpers.ParallelRecoveryMode() &&
		(nodePool.Persistence == nil || nodePool.Persistence.PersistenceSource.PVC != nil) {
		// This logic only works if the STS uses PVCs
		// First check if the STS already has a readable status (CurrentRevision == "" indicates the STS is newly created and the controller has not yet updated the status properly)
		if existing.Status.CurrentRevision == "" {
			existing, err = helpers.WaitForSTSStatus(r.ctx, r.Client, existing)
			if err != nil {
				return &ctrl.Result{Requeue: true}, err
			}
		}
		// Check number of PVCs for nodepool
		pvcCount, err := helpers.CountPVCsForNodePool(r.ctx, r.Client, r.instance, &nodePool)
		if err != nil {
			r.logger.Error(err, "Failed to determine PVC count. Continuing on normally")
		} else {
			// A failure is assumed if n PVCs exist but less than n-1 pods (one missing pod is allowed for rolling restart purposes)
			// We can assume the cluster is in a failure state and cannot recover on its own
			if !helpers.UpgradeInProgress(r.instance.Status) &&
				pvcCount >= int(nodePool.Replicas) && existing.Status.ReadyReplicas < nodePool.Replicas-1 {
				r.logger.Info(fmt.Sprintf("Detected recovery situation for nodepool %s: PVC count: %d, replicas: %d. Recreating STS with parallel mode", nodePool.Component, pvcCount, existing.Status.Replicas))
				if existing.Spec.PodManagementPolicy != appsv1.ParallelPodManagement {
					// Switch to Parallel to jumpstart the cluster
					// First delete existing STS
					if err := helpers.WaitForSTSDelete(r.ctx, r.Client, existing); err != nil {
						r.logger.Error(err, "Failed to delete STS")
						return result, err
					}
					// Recreate with PodManagementPolicy=Parallel
					sts.Spec.PodManagementPolicy = appsv1.ParallelPodManagement
					sts.ObjectMeta.ResourceVersion = ""
					sts.ObjectMeta.UID = ""
					result, err = r.ReconcileResource(sts, reconciler.StatePresent)
					if err != nil {
						r.logger.Error(err, "Failed to create STS")
						return result, err
					}
					// Wait for pods to appear
					err := helpers.WaitForSTSReplicas(r.ctx, r.Client, existing, nodePool.Replicas)
					// Abort normal logic and requeue
					return &ctrl.Result{Requeue: true}, err
				}
			} else if existing.Spec.PodManagementPolicy == appsv1.ParallelPodManagement {
				// We are in Parallel mode but appear to not have a failure situation any longer. Switch back to normal mode
				r.logger.Info(fmt.Sprintf("Ending recovery mode for nodepool %s", nodePool.Component))
				if err := helpers.WaitForSTSDelete(r.ctx, r.Client, existing); err != nil {
					r.logger.Error(err, "Failed to delete STS")
					return result, err
				}
				// STS will be recreated by the normal code below
			}
		}
	}

	// Handle PodDisruptionBudget
	result, err = r.handlePDB(&nodePool)
	if err != nil {
		return result, err
	}

	// Handle PVC resizing

	//Default is PVC, or explicit check for PersistenceSource as PVC
	// Handle volume resizing, but only if we are using PVCs
	if nodePool.Persistence == nil || nodePool.Persistence.PersistenceSource.PVC != nil {
		err := r.maybeUpdateVolumes(existing, nodePool)
		if err != nil {
			return result, err
		}
	}

	// Now set the desired replicas to be the existing replicas
	// This will allow the scaler reconciler to function correctly
	sts.Spec.Replicas = existing.Spec.Replicas

	// Don't update env vars on non data nodes while an upgrade is in progress
	// as we don't want uncontrolled restarts while we're doing an upgrade
	if r.instance.Status.Version != "" &&
		r.instance.Status.Version != r.instance.Spec.General.Version &&
		!helpers.HasDataRole(&nodePool) {
		sts.Spec.Template.Spec.Containers[0].Env = existing.Spec.Template.Spec.Containers[0].Env
	}

	// Finally we enforce the desired state
	return r.ReconcileResource(sts, reconciler.StatePresent)
}

func (r *ClusterReconciler) DeleteResources() (ctrl.Result, error) {
	result := reconciler.CombinedResult{}
	return result.Result, result.Err
}

// isEmptyDirCluster returns true only if every nodePool is using emptyDir
func (r *ClusterReconciler) isEmptyDirCluster() bool {

	for _, nodePool := range r.instance.Spec.NodePools {
		if nodePool.Persistence == nil {
			return false
		} else if nodePool.Persistence != nil && nodePool.Persistence.EmptyDir == nil {
			return false
		}
	}
	return true
}

// checkForEmptyDirRecovery checks if the cluster has failed and recreates the cluster if needed
func (r *ClusterReconciler) checkForEmptyDirRecovery() (*ctrl.Result, error) {
	lg := log.FromContext(r.ctx)
	// If cluster has not yet initialized, don't do anything
	if !r.instance.Status.Initialized {
		return &ctrl.Result{}, nil
	}

	// If any scaling operation is going on, don't do anything
	for _, nodePool := range r.instance.Spec.NodePools {
		componentStatus := opsterv1.ComponentStatus{
			Component:   "Scaler",
			Description: nodePool.Component,
		}
		comp := r.instance.Status.ComponentsStatus
		_, found := helpers.FindFirstPartial(comp, componentStatus, helpers.GetByDescriptionAndGroup)

		if found {
			return &ctrl.Result{}, nil
		}
	}

	// Check at least one data node is running
	// Check at least half of master pods are running
	var readyDataNodes int32
	var readyMasterNodes int32
	var totalMasterNodes int32

	clusterName := r.instance.Name
	clusterNamespace := r.instance.Namespace

	for _, nodePool := range r.instance.Spec.NodePools {
		var sts *appsv1.StatefulSet
		var err error
		if helpers.HasDataRole(&nodePool) || helpers.HasManagerRole(&nodePool) {
			sts, err = helpers.GetSTSForNodePool(r.ctx, r.Client, nodePool, clusterName, clusterNamespace)
			if err != nil {
				return &ctrl.Result{Requeue: true}, err
			}
		}

		if helpers.HasDataRole(&nodePool) {
			readyDataNodes += sts.Status.ReadyReplicas
		}

		if helpers.HasManagerRole(&nodePool) {
			totalMasterNodes += *sts.Spec.Replicas
			readyMasterNodes += sts.Status.ReadyReplicas
		}
	}

	// If the failure condition is met,
	// Delete all the sts so that everything will be created
	// Then delete the securityconfig job and set cluster initialized to false
	// This will cause the bootstrap pod to run again and security indices to be initialized again
	if readyDataNodes == 0 || readyMasterNodes < (totalMasterNodes+1)/2 {
		lg.Info("Detected failure for cluster with emptyDir %s in ns %s", clusterName, clusterNamespace)
		lg.Info("Deleting all sts and securityconfig job to re-create cluster")
		for _, nodePool := range r.instance.Spec.NodePools {
			err := helpers.DeleteSTSForNodePool(r.ctx, r.Client, nodePool, clusterName, clusterNamespace)
			if err != nil {
				lg.Error(err, fmt.Sprintf("Failed to delete sts for nodePool %s", nodePool.Component))
				return &ctrl.Result{Requeue: true}, err
			}
		}

		err := helpers.DeleteSecurityUpdateJob(r.ctx, r.Client, clusterName, clusterNamespace)
		if err != nil {
			lg.Error(err, "Failed to delete security update job")
			return &ctrl.Result{Requeue: true}, err
		}

		if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			if err := r.Get(r.ctx, client.ObjectKeyFromObject(r.instance), r.instance); err != nil {
				return err
			}
			r.instance.Status.Initialized = false
			return r.Status().Update(r.ctx, r.instance)
		}); err != nil {
			lg.Error(err, "Failed to update cluster status")
			return &ctrl.Result{Requeue: true}, err
		}
	}

	return &ctrl.Result{}, nil
}

func (r *ClusterReconciler) handlePDB(nodePool *opsterv1.NodePool) (*ctrl.Result, error) {
	pdb := policyv1.PodDisruptionBudget{}

	if nodePool.Pdb != nil && nodePool.Pdb.Enable {
		// Check if provided parameters are valid
		if (nodePool.Pdb.MinAvailable != nil && nodePool.Pdb.MaxUnavailable != nil) || (nodePool.Pdb.MinAvailable == nil && nodePool.Pdb.MaxUnavailable == nil) {
			r.logger.Info("Please provide only one parameter (minAvailable OR maxUnavailable) in order to configure a PodDisruptionBudget")
			return &ctrl.Result{}, fmt.Errorf("please provide only one parameter (minAvailable OR maxUnavailable) in order to configure a PodDisruptionBudget")

		}
		pdb = helpers.ComposePDB(r.instance, nodePool)
		if err := ctrl.SetControllerReference(r.instance, &pdb, r.Client.Scheme()); err != nil {
			return &ctrl.Result{}, err
		}

		return r.ReconcileResource(&pdb, reconciler.StatePresent)
	} else {
		// Make sure any existing PDB is removed if the feature is not enabled
		pdb = policyv1.PodDisruptionBudget{
			ObjectMeta: metav1.ObjectMeta{
				Name:       r.instance.Name + "-" + nodePool.Component + "-pdb",
				Namespace:  r.instance.Namespace,
				Finalizers: r.instance.Finalizers,
			},
		}
		return r.ReconcileResource(&pdb, reconciler.StateAbsent)
	}
}

func (r *ClusterReconciler) maybeUpdateVolumes(existing *appsv1.StatefulSet, nodePool opsterv1.NodePool) error {
	if nodePool.DiskSize == "" { // Default case
		nodePool.DiskSize = builders.DefaultDiskSize
	}

	// If we are changing from ephemeral storage to persistent
	// just delete the statefulset and let it be recreated
	if len(existing.Spec.VolumeClaimTemplates) < 1 {
		if err := r.deleteSTSWithOrphan(existing); err != nil {
			return err
		}
		return nil
	}

	existingDisk := lo.FromPtr(existing.Spec.VolumeClaimTemplates[0].Spec.Resources.Requests.Storage())
	nodePoolDiskSize, err := resource.ParseQuantity(nodePool.DiskSize)
	if err != nil {
		r.logger.Error(err, fmt.Sprintf("Invalid diskSize '%s' for nodepool %s", nodePool.DiskSize, nodePool.Component))
		return err
	}

	if existingDisk.Equal(nodePoolDiskSize) {
		r.logger.Info("The existing disk size " + existingDisk.String() + " is same as passed in disk size " + nodePoolDiskSize.String())
		return nil
	}
	r.logger.Info("Disk sizes differ for nodePool %s: current: %s, desired: %s", nodePool.Component, existingDisk.String(), nodePoolDiskSize.String())
	annotations := map[string]string{"cluster-name": r.instance.GetName()}
	r.recorder.AnnotatedEventf(r.instance, annotations, "Normal", "PVC", "Starting to resize PVC %s/%s from %s to  %s ", existing.Namespace, existing.Name, existingDisk.String(), nodePoolDiskSize.String())
	// To update the PVCs we need to temporarily delete the StatefulSet while allowing the pods to continue to run
	if err := r.deleteSTSWithOrphan(existing); err != nil {
		return err
	}

	// Identify the PVC for each statefulset pod and patch with the new size
	for i := 0; i < int(lo.FromPtrOr(existing.Spec.Replicas, 1)); i++ {
		clusterName := r.instance.Name
		claimName := fmt.Sprintf("data-%s-%s-%d", clusterName, nodePool.Component, i)
		var pvc corev1.PersistentVolumeClaim
		nsn := types.NamespacedName{
			Namespace: existing.Namespace,
			Name:      claimName,
		}
		if err := r.Get(r.ctx, nsn, &pvc); err != nil {
			r.logger.Info("Failed to get pvc" + pvc.Name)
			return err
		}

		pvc.Spec.Resources.Requests["storage"] = nodePoolDiskSize

		if err := r.Update(r.ctx, &pvc); err != nil {
			r.logger.Error(err, fmt.Sprintf("Failed to resize statefulset pvc %s", pvc.Name))
			r.recorder.AnnotatedEventf(r.instance, annotations, "Warning", "PVC", "Failed to Resize %s/%s", existing.Namespace, existing.Name)
			return err
		}
	}
	return nil
}

func (r *ClusterReconciler) deleteSTSWithOrphan(existing *appsv1.StatefulSet) error {
	r.logger.Info("Deleting statefulset while orphaning pods " + existing.Name)
	opts := client.DeleteOptions{}
	client.PropagationPolicy(metav1.DeletePropagationOrphan).ApplyToDelete(&opts)
	if err := r.Delete(r.ctx, existing, &opts); err != nil {
		r.logger.Info("Failed to delete statefulset" + existing.Name)
		return err
	}
	return nil
}
