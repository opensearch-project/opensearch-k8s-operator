package reconcilers

import (
	"context"
	"fmt"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"strings"
	"time"

	"github.com/banzaicloud/k8s-objectmatcher/patch"
	"github.com/banzaicloud/operator-tools/pkg/reconciler"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
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

const waitLimit = 2 * 60 * 60

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

	passwordSecret := builders.PasswordSecret(r.instance, password)
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
	//Checking for existing statefulset disksize

	//Default is PVC, or explicit check for PersistenceSource as PVC
	if nodePool.Persistence == nil || nodePool.Persistence.PersistenceSource.PVC != nil {
		existingDisk := existing.Spec.VolumeClaimTemplates[0].Spec.Resources.Requests.Storage().String()
		r.logger.Info("The existing statefulset VolumeClaimTemplate disk size is: " + existingDisk)
		r.logger.Info("The cluster definition nodePool disk size is: " + nodePool.DiskSize)
		if existingDisk == nodePool.DiskSize {
			r.logger.Info("The existing disk size " + existingDisk + " is same as passed in disk size " + nodePool.DiskSize)
		} else {
			if err := r.dealWithExpandingPvc(existing, nodePool, existingDisk); err != nil {
				return result, err
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
		!helpers.ContainsString(nodePool.Roles, "data") {
		sts.Spec.Template.Spec.Containers[0].Env = existing.Spec.Template.Spec.Containers[0].Env
	}

	// Finally we enforce the desired state
	return r.ReconcileResource(sts, reconciler.StatePresent)
}

func (r *ClusterReconciler) dealWithExpandingPvc(existing *appsv1.StatefulSet, nodePool opsterv1.NodePool, existingDisk string) error {
	annotations := map[string]string{"cluster-name": r.instance.GetName()}
	r.recorder.AnnotatedEventf(r.instance, annotations, "Normal", "PVC", "Starting to resize PVC %s/%s from %s to  %s ", existing.Namespace, existing.Name, existingDisk, nodePool.DiskSize)

	//1. Removing statefulset while reserving Pods
	r.logger.Info("deleting statefulset while not orphaning pods " + existing.Name)
	opts := client.DeleteOptions{}
	client.PropagationPolicy(metav1.DeletePropagationOrphan).ApplyToDelete(&opts)
	if err := r.Delete(r.ctx, existing, &opts); err != nil {
		r.logger.Info("failed to delete statefulset" + existing.Name)
		return err
	}

	//2. Getting pods
	pods := corev1.PodList{}
	if err := r.Client.List(r.ctx,
		&pods,
		&client.ListOptions{
			Namespace:     r.instance.Namespace,
			LabelSelector: labels.SelectorFromSet(existing.Labels),
		},
	); err != nil {
		return err
	}

	for _, item := range pods.Items {
		//2.1 Getting pvc
		r.logger.Info("start to get pvc")
		pvc, err1 := r.getPVC(item, nodePool, existing)
		if err1 != nil {
			r.logger.Info("failed to get pvc, pod name:" + item.Name)
			return err1
		}

		//2.2 Deleting pod
		r.logger.Info("start to delete pod: " + item.Name)
		err2 := r.Delete(r.ctx, &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      item.Name,
				Namespace: item.Namespace,
			},
		})
		if err2 != nil {
			r.logger.Info("failed to delete pod: " + item.Name)
			return err2
		}

		//2.3 Expanding pvc
		r.logger.Info("start to expand pvc: " + pvc.Name)
		err3 := r.doExpandPVC(pvc, nodePool, existing, annotations)
		if err3 != nil {
			r.logger.Info("failed to expand pvc: " + pvc.Name)
			return err3
		}

		//2.4 Recreating pod
		r.logger.Info("start to recreate pod: " + item.Name)
		err := r.doRebuildPod(item)
		if err != nil {
			r.logger.Info("failed to rereating pod: " + item.Name)
			r.logger.Info("err : " + err.Error())
		}
	}
	return nil
}

func (r *ClusterReconciler) doRebuildPod(pod corev1.Pod) error {
	newPod := pod
	newPod.Annotations = nil
	newPod.ResourceVersion = ""
	newPod.UID = ""
	newPod.DeletionTimestamp = nil
	newPod.OwnerReferences = nil
	newPod.Status = corev1.PodStatus{}

	err1 := r.Create(r.ctx, &newPod)
	if err1 != nil {
		r.logger.Info(newPod.Name+"create failed ", err1)
		return err1
	}

	err2 := Retry(time.Second*2, time.Duration(waitLimit)*time.Second, func() (bool, error) {
		var currentPod corev1.Pod
		if err3 := r.Get(r.ctx, client.ObjectKeyFromObject(&pod), &currentPod); err3 != nil {
			return false, err3
		}
		if currentPod.Status.Phase != "Running" {
			r.logger.Info(currentPod.Name + " is not running yet")
			return false, nil
		}
		for _, c := range currentPod.Status.ContainerStatuses {
			if !c.Ready {
				r.logger.Info(currentPod.Name + "|" + c.Image + " is not ready yet")
				return false, nil
			}
		}
		return true, nil
	})
	return err2
}

func (r *ClusterReconciler) getPVC(pod corev1.Pod, nodePool opsterv1.NodePool, existing *appsv1.StatefulSet) (corev1.PersistentVolumeClaim, error) {
	split := strings.Split(pod.Name, "-")
	pvcName := fmt.Sprintf("data-%s-%s-%s", r.instance.Name, nodePool.Component, split[len(split)-1])

	r.logger.Info("The pvc identified as " + pvcName)
	var pvc corev1.PersistentVolumeClaim
	nsn := types.NamespacedName{
		Namespace: existing.Namespace,
		Name:      pvcName,
	}
	if err := r.Get(r.ctx, nsn, &pvc); err != nil {
		return pvc, err
	}
	return pvc, nil
}

func (r *ClusterReconciler) doExpandPVC(pvc corev1.PersistentVolumeClaim, nodePool opsterv1.NodePool, existing *appsv1.StatefulSet, annotations map[string]string) error {
	newDiskSize, err := resource.ParseQuantity(nodePool.DiskSize)
	if err != nil {
		r.logger.Info("failed to parse size " + nodePool.DiskSize)
		return err
	}

	pvc.Spec.Resources.Requests["storage"] = newDiskSize

	if err := r.Update(r.ctx, &pvc); err != nil {
		r.logger.Info("failed to resize statefulset pvc " + pvc.Name)
		r.recorder.AnnotatedEventf(r.instance, annotations, "Warning", "PVC", "Failed to Resize %s/%s", existing.Namespace, existing.Name)
		return err
	}

	if err := Retry(time.Second*2, time.Duration(waitLimit)*time.Second, func() (bool, error) {
		// Check the pvc status.
		var currentPVC corev1.PersistentVolumeClaim
		if err2 := r.Get(r.ctx, client.ObjectKeyFromObject(&pvc), &currentPVC); err2 != nil {
			return true, err2
		}
		var conditons = currentPVC.Status.Conditions
		capacity := currentPVC.Status.Capacity
		// Notice: When expanding not start, or been completed, conditons is nil
		if conditons == nil {
			// If change storage request when replicas are creating, should check the currentPVC.Status.Capacity.
			// for example:
			// Pod0 has created successful,but Pod1 is creating. then change PVC from 20Gi to 30Gi .
			// Pod0's PVC need to expand, but Pod1's PVC has created as 30Gi, so need to skip it.

			if equality.Semantic.DeepEqual(capacity, pvc.Spec.Resources.Requests) {
				r.logger.Info("Executing expand PVC【", pvc.Name, "】 completed")
				return true, nil
			}
			r.logger.Info("Executing expand PVC【", pvc.Name, "】 not start")
			return false, nil
		}
		status := conditons[0].Type
		storage := capacity.Storage()
		r.logger.Info("Executing expand PVC【" + pvc.Name + "】, current【" + storage.String() + "】, target【" + newDiskSize.String() + "】, status【" + string(status) + "】")
		if status == "FileSystemResizePending" {
			return true, nil
		}
		return false, nil
	}); err != nil {
		return err
	}
	return nil
}

func (r *ClusterReconciler) DeleteResources() (ctrl.Result, error) {
	// deleting pvc
	pvcLabels := map[string]string{
		builders.ClusterLabel: r.instance.Name,
	}

	pvcs := corev1.PersistentVolumeClaimList{}
	if err := r.Client.List(r.ctx,
		&pvcs,
		&client.ListOptions{
			Namespace:     r.instance.Namespace,
			LabelSelector: labels.SelectorFromSet(pvcLabels),
		},
	); err != nil {
		return ctrl.Result{}, err
	}

	r.logger.Info("start to delete pvcs.")
	for i := range pvcs.Items {
		pvc := pvcs.Items[i]
		r.logger.Info("deleting " + pvc.Name)
		if err := r.Delete(r.ctx, &pvc); err != nil {
			return ctrl.Result{}, err
		}
	}
	r.logger.Info("finished deleting pvcs.")
	return ctrl.Result{}, nil
}

// retry runs func "f" every "in" time until "limit" is reached.
// it also doesn't have an extra tail wait after the limit is reached
// and f func runs first time instantly
func Retry(in, limit time.Duration, f func() (bool, error)) error {
	fdone, err := f()
	if err != nil {
		return err
	}
	if fdone {
		return nil
	}

	done := time.NewTimer(limit)
	defer done.Stop()
	tk := time.NewTicker(in)
	defer tk.Stop()

	for {
		select {
		case <-done.C:
			return fmt.Errorf("reach pod wait limit")
		case <-tk.C:
			fdone, err := f()
			if err != nil {
				return err
			}
			if fdone {
				return nil
			}
		}
	}
}
