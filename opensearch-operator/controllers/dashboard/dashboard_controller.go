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
	Component string `json:"compenent,omitempty"`
	Status    string `json:"status,omitempty"`
	Err       error  `json:"err,omitempty"`
}

type DashboardReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
	State    State
	Instance *opsterv1.Os
}

//+kubebuilder:rbac:groups="opster.os-operator.opster.io",resources=events,verbs=create;patch
//+kubebuilder:rbac:groups=opster.os-operator.opster.io,resources=os,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=opster.os-operator.opster.io,resources=os/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=opster.os-operator.opster.io,resources=os/finalizers,verbs=update

func (r *DashboardReconciler) Reconcile(ctx context.Context, request ctrl.Request) (ctrl.Result, error) {
	/// ------ create opensearch dashboard cm ------- ///

	kibanaDeploy := sts.Deployment{}
	deployName := r.Instance.Spec.General.ClusterName + "-os-dash"
	deployNamespace := r.Instance.Spec.General.ClusterName
	if err := r.Get(context.TODO(), client.ObjectKey{Name: deployName, Namespace: deployNamespace}, &kibanaDeploy); err != nil {
		/// ------- create Opensearch-Dashboard sts ------- ///
		os_dash := builders.NewOsDashboardForCR(r.Instance)

		err = r.Create(context.TODO(), os_dash)
		if err != nil {
			fmt.Println(err, "Cannot create Opensearch-Dashboard STS "+os_dash.Name)
			return ctrl.Result{}, err
		}
		fmt.Println("Opensearch-Dashboard STS Created successfully - ", "name : ", os_dash.Name)
	}

	kibanaCm := v1.ConfigMap{}
	cmName := "os-dash"
	if err := r.Get(context.TODO(), client.ObjectKey{Name: cmName, Namespace: deployNamespace}, &kibanaCm); err != nil {
		/// ------- create Opensearch-Dashboard Configmap ------- ///
		os_dash_cm := builders.NewCmOsDashboardForCR(r.Instance)

		var err = r.Create(context.TODO(), os_dash_cm)
		if err != nil {
			fmt.Println(err, "Cannot create Opensearch-Dashboard Configmap "+os_dash_cm.Name)
			return ctrl.Result{}, err
		}
		fmt.Println("Opensearch-Dashboard Cm Created successfully", "name", os_dash_cm.Name)

	}

	kibanaService := v1.Service{}
	serviceName := r.Instance.Spec.General.ServiceName + "-dash-svc"

	if err := r.Get(context.TODO(), client.ObjectKey{Name: serviceName, Namespace: deployNamespace}, &kibanaService); err != nil {
		/// -------- create Opensearch-Dashboard service ------- ///
		os_dash_service := builders.NewOsDashboardSvcForCr(r.Instance)

		err = r.Create(context.TODO(), os_dash_service)
		if err != nil {
			fmt.Println(err, "Cannot create Opensearch-Dashboard service "+os_dash_service.Name)
			return ctrl.Result{}, err
		}
		fmt.Println("Opensearch-Dashboard service Created successfully", "name", os_dash_service.Name)
	}

	return ctrl.Result{}, nil
}
