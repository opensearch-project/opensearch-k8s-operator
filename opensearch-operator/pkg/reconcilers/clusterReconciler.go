package reconcilers

import (
	"context"

	"github.com/banzaicloud/operator-tools/pkg/reconciler"
	"k8s.io/client-go/tools/record"
	opsterv1 "opensearch.opster.io/api/v1"
	"opensearch.opster.io/pkg/builders"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type State struct {
	Component string `json:"component,omitempty"`
	Status    string `json:"status,omitempty"`
	Err       error  `json:"err,omitempty"`
}

type ClusterReconciler struct {
	client.Client
	reconciler.ResourceReconciler
	ctx      context.Context
	recorder record.EventRecorder
	state    State
	instance *opsterv1.OpenSearchCluster
}

func NewClusterReconciler(
	client client.Client,
	ctx context.Context,
	recorder record.EventRecorder,
	instance *opsterv1.OpenSearchCluster,
	opts ...reconciler.ResourceReconcilerOption,
) *ClusterReconciler {
	return &ClusterReconciler{
		Client: client,
		ResourceReconciler: reconciler.NewReconcilerWith(client,
			append(opts, reconciler.WithLog(log.FromContext(ctx)))...),
		ctx:      ctx,
		recorder: recorder,
		state:    State{},
		instance: instance,
	}
}

func (r *ClusterReconciler) Reconcile() (ctrl.Result, error) {
	//lg := log.FromContext(r.ctx)
	result := reconciler.CombinedResult{}

	clusterCm := builders.NewCmForCR(r.instance)
	result.Combine(r.ReconcileResource(clusterCm, reconciler.StatePresent))

	headlessService := builders.NewHeadlessServiceForCR(r.instance)
	result.Combine(r.ReconcileResource(headlessService, reconciler.StatePresent))

	clusterService := builders.NewServiceForCR(r.instance)
	result.Combine(r.ReconcileResource(clusterService, reconciler.StatePresent))

	NodesCount := len(r.instance.Spec.NodePools)

	for x := 0; x < NodesCount; x++ {
		sts := builders.NewSTSForCR(r.instance, r.instance.Spec.NodePools[x])
		result.Combine(r.ReconcileResource(sts, reconciler.StatePresent))
	}

	return result.Result, result.Err
}
