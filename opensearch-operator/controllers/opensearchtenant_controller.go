package controllers

import (
	"context"

	opensearchv1 "github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/api/opensearch.org/v1"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/reconcilers"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// OpensearchTenantReconciler reconciles a OpensearchTenant object.
type OpensearchTenantReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

//+kubebuilder:rbac:groups=opensearch.org,resources=opensearchtenants,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=opensearch.org,resources=opensearchtenants/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=opensearch.org,resources=opensearchtenants/finalizers,verbs=update
//+kubebuilder:rbac:groups=opensearch.opster.io,resources=opensearchtenants,verbs=get;list;watch
//+kubebuilder:rbac:groups=opensearch.org,resources=opensearchclusters,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *OpensearchTenantReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("tenant", req.NamespacedName)
	logger.Info("Reconciling OpensearchTenant")

	instance := &opensearchv1.OpensearchTenant{}
	err := r.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	tenantReconciler := reconcilers.NewTenantReconciler(
		r.Client,
		ctx,
		r.Recorder,
		instance,
	)

	if instance.DeletionTimestamp.IsZero() {
		controllerutil.AddFinalizer(instance, OpensearchFinalizer)
		err = r.Update(ctx, instance)
		if err != nil {
			return ctrl.Result{}, err
		}
		return tenantReconciler.Reconcile()
	} else {
		if controllerutil.ContainsFinalizer(instance, OpensearchFinalizer) {
			err = tenantReconciler.Delete()
			if err != nil {
				return ctrl.Result{}, err
			}
			controllerutil.RemoveFinalizer(instance, OpensearchFinalizer)
			return ctrl.Result{}, r.Update(ctx, instance)
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *OpensearchTenantReconciler) SetupWithManager(mgr ctrl.Manager, maxConcurrentReconciles int) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&opensearchv1.OpensearchTenant{}).
		Owns(&opensearchv1.OpenSearchCluster{}). // Get notified when opensearch clusters change
		WithOptions(controller.Options{MaxConcurrentReconciles: maxConcurrentReconciles}).
		Complete(r)
}
