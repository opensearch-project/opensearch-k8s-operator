package reconcilers

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/utils/pointer"
	"opensearch.opster.io/opensearch-gateway/services"
	"opensearch.opster.io/pkg/builders"

	"github.com/banzaicloud/operator-tools/pkg/reconciler"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/client-go/tools/record"
	opsterv1 "opensearch.opster.io/api/v1"
	"opensearch.opster.io/pkg/helpers"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type ScalerReconciler struct {
	client.Client
	reconciler.ResourceReconciler
	ctx               context.Context
	recorder          record.EventRecorder
	reconcilerContext *ReconcilerContext
	instance          *opsterv1.OpenSearchCluster
}

func NewScalerReconciler(
	client client.Client,
	ctx context.Context,
	recorder record.EventRecorder,
	reconcilerContext *ReconcilerContext,
	instance *opsterv1.OpenSearchCluster,
	opts ...reconciler.ResourceReconcilerOption,
) *ScalerReconciler {
	return &ScalerReconciler{
		Client: client,
		ResourceReconciler: reconciler.NewReconcilerWith(client,
			append(opts, reconciler.WithLog(log.FromContext(ctx).WithValues("reconciler", "scaler")))...),
		ctx:               ctx,
		recorder:          recorder,
		reconcilerContext: reconcilerContext,
		instance:          instance,
	}
}

func (r *ScalerReconciler) Reconcile() (ctrl.Result, error) {
	requeue := false
	results := &reconciler.CombinedResult{}
	var err error
	for _, nodePool := range r.instance.Spec.NodePools {
		requeue, err = r.reconcileNodePool(&nodePool)
		if err != nil {
			results.Combine(&ctrl.Result{Requeue: requeue}, err)
		}
	}
	results.Combine(&ctrl.Result{Requeue: requeue}, nil)

	// Clean up old node pools
	r.cleanupStatefulSets(results)

	return results.Result, results.Err
}

func (r *ScalerReconciler) reconcileNodePool(nodePool *opsterv1.NodePool) (bool, error) {
	namespace := r.instance.Namespace
	sts_name := builders.StsName(r.instance, nodePool)
	currentSts := appsv1.StatefulSet{}
	if err := r.Get(r.ctx, client.ObjectKey{Name: sts_name, Namespace: namespace}, &currentSts); err != nil {
		return false, err
	}

	var desireReplicaDiff = *currentSts.Spec.Replicas - nodePool.Replicas
	if desireReplicaDiff == 0 {
		return false, nil
	}
	componentStatus := opsterv1.ComponentStatus{
		Component:   "Scaler",
		Description: nodePool.Component,
	}
	comp := r.instance.Status.ComponentsStatus
	currentStatus, found := helpers.FindFirstPartial(comp, componentStatus, helpers.GetByDescriptionAndGroup)
	if !found {
		if desireReplicaDiff > 0 {
			r.recorder.Event(r.instance, "Normal", "Scaler", "Starting to scaling")
			if !r.instance.Spec.ConfMgmt.SmartScaler {
				requeue, err := r.decreaseOneNode(currentStatus, currentSts, nodePool.Component, r.instance.Spec.ConfMgmt.SmartScaler)
				r.recorder.Event(r.instance, "Normal", "Scaler", "Notice - your SmartScaler is not enable")
				r.recorder.Event(r.instance, "Normal", "Scaler", "Starting to decrease node")
				return requeue, err
			}
			err := r.excludeNode(currentStatus, currentSts, nodePool.Component)
			return true, err

		}
		if desireReplicaDiff < 0 {
			requeue, err := r.increaseOneNode(currentSts, nodePool.Component)
			return requeue, err
		}
	}
	if currentStatus.Status == "Excluded" {
		r.recorder.Event(r.instance, "Normal", "Scaler", fmt.Sprintf("Start to Exclude %s/%s", r.instance.Namespace, r.instance.Name))
		err := r.drainNode(currentStatus, currentSts, nodePool.Component)
		return true, err
	}
	if currentStatus.Status == "Drained" {
		r.recorder.Event(r.instance, "Normal", "Scaler", fmt.Sprintf("Start to Drain %s/%s", r.instance.Namespace, r.instance.Name))

		requeue, err := r.decreaseOneNode(currentStatus, currentSts, nodePool.Component, r.instance.Spec.ConfMgmt.SmartScaler)
		return requeue, err
	}
	return false, nil
}

func (r *ScalerReconciler) increaseOneNode(currentSts appsv1.StatefulSet, nodePoolGroupName string) (bool, error) {
	lg := log.FromContext(r.ctx)
	*currentSts.Spec.Replicas++
	lastReplicaNodeName := builders.ReplicaHostName(currentSts, *currentSts.Spec.Replicas)
	r.recorder.Event(r.instance, "Normal", "Scaler", fmt.Sprintf("Start increaseing node %s on %s ", lastReplicaNodeName, nodePoolGroupName))
	_, err := r.ReconcileResource(&currentSts, reconciler.StatePresent)
	if err != nil {
		r.recorder.Event(r.instance, "Normal", "Scaler", fmt.Sprintf("failed to add node %s/%s", r.instance.Namespace, r.instance.Name))
		return true, err
	}
	lg.Info(fmt.Sprintf("Group-%s . added node %s", nodePoolGroupName, lastReplicaNodeName))
	r.recorder.Event(r.instance, "Normal", "Scaler", fmt.Sprintf("Added new node %s", lastReplicaNodeName))
	return false, nil
}

func (r *ScalerReconciler) decreaseOneNode(currentStatus opsterv1.ComponentStatus, currentSts appsv1.StatefulSet, nodePoolGroupName string, smartDecrease bool) (bool, error) {
	lg := log.FromContext(r.ctx)
	*currentSts.Spec.Replicas--
	lastReplicaNodeName := builders.ReplicaHostName(currentSts, *currentSts.Spec.Replicas)
	r.recorder.Event(r.instance, "Normal", "Scaler", fmt.Sprintf("Start to decreaseing node %s on %s ", lastReplicaNodeName, nodePoolGroupName))
	_, err := r.ReconcileResource(&currentSts, reconciler.StatePresent)
	if err != nil {
		r.recorder.Event(r.instance, "Normal", "Scaler", fmt.Sprintf("Failed to remove node - Group-%s . Failed to remove node %s", nodePoolGroupName, lastReplicaNodeName))
		lg.Error(err, fmt.Sprintf("failed to remove node %s", lastReplicaNodeName))
		return true, err
	}
	lg.Info(fmt.Sprintf("Group-%s . removed node %s", nodePoolGroupName, lastReplicaNodeName))
	r.instance.Status.ComponentsStatus = helpers.RemoveIt(currentStatus, r.instance.Status.ComponentsStatus)
	err = r.Status().Update(r.ctx, r.instance)
	if err != nil {
		lg.Error(err, "failed to update status")
		return false, err
	}

	if !smartDecrease {
		return false, err
	}
	username, password, err := helpers.UsernameAndPassword(r.ctx, r.Client, r.instance)
	if err != nil {
		return true, err
	}
	service, created, err := r.CreateNodePortServiceIfNotExists()
	if err != nil {
		return true, err
	}
	clusterClient, err := services.NewOsClusterClient(builders.URLForCluster(r.instance), username, password)
	if err != nil {
		lg.Error(err, "failed to create os client")
		r.recorder.Event(r.instance, "WARN", "failed to remove node exclude", fmt.Sprintf("Group-%s . failed to remove node exclude %s", nodePoolGroupName, lastReplicaNodeName))
		if created {
			r.DeleteNodePortService(service)
		}
		return true, err
	}
	success, err := services.RemoveExcludeNodeHost(clusterClient, lastReplicaNodeName)
	if !success || err != nil {
		lg.Error(err, fmt.Sprintf("failed to remove exclude node %s", lastReplicaNodeName))
		r.recorder.Event(r.instance, "Warning", "Scaler", fmt.Sprintf("Failed to remove node exclude - Group-%s , node  %s", nodePoolGroupName, lastReplicaNodeName))
	}
	if created {
		r.DeleteNodePortService(service)
	}
	return false, err
}

func (r *ScalerReconciler) excludeNode(currentStatus opsterv1.ComponentStatus, currentSts appsv1.StatefulSet, nodePoolGroupName string) error {
	lg := log.FromContext(r.ctx)
	username, password, err := helpers.UsernameAndPassword(r.ctx, r.Client, r.instance)
	if err != nil {
		return err
	}
	service, created, err := r.CreateNodePortServiceIfNotExists()
	if err != nil {
		return err
	}

	// Clean up created service at the end of the function
	defer func() {
		if created {
			r.DeleteNodePortService(service)
		}
	}()

	clusterClient, err := services.NewOsClusterClient(fmt.Sprintf("https://localhost:%d", service.Spec.Ports[0].NodePort), username, password)
	if err != nil {
		lg.Error(err, "failed to create os client")
		r.recorder.Event(r.instance, "Warning", "Scaler", "Failed to create os client for scaling")
		return err
	}
	// -----  Now start remove node ------
	lastReplicaNodeName := builders.ReplicaHostName(currentSts, *currentSts.Spec.Replicas-1)

	excluded, err := services.AppendExcludeNodeHost(clusterClient, lastReplicaNodeName)
	if err != nil {
		lg.Error(err, fmt.Sprintf("failed to exclude node %s", lastReplicaNodeName))
		return err
	}
	if excluded {
		componentStatus := opsterv1.ComponentStatus{
			Component:   "Scaler",
			Status:      "Excluded",
			Description: nodePoolGroupName,
		}
		r.recorder.Event(r.instance, "Normal", "Scaler", fmt.Sprintf("Finished to Exclude %s/%s", r.instance.Namespace, r.instance.Name))
		lg.Info(fmt.Sprintf("Group-%s .excluded node %s", nodePoolGroupName, lastReplicaNodeName))
		r.instance.Status.ComponentsStatus = helpers.Replace(currentStatus, componentStatus, r.instance.Status.ComponentsStatus)
		err = r.Status().Update(r.ctx, r.instance)
		if err != nil {
			lg.Error(err, "failed to update status")
			r.recorder.Event(r.instance, "Warning", "Scaler", "Failed to update operator status")
			return err
		}

		return err
	}

	componentStatus := opsterv1.ComponentStatus{
		Component:   "Scaler",
		Status:      "Running",
		Description: nodePoolGroupName,
	}
	r.recorder.Event(r.instance, "Normal", "Scaler", fmt.Sprintf("Start sacle %s/%s from %d to %d", r.instance.Namespace, r.instance.Name, *currentSts.Spec.Replicas, *currentSts.Spec.Replicas-1))
	lg.Info(fmt.Sprintf("Group-%s . Failed to exclude node %s", nodePoolGroupName, lastReplicaNodeName))
	r.instance.Status.ComponentsStatus = helpers.Replace(currentStatus, componentStatus, r.instance.Status.ComponentsStatus)
	err = r.Status().Update(r.ctx, r.instance)
	if err != nil {
		r.recorder.Event(r.instance, "Warning", "Scaler", fmt.Sprintf("Group-%s . failed to remove node exclude %s", nodePoolGroupName, lastReplicaNodeName))
		lg.Error(err, "failed to update status")
		return err
	}

	return err
}

func (r *ScalerReconciler) drainNode(currentStatus opsterv1.ComponentStatus, currentSts appsv1.StatefulSet, nodePoolGroupName string) error {
	lg := log.FromContext(r.ctx)
	lastReplicaNodeName := builders.ReplicaHostName(currentSts, *currentSts.Spec.Replicas-1)
	username, password, err := helpers.UsernameAndPassword(r.ctx, r.Client, r.instance)
	if err != nil {
		return err
	}
	service, created, err := r.CreateNodePortServiceIfNotExists()
	if err != nil {
		return err
	}

	// Clean up created service at the end of the function
	defer func() {
		if created {
			r.DeleteNodePortService(service)
		}
	}()

	clusterClient, err := services.NewOsClusterClient(fmt.Sprintf("https://localhost:%d", service.Spec.Ports[0].NodePort), username, password)
	if err != nil {
		return err
	}
	nodeNotEmpty, err := services.HasShardsOnNode(clusterClient, lastReplicaNodeName)
	if nodeNotEmpty {
		lg.Info(fmt.Sprintf("Group-%s . draining node %s", nodePoolGroupName, lastReplicaNodeName))
		return err
	}
	success, err := services.RemoveExcludeNodeHost(clusterClient, lastReplicaNodeName)
	if !success {
		r.recorder.Event(r.instance, "Normal", "Scaler", fmt.Sprintf("Group-%s . node %s node is empty but node is still excluded from allocation", nodePoolGroupName, lastReplicaNodeName))
		return err
	}

	componentStatus := opsterv1.ComponentStatus{
		Component:   "Scaler",
		Status:      "Drained",
		Description: nodePoolGroupName,
	}
	lg.Info(fmt.Sprintf("Group-%s .node %s node is drained", nodePoolGroupName, lastReplicaNodeName))
	r.instance.Status.ComponentsStatus = helpers.Replace(currentStatus, componentStatus, r.instance.Status.ComponentsStatus)
	err = r.Status().Update(r.ctx, r.instance)
	if err != nil {
		lg.Error(err, "failed to update status")
		r.recorder.Event(r.instance, "Warning", "Scaler", "Failed to update operator status")
		return err
	}
	return err
}

func (r *ScalerReconciler) CreateNodePortServiceIfNotExists() (corev1.Service, bool, error) {
	lg := log.FromContext(r.ctx)
	namespace := r.instance.Namespace
	targetService := builders.NewNodePortService(r.instance)
	existingService := corev1.Service{}
	if err := r.Get(r.ctx, client.ObjectKey{Name: targetService.Name, Namespace: namespace}, &existingService); err != nil {
		if err := ctrl.SetControllerReference(r.instance, targetService, r.Client.Scheme()); err != nil {
			return *targetService, false, err
		}
		err = r.Create(r.ctx, targetService)
		if err != nil {
			if !errors.IsAlreadyExists(err) {
				lg.Error(err, "Cannot create service")
				r.recorder.Event(r.instance, "Warning", "Scaler", "Cannot create Headless service -  Requeue - Fix the problem you have on main Opensearch Headless Service ")
				return *targetService, false, err
			}
		}
		lg.Info("service created successfully")
		return *targetService, true, nil
	}
	return existingService, false, nil
}

func (r *ScalerReconciler) DeleteNodePortService(service corev1.Service) {
	lg := log.FromContext(r.ctx)
	err := r.Delete(r.ctx, &service)
	if err != nil {
		lg.Error(err, "Cannot delete service")
		r.recorder.Event(r.instance, "Warning", "Scaler", "Cannot delete service - Requeue - Fix the problem you have on main Opensearch Headless Service ")
	}
}

func (r *ScalerReconciler) cleanupStatefulSets(result *reconciler.CombinedResult) {
	stsList := &appsv1.StatefulSetList{}
	if err := r.Client.List(
		r.ctx,
		stsList,
		client.InNamespace(r.instance.Name),
		client.MatchingLabels{builders.ClusterLabel: r.instance.Name},
	); err != nil {
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
	if !r.instance.Spec.ConfMgmt.SmartScaler {
		return r.ReconcileResource(&sts, reconciler.StateAbsent)
	}

	// Gracefully remove nodes
	lg := log.FromContext(r.ctx)
	username, password, err := helpers.UsernameAndPassword(r.ctx, r.Client, r.instance)
	if err != nil {
		return nil, err
	}

	clusterClient, err := services.NewOsClusterClient(fmt.Sprintf("https://%s.%s:9200", r.instance.Spec.General.ServiceName, r.instance.Name), username, password)
	if err != nil {
		lg.Error(err, "failed to create os client")
		r.recorder.Event(r.instance, "Warning", "Scaler", "Failed to create os client")
		return nil, err
	}
	r.recorder.Event(r.instance, "Normal", "Scaler", "Finished os client for scaling ")

	workingOrdinal := pointer.Int32Deref(sts.Spec.Replicas, 1) - 1
	lastReplicaNodeName := builders.ReplicaHostName(sts, workingOrdinal)
	_, err = services.AppendExcludeNodeHost(clusterClient, lastReplicaNodeName)
	if err != nil {
		lg.Error(err, fmt.Sprintf("failed to exclude node %s", lastReplicaNodeName))
		return nil, err
	}

	nodeNotEmpty, err := services.HasShardsOnNode(clusterClient, lastReplicaNodeName)
	if err != nil {
		lg.Error(err, "failed to check shards on node")
		r.recorder.Event(r.instance, "Warning", "Scaler", "Failed to check shards on node")
		return nil, err
	}

	if nodeNotEmpty {
		return &ctrl.Result{
			Requeue:      true,
			RequeueAfter: 15 * time.Second,
		}, nil
	}

	if workingOrdinal == 0 {
		result, err := r.ReconcileResource(&sts, reconciler.StateAbsent)
		if err != nil {
			return result, err
		}
		_, err = services.RemoveExcludeNodeHost(clusterClient, lastReplicaNodeName)
		if err != nil {
			lg.Error(err, fmt.Sprintf("failed to remove node exclusion for %s", lastReplicaNodeName))
		}
		return result, err
	}

	sts.Spec.Replicas = &workingOrdinal
	result, err := r.ReconcileResource(&sts, reconciler.StatePresent)
	if err != nil {
		return result, err
	}

	_, err = services.RemoveExcludeNodeHost(clusterClient, lastReplicaNodeName)
	if err != nil {
		lg.Error(err, fmt.Sprintf("failed to remove node exclusion for %s", lastReplicaNodeName))
	}
	r.recorder.Event(r.instance, "Noraml", "Scaler", "Finished scaling")
	return result, err
}
