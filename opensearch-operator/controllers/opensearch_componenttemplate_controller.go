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

// OpensearchComponentTemplateReconciler reconciles a OpensearchComponentTemplate object.
type OpensearchComponentTemplateReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

//+kubebuilder:rbac:groups=opensearch.org,resources=opensearchcomponenttemplates,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=opensearch.org,resources=opensearchcomponenttemplates/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=opensearch.org,resources=opensearchcomponenttemplates/finalizers,verbs=update
//+kubebuilder:rbac:groups=opensearch.opster.io,resources=opensearchcomponenttemplates,verbs=get;list;watch
//+kubebuilder:rbac:groups=opensearch.org,resources=opensearchclusters,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *OpensearchComponentTemplateReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("componenttemplate", req.NamespacedName)
	logger.Info("Reconciling OpensearchComponentTemplate")

	instance := &opensearchv1.OpensearchComponentTemplate{}
	err := r.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	componentTemplateReconciler := reconcilers.NewComponentTemplateReconciler(
		ctx,
		r.Client,
		r.Recorder,
		instance,
	)

	if instance.DeletionTimestamp.IsZero() {
		controllerutil.AddFinalizer(instance, OpensearchFinalizer)
		err = r.Update(ctx, instance)
		if err != nil {
			return ctrl.Result{}, err
		}
		return componentTemplateReconciler.Reconcile()
	} else {
		if controllerutil.ContainsFinalizer(instance, OpensearchFinalizer) {
			err = componentTemplateReconciler.Delete()
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
func (r *OpensearchComponentTemplateReconciler) SetupWithManager(mgr ctrl.Manager, maxConcurrentReconciles int) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&opensearchv1.OpensearchComponentTemplate{}).
		Owns(&opensearchv1.OpenSearchCluster{}). // Get notified when opensearch clusters change
		WithOptions(controller.Options{MaxConcurrentReconciles: maxConcurrentReconciles}).
		Complete(r)
}
