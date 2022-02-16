package controllers

import (
	"context"
	"fmt"
	sts "k8s.io/api/apps/v1"
	opsterv1 "opensearch.opster.io/api/v1"
	"opensearch.opster.io/opensearch-gateway/services"
	"opensearch.opster.io/pkg/builders"
	"opensearch.opster.io/pkg/helpers"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ScalerReconciler struct {
	client.Client
	Scheme     *runtime.Scheme
	Recorder   record.EventRecorder
	State      State
	Instance   *opsterv1.OpenSearchCluster
	StsFromEnv sts.StatefulSet
	Group      int
}

//+kubebuilder:rbac:groups="opensearch.opster.io",resources=events,verbs=create;patch
//+kubebuilder:rbac:groups=opensearch.opster.io,resources=opensearchcluster,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=opensearch.opster.io,resources=opensearchcluster/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=opensearch.opster.io,resources=opensearchcluster/finalizers,verbs=update

func (r *ScalerReconciler) Reconcile(ctx context.Context, request ctrl.Request) (ctrl.Result, error) {
	var desireReplicaDiff = *r.StsFromEnv.Spec.Replicas - r.Instance.Spec.NodePools[r.Group].Replicas

	group := fmt.Sprintf("Group-%d", r.Group)
	componentStatus := opsterv1.ComponentStatus{
		Component:   "Scaler",
		Description: group,
	}
	comp := r.Instance.Status.ComponentsStatus
	currentStatus, found := helpers.FindFirstPartial(comp, componentStatus, getByDescriptionAndGroup)
	if !found {
		if desireReplicaDiff > 0 {
			return r.excludeNode(ctx, currentStatus)
		}
		if desireReplicaDiff < 0 {
			return r.increaseOneNode(ctx)
		}
	}
	if currentStatus.Status == "Excluded" {
		return r.drainNode(ctx, currentStatus)
	}
	if currentStatus.Status == "Drained" {
		return r.decreaseOneNode(ctx, currentStatus)
	}
	return ctrl.Result{}, nil
}

func (r *ScalerReconciler) increaseOneNode(ctx context.Context) (ctrl.Result, error) {
	// -----  Now start add node ------
	*r.StsFromEnv.Spec.Replicas++
	lastReplicaNodeName := fmt.Sprintf("%s-%d", r.StsFromEnv.ObjectMeta.Name, *r.StsFromEnv.Spec.Replicas)
	if err := r.Update(ctx, &r.StsFromEnv); err != nil {
		r.Recorder.Event(r.Instance, "Normal", "failed to add node ", fmt.Sprintf("Group-%d . Failed to add node %s", r.Group, lastReplicaNodeName))
		return ctrl.Result{}, err
	}
	r.Recorder.Event(r.Instance, "Normal", "added node ", fmt.Sprintf("Group-%d . added node %s", r.Group, lastReplicaNodeName))
	return ctrl.Result{}, nil
}

func (r *ScalerReconciler) decreaseOneNode(ctx context.Context, currentStatus opsterv1.ComponentStatus) (ctrl.Result, error) {
	// -----  Now start add node ------
	*r.StsFromEnv.Spec.Replicas--
	lastReplicaNodeName := fmt.Sprintf("%s-%d", r.StsFromEnv.ObjectMeta.Name, *r.StsFromEnv.Spec.Replicas)
	if err := r.Update(ctx, &r.StsFromEnv); err != nil {
		r.Recorder.Event(r.Instance, "Normal", "failed to remove node ", fmt.Sprintf("Group-%d . Failed to remove node %s", r.Group, lastReplicaNodeName))
		return ctrl.Result{Requeue: true}, err
	}
	r.Recorder.Event(r.Instance, "Normal", "decrease node ", fmt.Sprintf("Group-%d . removed node %s", r.Group, lastReplicaNodeName))
	r.Instance.Status.ComponentsStatus = helpers.RemoveIt(currentStatus, r.Instance.Status.ComponentsStatus)
	err := r.Status().Update(ctx, r.Instance)
	clusterClient, err := builders.NewOsClusterClient(r.Instance)
	if err != nil {
		r.Recorder.Event(r.Instance, "WARN", "failed to remove node exclude", fmt.Sprintf("Group-%d . failed to remove node exclude %s", r.Group, lastReplicaNodeName))
		return ctrl.Result{}, err
	}
	success, err := services.RemoveExcludeNodeHost(clusterClient, lastReplicaNodeName)
	if !success || err != nil {
		r.Recorder.Event(r.Instance, "WARN", "failed to remove node exclude", fmt.Sprintf("Group-%d . failed to remove node exclude %s", r.Group, lastReplicaNodeName))
	}
	return ctrl.Result{}, err
}

func (r *ScalerReconciler) excludeNode(ctx context.Context, currentStatus opsterv1.ComponentStatus) (ctrl.Result, error) {
	clusterClient, err := builders.NewOsClusterClient(r.Instance)
	if err != nil {
		return ctrl.Result{}, err
	}
	// -----  Now start remove node ------
	lastReplicaNodeName := fmt.Sprintf("%s-%d", r.StsFromEnv.ObjectMeta.Name, *r.StsFromEnv.Spec.Replicas-1)

	excluded, err := services.AppendExcludeNodeHost(clusterClient, lastReplicaNodeName)
	if err != nil {
		return ctrl.Result{}, err
	}
	if err := r.Update(ctx, &r.StsFromEnv); err != nil {
		return ctrl.Result{Requeue: true}, err
	}
	group := fmt.Sprintf("Group-%d", r.Group)
	if excluded {
		componentStatus := opsterv1.ComponentStatus{
			Component:   "Scaler",
			Status:      "Excluded",
			Description: group,
		}
		r.Recorder.Event(r.Instance, "Normal", "excluded node ", fmt.Sprintf("Group-%d .excluded node %s", r.Group, lastReplicaNodeName))
		r.Instance.Status.ComponentsStatus = helpers.Replace(currentStatus, componentStatus, r.Instance.Status.ComponentsStatus)
		err = r.Status().Update(ctx, r.Instance)
		return ctrl.Result{Requeue: true}, err
	}

	componentStatus := opsterv1.ComponentStatus{
		Component:   "Scaler",
		Status:      "Running",
		Description: group,
	}
	r.Recorder.Event(r.Instance, "Normal", "failed to exclude node ", fmt.Sprintf("Group-%d . Failed to exclude node %s", r.Group, lastReplicaNodeName))
	r.Instance.Status.ComponentsStatus = helpers.Replace(currentStatus, componentStatus, r.Instance.Status.ComponentsStatus)
	err = r.Status().Update(ctx, r.Instance)
	return ctrl.Result{Requeue: true}, err
}

func (r *ScalerReconciler) drainNode(ctx context.Context, currentStatus opsterv1.ComponentStatus) (ctrl.Result, error) {
	// -----  Now start add node ------
	lastReplicaNodeName := fmt.Sprintf("%s-%d", r.StsFromEnv.ObjectMeta.Name, *r.StsFromEnv.Spec.Replicas-1)

	clusterClient, err := builders.NewOsClusterClient(r.Instance)
	if err != nil {
		return ctrl.Result{}, err
	}
	nodeNotEmpty, err := services.HasShardsOnNode(clusterClient, lastReplicaNodeName)
	if nodeNotEmpty {
		r.Recorder.Event(r.Instance, "Normal", "draining node ", fmt.Sprintf("Group-%d . draining node %s", r.Group, lastReplicaNodeName))
		return ctrl.Result{Requeue: true}, err
	}
	success, err := services.RemoveExcludeNodeHost(clusterClient, lastReplicaNodeName)
	if !success {
		r.Recorder.Event(r.Instance, "Normal", "node is empty but node is still excluded from allocation", fmt.Sprintf("Group-%d . node %s node is empty but node is still excluded from allocation", r.Group, lastReplicaNodeName))
		return ctrl.Result{Requeue: true}, err
	}
	group := fmt.Sprintf("Group-%d", r.Group)
	componentStatus := opsterv1.ComponentStatus{
		Component:   "Scaler",
		Status:      "Drained",
		Description: group,
	}
	r.Recorder.Event(r.Instance, "Normal", "node has drained", fmt.Sprintf("Group-%d .node %s node is drained", r.Group, lastReplicaNodeName))
	r.Instance.Status.ComponentsStatus = helpers.Replace(currentStatus, componentStatus, r.Instance.Status.ComponentsStatus)
	err = r.Status().Update(ctx, r.Instance)
	return ctrl.Result{Requeue: true}, err
}

func getByDescriptionAndGroup(left opsterv1.ComponentStatus, right opsterv1.ComponentStatus) (opsterv1.ComponentStatus, bool) {
	if left.Description == right.Description && left.Component == left.Component {
		return left, true
	}
	return right, false
}
