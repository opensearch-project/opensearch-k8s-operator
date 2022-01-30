package dashboard

import (
	"context"
	"fmt"

	sts "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	opsterv1 "opensearch.opster.io/api/v1"
	"opensearch.opster.io/pkg/builders"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type State struct {
	Component string `json:"compenent,omitempty"`
	Status    string `json:"status,omitempty"`
	Err       error  `json:"err,omitempty"`
}

type DashboardReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
	State    State
	Instance *opsterv1.OpenSearchCluster
}

//+kubebuilder:rbac:groups="opensearch.opster.io",resources=events,verbs=create;patch
//+kubebuilder:rbac:groups=opensearch.opster.io,resources=opensearchcluster,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=opensearch.opster.io,resources=opensearchcluster/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=opensearch.opster.io,resources=opensearchcluster/finalizers,verbs=update

func (r *DashboardReconciler) Reconcile(ctx context.Context, request ctrl.Request) (ctrl.Result, error) {
	/// ------ create opensearch dashboard cm ------- ///

	kibanaDeploy := sts.Deployment{}
	deployName := r.Instance.Spec.General.ClusterName + "-dashboards"
	deployNamespace := r.Instance.Spec.General.ClusterName
	if err := r.Get(context.TODO(), client.ObjectKey{Name: deployName, Namespace: deployNamespace}, &kibanaDeploy); err != nil {
		/// ------- create Opensearch-Dashboard deployment ------- ///
		dashboards_deployment := builders.NewDashboardsDeploymentForCR(r.Instance)

		err = r.Create(context.TODO(), dashboards_deployment)
		if err != nil {
			fmt.Println(err, "Cannot create Opensearch-Dashboard STS "+dashboards_deployment.Name)
			return ctrl.Result{}, err
		}
		fmt.Println("Opensearch-Dashboard STS Created successfully - ", "name : ", dashboards_deployment.Name)
	}

	kibanaCm := v1.ConfigMap{}
	cmName := "opensearch-dashboards"
	if err := r.Get(context.TODO(), client.ObjectKey{Name: cmName, Namespace: deployNamespace}, &kibanaCm); err != nil {
		/// ------- create Opensearch-Dashboard Configmap ------- ///
		dashboards_cm := builders.NewDashboardsConfigMapForCR(r.Instance)

		var err = r.Create(context.TODO(), dashboards_cm)
		if err != nil {
			fmt.Println(err, "Cannot create Opensearch-Dashboard Configmap "+dashboards_cm.Name)
			return ctrl.Result{}, err
		}
		fmt.Println("Opensearch-Dashboard Cm Created successfully", "name", dashboards_cm.Name)

	}

	kibanaService := v1.Service{}
	serviceName := r.Instance.Spec.General.ServiceName + "-dash-svc"

	if err := r.Get(context.TODO(), client.ObjectKey{Name: serviceName, Namespace: deployNamespace}, &kibanaService); err != nil {
		/// -------- create Opensearch-Dashboard service ------- ///
		dashboards_svc := builders.NewDashboardsSvcForCr(r.Instance)

		err = r.Create(context.TODO(), dashboards_svc)
		if err != nil {
			fmt.Println(err, "Cannot create Opensearch-Dashboard service "+dashboards_svc.Name)
			return ctrl.Result{}, err
		}
		fmt.Println("Opensearch-Dashboard service Created successfully", "name", dashboards_svc.Name)
	}

	return ctrl.Result{}, nil
}
