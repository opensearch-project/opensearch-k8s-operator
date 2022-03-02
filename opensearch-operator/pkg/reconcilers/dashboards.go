package reconcilers

import (
	"context"
	"fmt"

	"github.com/banzaicloud/operator-tools/pkg/reconciler"
	"k8s.io/client-go/tools/record"
	opsterv1 "opensearch.opster.io/api/v1"
	"opensearch.opster.io/pkg/builders"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type DashboardsReconciler struct {
	reconciler.ResourceReconciler
	client.Client
	ctx               context.Context
	recorder          record.EventRecorder
	reconcilerContext *ReconcilerContext
	instance          *opsterv1.OpenSearchCluster
}

func NewDashboardsReconciler(
	client client.Client,
	ctx context.Context,
	recorder record.EventRecorder,
	reconcilerContext *ReconcilerContext,
	instance *opsterv1.OpenSearchCluster,
	opts ...reconciler.ResourceReconcilerOption,
) *DashboardsReconciler {
	return &DashboardsReconciler{
		Client: client,
		ResourceReconciler: reconciler.NewReconcilerWith(client,
			append(opts, reconciler.WithLog(log.FromContext(ctx).WithValues("reconciler", "dashboards")))...),
		ctx:               ctx,
		reconcilerContext: reconcilerContext,
		recorder:          recorder,
		instance:          instance,
	}
}

func (r *DashboardsReconciler) Reconcile() (ctrl.Result, error) {
	if !r.instance.Spec.Dashboards.Enable {
		return ctrl.Result{}, nil
	}

	result := reconciler.CombinedResult{}

	cm := builders.NewDashboardsConfigMapForCR(r.instance, fmt.Sprintf("%s-dashboards-config", r.instance.Spec.General.ClusterName))
	result.Combine(r.ReconcileResource(cm, reconciler.StatePresent))

	deployment := builders.NewDashboardsDeploymentForCR(r.instance)
	result.Combine(r.ReconcileResource(deployment, reconciler.StatePresent))

	svc := builders.NewDashboardsSvcForCr(r.instance)
	result.Combine(r.ReconcileResource(svc, reconciler.StatePresent))

	return result.Result, result.Err
}
