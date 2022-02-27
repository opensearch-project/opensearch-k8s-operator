package controllers

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	opsterv1 "opensearch.opster.io/api/v1"
	"opensearch.opster.io/opensearch-gateway/services"
	"opensearch.opster.io/pkg/builders"
	"opensearch.opster.io/pkg/helpers"

	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ScalerReconciler struct {
	client.Client
	Recorder record.EventRecorder
	logr.Logger
	Instance *opsterv1.OpenSearchCluster
}

//+kubebuilder:rbac:groups="opensearch.opster.io",resources=events,verbs=create;patch
//+kubebuilder:rbac:groups=opensearch.opster.io,resources=opensearchcluster,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=opensearch.opster.io,resources=opensearchcluster/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=opensearch.opster.io,resources=opensearchcluster/finalizers,verbs=update

func (r *ScalerReconciler) Reconcile(controllerContext *ControllerContext) ([]opsterv1.ComponentStatus, error) {
	var statusList []opsterv1.ComponentStatus
	for _, nodePool := range r.Instance.Spec.NodePools {
		status, err := r.reconcileNodePool(&nodePool, controllerContext)
		if status != nil {
			statusList = append(statusList, *status)
		}
		if err != nil {
			return statusList, err
		}
	}
	return statusList, nil
}

func (r *ScalerReconciler) reconcileNodePool(nodePool *opsterv1.NodePool, controllerContext *ControllerContext) (*opsterv1.ComponentStatus, error) {
	namespace := r.Instance.Spec.General.ClusterName
	sts_name := builders.StsName(r.Instance, nodePool)
	currentSts := appsv1.StatefulSet{}

	if err := r.Get(context.TODO(), client.ObjectKey{Name: sts_name, Namespace: namespace}, &currentSts); err != nil {
		return nil, err
	}

	var desireReplicaDiff = *currentSts.Spec.Replicas - nodePool.Replicas
	if desireReplicaDiff == 0 {
		return nil, nil
	}
	componentStatus := opsterv1.ComponentStatus{
		Component:   "Scaler",
		Description: nodePool.Component,
	}
	comp := r.Instance.Status.ComponentsStatus
	currentStatus, found := helpers.FindFirstPartial(comp, componentStatus, getByDescriptionAndGroup)
	if !found {
		if desireReplicaDiff > 0 {
			status, err := r.excludeNode(context.TODO(), currentStatus, currentSts, nodePool.Component)
			return status, err

		}
		if desireReplicaDiff < 0 {
			status, err := r.increaseOneNode(context.TODO(), currentSts, nodePool.Component)
			return status, err
		}
	}
	if currentStatus.Status == "Excluded" {
		status, err := r.drainNode(context.TODO(), currentStatus, currentSts, nodePool.Component)
		return status, err
	}
	if currentStatus.Status == "Drained" {
		status, err := r.decreaseOneNode(context.TODO(), currentStatus, currentSts, nodePool.Component)
		return status, err
	}
	return nil, nil
}

func (r *ScalerReconciler) increaseOneNode(ctx context.Context, currentSts appsv1.StatefulSet, nodePoolGroupName string) (*opsterv1.ComponentStatus, error) {
	// -----  Now start add node ------
	*currentSts.Spec.Replicas++
	lastReplicaNodeName := fmt.Sprintf("%s-%d", currentSts.ObjectMeta.Name, currentSts.Spec.Replicas)
	if err := r.Update(ctx, &currentSts); err != nil {
		r.Recorder.Event(r.Instance, "Normal", "failed to add node ", fmt.Sprintf("Group name-%s . Failed to add node %s", currentSts.Name, lastReplicaNodeName))
		return nil, err
	}
	r.Recorder.Event(r.Instance, "Normal", "added node ", fmt.Sprintf("Group-%s . added node %s", nodePoolGroupName, lastReplicaNodeName))
	return nil, nil
}

func (r *ScalerReconciler) decreaseOneNode(ctx context.Context, currentStatus opsterv1.ComponentStatus, currentSts appsv1.StatefulSet, nodePoolGroupName string) (*opsterv1.ComponentStatus, error) {
	// -----  Now start add node ------
	*currentSts.Spec.Replicas--
	lastReplicaNodeName := fmt.Sprintf("%s-%d", currentSts.ObjectMeta.Name, *currentSts.Spec.Replicas)
	if err := r.Update(ctx, &currentSts); err != nil {
		r.Recorder.Event(r.Instance, "Normal", "failed to remove node ", fmt.Sprintf("Group-%s . Failed to remove node %s", nodePoolGroupName, lastReplicaNodeName))
		r.Logger.Error(err, fmt.Sprintf("failed to remove node %s", lastReplicaNodeName))
		return nil, err
	}
	r.Recorder.Event(r.Instance, "Normal", "decrease node ", fmt.Sprintf("Group-%s . removed node %s", nodePoolGroupName, lastReplicaNodeName))
	r.Instance.Status.ComponentsStatus = helpers.RemoveIt(currentStatus, r.Instance.Status.ComponentsStatus)
	err := r.Status().Update(ctx, r.Instance)
	if err != nil {
		r.Recorder.Event(r.Instance, "WARN", "failed to remove node exclude", fmt.Sprintf("Group-%s . failed to remove node exclude %s", nodePoolGroupName, lastReplicaNodeName))
		return nil, err
	}
	username, password := builders.UsernameAndPassword(r.Instance)
	clusterClient, err := services.NewOsClusterClient(builders.ClusterUrl(r.Instance), username, password)
	if err != nil {
		r.Logger.Error(err, "failed to create os client")
		r.Recorder.Event(r.Instance, "WARN", "failed to remove node exclude", fmt.Sprintf("Group-%s . failed to remove node exclude %s", nodePoolGroupName, lastReplicaNodeName))
		return nil, err
	}
	success, err := services.RemoveExcludeNodeHost(clusterClient, lastReplicaNodeName)
	if !success || err != nil {
		r.Logger.Error(err, fmt.Sprintf("failed to remove exclude node %s", lastReplicaNodeName))
		r.Recorder.Event(r.Instance, "WARN", "failed to remove node exclude", fmt.Sprintf("Group-%s . failed to remove node exclude %s", nodePoolGroupName, lastReplicaNodeName))
	}
	return nil, err
}

func (r *ScalerReconciler) excludeNode(ctx context.Context, currentStatus opsterv1.ComponentStatus, currentSts appsv1.StatefulSet, nodePoolGroupName string) (*opsterv1.ComponentStatus, error) {
	username, password := builders.UsernameAndPassword(r.Instance)
	clusterClient, err := services.NewOsClusterClient(builders.ClusterUrl(r.Instance), username, password)
	if err != nil {
		r.Logger.Error(err, "failed to create os client")
		return nil, err
	}
	// -----  Now start remove node ------
	lastReplicaNodeName := fmt.Sprintf("%s-%d", currentSts.ObjectMeta.Name, *currentSts.Spec.Replicas-1)

	excluded, err := services.AppendExcludeNodeHost(clusterClient, lastReplicaNodeName)
	if err != nil {
		r.Logger.Error(err, fmt.Sprintf("failed to exclude node %s", lastReplicaNodeName))
		return nil, err
	}
	if err := r.Update(ctx, &currentSts); err != nil {
		return nil, err
	}
	if excluded {
		componentStatus := opsterv1.ComponentStatus{
			Component:   "Scaler",
			Status:      "Excluded",
			Description: nodePoolGroupName,
		}
		r.Recorder.Event(r.Instance, "Normal", "excluded node ", fmt.Sprintf("Group-%s .excluded node %s", nodePoolGroupName, lastReplicaNodeName))
		r.Instance.Status.ComponentsStatus = helpers.Replace(currentStatus, componentStatus, r.Instance.Status.ComponentsStatus)
		err = r.Status().Update(ctx, r.Instance)
		return &componentStatus, err
	}

	componentStatus := opsterv1.ComponentStatus{
		Component:   "Scaler",
		Status:      "Running",
		Description: nodePoolGroupName,
	}
	r.Recorder.Event(r.Instance, "Normal", "failed to exclude node ", fmt.Sprintf("Group-%s . Failed to exclude node %s", nodePoolGroupName, lastReplicaNodeName))
	r.Instance.Status.ComponentsStatus = helpers.Replace(currentStatus, componentStatus, r.Instance.Status.ComponentsStatus)
	err = r.Status().Update(ctx, r.Instance)
	return &componentStatus, err
}

func (r *ScalerReconciler) drainNode(ctx context.Context, currentStatus opsterv1.ComponentStatus, currentSts appsv1.StatefulSet, nodePoolGroupName string) (*opsterv1.ComponentStatus, error) {
	// -----  Now start add node ------
	lastReplicaNodeName := fmt.Sprintf("%s-%d", currentSts.ObjectMeta.Name, *currentSts.Spec.Replicas-1)

	username, password := builders.UsernameAndPassword(r.Instance)
	clusterClient, err := services.NewOsClusterClient(builders.ClusterUrl(r.Instance), username, password)
	if err != nil {
		return nil, err
	}
	nodeNotEmpty, err := services.HasShardsOnNode(clusterClient, lastReplicaNodeName)
	if nodeNotEmpty {
		r.Recorder.Event(r.Instance, "Normal", "draining node ", fmt.Sprintf("Group-%s . draining node %s", nodePoolGroupName, lastReplicaNodeName))
		return nil, err
	}
	success, err := services.RemoveExcludeNodeHost(clusterClient, lastReplicaNodeName)
	if !success {
		r.Recorder.Event(r.Instance, "Normal", "node is empty but node is still excluded from allocation", fmt.Sprintf("Group-%s . node %s node is empty but node is still excluded from allocation", nodePoolGroupName, lastReplicaNodeName))
		return nil, err
	}

	componentStatus := opsterv1.ComponentStatus{
		Component:   "Scaler",
		Status:      "Drained",
		Description: nodePoolGroupName,
	}
	r.Recorder.Event(r.Instance, "Normal", "node has drained", fmt.Sprintf("Group-%s .node %s node is drained", nodePoolGroupName, lastReplicaNodeName))
	r.Instance.Status.ComponentsStatus = helpers.Replace(currentStatus, componentStatus, r.Instance.Status.ComponentsStatus)
	err = r.Status().Update(ctx, r.Instance)
	return &componentStatus, err
}

func getByDescriptionAndGroup(left opsterv1.ComponentStatus, right opsterv1.ComponentStatus) (opsterv1.ComponentStatus, bool) {
	if left.Description == right.Description && left.Component == right.Component {
		return left, true
	}
	return right, false
}
