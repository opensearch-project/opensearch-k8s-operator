package dashboard

import (
	"context"
	"fmt"
	sts "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	opsterv1 "os-operator.io/api/v1"
	"os-operator.io/pkg/builders"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	controllerName           = "kibana-controller"
	configHashAnnotationName = "kibana.k8s.elastic.co/config-hash"
)

type State struct {
	Compenent string `json:"compenent,omitempty"`
	Status    string `json:"status,omitempty"`
	Err       error  `json:"err,omitempty"`
}

type DashboardReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
	State    State
	Instnce  *opsterv1.Os
}

//+kubebuilder:rbac:groups="opster.os-operator.opster.io",resources=events,verbs=create;patch
//+kubebuilder:rbac:groups=opster.os-operator.opster.io,resources=os,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=opster.os-operator.opster.io,resources=os/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=opster.os-operator.opster.io,resources=os/finalizers,verbs=update

func (r *DashboardReconciler) InternalReconcile(ctx context.Context) (DashboardReconciler, ctrl.Result, error) {
	/// ------ create opensearch dashboard cm ------- ///

	dashboard_reconciler := DashboardReconciler{}

	kibanaDeploy := sts.Deployment{}
	deployName := r.Instnce.Spec.General.ClusterName + "-os-dash"
	deployNamespace := r.Instnce.Spec.General.ClusterName
	if err := r.Get(context.TODO(), client.ObjectKey{Name: deployName, Namespace: deployNamespace}, &kibanaDeploy); err != nil {
		/// ------- create Opensearch-Dashboard sts ------- ///
		os_dash := builders.New_OS_Dashboard_ForCR(r.Instnce)

		err = r.Create(context.TODO(), os_dash)
		if err != nil {
			fmt.Println(err, "Cannot create Opensearch-Dashboard STS "+os_dash.Name)
			dashboard_reconciler = setReconcilerStatus(&dashboard_reconciler, "Failed", err)
			return DashboardReconciler{}, ctrl.Result{}, err
		}
		fmt.Println("Opensearch-Dashboard STS Created successfully - ", "name : ", os_dash.Name)
	}

	kibanaCm := v1.ConfigMap{}
	cmName := "os-dash"
	if err := r.Get(context.TODO(), client.ObjectKey{Name: cmName, Namespace: deployNamespace}, &kibanaCm); err != nil {
		/// ------- create Opensearch-Dashboard Configmap ------- ///
		os_dash_cm := builders.NewCm_OS_Dashboard_ForCR(r.Instnce)

		var err = r.Create(context.TODO(), os_dash_cm)
		if err != nil {
			fmt.Println(err, "Cannot create Opensearch-Dashboard Configmap "+os_dash_cm.Name)
			dashboard_reconciler = setReconcilerStatus(&dashboard_reconciler, "Failed", err)
			return DashboardReconciler{}, ctrl.Result{}, err
		}
		fmt.Println("Opensearch-Dashboard Cm Created successfully", "name", os_dash_cm.Name)

	}

	kibanaService := v1.Service{}
	serviceName := r.Instnce.Spec.General.ServiceName + "-dash-svc"

	if err := r.Get(context.TODO(), client.ObjectKey{Name: serviceName, Namespace: deployNamespace}, &kibanaService); err != nil {
		/// -------- create Opensearch-Dashboard service ------- ///
		os_dash_service := builders.New_OS_Dashboard_SvcForCr(r.Instnce)

		err = r.Create(context.TODO(), os_dash_service)
		if err != nil {
			fmt.Println(err, "Cannot create Opensearch-Dashboard service "+os_dash_service.Name)
			dashboard_reconciler = setReconcilerStatus(&dashboard_reconciler, "Failed", err)
			return DashboardReconciler{}, ctrl.Result{}, err
		}
		fmt.Println("Opensearch-Dashboard service Created successfully", "name", os_dash_service.Name)
	}

	dashboard_reconciler = setReconcilerStatus(&dashboard_reconciler, "Done", nil)
	return dashboard_reconciler, ctrl.Result{}, nil
}

func setReconcilerStatus(dashboard *DashboardReconciler, stat string, err error) DashboardReconciler {

	new := DashboardReconciler{
		Client:   dashboard.Client,
		Scheme:   dashboard.Scheme,
		Recorder: dashboard.Recorder,
		State: State{
			Compenent: controllerName,
			Status:    stat,
		},
		Instnce: dashboard.Instnce,
	}
	return new
}
