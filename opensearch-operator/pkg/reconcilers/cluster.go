package reconcilers

import (
	"context"
	"fmt"

	"github.com/banzaicloud/k8s-objectmatcher/patch"
	"github.com/banzaicloud/operator-tools/pkg/reconciler"
	"github.com/go-logr/logr"
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

	return result.Result, result.Err
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
	if helpers.ParallelRecoveryMode() && (nodePool.Persistence == nil || nodePool.Persistence.PersistenceSource.PVC != nil) {
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
			if pvcCount >= int(nodePool.Replicas) && existing.Status.Replicas < nodePool.Replicas-1 {
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

	// Handle PVC resizing

	//Default is PVC, or explicit check for PersistenceSource as PVC
	if nodePool.Persistence == nil || nodePool.Persistence.PersistenceSource.PVC != nil {
		existingDisk := existing.Spec.VolumeClaimTemplates[0].Spec.Resources.Requests.Storage().String()
		r.logger.Info("The existing statefulset VolumeClaimTemplate disk size is: " + existingDisk)
		r.logger.Info("The cluster definition nodePool disk size is: " + nodePool.DiskSize)
		if nodePool.DiskSize == "" { // Default case
			nodePool.DiskSize = builders.DefaultDiskSize
		}
		if existingDisk == nodePool.DiskSize {
			r.logger.Info("The existing disk size " + existingDisk + " is same as passed in disk size " + nodePool.DiskSize)
		} else {
			annotations := map[string]string{"cluster-name": r.instance.GetName()}
			r.recorder.AnnotatedEventf(r.instance, annotations, "Normal", "PVC", "Starting to resize PVC %s/%s from %s to  %s ", existing.Namespace, existing.Name, existingDisk, nodePool.DiskSize)
			//Removing statefulset while allowing pods to run
			r.logger.Info("deleting statefulset while orphaning pods " + existing.Name)
			opts := client.DeleteOptions{}
			client.PropagationPolicy(metav1.DeletePropagationOrphan).ApplyToDelete(&opts)
			if err := r.Delete(r.ctx, existing, &opts); err != nil {
				r.logger.Info("failed to delete statefulset" + existing.Name)
				return result, err
			}
			//Identifying the PVC per statefulset pod and patching the new size
			for i := 0; i < int(*existing.Spec.Replicas); i++ {
				clusterName := r.instance.Name
				claimName := fmt.Sprintf("data-%s-%s-%d", clusterName, nodePool.Component, i)
				r.logger.Info("The claimName identified as " + claimName)
				var pvc corev1.PersistentVolumeClaim
				nsn := types.NamespacedName{
					Namespace: existing.Namespace,
					Name:      claimName,
				}
				if err := r.Get(r.ctx, nsn, &pvc); err != nil {
					r.logger.Info("failed to get pvc" + pvc.Name)
					return result, err
				}
				newDiskSize, err := resource.ParseQuantity(nodePool.DiskSize)
				if err != nil {
					r.logger.Info("failed to parse size " + nodePool.DiskSize)
					return result, err
				}

				pvc.Spec.Resources.Requests["storage"] = newDiskSize

				if err := r.Update(r.ctx, &pvc); err != nil {
					r.logger.Info("failed to resize statefulset pvc " + pvc.Name)
					r.recorder.AnnotatedEventf(r.instance, annotations, "Warning", "PVC", "Failed to Resize %s/%s", existing.Namespace, existing.Name)
					return result, err
				}
			}

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
