package controllers

import (
	"context"

	opensearchv1 "github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/api/opensearch.org/v1"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/reconcilers"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/reconcilers/k8s"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// OpensearchActionGroupReconciler reconciles a OpensearchActionGroup object.
type OpensearchActionGroupReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

//+kubebuilder:rbac:groups=opensearch.org,resources=opensearchactiongroups,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=opensearch.org,resources=opensearchactiongroups/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=opensearch.org,resources=opensearchactiongroups/finalizers,verbs=update
//+kubebuilder:rbac:groups=opensearch.opster.io,resources=opensearchactiongroups,verbs=get;list;watch
//+kubebuilder:rbac:groups=opensearch.org,resources=opensearchclusters,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *OpensearchActionGroupReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("actiongroup", req.NamespacedName)
	logger.Info("Reconciling OpensearchActionGroup")

	instance := &opensearchv1.OpensearchActionGroup{}
	err := r.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	actionGroupReconciler := reconcilers.NewActionGroupReconciler(
		k8s.NewK8sClient(r.Client, ctx),
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
		return actionGroupReconciler.Reconcile()
	} else {
		if controllerutil.ContainsFinalizer(instance, OpensearchFinalizer) {
			err = actionGroupReconciler.Delete()
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
func (r *OpensearchActionGroupReconciler) SetupWithManager(mgr ctrl.Manager, maxConcurrentReconciles int) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&opensearchv1.OpensearchActionGroup{}).
		Owns(&opensearchv1.OpenSearchCluster{}). // Get notified when opensearch clusters change
		WithOptions(controller.Options{MaxConcurrentReconciles: maxConcurrentReconciles}).
		Complete(r)
}
