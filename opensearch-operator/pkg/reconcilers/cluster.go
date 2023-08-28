package reconcilers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"opensearch.opster.io/pkg/reconcilers/k8s"
	"opensearch.opster.io/pkg/reconcilers/util"

	"github.com/cisco-open/k8s-objectmatcher/patch"
	"github.com/cisco-open/operator-tools/pkg/reconciler"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/client-go/tools/record"
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
	client            k8s.K8sClient
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
		client: k8s.NewK8sClient(client, ctx, append(
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
	username, password, err := helpers.UsernameAndPassword(r.client, r.instance)
	if err != nil {
		return ctrl.Result{}, err
	}

	if r.instance.Spec.General.Monitoring.Enable {
		serviceMonitor := builders.NewServiceMonitor(r.instance)
		result.CombineErr(ctrl.SetControllerReference(r.instance, serviceMonitor, r.client.Scheme()))
		result.Combine(r.client.ReconcileResource(serviceMonitor, reconciler.StatePresent))

	} else {
		serviceMonitor := builders.NewServiceMonitor(r.instance)
		res, err := r.client.ReconcileResource(serviceMonitor, reconciler.StateAbsent)
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
	result.CombineErr(ctrl.SetControllerReference(r.instance, clusterService, r.client.Scheme()))
	result.Combine(r.client.ReconcileResource(clusterService, reconciler.StatePresent))

	discoveryService := builders.NewDiscoveryServiceForCR(r.instance)
	result.CombineErr(ctrl.SetControllerReference(r.instance, discoveryService, r.client.Scheme()))
	result.Combine(r.client.ReconcileResource(discoveryService, reconciler.StatePresent))

	passwordSecret := builders.PasswordSecret(r.instance, username, password)
	result.CombineErr(ctrl.SetControllerReference(r.instance, passwordSecret, r.client.Scheme()))
	result.Combine(r.client.ReconcileResource(passwordSecret, reconciler.StatePresent))

	bootstrapPod := builders.NewBootstrapPod(r.instance, r.reconcilerContext.Volumes, r.reconcilerContext.VolumeMounts)
	result.CombineErr(ctrl.SetControllerReference(r.instance, bootstrapPod, r.client.Scheme()))
	if r.instance.Status.Initialized {
		result.Combine(r.client.ReconcileResource(bootstrapPod, reconciler.StateAbsent))
	} else {
		result.Combine(r.client.ReconcileResource(bootstrapPod, reconciler.StatePresent))
	}

	for _, nodePool := range r.instance.Spec.NodePools {
		headlessService := builders.NewHeadlessServiceForNodePool(r.instance, &nodePool)
		result.CombineErr(ctrl.SetControllerReference(r.instance, headlessService, r.client.Scheme()))
		result.Combine(r.client.ReconcileResource(headlessService, reconciler.StatePresent))

		result.Combine(r.reconcileNodeStatefulSet(nodePool, username))
	}

	// if Version isn't set we set it now to check for upgrades later.
	if r.instance.Status.Version == "" {
		err := r.client.UpdateOpenSearchClusterStatus(client.ObjectKeyFromObject(r.instance), func(instance *opsterv1.OpenSearchCluster) {
			instance.Status.Version = r.instance.Spec.General.Version
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
	job, err := r.client.GetJob(jobName, r.instance.Namespace)
	if err == nil {
		value, exists := job.ObjectMeta.Annotations[snapshotRepoConfigChecksumAnnotation]
		if exists && value == checksumval {
			// Nothing to do, current snapshotconfig already applied
			return &ctrl.Result{}, nil
		}
		// Delete old job
		r.logger.Info("Deleting old snapshotconfig job")
		err = r.client.DeleteJob(&job)
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
	if err := ctrl.SetControllerReference(r.instance, &snapshotRepoJob, r.client.Scheme()); err != nil {
		return &ctrl.Result{}, err
	}
	return r.client.ReconcileResource(&snapshotRepoJob, reconciler.StatePresent)
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
	if err := ctrl.SetControllerReference(r.instance, sts, r.client.Scheme()); err != nil {
		return &ctrl.Result{}, err
	}

	// First ensure that the statefulset exists
	result, err := r.client.ReconcileResource(sts, reconciler.StateCreated)
	if err != nil || result != nil {
		return result, err
	}

	// Next get the existing statefulset
	existing, err := r.client.GetStatefulSet(sts.Name, sts.Namespace)

	if err != nil {
		return result, err
	}

	// Fix selector.matchLabels (issue #311), need to recreate the STS for it as spec.selector is immutable
	if _, exists := existing.Spec.Selector.MatchLabels["opensearch.role"]; exists {
		r.logger.Info("deleting statefulset while orphaning pods to fix labels " + existing.Name)
		if err := helpers.WaitForSTSDelete(r.client, &existing); err != nil {
			r.logger.Error(err, "Failed to delete Statefulset for nodePool "+nodePool.Component)
			return result, err
		}
		result, err := r.client.ReconcileResource(sts, reconciler.StateCreated)
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
			new, err := helpers.WaitForSTSStatus(r.client, &existing)
			if err != nil {
				return &ctrl.Result{Requeue: true}, err
			}
			existing = *new
		}
		// Check number of PVCs for nodepool
		pvcCount, err := helpers.CountPVCsForNodePool(r.client, r.instance, &nodePool)
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
					if err := helpers.WaitForSTSDelete(r.client, &existing); err != nil {
						r.logger.Error(err, "Failed to delete STS")
						return result, err
					}
					// Recreate with PodManagementPolicy=Parallel
					sts.Spec.PodManagementPolicy = appsv1.ParallelPodManagement
					sts.ObjectMeta.ResourceVersion = ""
					sts.ObjectMeta.UID = ""
					result, err = r.client.ReconcileResource(sts, reconciler.StatePresent)
					if err != nil {
						r.logger.Error(err, "Failed to create STS")
						return result, err
					}
					// Wait for pods to appear
					err := helpers.WaitForSTSReplicas(r.client, &existing, nodePool.Replicas)
					// Abort normal logic and requeue
					return &ctrl.Result{Requeue: true}, err
				}
			} else if existing.Spec.PodManagementPolicy == appsv1.ParallelPodManagement {
				// We are in Parallel mode but appear to not have a failure situation any longer. Switch back to normal mode
				r.logger.Info(fmt.Sprintf("Ending recovery mode for nodepool %s", nodePool.Component))
				if err := helpers.WaitForSTSDelete(r.client, &existing); err != nil {
					r.logger.Error(err, "Failed to delete STS")
					return result, err
				}
				// STS will be recreated by the normal code below
			}
		}
	}

	// Handle volume resizing, but only if we are using PVCs
	if nodePool.Persistence == nil || nodePool.Persistence.PersistenceSource.PVC != nil {
		if nodePool.DiskSize == "" { // Default case
			nodePool.DiskSize = builders.DefaultDiskSize
		}

		existingDisk := *existing.Spec.VolumeClaimTemplates[0].Spec.Resources.Requests.Storage()
		nodePoolDiskSize, err := resource.ParseQuantity(nodePool.DiskSize)
		if err != nil {
			r.logger.Error(err, fmt.Sprintf("Invalid diskSize '%s' for nodepool %s", nodePool.DiskSize, nodePool.Component))
			return result, err
		}

		if existingDisk.Equal(nodePoolDiskSize) {
			r.logger.Info("The existing disk size " + existingDisk.String() + " is same as passed in disk size " + nodePoolDiskSize.String())
		} else {
			r.logger.Info("Disk sizes differ for nodePool %s: current: %s, desired: %s", nodePool.Component, existingDisk.String(), nodePoolDiskSize.String())
			annotations := map[string]string{"cluster-name": r.instance.GetName()}
			r.recorder.AnnotatedEventf(r.instance, annotations, "Normal", "PVC", "Starting to resize PVC %s/%s from %s to  %s ", existing.Namespace, existing.Name, existingDisk.String(), nodePoolDiskSize.String())
			// To update the PVCs we need to temporarily delete the StatefulSet while allowing the pods to continue to run
			r.logger.Info("Deleting statefulset while orphaning pods " + existing.Name)
			err = r.client.DeleteStatefulSet(&existing, true)
			if err != nil {
				r.logger.Info("Failed to delete statefulset" + existing.Name)
				return result, err
			}

			// Identify the PVC for each statefulset pod and patch with the new size
			for i := 0; i < int(*existing.Spec.Replicas); i++ {
				clusterName := r.instance.Name
				claimName := fmt.Sprintf("data-%s-%s-%d", clusterName, nodePool.Component, i)
				pvc, err := r.client.GetPVC(claimName, existing.Namespace)
				if err != nil {
					r.logger.Info("Failed to get pvc" + pvc.Name)
					return result, err
				}

				pvc.Spec.Resources.Requests["storage"] = nodePoolDiskSize

				if err := r.client.UpdatePVC(&pvc); err != nil {
					r.logger.Error(err, fmt.Sprintf("Failed to resize statefulset pvc %s", pvc.Name))
					r.recorder.AnnotatedEventf(r.instance, annotations, "Warning", "PVC", "Failed to Resize %s/%s", existing.Namespace, existing.Name)
					return result, err
				}
			}
			// STS will be recreated by the normal reconcile below
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
	return r.client.ReconcileResource(sts, reconciler.StatePresent)
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
			sts, err = helpers.GetSTSForNodePool(r.client, nodePool, clusterName, clusterNamespace)
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
			err := helpers.DeleteSTSForNodePool(r.client, nodePool, clusterName, clusterNamespace)
			if err != nil {
				lg.Error(err, fmt.Sprintf("Failed to delete sts for nodePool %s", nodePool.Component))
				return &ctrl.Result{Requeue: true}, err
			}
		}

		err := helpers.DeleteSecurityUpdateJob(r.client, clusterName, clusterNamespace)
		if err != nil {
			lg.Error(err, "Failed to delete security update job")
			return &ctrl.Result{Requeue: true}, err
		}

		err = r.client.UpdateOpenSearchClusterStatus(client.ObjectKeyFromObject(r.instance), func(instance *opsterv1.OpenSearchCluster) {
			instance.Status.Initialized = false
		})
		if err != nil {
			lg.Error(err, "Failed to update cluster status")
			return &ctrl.Result{Requeue: true}, err
		}
	}

	return &ctrl.Result{}, nil
}
