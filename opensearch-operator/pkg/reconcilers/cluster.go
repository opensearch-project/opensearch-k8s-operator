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

type ClusterReconciler struct {
	client.Client
	reconciler.ResourceReconciler
	ctx               context.Context
	recorder          record.EventRecorder
	reconcilerContext *ReconcilerContext
	instance          *opsterv1.OpenSearchCluster
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
			append(opts, reconciler.WithLog(log.FromContext(ctx).WithValues("reconciler", "cluster")))...),
		ctx:               ctx,
		recorder:          recorder,
		reconcilerContext: reconcilerContext,
		instance:          instance,
	}
}

func (r *ClusterReconciler) Reconcile() (ctrl.Result, error) {
	//lg := log.FromContext(r.ctx)
	result := reconciler.CombinedResult{}

	clusterService := builders.NewServiceForCR(r.instance)
	result.Combine(r.ReconcileResource(clusterService, reconciler.StatePresent))

	for _, nodePool := range r.instance.Spec.NodePools {
		headlessService := builders.NewHeadlessServiceForNodePool(r.instance, &nodePool)
		result.Combine(r.ReconcileResource(headlessService, reconciler.StatePresent))

		sts := builders.NewSTSForNodePool(r.instance, nodePool, r.reconcilerContext.Volumes, r.reconcilerContext.VolumeMounts)
		result.Combine(r.ReconcileResource(sts, reconciler.StatePresent))
	}

	return result.Result, result.Err
}
